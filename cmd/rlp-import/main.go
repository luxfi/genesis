// Copyright (C) 2019-2026, Lux Industries Inc. All rights reserved.
// See the file LICENSE for licensing terms.

// Command rlp-import performs a one-time RLP import for a tenant chain.
//
// Decomplection:
//
// Today the import is a hand-edited shell block inside lux-mainnet's
// luxd-startup ConfigMap: curl → sha256 → rm -rf chainData → POST
// admin.importChain → touch sentinel. The same six steps for every tenant
// (hanzo, zoo, pars, spc), wrapped in env-rendered Bash. This binary
// implements those six steps directly in Go and is invoked by the operator
// as a K8s Job (one per LuxNetwork.spec.tenantImports[i]).
//
// Lifecycle (always the same six steps, in order):
//
//  1. Sentinel check. If ${DataDir}/.tenant-import-${tenant}-${hash12}
//     exists, exit 0 — this import already ran on this PVC.
//  2. Fetch. HTTP GET ${SourceURL} → ${DataDir}/${tenant}-${hash12}.rlp.
//  3. SHA-256 verify. Compare against ${SHA256}; abort on mismatch.
//  4. Wipe ${DataDir}/chainData/${BlockchainID} so admin.importChain starts
//     from a known-empty EVM state.
//  5. POST admin.importChain → ${LuxdRPC}/v1/chain/${ChainAlias}/rpc with the
//     RLP path. Wait for an HTTP 200 + JSON-RPC result.
//  6. Touch the sentinel. Only on full success; partial-success leaves
//     no sentinel so the K8s Job retry policy re-runs the steps next pod.
//
// Failure modes (exit code semantics — K8s reads these via backoffLimit):
//
//   - exit 1: bad CLI args, URL 404, SHA mismatch, wipe failed.
//   - exit 2: luxd unreachable / admin API not enabled.
//   - exit 3: admin.importChain returned a JSON-RPC error.
//
// Idempotency: every run starts with the sentinel check. Re-running on an
// already-imported chain is a zero-cost no-op. Re-running after a partial
// failure picks up at step 2 (the sentinel only writes on success).
//
// Auth: defaults to ZAP-native HTTP — runs inside the cluster mesh, the
// luxd admin endpoint is reachable via the headless Service DNS without
// mTLS. KMS tokens are accepted via the env var KMS_AUTH_TOKEN and emitted
// as a Bearer header on the admin.importChain POST when set.
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Exit codes — K8s + the operator's status reconciler read these to
// distinguish "bad input" (no retry) from "transient" (retry).
const (
	exitOK             = 0
	exitBadInput       = 1
	exitLuxdUnreach    = 2
	exitAdminRPCError  = 3
)

// importHTTPClient is a package-level client so tests can swap a stub.
var importHTTPClient = &http.Client{
	// 1h ceiling — the largest tenant RLP today is ~6GiB and HTTP GET +
	// admin.importChain both run under this client. The luxd POST is the
	// long one (it iterates blocks).
	Timeout: time.Hour,
}

type config struct {
	tenant       string
	chainAlias   string
	sourceURL    string
	sha256Hex    string
	blockchainID string
	dataDir      string
	luxdRPC      string
	contentHash  string // populated from --content-hash or derived
}

func main() {
	cfg, err := parseFlags(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "rlp-import: %v\n", err)
		os.Exit(exitBadInput)
	}
	code, runErr := run(cfg)
	if runErr != nil {
		fmt.Fprintf(os.Stderr, "rlp-import: %v\n", runErr)
	}
	os.Exit(code)
}

