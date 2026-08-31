// Copyright (C) 2019-2026, Lux Industries Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package main

import (
	"context"
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

// TestParseFlags_RequiredFields covers the bad-input exit path: missing
// any of chain/output-file must error out without contacting luxd.
func TestParseFlags_RequiredFields(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"missing chain", []string{"--output-file=/data/exports/x.rlp"}, "chain"},
		{"missing output-file", []string{"--chain=C"}, "output-file"},
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

// TestParseFlags_Defaults pins the defaults so a future refactor that
// silently changes fromHeight=0 → fromHeight=1 (off-by-one bug magnet)
// shows up as a test failure.
func TestParseFlags_Defaults(t *testing.T) {
	cfg, err := parseFlags([]string{"--chain=C", "--output-file=/x.rlp"})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.fromHeight != 0 {
		t.Errorf("default fromHeight = %d, want 0", cfg.fromHeight)
	}
	if cfg.toHeight != "latest" {
		t.Errorf("default toHeight = %q, want %q", cfg.toHeight, "latest")
	}
	if cfg.luxdRPC != "http://127.0.0.1:9630" {
		t.Errorf("default luxdRPC = %q", cfg.luxdRPC)
	}
}

// TestResolveToHeight_Latest maps "latest" to math.MaxUint64 (the server
// clamps to current head). Any other behavior would break the
// --to-height=latest use case which the operator's hourly Job relies on.
func TestResolveToHeight_Latest(t *testing.T) {
	got, err := resolveToHeight("latest")
	if err != nil {
		t.Fatal(err)
	}
	if got != ^uint64(0) {
		t.Errorf("resolveToHeight(latest) = %d, want max uint64", got)
	}
}

// TestResolveToHeight_Number ensures a numeric height passes through.
func TestResolveToHeight_Number(t *testing.T) {
	got, err := resolveToHeight("1082780")
	if err != nil {
		t.Fatal(err)
	}
	if got != 1082780 {
		t.Errorf("resolveToHeight(1082780) = %d, want 1082780", got)
	}
}

// TestResolveToHeight_Garbage covers the bad-input gate.
func TestResolveToHeight_Garbage(t *testing.T) {
	_, err := resolveToHeight("not-a-number")
	if err == nil {
		t.Error("expected error for garbage --to-height")
	}
}

// TestLocalIdempotencyShortCircuit_DoneSentinelHit is the optimization
// proof: a previously-completed export of the same range short-circuits
// without contacting luxd.
func TestLocalIdempotencyShortCircuit_DoneSentinelHit(t *testing.T) {
	dir := t.TempDir()
	outFile := filepath.Join(dir, "C-1082780.rlp")
	if err := os.WriteFile(outFile, []byte("rlp-bytes"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(outFile+".sentinel", []byte(`{
		"status": "done",
		"first": 0,
		"last": 1082780,
		"highestExported": 1082780,
		"firstHash": "0x3f4fa2a0",
		"lastHash": "0xdeadbeef"
	}`), 0644); err != nil {
		t.Fatal(err)
	}
	c := &config{
		chainAlias: "C",
		outputFile: outFile,
		fromHeight: 0,
		toHeight:   "1082780",
	}
	code, ok := localIdempotencyShortCircuit(c)
	if !ok {
		t.Fatal("expected short-circuit hit")
	}
	if code != exitOK {
		t.Errorf("code = %d, want %d", code, exitOK)
	}
}

// TestLocalIdempotencyShortCircuit_LatestFallsThrough ensures the
// --to-height=latest case always falls through to the server (the local
// check can't resolve "latest" without asking luxd).
func TestLocalIdempotencyShortCircuit_LatestFallsThrough(t *testing.T) {
	dir := t.TempDir()
	outFile := filepath.Join(dir, "x.rlp")
	if err := os.WriteFile(outFile, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(outFile+".sentinel", []byte(`{
		"status": "done", "first": 0, "last": 99,
		"highestExported": 99, "firstHash": "0x1", "lastHash": "0x2"
	}`), 0644); err != nil {
		t.Fatal(err)
	}
	c := &config{
		chainAlias: "C",
		outputFile: outFile,
		fromHeight: 0,
		toHeight:   "latest", // <-- must fall through
	}
	if _, ok := localIdempotencyShortCircuit(c); ok {
		t.Error("expected fall-through for --to-height=latest")
	}
}

// TestLocalIdempotencyShortCircuit_MissingFile rejects the case where the
// sentinel claims done but the actual RLP file has been deleted.
func TestLocalIdempotencyShortCircuit_MissingFile(t *testing.T) {
	dir := t.TempDir()
	outFile := filepath.Join(dir, "missing.rlp")
	if err := os.WriteFile(outFile+".sentinel", []byte(`{
		"status": "done", "first": 0, "last": 99,
		"highestExported": 99, "firstHash": "0x1", "lastHash": "0x2"
	}`), 0644); err != nil {
		t.Fatal(err)
	}
	c := &config{
		chainAlias: "C",
		outputFile: outFile,
		fromHeight: 0,
		toHeight:   "99",
	}
	if _, ok := localIdempotencyShortCircuit(c); ok {
		t.Error("expected fall-through when output file missing")
	}
}

// TestLocalIdempotencyShortCircuit_StatusInProgressFallsThrough covers the
// "previous run crashed mid-flight" case — the local check must not accept
// an in-progress sentinel as proof of completion.
func TestLocalIdempotencyShortCircuit_StatusInProgressFallsThrough(t *testing.T) {
	dir := t.TempDir()
	outFile := filepath.Join(dir, "partial.rlp")
	if err := os.WriteFile(outFile, []byte("partial"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(outFile+".sentinel", []byte(`{
		"status": "in_progress", "first": 0, "last": 1082780,
		"highestExported": 512000, "firstHash": "0x1", "lastHash": "0x2"
	}`), 0644); err != nil {
		t.Fatal(err)
	}
	c := &config{
		chainAlias: "C",
		outputFile: outFile,
		fromHeight: 0,
		toHeight:   "1082780",
	}
	if _, ok := localIdempotencyShortCircuit(c); ok {
		t.Error("expected fall-through on in_progress sentinel")
	}
}

// TestRun_HappyPath drives the full lifecycle against an httptest server
// that pretends to be luxd.
func TestRun_HappyPath(t *testing.T) {
	var exportCalled bool
	var exportBodyRaw []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/v1/health":
			w.WriteHeader(http.StatusOK)
		case strings.HasPrefix(r.URL.Path, "/v1/chain/"):
			exportCalled = true
			exportBodyRaw, _ = io.ReadAll(r.Body)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","result":{
				"success": true,
				"status": "ok",
				"blocksExported": 1082781,
				"first": 0,
				"last": 1082780,
				"highestExported": 1082780,
				"firstHash": "0x3f4fa2a0",
				"lastHash": "0xabcdef00",
				"sentinelPath": "/data/exports/C-1082780.rlp.sentinel",
				"message": "exported 1082781 blocks [0..1082780]"
			},"id":1}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	dir := t.TempDir()
	c := &config{
		chainAlias: "C",
		outputFile: filepath.Join(dir, "C-1082780.rlp"),
		fromHeight: 0,
		toHeight:   "1082780",
		luxdRPC:    srv.URL,
	}

	code, err := run(context.Background(), c)
	if code != exitOK {
		t.Fatalf("code = %d (err=%v), want %d", code, err, exitOK)
	}
	if !exportCalled {
		t.Error("admin_exportChain was not called")
	}
	var req jsonRPCRequest
	if err := json.Unmarshal(exportBodyRaw, &req); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if req.Method != "admin_exportChain" {
		t.Errorf("method = %q, want admin_exportChain", req.Method)
	}
	if len(req.Params) != 3 {
		t.Fatalf("params len = %d, want 3", len(req.Params))
	}
	if got, _ := req.Params[0].(string); got != c.outputFile {
		t.Errorf("params[0] (file) = %q, want %q", got, c.outputFile)
	}
}

// TestRun_NoopFromServer covers the "server returned status=noop" branch
// (a different host called rlp-export against an already-exported file).
func TestRun_NoopFromServer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/v1/health":
			w.WriteHeader(http.StatusOK)
		case strings.HasPrefix(r.URL.Path, "/v1/chain/"):
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","result":{
				"success": true,
				"status": "noop",
				"blocksExported": 100,
				"first": 0,
				"last": 99,
				"highestExported": 99,
				"firstHash": "0x1",
				"lastHash": "0x2",
				"sentinelPath": "x"
			},"id":1}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	dir := t.TempDir()
	c := &config{
		chainAlias: "C",
		outputFile: filepath.Join(dir, "noop.rlp"),
		fromHeight: 0,
		toHeight:   "99",
		luxdRPC:    srv.URL,
	}
	code, err := run(context.Background(), c)
	if code != exitOK {
		t.Errorf("code = %d (err=%v), want %d", code, err, exitOK)
	}
}

// TestRun_InterruptedFromServer covers the partial-completion exit code.
// The K8s Job restart policy reads this to schedule a resume run.
func TestRun_InterruptedFromServer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/v1/health":
			w.WriteHeader(http.StatusOK)
		case strings.HasPrefix(r.URL.Path, "/v1/chain/"):
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","result":{
				"success": false,
				"status": "interrupted",
				"blocksExported": 512000,
				"first": 0,
				"last": 1082780,
				"highestExported": 512000,
				"firstHash": "0x1",
				"lastHash": "0x2",
				"sentinelPath": "x"
			},"id":1}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	dir := t.TempDir()
	c := &config{
		chainAlias: "C",
		outputFile: filepath.Join(dir, "C.rlp"),
		fromHeight: 0,
		toHeight:   "1082780",
		luxdRPC:    srv.URL,
	}
	code, _ := run(context.Background(), c)
	if code != exitInterrupted {
		t.Errorf("code = %d, want %d (interrupted)", code, exitInterrupted)
	}
}

