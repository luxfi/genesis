// Copyright (C) 2019-2026, Lux Industries Inc. All rights reserved.
// See the file LICENSE for licensing terms.

// Command rlp-export streams a luxd chain segment to an RLP file via the
// admin_exportChain JSON-RPC endpoint, symmetric with cmd/rlp-import.
//
// Decomplection:
//
// admin_exportChain (server-side, ~/work/lux/evm/plugin/evm/admin_api.go)
// is the single source of truth — it owns the block iterator, the gzip
// optional pipe, and the sentinel write. This binary is a thin client that
// builds the JSON-RPC envelope, POSTs it, and translates the response into
// an exit code the operator-side Job controller can read.
//
// Lifecycle:
//
//  1. Idempotency precheck. Read the sentinel locally (mounted via the
//     same PVC as luxd) and if it shows Status=done + matching {first,last,
//     firstHash,lastHash} for THIS run's range, exit 0 without calling luxd.
//     This is the "operator schedules the same export every hour" optimization.
//  2. Health probe. Confirm luxd is reachable on ${LuxdRPC}/ext/health.
//  3. RPC call. POST admin_exportChain to ${LuxdRPC}/ext/bc/${ChainAlias}/rpc
//     with {file, first, last}. Respect KMS_AUTH_TOKEN bearer auth same as
//     rlp-import.
//  4. Translate the response status into an exit code:
//     Status="ok"          → exit 0
//     Status="noop"        → exit 0  (server-side idempotency hit)
//     Status="interrupted" → exit 4  (operator should re-run with the
//     sentinel's HighestExported as --from-height)
//     any error            → exit 3
//
// Failure modes (exit codes — K8s reads these via backoffLimit):
//
//   - exit 1: bad CLI args, missing required flag.
//   - exit 2: luxd unreachable / admin API not enabled.
//   - exit 3: admin_exportChain returned a JSON-RPC error.
//   - exit 4: admin_exportChain returned Status=interrupted (re-run needed).
//
// Idempotency: if the same (chain, fromHeight, toHeight) was previously
// completed (sentinel.status=="done"), the local precheck short-circuits
// before luxd is even contacted. If the precheck is skipped (e.g. CLI
// runs from a different host than luxd), the SERVER does the same check
// against its own sentinel and returns Status="noop".
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

const (
	exitOK            = 0
	exitBadInput      = 1
	exitLuxdUnreach   = 2
	exitAdminRPCError = 3
	exitInterrupted   = 4
)

// exportHTTPClient is package-level so tests can substitute a stub. The
// timeout caps a single export — for a 1M-block C-Chain (~25min on cold
// pebbledb), 6h is conservative.
var exportHTTPClient = &http.Client{Timeout: 6 * time.Hour}

type config struct {
	chainAlias string
	outputFile string
	fromHeight uint64
	toHeight   string // "latest" or a uint64
	luxdRPC    string
}

func main() {
	cfg, err := parseFlags(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "rlp-export: %v\n", err)
		os.Exit(exitBadInput)
	}

	// Ctrl-C → cancel the in-flight POST. The server-side admin_exportChain
	// checks ctx.Done() at every block boundary so this is honored quickly.
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	code, runErr := run(ctx, cfg)
	if runErr != nil {
		fmt.Fprintf(os.Stderr, "rlp-export: %v\n", runErr)
	}
	os.Exit(code)
}