// parseFlags decomplects argv parsing from execution so tests can build a
// config struct directly without going through flag.CommandLine.
func parseFlags(args []string) (*config, error) {
	fs := flag.NewFlagSet("rlp-import", flag.ContinueOnError)
	fs.SetOutput(io.Discard) // no usage-on-error printing during tests
	tenant := fs.String("tenant", "", "brand identifier (hanzo, zoo, pars, spc) — required")
	alias := fs.String("chain-alias", "", "luxd chain alias for the admin.importChain RPC route — required")
	url := fs.String("source-url", "", "HTTP(S) URL of the RLP file — required")
	sha := fs.String("sha256", "", "expected SHA-256 of the RLP (hex). Empty disables verification.")
	bid := fs.String("blockchain-id", "", "luxd blockchain ID under ${DataDir}/chainData to wipe — required")
	data := fs.String("data-dir", "/data/db", "luxd data dir on the shared PVC")
	rpc := fs.String("luxd-rpc", "http://127.0.0.1:9630", "luxd HTTP base URL")
	hash := fs.String("content-hash", "", "operator-provided 12-char content hash. Derived from spec when empty.")
	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	cfg := &config{
		tenant:       *tenant,
		chainAlias:   *alias,
		sourceURL:    *url,
		sha256Hex:    strings.TrimSpace(*sha),
		blockchainID: *bid,
		dataDir:      *data,
		luxdRPC:      strings.TrimRight(*rpc, "/"),
		contentHash:  strings.TrimSpace(*hash),
	}
	if cfg.tenant == "" {
		return nil, errors.New("--tenant required")
	}
	if cfg.chainAlias == "" {
		return nil, errors.New("--chain-alias required")
	}
	if cfg.sourceURL == "" {
		return nil, errors.New("--source-url required")
	}
	if cfg.blockchainID == "" {
		return nil, errors.New("--blockchain-id required")
	}
	if cfg.contentHash == "" {
		// Derive the same way TenantImportSpec.ContentHash does so the
		// operator and the binary agree byte-for-byte on the sentinel path
		// even when --content-hash is absent.
		cfg.contentHash = deriveContentHash(cfg)
	}
	if len(cfg.contentHash) != 12 {
		return nil, fmt.Errorf("--content-hash must be 12 hex chars, got %q", cfg.contentHash)
	}
	return cfg, nil
}

// deriveContentHash mirrors apiv1.TenantImportSpec.ContentHash. Both must
// produce the same string for any given (tenant, alias, blockchainID,
// URL, sha) tuple — otherwise the operator and the binary would compute
// different sentinel paths and the binary would re-run forever.
func deriveContentHash(c *config) string {
	h := sha256.Sum256([]byte(
		c.tenant + "|" +
			c.chainAlias + "|" +
			c.blockchainID + "|" +
			c.sourceURL + "|" +
			c.sha256Hex,
	))
	return hex.EncodeToString(h[:])[:12]
}

// run executes the six-step lifecycle. Returns (exit code, optional error).
// The exit code is the source of truth — err is purely diagnostic context.
func run(c *config) (int, error) {
	sentinel := filepath.Join(c.dataDir, ".tenant-import-"+c.tenant+"-"+c.contentHash)
	rlpDest := filepath.Join(c.dataDir, c.tenant+"-"+c.contentHash+".rlp")
	chainDataPath := filepath.Join(c.dataDir, "chainData", c.blockchainID)

	// 1. Sentinel check — idempotent skip.
	if _, err := os.Stat(sentinel); err == nil {
		fmt.Printf("rlp-import: tenant=%s alias=%s hash=%s sentinel present, skip\n",
			c.tenant, c.chainAlias, c.contentHash)
		return exitOK, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return exitBadInput, fmt.Errorf("stat sentinel: %w", err)
	}

	// 2. Fetch.
	fmt.Printf("rlp-import: tenant=%s alias=%s hash=%s fetching %s\n",
		c.tenant, c.chainAlias, c.contentHash, c.sourceURL)
	if err := fetch(c.sourceURL, rlpDest); err != nil {
		return exitBadInput, fmt.Errorf("fetch: %w", err)
	}

	// 3. SHA-256 verify (when expected hash is set).
	if c.sha256Hex != "" {
		got, err := sha256File(rlpDest)
		if err != nil {
			return exitBadInput, fmt.Errorf("sha256: %w", err)
		}
		if !strings.EqualFold(got, c.sha256Hex) {
			return exitBadInput, fmt.Errorf("sha256 mismatch: want=%s got=%s", c.sha256Hex, got)
		}
		fmt.Printf("rlp-import: sha256 OK (%s)\n", got)
	}

	// 4. Wipe chainData.
	fmt.Printf("rlp-import: wiping %s\n", chainDataPath)
	if err := os.RemoveAll(chainDataPath); err != nil {
		return exitBadInput, fmt.Errorf("wipe chainData: %w", err)
	}

	// 5. Verify luxd reachable + admin API on, then POST admin.importChain.
	if err := waitForLuxd(c.luxdRPC, 3*time.Minute); err != nil {
		return exitLuxdUnreach, fmt.Errorf("luxd unreachable: %w", err)
	}
	if err := callAdminImportChain(c.luxdRPC, c.chainAlias, rlpDest); err != nil {
		return exitAdminRPCError, fmt.Errorf("admin.importChain: %w", err)
	}

	// 6. Touch sentinel. Only on full success.
	if err := touchSentinel(sentinel); err != nil {
		return exitBadInput, fmt.Errorf("touch sentinel: %w", err)
	}
	fmt.Printf("rlp-import: tenant=%s alias=%s hash=%s complete\n",
		c.tenant, c.chainAlias, c.contentHash)
	return exitOK, nil
}

