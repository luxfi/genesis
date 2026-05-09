package configs

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestGetGenesis_DisableCChain pins the LUX_DISABLE_CCHAIN=1 contract:
// when set, the embedded C-Chain genesis is omitted from the primary
// genesis config, so builder.FromConfig's `if config.CChainGenesis != ""`
// guard skips creating a C-Chain entry on the primary network.
//
// Why this knob exists: the lqd binary has historically baked a C-Chain
// into every primary network it builds (chainId 31337 by default for
// LocalID/networkID 1337). Downstream forks like Liquidity that run
// their own EVM blockchain via CreateChainTx don't use the C-Chain at
// all, but found it silently mounted at /ext/bc/C/rpc — confusing for
// SREs and a foot-gun for any service that hard-codes the C alias.
//
// The user-facing assertion: with LUX_DISABLE_CCHAIN=1 the primary
// genesis JSON's `cChainGenesis` field is empty, and downstream
// FromConfig won't append a C-Chain entry.
func TestGetGenesis_DisableCChain(t *testing.T) {
	t.Setenv("LUX_DISABLE_CCHAIN", "1")
	t.Setenv("LUX_CCHAIN_GENESIS_FILE", "") // make sure file override doesn't shadow

	data, err := GetGenesis(LocalID)
	if err != nil {
		t.Fatalf("GetGenesis(LocalID) with LUX_DISABLE_CCHAIN=1: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("parse: %v", err)
	}
	got, _ := m["cChainGenesis"].(string)
	if got != "" {
		// Trim for log size — a 12 KiB hex blob is unhelpful in test
		// output. The first 80 chars are enough to identify which
		// embedded genesis leaked.
		head := got
		if len(head) > 80 {
			head = head[:80] + "..."
		}
		t.Fatalf("LUX_DISABLE_CCHAIN=1 should empty cChainGenesis; got %q", head)
	}
	if !strings.Contains(string(data), `"networkID":1337`) {
		t.Fatalf("expected networkID 1337 in genesis, got %s", string(data[:200]))
	}
}

// TestGetGenesis_DisableCChainOverridesFile pins the precedence: when
// both LUX_DISABLE_CCHAIN=1 and LUX_CCHAIN_GENESIS_FILE are set, the
// disable wins (file override is ignored). This matches the rule a
// human would expect — "disable" is a stronger statement than "use
// this file" — so an SRE setting both during a misconfig probe gets
// the no-C-Chain outcome rather than the file's content silently
// landing on the primary.
func TestGetGenesis_DisableCChainOverridesFile(t *testing.T) {
	t.Setenv("LUX_DISABLE_CCHAIN", "1")
	t.Setenv("LUX_CCHAIN_GENESIS_FILE", "/nonexistent/should-not-be-read.json")

	data, err := GetGenesis(LocalID)
	if err != nil {
		// If we hit "read /nonexistent/...: no such file or directory"
		// it means precedence is wrong — the file branch ran first.
		t.Fatalf("expected disable to win over file override; got: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got, _ := m["cChainGenesis"].(string); got != "" {
		t.Fatalf("disable should still win when file env is also set; got non-empty cChainGenesis")
	}
}

// TestGetGenesis_DisableCChainDefault confirms the default path still
// embeds the C-Chain when the disable knob is unset. Without this
// guard, a careless edit to the env-var precedence above could
// silently strip C-Chain from every Lux network — which would brick
// mainnet/testnet/devnet despite working locally.
func TestGetGenesis_DisableCChainDefault(t *testing.T) {
	t.Setenv("LUX_DISABLE_CCHAIN", "") // explicitly unset
	t.Setenv("LUX_CCHAIN_GENESIS_FILE", "")

	data, err := GetGenesis(LocalID)
	if err != nil {
		t.Fatalf("GetGenesis default: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got, _ := m["cChainGenesis"].(string); got == "" {
		t.Fatal("default path should embed cChainGenesis (unset disable)")
	}
}