// parseFlags decomplects argv parsing from execution so tests can build a
// config struct directly without going through flag.CommandLine.
func parseFlags(args []string) (*config, error) {
	fs := flag.NewFlagSet("rlp-export", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	alias := fs.String("chain", "", "luxd chain alias (e.g. C, X, hanzo) — required")
	out := fs.String("output-file", "", "absolute path on the luxd PVC to write the RLP into — required")
	from := fs.Uint64("from-height", 0, "first block height to export (inclusive). Default 0.")
	to := fs.String("to-height", "latest", `last block height to export (inclusive). "latest" → current head.`)
	rpc := fs.String("luxd-rpc", "http://127.0.0.1:9630", "luxd HTTP base URL")
	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	cfg := &config{
		chainAlias: strings.TrimSpace(*alias),
		outputFile: strings.TrimSpace(*out),
		fromHeight: *from,
		toHeight:   strings.TrimSpace(*to),
		luxdRPC:    strings.TrimRight(*rpc, "/"),
	}
	if cfg.chainAlias == "" {
		return nil, errors.New("--chain required")
	}
	if cfg.outputFile == "" {
		return nil, errors.New("--output-file required")
	}
	if cfg.toHeight == "" {
		return nil, errors.New("--to-height required (use 'latest' for current head)")
	}
	return cfg, nil
}

// run performs the four-step lifecycle. Returns (exit code, optional error).
// The exit code is the contract — err is diagnostic context.
func run(ctx context.Context, c *config) (int, error) {
	// 1. Idempotency precheck. The sentinel lives at <output-file>.sentinel
	// per the server-side contract. If we can read it and it shows a
	// matching "done" record for this exact request, skip the RPC.
	if code, ok := localIdempotencyShortCircuit(c); ok {
		return code, nil
	}

	// 2. Health probe.
	if err := waitForLuxd(ctx, c.luxdRPC, 30*time.Second); err != nil {
		return exitLuxdUnreach, fmt.Errorf("luxd unreachable: %w", err)
	}

	// 3. POST admin_exportChain. Use math.MaxUint64-style sentinel for
	// "latest" — the server clamps to the current head.
	first := c.fromHeight
	last, err := resolveToHeight(c.toHeight)
	if err != nil {
		return exitBadInput, err
	}

	result, err := callAdminExportChain(ctx, c.luxdRPC, c.chainAlias, c.outputFile, first, last)
	if err != nil {
		// Distinguish ctx.Done() from network/RPC failures.
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			fmt.Fprintf(os.Stderr, "rlp-export: canceled mid-flight: %v\n", err)
			return exitInterrupted, err
		}
		return exitAdminRPCError, fmt.Errorf("admin_exportChain: %w", err)
	}

	// 4. Status → exit code.
	switch result.Status {
	case "ok":
		fmt.Printf("rlp-export: ok chain=%s file=%s blocks=%d first=%d last=%d firstHash=%s lastHash=%s\n",
			c.chainAlias, c.outputFile, result.BlocksExported, result.First, result.Last,
			result.FirstHash, result.LastHash)
		return exitOK, nil
	case "noop":
		fmt.Printf("rlp-export: noop chain=%s file=%s first=%d last=%d (server-side idempotency hit)\n",
			c.chainAlias, c.outputFile, result.First, result.Last)
		return exitOK, nil
	case "interrupted":
		fmt.Fprintf(os.Stderr, "rlp-export: interrupted chain=%s file=%s highest=%d (re-run with --from-height=%d)\n",
			c.chainAlias, c.outputFile, result.HighestExported, result.HighestExported+1)
		return exitInterrupted, nil
	default:
		return exitAdminRPCError, fmt.Errorf("unknown status %q", result.Status)
	}
}

// localIdempotencyShortCircuit reads the sentinel file (when the CLI and
// luxd share the same PVC) and returns (exitOK, true) if it shows a
// completed export matching this run's range. Returns (0, false) for any
// other case so the caller falls through to the RPC path.
//
// This is a soft check — the SERVER is also idempotent. When the CLI runs
// on a different host (no PVC access), this just always falls through.
func localIdempotencyShortCircuit(c *config) (int, bool) {
	sentinel := c.outputFile + ".sentinel"
	data, err := os.ReadFile(sentinel)
	if err != nil {
		return 0, false
	}
	var s struct {
		Status          string `json:"status"`
		First           uint64 `json:"first"`
		Last            uint64 `json:"last"`
		HighestExported uint64 `json:"highestExported"`
	}
	if err := json.Unmarshal(data, &s); err != nil {
		return 0, false
	}
	if s.Status != "done" {
		return 0, false
	}
	if s.First != c.fromHeight {
		return 0, false
	}
	// For --to-height=latest we can't easily know the target last without
	// asking luxd. Fall through to the RPC, which will be a server-side
	// noop if the heights still match.
	if c.toHeight == "latest" {
		return 0, false
	}
	wantLast, err := resolveToHeight(c.toHeight)
	if err != nil {
		return 0, false
	}
	if s.Last != wantLast {
		return 0, false
	}
	if _, err := os.Stat(c.outputFile); err != nil {
		return 0, false
	}
	fmt.Printf("rlp-export: noop (local sentinel matches) chain=%s file=%s first=%d last=%d\n",
		c.chainAlias, c.outputFile, s.First, s.Last)
	return exitOK, true
}

