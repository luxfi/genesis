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

// TestGetGenesis_XChainShardPresentEmbedsXChainGenesis is the X-Chain
// equivalent of the C-Chain test above. Every Lux network ships an
// xchain.json shard (the small asset descriptor
// `{"symbol":"LUX","name":"Lux","denomination":9}`); the loader must
// thread it into the marshalled primary genesis JSON as the
// xChainGenesis field, where the builder later parses it to construct
// the XVM genesis and CreateChainTx.
//
// Together with TestBuildGenesisFromDir_AbsentShardEmptyOptIn (which
// covers the absent-shard case for X), this enforces the same data-
// driven contract for X that we already have for C/Q/Z: chain
// presence is a file-tree edit, not an operator switch. The previous
// hardcoded `Symbol: "LUX", Name: "Lux", Denomination: 9` literal in
// builder.FromConfig is now sourced from the shard, so a P-only
// network (Liquidity-shape) can opt out by simply omitting the file.
func TestGetGenesis_XChainShardPresentEmbedsXChainGenesis(t *testing.T) {
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
			got, _ := m["xChainGenesis"].(string)
			if got == "" {
				t.Fatalf("%s: expected non-empty xChainGenesis (embedded shard present)", name)
			}
			// Sanity-check the shard parses and pins the canonical Lux asset.
			var asset struct {
				Symbol       string `json:"symbol"`
				Name         string `json:"name"`
				Denomination byte   `json:"denomination"`
			}
			if err := json.Unmarshal([]byte(got), &asset); err != nil {
				t.Fatalf("%s: xChainGenesis shard unparseable: %v", name, err)
			}
			if asset.Symbol != "LUX" || asset.Name != "Lux" || asset.Denomination != 9 {
				t.Fatalf("%s: xChainGenesis shard drift: got %+v want {LUX Lux 9}", name, asset)
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
	// Minimal shards: network + P-Chain only. No X/C/Q/Z. This is the
	// P-only Liquidity shape — every primary-network chain (X included)
	// is opt-in by shard presence.
	write("network.json", `{"networkID":1337,"startTime":1735689600,"message":"P-only test"}`)
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
	for _, field := range []string{"xChainGenesis", "cChainGenesis", "qChainGenesis", "zChainGenesis"} {
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