// TestRun_LuxdUnreachable covers the transient-failure exit code.
func TestRun_LuxdUnreachable(t *testing.T) {
	// Use a closed port — no server bound there.
	c := &config{
		chainAlias: "C",
		outputFile: filepath.Join(t.TempDir(), "x.rlp"),
		fromHeight: 0,
		toHeight:   "99",
		luxdRPC:    "http://127.0.0.1:1", // closed
	}
	// Short timeout via context so the test runs in seconds.
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	code, err := run(ctx, c)
	// Either we hit the health-check timeout (exitLuxdUnreach) or the
	// outer context cancels us before the deadline (also acceptable —
	// both indicate "luxd not there").
	if code != exitLuxdUnreach && code != exitInterrupted {
		t.Errorf("code = %d (err=%v), want %d or %d",
			code, err, exitLuxdUnreach, exitInterrupted)
	}
}

// TestRun_AdminRPCError covers a luxd-side RPC failure (admin API disabled
// is the common case in production envs that haven't opted in).
func TestRun_AdminRPCError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/v1/health":
			w.WriteHeader(http.StatusOK)
		case strings.HasPrefix(r.URL.Path, "/v1/chain/"):
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","error":{"code":-32601,"message":"admin API not enabled"},"id":1}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := &config{
		chainAlias: "C",
		outputFile: filepath.Join(t.TempDir(), "x.rlp"),
		fromHeight: 0,
		toHeight:   "99",
		luxdRPC:    srv.URL,
	}
	code, err := run(context.Background(), c)
	if code != exitAdminRPCError {
		t.Errorf("code = %d, want %d", code, exitAdminRPCError)
	}
	if err == nil || !strings.Contains(err.Error(), "admin API not enabled") {
		t.Errorf("err = %v, want admin API not enabled", err)
	}
}

// TestCallAdminExportChain_BearerAuthSet asserts KMS_AUTH_TOKEN flows into
// the Authorization header — symmetric with rlp-import's same gate.
func TestCallAdminExportChain_BearerAuthSet(t *testing.T) {
	t.Setenv("KMS_AUTH_TOKEN", "secret-token")
	var sawAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","result":{
			"success": true, "status": "ok",
			"blocksExported": 1, "first": 0, "last": 0,
			"highestExported": 0, "firstHash": "0x1", "lastHash": "0x1",
			"sentinelPath": "x"
		},"id":1}`))
	}))
	defer srv.Close()

	_, err := callAdminExportChain(context.Background(), srv.URL, "C", "/x.rlp", 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if sawAuth != "Bearer secret-token" {
		t.Errorf("Authorization = %q, want %q", sawAuth, "Bearer secret-token")
	}
}
