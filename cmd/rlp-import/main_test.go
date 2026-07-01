// Copyright (C) 2019-2026, Lux Industries Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestDeriveContentHash_StableAndShort asserts the binary's content-hash
// computation matches the operator's TenantImportSpec.ContentHash:
// SHA256("tenant|alias|bid|url|sha")[:12]. If these ever drift the
// sentinel paths diverge and the import re-runs forever.
func TestDeriveContentHash_StableAndShort(t *testing.T) {
	c := &config{
		tenant:       "hanzo",
		chainAlias:   "C",
		blockchainID: "BCID",
		sourceURL:    "https://s3.lux.network/h.rlp",
		sha256Hex:    "deadbeef",
	}
	got := deriveContentHash(c)
	if len(got) != 12 {
		t.Fatalf("hash length = %d, want 12", len(got))
	}
	// Compute it the way the operator does, byte-for-byte.
	h := sha256.Sum256([]byte("hanzo|C|BCID|https://s3.lux.network/h.rlp|deadbeef"))
	want := hex.EncodeToString(h[:])[:12]
	if got != want {
		t.Fatalf("hash mismatch: got=%s want=%s", got, want)
	}
}

// TestParseFlags_RequiredFields covers the bad-input exit path: missing
// any of tenant/alias/url/blockchain-id must error out without touching
// the filesystem.
func TestParseFlags_RequiredFields(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"missing tenant", []string{"--chain-alias=C", "--source-url=x", "--blockchain-id=y"}, "tenant"},
		{"missing alias", []string{"--tenant=t", "--source-url=x", "--blockchain-id=y"}, "chain-alias"},
		{"missing url", []string{"--tenant=t", "--chain-alias=C", "--blockchain-id=y"}, "source-url"},
		{"missing bid", []string{"--tenant=t", "--chain-alias=C", "--source-url=x"}, "blockchain-id"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseFlags(tc.args)
			if err == nil {
				t.Fatalf("expected error containing %q", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %v, want substring %q", err, tc.want)
			}
		})
	}
}

// TestParseFlags_RejectsBadContentHash catches typos in operator-provided
// hashes that would otherwise drift from the sentinel path.
func TestParseFlags_RejectsBadContentHash(t *testing.T) {
	_, err := parseFlags([]string{
		"--tenant=t", "--chain-alias=C", "--source-url=x",
		"--blockchain-id=y", "--content-hash=tooshort",
	})
	if err == nil || !strings.Contains(err.Error(), "content-hash") {
		t.Fatalf("err = %v, want content-hash validation error", err)
	}
}