// resolveToHeight parses --to-height. "latest" maps to math.MaxUint64 — the
// server clamps to the current head, so this is the canonical way to say
// "everything you have right now".
func resolveToHeight(s string) (uint64, error) {
	if s == "latest" {
		// Maximum uint64; the server clamps to current head.
		return ^uint64(0), nil
	}
	var v uint64
	if _, err := fmt.Sscanf(s, "%d", &v); err != nil {
		return 0, fmt.Errorf("invalid --to-height %q: %w", s, err)
	}
	return v, nil
}

// waitForLuxd polls ${rpc}/ext/health until 200, capped at timeout.
func waitForLuxd(ctx context.Context, rpc string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	url := rpc + "/ext/health"
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return err
		}
		resp, err := exportHTTPClient.Do(req)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
		time.Sleep(2 * time.Second)
	}
	return fmt.Errorf("luxd at %s never became healthy within %s", rpc, timeout)
}

// jsonRPCRequest and jsonRPCResponse are inlined to keep this binary
// dependency-light (no luxfi/sdk import — matches rlp-import's choice).
type jsonRPCRequest struct {
	JSONRPC string        `json:"jsonrpc"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
	ID      int           `json:"id"`
}

type jsonRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type jsonRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *jsonRPCError   `json:"error,omitempty"`
	ID      int             `json:"id"`
}

// exportResult mirrors the server-side ExportChainResult so we can decode
// without importing the evm plugin (which would pull in geth/coreth and
// blow up the binary size for what is fundamentally an RPC client).
type exportResult struct {
	Success         bool   `json:"success"`
	Status          string `json:"status"`
	BlocksExported  uint64 `json:"blocksExported"`
	First           uint64 `json:"first"`
	Last            uint64 `json:"last"`
	HighestExported uint64 `json:"highestExported"`
	FirstHash       string `json:"firstHash"`
	LastHash        string `json:"lastHash"`
	SentinelPath    string `json:"sentinelPath"`
	Message         string `json:"message,omitempty"`
}

// callAdminExportChain POSTs admin_exportChain to the per-alias /ext/bc/
// endpoint. Returns the parsed result on success, err on luxd-side failure.
func callAdminExportChain(ctx context.Context, rpc, alias, file string, first, last uint64) (*exportResult, error) {
	body, err := json.Marshal(jsonRPCRequest{
		JSONRPC: "2.0",
		// Geth/coreth admin namespace uses underscore notation:
		// admin_exportChain (not admin.exportChain).
		Method: "admin_exportChain",
		Params: []interface{}{file, first, last},
		ID:     1,
	})
	if err != nil {
		return nil, err
	}
	url := rpc + "/ext/bc/" + alias + "/rpc"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if tok := strings.TrimSpace(os.Getenv("KMS_AUTH_TOKEN")); tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	resp, err := exportHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	var out jsonRPCResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("decode response: %w (body=%q)", err, string(raw))
	}
	if out.Error != nil {
		return nil, fmt.Errorf("RPC error %d: %s", out.Error.Code, out.Error.Message)
	}
	var result exportResult
	if err := json.Unmarshal(out.Result, &result); err != nil {
		return nil, fmt.Errorf("decode result: %w (raw=%q)", err, string(out.Result))
	}
	return &result, nil
}
