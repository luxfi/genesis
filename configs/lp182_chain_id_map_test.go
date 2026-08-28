// Copyright (C) 2024-2026, Lux Industries Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package configs

import "testing"

// TestEVMChainID_KnownCells locks the canonical map per LP-018
// §Canonical EVM chainID map. Any change to these values is a
// backward-incompatible chainID rebrand and would replay-protect-conflict
// with already-deployed contracts; this test is the protective floor.
func TestEVMChainID_KnownCells(t *testing.T) {
	cases := []struct {
		family NetworkFamily
		env    uint32
		want   uint64
	}{
		{FamilyLux, MainnetID, 96369},
		{FamilyLux, TestnetID, 96368},
		{FamilyLux, DevnetID, 96367},
		{FamilyLux, LocalID, 31337},
		{FamilyHanzo, MainnetID, 36963},
		{FamilyHanzo, TestnetID, 36962},
		{FamilyHanzo, DevnetID, 36964},
		{FamilyZoo, MainnetID, 200200},
		{FamilyZoo, TestnetID, 200201},
		{FamilyZoo, DevnetID, 200202},
		{FamilySPC, MainnetID, 36911},
		{FamilySPC, TestnetID, 36910},
		{FamilySPC, DevnetID, 36912},
		{FamilyPars, MainnetID, 494949},
		{FamilyPars, TestnetID, 494950},
		{FamilyPars, DevnetID, 494951},
	}
	for _, c := range cases {
		got, ok := EVMChainID(c.family, c.env)
		if !ok {
			t.Errorf("EVMChainID(%q, %d) = (_, false), want (%d, true)", c.family, c.env, c.want)
			continue
		}
		if got != c.want {
			t.Errorf("EVMChainID(%q, %d) = %d, want %d", c.family, c.env, got, c.want)
		}
	}
}

// TestEVMChainID_UnknownReturnsFalse — an unknown family or an env
// outside the canonical four must return (0, false), not a silent
// default. This protects against typos in operator CRs.
//
// (Filename retains lp182_ prefix because that was the file's name at
// commit time and the rename produces churn unrelated to behavior.)
func TestEVMChainID_UnknownReturnsFalse(t *testing.T) {
	if got, ok := EVMChainID(NetworkFamily("bogus"), MainnetID); ok || got != 0 {
		t.Errorf("EVMChainID(bogus, mainnet) = (%d, %v), want (0, false)", got, ok)
	}
	if got, ok := EVMChainID(FamilyLux, 9999); ok || got != 0 {
		t.Errorf("EVMChainID(lux, 9999) = (%d, %v), want (0, false)", got, ok)
	}
	if got, ok := EVMChainID(FamilyHanzo, LocalID); ok || got != 0 {
		t.Errorf("EVMChainID(hanzo, local) = (%d, %v), want (0, false) — TBD cell", got, ok)
	}
}

// TestNetworkFamilyOf_RoundTrip — given any canonical chainID,
// NetworkFamilyOf must return the family that originally produced it,
// and EVMChainID on that family must yield the original chainID back.
func TestNetworkFamilyOf_RoundTrip(t *testing.T) {
	cases := []struct {
		chainID uint64
		family  NetworkFamily
	}{
		{96369, FamilyLux},
		{96368, FamilyLux},
		{96367, FamilyLux},
		{31337, FamilyLux},
		{36963, FamilyHanzo},
		{200200, FamilyZoo},
		{36911, FamilySPC},
		{494949, FamilyPars},
	}
	for _, c := range cases {
		got, ok := NetworkFamilyOf(c.chainID)
		if !ok {
			t.Errorf("NetworkFamilyOf(%d) = (_, false), want (%q, true)", c.chainID, c.family)
			continue
		}
		if got != c.family {
			t.Errorf("NetworkFamilyOf(%d) = %q, want %q", c.chainID, got, c.family)
		}
	}
}

// TestNetworkFamilyOf_UnknownReturnsFalse — an unknown chainID must
// return ("", false). This is how a CR validator can detect a typo
// in `evmChainID` early.
func TestNetworkFamilyOf_UnknownReturnsFalse(t *testing.T) {
	if got, ok := NetworkFamilyOf(0); ok || got != "" {
		t.Errorf("NetworkFamilyOf(0) = (%q, %v), want (\"\", false)", got, ok)
	}
	if got, ok := NetworkFamilyOf(99999999); ok || got != "" {
		t.Errorf("NetworkFamilyOf(99999999) = (%q, %v), want (\"\", false)", got, ok)
	}
}

// TestParseNetworkFamily_CaseInsensitive — the parser must accept
// "Lux", "LUX", "lux" all as FamilyLux. The canonical wire form is
// lowercase but humans type whatever they type.
func TestParseNetworkFamily_CaseInsensitive(t *testing.T) {
	for _, in := range []string{"lux", "LUX", "Lux", "luX"} {
		got, ok := ParseNetworkFamily(in)
		if !ok {
			t.Errorf("ParseNetworkFamily(%q) = (_, false), want (lux, true)", in)
			continue
		}
		if got != FamilyLux {
			t.Errorf("ParseNetworkFamily(%q) = %q, want %q", in, got, FamilyLux)
		}
	}
	// Unknown family stays unknown.
	if got, ok := ParseNetworkFamily("bogus"); ok || got != "" {
		t.Errorf("ParseNetworkFamily(bogus) = (%q, %v), want (\"\", false)", got, ok)
	}
}

// TestEVMChainIDFor_StringInputs locks the string-input convenience
// wrapper to the full LP-182 §Appendix A table. This is the API
// CRD validators / operator webhooks call.
func TestEVMChainIDFor_StringInputs(t *testing.T) {
	cases := []struct {
		brand, env string
		want       uint64
	}{
		{"lux", "mainnet", 96369},
		{"lux", "testnet", 96368},
		{"lux", "devnet", 96367},
		{"lux", "localnet", 31337},
		{"LUX", "MAINNET", 96369},
		{"Lux", "Main", 96369},
		{"hanzo", "mainnet", 36963},
		{"hanzo", "testnet", 36962},
		{"hanzo", "devnet", 36964},
		{"zoo", "mainnet", 200200},
		{"zoo", "testnet", 200201},
		{"zoo", "devnet", 200202},
		{"spc", "mainnet", 36911},
		{"spc", "testnet", 36910},
		{"spc", "devnet", 36912},
		{"pars", "mainnet", 494949},
		{"pars", "testnet", 494950},
		{"pars", "devnet", 494951},
	}
	for _, c := range cases {
		got, ok := EVMChainIDFor(c.brand, c.env)
		if !ok {
			t.Errorf("EVMChainIDFor(%q,%q) = (_, false), want (%d, true)", c.brand, c.env, c.want)
			continue
		}
		if got != c.want {
			t.Errorf("EVMChainIDFor(%q,%q) = %d, want %d", c.brand, c.env, got, c.want)
		}
	}
}

// TestEVMChainIDFor_UnknownReturnsFalse — typos and TBD cells return
// (0, false) instead of a silent default.
func TestEVMChainIDFor_UnknownReturnsFalse(t *testing.T) {
	for _, c := range []struct{ brand, env string }{
		{"bogus", "mainnet"},
		{"lux", "bogus"},
		{"hanzo", "localnet"}, // hanzo has no localnet cell
		{"", ""},
	} {
		if got, ok := EVMChainIDFor(c.brand, c.env); ok || got != 0 {
			t.Errorf("EVMChainIDFor(%q,%q) = (%d, %v), want (0, false)", c.brand, c.env, got, ok)
		}
	}
}