// TestRun_SentinelShortCircuits is the idempotency proof: pre-create the
// sentinel and run() returns OK without touching luxd at all.
func TestRun_SentinelShortCircuits(t *testing.T) {
	dir := t.TempDir()
	c := &config{
		tenant:       "hanzo",
		chainAlias:   "C",
		blockchainID: "BCID",
		sourceURL:    "http://invalid.example/", // would fail if reached
		dataDir:      dir,
		luxdRPC:      "http://127.0.0.1:0", // unreachable
	}
	c.contentHash = deriveContentHash(c)
	sentinel := filepath.Join(dir, ".tenant-import-"+c.tenant+"-"+c.contentHash)
	if err := os.WriteFile(sentinel, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	code, err := run(c)
	if code != exitOK {
		t.Errorf("exit code = %d, want %d (sentinel skip)", code, exitOK)
	}
	if err != nil {
		t.Errorf("unexpected err: %v", err)
	}
}

// TestRun_HappyPath drives the full six-step lifecycle against an httptest
// server that serves a fake RLP and accepts the admin.importChain POST.
func TestRun_HappyPath(t *testing.T) {
	body := []byte("fake-rlp-bytes")
	bodySum := sha256.Sum256(body)
	expectedSHA := hex.EncodeToString(bodySum[:])

	var importCalled bool
	var importBodyRaw []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/rlp":
			w.Header().Set("Content-Type", "application/octet-stream")
			_, _ = w.Write(body)
		case r.URL.Path == "/v1/health":
			w.WriteHeader(http.StatusOK)
		case strings.HasPrefix(r.URL.Path, "/v1/bc/"):
			importCalled = true
			importBodyRaw, _ = io.ReadAll(r.Body)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","result":null,"id":1}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	dir := t.TempDir()
	// Pre-create the chainData dir so we can assert it gets wiped.
	chainData := filepath.Join(dir, "chainData", "BCID")
	if err := os.MkdirAll(chainData, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(chainData, "stale"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	c := &config{
		tenant:       "hanzo",
		chainAlias:   "C",
		blockchainID: "BCID",
		sourceURL:    srv.URL + "/rlp",
		sha256Hex:    expectedSHA,
		dataDir:      dir,
		luxdRPC:      srv.URL,
	}
	c.contentHash = deriveContentHash(c)

	code, err := run(c)
	if code != exitOK {
		t.Fatalf("exit code = %d (err=%v), want %d", code, err, exitOK)
	}
	if !importCalled {
		t.Error("admin.importChain was not called")
	}
	var req jsonRPCRequest
	if err := json.Unmarshal(importBodyRaw, &req); err != nil {
		t.Fatalf("decode import req: %v", err)
	}
	if req.Method != "admin.importChain" {
		t.Errorf("method = %q, want admin.importChain", req.Method)
	}
	if len(req.Params) != 1 {
		t.Fatalf("params len = %d, want 1", len(req.Params))
	}
	rlpPath, _ := req.Params[0].(string)
	if !strings.HasSuffix(rlpPath, ".rlp") {
		t.Errorf("rlp path = %q, want *.rlp suffix", rlpPath)
	}
	// chainData wiped?
	if _, statErr := os.Stat(filepath.Join(chainData, "stale")); !os.IsNotExist(statErr) {
		t.Errorf("chainData not wiped: err=%v", statErr)
	}
	// Sentinel touched?
	sentinel := filepath.Join(dir, ".tenant-import-"+c.tenant+"-"+c.contentHash)
	if _, statErr := os.Stat(sentinel); statErr != nil {
		t.Errorf("sentinel not touched: %v", statErr)
	}
}

// TestRun_SHAMismatchExits1 covers the security gate: a tampered RLP must
// short-circuit before luxd ever sees it.
func TestRun_SHAMismatchExits1(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/rlp":
			_, _ = w.Write([]byte("real-rlp"))
		case "/v1/health":
			w.WriteHeader(http.StatusOK)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	dir := t.TempDir()
	c := &config{
		tenant:       "hanzo",
		chainAlias:   "C",
		blockchainID: "BCID",
		sourceURL:    srv.URL + "/rlp",
		sha256Hex:    "ff00", // anything but real SHA
		dataDir:      dir,
		luxdRPC:      srv.URL,
	}
	c.contentHash = deriveContentHash(c)

	code, err := run(c)
	if code != exitBadInput {
		t.Errorf("exit code = %d, want %d (sha mismatch)", code, exitBadInput)
	}
	if err == nil || !strings.Contains(err.Error(), "sha256") {
		t.Errorf("err = %v, want sha256 mismatch error", err)
	}
}

// TestRun_LuxdUnreachableExits2 covers the transient failure path.
func TestRun_LuxdUnreachableExits2(t *testing.T) {
	// Source server is up so fetch + sha succeed; luxd is fake at a
	// blackholed port so the health probe times out.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/rlp":
			_, _ = w.Write([]byte("x"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	dir := t.TempDir()
	c := &config{
		tenant:       "hanzo",
		chainAlias:   "C",
		blockchainID: "BCID",
		sourceURL:    srv.URL + "/rlp",
		dataDir:      dir,
		luxdRPC:      "http://127.0.0.1:1", // closed port
	}
	c.contentHash = deriveContentHash(c)

	// Shrink the wait window so the test doesn't take 3min.
	t.Setenv("RLP_IMPORT_TEST", "1")
	code, _ := runWithLuxdTimeout(c, 1) // 1s
	if code != exitLuxdUnreach {
		t.Errorf("exit code = %d, want %d (luxd unreachable)", code, exitLuxdUnreach)
	}
}

// TestRun_AdminRPCErrorExits3 covers a luxd-side RPC failure.
func TestRun_AdminRPCErrorExits3(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/rlp":
			_, _ = w.Write([]byte("x"))
		case r.URL.Path == "/v1/health":
			w.WriteHeader(http.StatusOK)
		case strings.HasPrefix(r.URL.Path, "/v1/bc/"):
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","error":{"code":-32601,"message":"admin API not enabled"},"id":1}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	dir := t.TempDir()
	c := &config{
		tenant:       "hanzo",
		chainAlias:   "C",
		blockchainID: "BCID",
		sourceURL:    srv.URL + "/rlp",
		dataDir:      dir,
		luxdRPC:      srv.URL,
	}
	c.contentHash = deriveContentHash(c)

	code, err := run(c)
	if code != exitAdminRPCError {
		t.Errorf("exit code = %d, want %d", code, exitAdminRPCError)
	}
	if err == nil || !strings.Contains(err.Error(), "admin API not enabled") {
		t.Errorf("err = %v, want admin API not enabled", err)
	}
	// Sentinel must NOT be touched on RPC error.
	sentinel := filepath.Join(dir, ".tenant-import-"+c.tenant+"-"+c.contentHash)
	if _, statErr := os.Stat(sentinel); statErr == nil {
		t.Error("sentinel was touched despite RPC error — re-fire on next pod would be broken")
	}
}

// TestRun_404FetchExits1 covers the URL-404 failure mode in the design doc.
func TestRun_404FetchExits1(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()

	dir := t.TempDir()
	c := &config{
		tenant:       "hanzo",
		chainAlias:   "C",
		blockchainID: "BCID",
		sourceURL:    srv.URL + "/missing.rlp",
		dataDir:      dir,
		luxdRPC:      srv.URL,
	}
	c.contentHash = deriveContentHash(c)
	code, err := run(c)
	if code != exitBadInput {
		t.Errorf("exit code = %d, want %d (404)", code, exitBadInput)
	}
	if err == nil || !strings.Contains(err.Error(), "fetch") {
		t.Errorf("err = %v, want fetch error", err)
	}
}

// runWithLuxdTimeout is a test-only helper that swaps the long luxd
// health-probe timeout for a short one so unreachable-luxd tests run in
// seconds, not minutes. Kept here (not in main.go) because production
// always uses the 3min default.
func runWithLuxdTimeout(c *config, seconds int) (int, error) {
	sentinel := filepath.Join(c.dataDir, ".tenant-import-"+c.tenant+"-"+c.contentHash)
	rlpDest := filepath.Join(c.dataDir, c.tenant+"-"+c.contentHash+".rlp")
	if _, err := os.Stat(sentinel); err == nil {
		return exitOK, nil
	}
	if err := fetch(c.sourceURL, rlpDest); err != nil {
		return exitBadInput, err
	}
	if err := waitForLuxd(c.luxdRPC, time.Duration(seconds)*time.Second); err != nil {
		return exitLuxdUnreach, err
	}
	return exitOK, nil
}
