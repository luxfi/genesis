// Copyright (C) 2026, Lux Industries Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/luxfi/constants"
)

// TestResolveDChainVMID locks the D-Chain vmID to the DexVMID and asserts its
// CB58 string matches the canonical value baked into luxd. If this ever drifts,
// every D-Chain registration would point at the wrong VM.
func TestResolveDChainVMID(t *testing.T) {
	const wantCB58 = "mDVT5EWMumBp3LCqvKwuyZQeY1VXr1jvjGNAt8nL4UFiXvqXr"
	got := constants.DexVMID.String()
	if got != wantCB58 {
		t.Fatalf("DexVMID CB58 = %q, want %q", got, wantCB58)
	}
	// The byte layout must be exactly {'d','e','x','v','m'} (rest zero).
	want := [32]byte{'d', 'e', 'x', 'v', 'm'}
	if constants.DexVMID != want {
		t.Fatalf("DexVMID bytes = %v, want %v", constants.DexVMID, want)
	}
}

// TestLoadDChainGenesis_Valid writes a fake configs dir with a non-empty valid
// dchain.json and confirms loadDChainGenesis returns the exact bytes + path.
func TestLoadDChainGenesis_Valid(t *testing.T) {
	dir := t.TempDir()
	netDir := filepath.Join(dir, "testnet")
	if err := os.MkdirAll(netDir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := []byte(`{"name":"D-Chain","vm":"DexVM","chainId":96470}`)
	path := filepath.Join(netDir, "dchain.json")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}

	// Env case is normalized to lower for the path.
	raw, gotPath, err := loadDChainGenesis(dir, "TESTNET")
	if err != nil {
		t.Fatalf("loadDChainGenesis: %v", err)
	}
	if gotPath != path {
		t.Fatalf("path = %q, want %q", gotPath, path)
	}
	if string(raw) != string(content) {
		t.Fatalf("genesis = %q, want %q", raw, content)
	}
}

// TestLoadDChainGenesis_Errors covers the three fail-before-spend cases:
// missing file, empty file, and invalid JSON. Each must return a non-nil error
// and nil bytes so no CreateChainTx is ever built from a bad genesis.
func TestLoadDChainGenesis_Errors(t *testing.T) {
	dir := t.TempDir()

	t.Run("missing", func(t *testing.T) {
		raw, _, err := loadDChainGenesis(dir, "devnet")
		if err == nil {
			t.Fatal("expected error for missing file")
		}
		if raw != nil {
			t.Fatalf("expected nil bytes, got %d", len(raw))
		}
	})

	t.Run("empty", func(t *testing.T) {
		nd := filepath.Join(dir, "mainnet")
		if err := os.MkdirAll(nd, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(nd, "dchain.json"), []byte("   \n"), 0o644); err != nil {
			t.Fatal(err)
		}
		raw, _, err := loadDChainGenesis(dir, "mainnet")
		if err == nil {
			t.Fatal("expected error for empty file")
		}
		if raw != nil {
			t.Fatalf("expected nil bytes, got %d", len(raw))
		}
	})

	t.Run("invalid-json", func(t *testing.T) {
		nd := filepath.Join(dir, "local")
		if err := os.MkdirAll(nd, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(nd, "dchain.json"), []byte("{not json"), 0o644); err != nil {
			t.Fatal(err)
		}
		raw, _, err := loadDChainGenesis(dir, "local")
		if err == nil {
			t.Fatal("expected error for invalid JSON")
		}
		if raw != nil {
			t.Fatalf("expected nil bytes, got %d", len(raw))
		}
	})
}

// TestResolveDChainMode exercises the dry-run/broadcast guard purely from the
// two operator inputs — no network, no spend. The default (neither flag) MUST
// be dry-run so an accidental invocation never broadcasts.
func TestResolveDChainMode(t *testing.T) {
	cases := []struct {
		name        string
		unsignedOut string
		broadcast   bool
		want        dchainMode
	}{
		{"default-is-dryrun", "", false, dchainDryRun},
		{"unsigned-out-stages", "/tmp/x.json", false, dchainUnsigned},
		{"broadcast-issues", "", true, dchainBroadcast},
		{"unsigned-wins-over-broadcast", "/tmp/x.json", true, dchainUnsigned},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := resolveDChainMode(c.unsignedOut, c.broadcast); got != c.want {
				t.Fatalf("resolveDChainMode(%q,%v) = %d, want %d", c.unsignedOut, c.broadcast, got, c.want)
			}
		})
	}
}

// TestDChainArtifactRoundTrip confirms the staging wrapper marshals to the
// documented JSON shape and round-trips, including the signWith provenance and
// the unsignedTxHex field that the owner signs.
func TestDChainArtifactRoundTrip(t *testing.T) {
	art := dchainArtifact{
		Network:         "mainnet",
		VMID:            constants.DexVMID.String(),
		ChainName:       dchainName,
		OwnerNetwork:    "2bRCr6B4MiEfSjidDwxDpdCyviwnfUVqB2HGwhm947w9YYqb7r",
		GenesisBytesLen: 375,
		UnsignedTxHex:   "00000000abcd",
		SignWith:        "P-Chain key m/44'/9000'/0'/0/5 derived from KMS secret lux/mainnet/staking[mnemonic]",
		BroadcastHint:   "platform.issueTx",
	}
	body, err := json.MarshalIndent(art, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	// Assert the documented JSON keys are present (downstream tooling depends
	// on these names).
	var generic map[string]any
	if err := json.Unmarshal(body, &generic); err != nil {
		t.Fatalf("unmarshal generic: %v", err)
	}
	for _, k := range []string{"network", "vmId", "chainName", "ownerNetwork", "genesisBytesLen", "unsignedTxHex", "signWith", "broadcastHint"} {
		if _, ok := generic[k]; !ok {
			t.Fatalf("artifact JSON missing key %q; got %s", k, body)
		}
	}

	var back dchainArtifact
	if err := json.Unmarshal(body, &back); err != nil {
		t.Fatalf("unmarshal typed: %v", err)
	}
	if back != art {
		t.Fatalf("round-trip mismatch:\n got %+v\nwant %+v", back, art)
	}
	if back.ChainName != "D-Chain" {
		t.Fatalf("chainName = %q, want D-Chain (must match genesis registry name)", back.ChainName)
	}
}

// TestDChainNameMatchesRegistry guards the idempotency contract: the chainName
// CreateChainTx records must equal the genesis builder's registry name so a
// genesis-registered D-Chain is detected + skipped.
func TestDChainNameMatchesRegistry(t *testing.T) {
	if dchainName != "D-Chain" {
		t.Fatalf("dchainName = %q, want D-Chain", dchainName)
	}
}
