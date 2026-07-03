// Copyright (C) 2019-2026, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package configs

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// LP-134 chain-level decomplect invariants.
//
// The legacy T-Chain (ThresholdVM serving BOTH threshold-FHE and
// threshold-MPC) is split into two orthogonal chains — F-Chain (FHE only,
// confidential DECRYPT) and M-Chain (MPC only, CGGMP21/FROST bridge-custody
// SIGNING) — plus a K-Chain (KeyVM/KMS) cleaned of the MPC conflation
// ("K stays keyvm"). Both F and M run the same ThresholdVM substrate but are
// distinct chains with disjoint purposes, params, precompile families, and
// aliases. These tests fail closed if any concern leaks across the split:
//   - no MPC field/alias/precompile-family on F-Chain,
//   - no FHE field/alias/precompile-family on M-Chain,
//   - no alias claimed by two of {F, M, K},
//   - the retired tchain.json is gone (forward-only).

var lp134Nets = []string{"mainnet", "testnet", "devnet", "localnet"}

func lp134ShardPath(net, letter string) string {
	return filepath.Join("..", "configs", net, letter+"chain.json")
}

func lp134ReadShard(t *testing.T, net, letter string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(lp134ShardPath(net, letter))
	if err != nil {
		t.Fatalf("%s/%schain.json: %v", net, letter, err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("%s/%schain.json parse: %v", net, letter, err)
	}
	return m
}

func lp134ShardText(t *testing.T, net, letter string) string {
	t.Helper()
	data, err := os.ReadFile(lp134ShardPath(net, letter))
	if err != nil {
		t.Fatalf("%s/%schain.json: %v", net, letter, err)
	}
	return string(data)
}

func lp134AliasSet(t *testing.T, m map[string]any) map[string]bool {
	t.Helper()
	raw, ok := m["aliases"].([]any)
	if !ok {
		t.Fatalf("aliases missing or wrong type: %T", m["aliases"])
	}
	out := make(map[string]bool, len(raw))
	for _, a := range raw {
		s, _ := a.(string)
		out[strings.ToLower(s)] = true
	}
	return out
}

// TestLP134_TChainRetired: the legacy tchain.json is gone on every net
// (forward-only split — F-Chain + M-Chain supersede it).
func TestLP134_TChainRetired(t *testing.T) {
	for _, net := range lp134Nets {
		if _, err := os.Stat(lp134ShardPath(net, "t")); !os.IsNotExist(err) {
			t.Fatalf("%s/tchain.json must be retired (LP-134 forward-only split); stat err=%v", net, err)
		}
	}
}

// TestLP134_FChainIsFHEOnly: F-Chain carries FHE concepts only — ThresholdVM
// in FHE mode, FHE ring params, the 0x07 FHE precompile family, and NO MPC
// field, alias, or 0x08 (threshold-signature) precompile-family reference.
func TestLP134_FChainIsFHEOnly(t *testing.T) {
	for _, net := range lp134Nets {
		t.Run(net, func(t *testing.T) {
			m := lp134ReadShard(t, net, "f")
			if got, _ := m["vm"].(string); got != "ThresholdVM" {
				t.Fatalf("F-Chain vm = %q, want ThresholdVM", got)
			}
			if got, _ := m["mode"].(string); got != "fhe" {
				t.Fatalf("F-Chain mode = %q, want fhe", got)
			}
			if m["ringDegreeN"] == nil || m["ringModulusQ"] == nil {
				t.Fatalf("F-Chain must carry the FHE ring params (ringDegreeN/ringModulusQ)")
			}
			if got, _ := m["fhePrecompilePrefix"].(string); got != "0x07" {
				t.Fatalf("F-Chain fhePrecompilePrefix = %q, want 0x07", got)
			}
			// Decomplect: no MPC concepts on F-Chain.
			for _, banned := range []string{"mpcParties", "mpcThreshold", "mpcVerifyPrecompiles"} {
				if m[banned] != nil {
					t.Fatalf("F-Chain must not carry MPC field %q (LP-134 decomplect)", banned)
				}
			}
			if strings.Contains(lp134ShardText(t, net, "f"), "0x08") {
				t.Fatalf("F-Chain must not reference the 0x08 threshold-signature precompile family")
			}
			al := lp134AliasSet(t, m)
			for _, want := range []string{"f", "fhe", "fhevm"} {
				if !al[want] {
					t.Fatalf("F-Chain aliases must include %q", want)
				}
			}
			for _, banned := range []string{"m", "mpc", "mpcvm", "t", "threshold", "mpcvm"} {
				if al[banned] {
					t.Fatalf("F-Chain must not claim alias %q", banned)
				}
			}
		})
	}
}

// TestLP134_MChainIsMPCOnly: M-Chain carries MPC concepts only — ThresholdVM
// in MPC mode, MPC params, the CGGMP21 (0x0800..03) + FROST (0x0800..02)
// verify precompiles, and NO FHE field, alias, or 0x07 precompile-family ref.
func TestLP134_MChainIsMPCOnly(t *testing.T) {
	const (
		frostAddr   = "0x0800000000000000000000000000000000000002"
		cggmp21Addr = "0x0800000000000000000000000000000000000003"
	)
	for _, net := range lp134Nets {
		t.Run(net, func(t *testing.T) {
			m := lp134ReadShard(t, net, "m")
			if got, _ := m["vm"].(string); got != "ThresholdVM" {
				t.Fatalf("M-Chain vm = %q, want ThresholdVM", got)
			}
			if got, _ := m["mode"].(string); got != "mpc" {
				t.Fatalf("M-Chain mode = %q, want mpc", got)
			}
			if m["mpcParties"] == nil || m["mpcThreshold"] == nil {
				t.Fatalf("M-Chain must carry the MPC params (mpcParties/mpcThreshold)")
			}
			// EVM verify surface: exactly the two threshold-signature precompiles.
			pcs, ok := m["mpcVerifyPrecompiles"].([]any)
			if !ok || len(pcs) != 2 {
				t.Fatalf("M-Chain must declare its 2 MPC verify precompiles, got %v", m["mpcVerifyPrecompiles"])
			}
			seen := map[string]bool{frostAddr: false, cggmp21Addr: false}
			for _, p := range pcs {
				ps, _ := p.(string)
				ps = strings.ToLower(ps)
				if _, known := seen[ps]; !known {
					t.Fatalf("M-Chain unexpected precompile %q (want only FROST/CGGMP21)", ps)
				}
				seen[ps] = true
			}
			for addr, ok := range seen {
				if !ok {
					t.Fatalf("M-Chain missing MPC verify precompile %s", addr)
				}
			}
			// Decomplect: no FHE concepts on M-Chain.
			for _, banned := range []string{"ringDegreeN", "ringModulusQ", "dkgParties", "dkgThreshold", "fhePrecompilePrefix"} {
				if m[banned] != nil {
					t.Fatalf("M-Chain must not carry FHE field %q (LP-134 decomplect)", banned)
				}
			}
			if strings.Contains(lp134ShardText(t, net, "m"), "0x07") {
				t.Fatalf("M-Chain must not reference the 0x07 FHE precompile family")
			}
			al := lp134AliasSet(t, m)
			for _, want := range []string{"m", "mpc", "mpcvm"} {
				if !al[want] {
					t.Fatalf("M-Chain aliases must include %q", want)
				}
			}
			// M-Chain IS mpcvm (LP-134), so it claims m/mpc/mpcvm (asserted above);
			// it must not claim the F-Chain or retired-T aliases. ("mpcvm" was
			// erroneously in this banned list, contradicting the required set.)
			for _, banned := range []string{"f", "fhe", "fhevm", "t", "threshold"} {
				if al[banned] {
					t.Fatalf("M-Chain must not claim alias %q", banned)
				}
			}
		})
	}
}

// TestLP134_KChainIsKMSOnly: K-Chain is pure KeyVM/KMS — the M/mpc aliases and
// MPC params moved to M-Chain ("K stays keyvm" per LP-134).
func TestLP134_KChainIsKMSOnly(t *testing.T) {
	for _, net := range lp134Nets {
		t.Run(net, func(t *testing.T) {
			m := lp134ReadShard(t, net, "k")
			if got, _ := m["vm"].(string); got != "KeyVM" {
				t.Fatalf("K-Chain vm = %q, want KeyVM", got)
			}
			for _, banned := range []string{"mpcParties", "mpcThreshold", "mpcVerifyPrecompiles", "mode", "ringDegreeN"} {
				if m[banned] != nil {
					t.Fatalf("K-Chain must not carry %q (LP-134: MPC/FHE concerns live on M/F)", banned)
				}
			}
			al := lp134AliasSet(t, m)
			if !al["k"] {
				t.Fatalf("K-Chain must claim alias K")
			}
			for _, banned := range []string{"m", "mpc", "mpcvm", "f", "fhe", "fhevm"} {
				if al[banned] {
					t.Fatalf("K-Chain must not claim alias %q (LP-134 decomplect)", banned)
				}
			}
		})
	}
}

// TestLP134_NoAliasCollision: the threshold-family chains {F, M, K} claim
// mutually-disjoint alias sets — the core decomplect invariant (a single alias
// resolving to two chains is the exact bug LP-134 removes).
func TestLP134_NoAliasCollision(t *testing.T) {
	for _, net := range lp134Nets {
		t.Run(net, func(t *testing.T) {
			f := lp134AliasSet(t, lp134ReadShard(t, net, "f"))
			mm := lp134AliasSet(t, lp134ReadShard(t, net, "m"))
			k := lp134AliasSet(t, lp134ReadShard(t, net, "k"))
			for _, pair := range []struct {
				a, b   map[string]bool
				an, bn string
			}{
				{f, mm, "F", "M"},
				{f, k, "F", "K"},
				{mm, k, "M", "K"},
			} {
				for alias := range pair.a {
					if pair.b[alias] {
						t.Fatalf("%s-Chain alias %q collides with %s-Chain (LP-134: aliases must be disjoint)", pair.an, alias, pair.bn)
					}
				}
			}
		})
	}
}
