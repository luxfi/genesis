// Copyright (C) 2025-2026, Lux Industries Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package genesis

import (
	"testing"

	"github.com/luxfi/constants"
)

// TestIsLightMnemonic_Exact pins constant-time equality with the
// well-known dev seed.
func TestIsLightMnemonic_Exact(t *testing.T) {
	if !IsLightMnemonic(LightMnemonic) {
		t.Fatalf("LightMnemonic must equal itself")
	}
	if IsLightMnemonic("") {
		t.Fatalf("empty string must not equal LightMnemonic")
	}
	// One-character delta — must not match.
	bad := "light light light light light light light light light light light eneRgy"
	if IsLightMnemonic(bad) {
		t.Fatalf("LightMnemonic must reject %q", bad)
	}
	// Real production-style mnemonic — must not match.
	prod := "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about"
	if IsLightMnemonic(prod) {
		t.Fatalf("LightMnemonic must reject production mnemonic")
	}
}

// TestIsProductionNetwork covers the network-id classification.
func TestIsProductionNetwork(t *testing.T) {
	cases := []struct {
		nid  uint32
		want bool
		why  string
	}{
		{constants.MainnetID, true, "mainnet"},
		{constants.TestnetID, true, "testnet"},
		{constants.LocalID, false, "LocalID = 1337 dev mesh"},
		{1338, false, "1337+ dev mesh range"},
		{99999, false, "tenant dev cluster"},
		{3, true, "reserved low ID, treated as prod"},
		{1336, true, "below 1337 not an explicit dev ID"},
	}
	for _, c := range cases {
		if got := IsProductionNetwork(c.nid); got != c.want {
			t.Errorf("IsProductionNetwork(%d) = %v, want %v (%s)", c.nid, got, c.want, c.why)
		}
	}
}

// TestIsKnownPublicMnemonic — F31 expansion. The guard now refuses every
// known-public mnemonic, not just LIGHT_MNEMONIC. The BIP-39 abandon
// vector, Hardhat default, and Trezor demo previously passed the
// LIGHT_MNEMONIC-only check.
func TestIsKnownPublicMnemonic(t *testing.T) {
	cases := []struct {
		name     string
		mnemonic string
		want     bool
	}{
		{"LIGHT_MNEMONIC", LightMnemonic, true},
		{"BIP-39 abandon vector #1",
			"abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about",
			true},
		{"BIP-39 vector #5",
			"legal winner thank year wave sausage worth useful legal winner thank yellow",
			true},
		{"Hardhat / Foundry default",
			"test test test test test test test test test test test junk",
			true},
		{"Trezor demo",
			"witch collapse practice feed shame open despair creek road again ice least",
			true},
		{"Real-looking 12-word mnemonic (random bytes)",
			"venue armor mouse cheese fork stem siren acquire rocket cabbage sentence vibrant",
			false},
		{"Empty string",
			"",
			false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := IsKnownPublicMnemonic(c.mnemonic); got != c.want {
				t.Errorf("IsKnownPublicMnemonic(%q) = %v, want %v", c.mnemonic, got, c.want)
			}
		})
	}
}

