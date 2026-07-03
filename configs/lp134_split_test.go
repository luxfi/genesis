// Copyright (C) 2019-2026, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package configs

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// LP-134 decomplect: the T-Chain braid (one ThresholdVM serving both threshold-FHE
// and threshold-MPC) splits into orthogonal chains. Each threshold-family shard
// declares exactly what it IS — disjoint aliases, fields, and precompile families.
//
// One spec table, one loop. Exact-set alias equality is the whole invariant: it
// subsumes "must include", "must not claim the other chain's aliases", and
// "no two chains share an alias" in a single assertion — and makes the class of
// self-contradictory include/exclude lists impossible by construction.

var lp134Nets = []string{"mainnet", "testnet", "devnet", "localnet"}

type lp134Spec struct {
	letter      string
	vm          string
	mode        string   // "" = no mode field
	aliases     []string // the EXACT alias set (case-insensitive)
	precompiles []string // exact mpcVerifyPrecompiles set, if any
	has         []string // fields that must be present
	hasNot      []string // fields that must be absent
	noText      string   // substring that must not appear in the shard ("" = skip)
}

var lp134Shards = []lp134Spec{
	{letter: "f", vm: "ThresholdVM", mode: "fhe", aliases: []string{"f", "fhe", "fhevm"},
		has:    []string{"ringDegreeN", "ringModulusQ", "fhePrecompilePrefix"},
		hasNot: []string{"mpcParties", "mpcThreshold", "mpcVerifyPrecompiles"}, noText: "0x08"},
	{letter: "m", vm: "ThresholdVM", mode: "mpc", aliases: []string{"m", "mpc", "mpcvm"},
		precompiles: []string{
			"0x0800000000000000000000000000000000000002", // FROST
			"0x0800000000000000000000000000000000000003", // CGGMP21
		},
		has:    []string{"mpcParties", "mpcThreshold"},
		hasNot: []string{"ringDegreeN", "ringModulusQ", "dkgParties", "dkgThreshold", "fhePrecompilePrefix"}, noText: "0x07"},
	{letter: "k", vm: "KeyVM", aliases: []string{"k", "key", "keyvm", "kms"},
		hasNot: []string{"mpcParties", "mpcThreshold", "mpcVerifyPrecompiles", "mode", "ringDegreeN"}},
}

func TestLP134Split(t *testing.T) {
	for _, net := range lp134Nets {
		// Retired T-Chain: forward-only, no tchain.json anywhere.
		if _, err := os.Stat(lp134Path(net, "t")); !os.IsNotExist(err) {
			t.Errorf("%s: tchain.json must be retired (LP-134 forward-only split)", net)
		}
		for _, s := range lp134Shards {
			t.Run(net+"/"+s.letter, func(t *testing.T) {
				text, m := lp134Shard(t, net, s.letter)
				if got, _ := m["vm"].(string); got != s.vm {
					t.Errorf("vm = %q, want %q", got, s.vm)
				}
				if got, _ := m["mode"].(string); got != s.mode {
					t.Errorf("mode = %q, want %q", got, s.mode)
				}
				for _, f := range s.has {
					if m[f] == nil {
						t.Errorf("missing required field %q", f)
					}
				}
				for _, f := range s.hasNot {
					if m[f] != nil {
						t.Errorf("must not carry field %q (decomplect leak)", f)
					}
				}
				if s.noText != "" && strings.Contains(text, s.noText) {
					t.Errorf("must not reference %q (foreign precompile family)", s.noText)
				}
				if got := lp134Set(m["aliases"]); !lp134Equal(got, s.aliases) {
					t.Errorf("aliases = %v, want exactly %v", got, s.aliases)
				}
				if s.precompiles != nil {
					if got := lp134Set(m["mpcVerifyPrecompiles"]); !lp134Equal(got, s.precompiles) {
						t.Errorf("mpcVerifyPrecompiles = %v, want exactly %v", got, s.precompiles)
					}
				}
			})
		}
	}
}

func lp134Path(net, letter string) string {
	return filepath.Join("..", "configs", net, letter+"chain.json")
}

func lp134Shard(t *testing.T, net, letter string) (string, map[string]any) {
	t.Helper()
	data, err := os.ReadFile(lp134Path(net, letter))
	if err != nil {
		t.Fatalf("%s/%schain.json: %v", net, letter, err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("%s/%schain.json parse: %v", net, letter, err)
	}
	return string(data), m
}

// lp134Set lower-cases a JSON string array into a sorted, deduped slice.
func lp134Set(v any) []string {
	raw, _ := v.([]any)
	seen := map[string]bool{}
	for _, a := range raw {
		if s, ok := a.(string); ok {
			seen[strings.ToLower(s)] = true
		}
	}
	out := make([]string, 0, len(seen))
	for s := range seen {
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

func lp134Equal(got, want []string) bool {
	w := append([]string(nil), want...)
	for i := range w {
		w[i] = strings.ToLower(w[i])
	}
	sort.Strings(w)
	if len(got) != len(w) {
		return false
	}
	for i := range got {
		if got[i] != w[i] {
			return false
		}
	}
	return true
}
