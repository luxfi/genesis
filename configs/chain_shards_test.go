// Copyright (C) 2019-2026, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package configs

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestGetGenesis_CChainShardPresentEmbedsCChainGenesis confirms that for
// every embedded network that ships a cchain.json shard, the marshalled
// primary genesis JSON contains a non-empty cChainGenesis field — i.e. the
// builder will emit a C-Chain CreateChainTx for that network.
//
// This is the new contract that replaced the LUX_DISABLE_CCHAIN env knob:
// chain presence is data-driven (does the shard exist?) rather than
// runtime-toggled (is an env set?). Adding/removing a chain from a
// network's primary genesis is a file-tree edit, not an operator switch.
func TestGetGenesis_CChainShardPresentEmbedsCChainGenesis(t *testing.T) {
	for _, name := range []string{"mainnet", "testnet", "localnet"} {
		t.Run(name, func(t *testing.T) {
			data, err := GetGenesis(networkIDFromName(t, name))
			if err != nil {
				t.Fatalf("GetGenesis(%s): %v", name, err)
			}
			var m map[string]any
			if err := json.Unmarshal(data, &m); err != nil {
				t.Fatalf("parse %s: %v", name, err)
			}
			got, _ := m["cChainGenesis"].(string)
			if got == "" {
				t.Fatalf("%s: expected non-empty cChainGenesis (embedded shard present)", name)
			}
		})
	}
}

// TestBuildGenesisFromDir_AbsentShardEmptyOptIn confirms the FS-fallback
// loader honours the same "shard absent → opt-out" semantics as the
// embedded loader. An operator who wants a P+X-only network just omits
// the relevant chain shards from their dir; no env knob, no flag.
func TestBuildGenesisFromDir_AbsentShardEmptyOptIn(t *testing.T) {
	dir := t.TempDir()

	write := func(name, body string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	// Minimal shards: network + P-Chain only. No C/Q/Z. This is the
	// Liquidity-shape: primary network with P+X, everything else opt-in
	// post-genesis via CreateChainTx.
	write("network.json", `{"networkID":1337,"startTime":1735689600,"message":"P+X only test"}`)
	write("pchain.json", `{
		"allocations":[],
		"initialStakeDuration":31536000,
		"initialStakeDurationOffset":5400,
		"initialStakedFunds":[],
		"initialStakers":[]
	}`)

	data, err := buildGenesisFromDir(dir)
	if err != nil {
		t.Fatalf("buildGenesisFromDir: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("parse: %v", err)
	}
	for _, field := range []string{"cChainGenesis", "qChainGenesis", "zChainGenesis"} {
		if got, _ := m[field].(string); got != "" {
			t.Fatalf("absent shard should yield empty %s; got %q", field, got)
		}
	}
}

// networkIDFromName resolves a network directory name to its canonical
// network ID. Test helper that mirrors networkNameFromID's inverse.
func networkIDFromName(t *testing.T, name string) uint32 {
	t.Helper()
	switch name {
	case "mainnet":
		return MainnetID
	case "testnet":
		return TestnetID
	case "devnet":
		return DevnetID
	case "localnet":
		return LocalID
	default:
		t.Fatalf("unknown network name %q", name)
		return 0
	}
}