// TestRefuseLightMnemonicOnProduction is the headline F12 fix +
// F31 expansion: derivation MUST refuse on production for any
// publicly-known mnemonic, not just the literal LIGHT_MNEMONIC.
func TestRefuseLightMnemonicOnProduction(t *testing.T) {
	// 1. No mnemonic set: not our problem at this layer.
	t.Setenv("MNEMONIC", "")
	t.Setenv("LUX_MNEMONIC", "")
	t.Setenv("LIGHT_MNEMONIC", "")
	if err := RefuseLightMnemonicOnProduction(constants.MainnetID); err != nil {
		t.Fatalf("no-mnemonic case: unexpected error: %v", err)
	}

	// 2. LIGHT_MNEMONIC on dev mesh: allowed.
	t.Setenv("LIGHT_MNEMONIC", LightMnemonic)
	if err := RefuseLightMnemonicOnProduction(constants.LocalID); err != nil {
		t.Fatalf("LIGHT_MNEMONIC on LocalID(1337): unexpected error: %v", err)
	}

	// 3. LIGHT_MNEMONIC on mainnet: REFUSED.
	if err := RefuseLightMnemonicOnProduction(constants.MainnetID); err == nil {
		t.Fatalf("LIGHT_MNEMONIC on MainnetID: expected refusal, got nil")
	}

	// 4. LIGHT_MNEMONIC on testnet: REFUSED.
	if err := RefuseLightMnemonicOnProduction(constants.TestnetID); err == nil {
		t.Fatalf("LIGHT_MNEMONIC on TestnetID: expected refusal, got nil")
	}

	// 5. F31: BIP-39 abandon vector via MNEMONIC on mainnet → REFUSED
	// (was previously ALLOWED — see prior red-review F31).
	t.Setenv("LIGHT_MNEMONIC", "")
	t.Setenv("MNEMONIC", "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about")
	if err := RefuseLightMnemonicOnProduction(constants.MainnetID); err == nil {
		t.Fatalf("BIP-39 abandon vector on mainnet: expected refusal, got nil")
	}

	// 6. F31: Hardhat default on mainnet → REFUSED.
	t.Setenv("MNEMONIC", "test test test test test test test test test test test junk")
	if err := RefuseLightMnemonicOnProduction(constants.MainnetID); err == nil {
		t.Fatalf("Hardhat default on mainnet: expected refusal, got nil")
	}

	// 7. Real (non-public) mnemonic on mainnet: allowed.
	realMnemonic := "venue armor mouse cheese fork stem siren acquire rocket cabbage sentence vibrant"
	t.Setenv("MNEMONIC", realMnemonic)
	if err := RefuseLightMnemonicOnProduction(constants.MainnetID); err != nil {
		t.Fatalf("real mnemonic on mainnet: unexpected error: %v", err)
	}

	// 8. Real LUX_MNEMONIC on mainnet: allowed.
	t.Setenv("MNEMONIC", "")
	t.Setenv("LUX_MNEMONIC", realMnemonic)
	if err := RefuseLightMnemonicOnProduction(constants.MainnetID); err != nil {
		t.Fatalf("real LUX_MNEMONIC on mainnet: unexpected error: %v", err)
	}
}

// TestLoadKeysFromMnemonicEnvForNetwork — F30 fix. The new entry point
// embeds the public-mnemonic guard so callers can't bypass it. Unlike
// LoadKeysFromMnemonicEnv (which still exists for legacy paths and
// trusted callers), this one fails-closed on a production network with
// any public mnemonic.
func TestLoadKeysFromMnemonicEnvForNetwork(t *testing.T) {
	// Setup: LIGHT_MNEMONIC on mainnet should refuse derivation.
	t.Setenv("MNEMONIC", "")
	t.Setenv("LUX_MNEMONIC", "")
	t.Setenv("LIGHT_MNEMONIC", LightMnemonic)
	_, err := LoadKeysFromMnemonicEnvForNetwork(constants.MainnetID, 1)
	if err == nil {
		t.Fatalf("LoadKeysFromMnemonicEnvForNetwork: LIGHT_MNEMONIC on mainnet must refuse, got nil")
	}

	// Same env on a dev mesh should succeed (the guard does not refuse;
	// derivation proceeds via the underlying LoadKeysFromMnemonicEnv).
	keys, err := LoadKeysFromMnemonicEnvForNetwork(constants.LocalID, 1)
	if err != nil {
		t.Fatalf("LoadKeysFromMnemonicEnvForNetwork: LIGHT_MNEMONIC on dev (LocalID=1337): unexpected error: %v", err)
	}
	if len(keys) != 1 {
		t.Fatalf("expected 1 key, got %d", len(keys))
	}
}