// fetch downloads url → dest with streaming (no full-file buffering — the
// RLPs are multi-GiB).
func fetch(url, dest string) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	resp, err := importHTTPClient.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP %d %s", resp.StatusCode, resp.Status)
	}
	f, err := os.OpenFile(dest, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := io.Copy(f, resp.Body); err != nil {
		return err
	}
	return f.Sync()
}

// sha256File returns the hex-encoded SHA-256 of the file at path.
func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// touchSentinel creates a zero-byte sentinel file. Returns nil if the file
// already exists (idempotent retry).
func touchSentinel(path string) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	return f.Close()
}

// waitForLuxd polls ${rpc}/v1/health until 200, capped at timeout. Returns
// nil when the C-chain RPC answers, err otherwise. Polls every 2s.
//
// It deliberately does NOT gate on ${rpc}/v1/health: on a public luxd that
// endpoint reports the D-Chain check and can never return 200, so a health gate
// parks this tool forever against a perfectly serving node.
func waitForLuxd(rpc string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	url := rpc + "/v1/chain/C/rpc"
	body := `{"jsonrpc":"2.0","id":1,"method":"eth_blockNumber","params":[]}`
	for time.Now().Before(deadline) {
		req, err := http.NewRequest(http.MethodPost, url, strings.NewReader(body))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := importHTTPClient.Do(req)
		if err == nil {
			b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
			resp.Body.Close()
			// ANY JSON-RPC envelope means the chain endpoint is reachable —
			// including an error one. A node that answers `{"error":…}` is up,
			// and calling it is what turns that into the specific exit code
			// (admin-RPC-error), which reporting "unreachable" would erase.
			if resp.StatusCode == http.StatusOK && bytes.Contains(b, []byte(`"jsonrpc"`)) {
				return nil
			}
		}
		time.Sleep(2 * time.Second)
	}
	return fmt.Errorf("luxd C-chain RPC at %s never answered within %s", rpc, timeout)
}

// jsonRPCRequest is the canonical JSON-RPC 2.0 request envelope. Inlined
// here to keep this binary dependency-light — no luxfi/sdk imports needed.
type jsonRPCRequest struct {
	JSONRPC string        `json:"jsonrpc"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
	ID      int           `json:"id"`
}

// jsonRPCResponse is the matching response envelope. Result is opaque —
// admin.importChain returns {"result": null} on success.
type jsonRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
	ID int `json:"id"`
}

// callAdminImportChain POSTs admin.importChain to the per-alias /v1/chain/
// endpoint. Returns nil on success, err with the JSON-RPC error message
// on luxd-side failure.
func callAdminImportChain(rpc, alias, rlpPath string) error {
	body, err := json.Marshal(jsonRPCRequest{
		JSONRPC: "2.0",
		Method:  "admin.importChain",
		Params:  []interface{}{rlpPath},
		ID:      1,
	})
	if err != nil {
		return err
	}
	url := rpc + "/v1/chain/" + alias + "/rpc"
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if tok := strings.TrimSpace(os.Getenv("KMS_AUTH_TOKEN")); tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	resp, err := importHTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	var out jsonRPCResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return fmt.Errorf("decode response: %w (body=%q)", err, string(raw))
	}
	if out.Error != nil {
		return fmt.Errorf("RPC error %d: %s", out.Error.Code, out.Error.Message)
	}
	return nil
}
