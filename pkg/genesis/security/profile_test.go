// Copyright (C) 2019-2026, Lux Industries Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package security

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	consensusconfig "github.com/luxfi/consensus/config"
	"github.com/luxfi/genesis/pkg/genesis"
)

// TestSecurityProfile_Resolve_StrictPQ proves a genesis pin for the
// canonical StrictPQ profile resolves, validates, and round-trips
// through ComputeHash without mutation. Closes F102 at the genesis
// load layer: the profile that ships in consensus/config is reachable
// from the genesis package and its content hash is checked.
func TestSecurityProfile_Resolve_StrictPQ(t *testing.T) {
	canonical := consensusconfig.StrictPQ()
	live, err := canonical.ComputeHash()
	if err != nil {
		t.Fatalf("StrictPQ().ComputeHash() returned error: %v", err)
	}

	pin := &genesis.SecurityProfile{
		ProfileID:      uint8(consensusconfig.ProfileStrictPQ),
		ProfileHashHex: hex.EncodeToString(live[:]),
	}
	got, err := ResolveProfile(pin)
	if err != nil {
		t.Fatalf("Resolve() returned error: %v", err)
	}
	if got == nil {
		t.Fatalf("Resolve() returned nil profile")
	}
	if got.ProfileID != uint32(consensusconfig.ProfileStrictPQ) {
		t.Errorf("Resolve() returned profile with ProfileID=%d; want %d",
			got.ProfileID, consensusconfig.ProfileStrictPQ)
	}
	if got.ProfileHash != live {
		t.Errorf("Resolve() did not stamp the live hash onto the returned profile")
	}
}

// TestSecurityProfile_Resolve_HashMismatchRejected proves the pin
// refuses a profile whose live hash diverges from the pinned hex.
// This is the load-bearing F102 anti-regression: a forked binary that
// swaps in a different canonical profile content must fail genesis
// boot.
func TestSecurityProfile_Resolve_HashMismatchRejected(t *testing.T) {
	// Pin a wrong hash for StrictPQ.
	pin := &genesis.SecurityProfile{
		ProfileID:      uint8(consensusconfig.ProfileStrictPQ),
		ProfileHashHex: strings.Repeat("00", 48), // 96 hex zeros
	}
	_, err := ResolveProfile(pin)
	if err == nil {
		t.Fatal("Resolve() accepted a wrong-hash pin")
	}
	if !errors.Is(err, ErrSecurityProfileHashMismatch) {
		t.Errorf("Resolve() returned %v; want ErrSecurityProfileHashMismatch", err)
	}
}

// TestSecurityProfile_Resolve_UnknownProfileIDRejected proves the
// pin refuses an unknown ProfileID byte.
func TestSecurityProfile_Resolve_UnknownProfileIDRejected(t *testing.T) {
	pin := &genesis.SecurityProfile{
		ProfileID:      0xFE, // unregistered
		ProfileHashHex: strings.Repeat("ab", 48),
	}
	_, err := ResolveProfile(pin)
	if err == nil {
		t.Fatal("Resolve() accepted an unknown ProfileID")
	}
	if !errors.Is(err, ErrSecurityProfileInvalidID) {
		t.Errorf("Resolve() returned %v; want ErrSecurityProfileInvalidID", err)
	}
}

// TestSecurityProfile_Resolve_InvalidHashHexRejected proves
// non-hex / wrong-length hash strings are refused.
func TestSecurityProfile_Resolve_InvalidHashHexRejected(t *testing.T) {
	cases := map[string]string{
		"empty":         "",
		"short":         "abcd",
		"odd":           strings.Repeat("a", 95),
		"too long":      strings.Repeat("ab", 49),
		"not hex":       strings.Repeat("zz", 48),
		"wrong padding": "0x" + strings.Repeat("ab", 48), // hex.DecodeString refuses 0x prefix
	}
	for name, hashHex := range cases {
		pin := &genesis.SecurityProfile{
			ProfileID:      uint8(consensusconfig.ProfileStrictPQ),
			ProfileHashHex: hashHex,
		}
		_, err := ResolveProfile(pin)
		if err == nil {
			t.Errorf("%s: Resolve() accepted an invalid hash hex %q", name, hashHex)
			continue
		}
		if !errors.Is(err, ErrSecurityProfileInvalidHashHex) {
			t.Errorf("%s: Resolve() returned %v; want ErrSecurityProfileInvalidHashHex", name, err)
		}
	}
}

// TestSecurityProfile_Resolve_NilRejected proves a missing pin is
// detected at the API boundary.
func TestSecurityProfile_Resolve_NilRejected(t *testing.T) {
	var pin *genesis.SecurityProfile
	_, err := ResolveProfile(pin)
	if !errors.Is(err, ErrSecurityProfileMissing) {
		t.Errorf("nil pin: Resolve() returned %v; want ErrSecurityProfileMissing", err)
	}
}

// TestConfig_SecurityProfile_JSONRoundtrip proves the SecurityProfile
// field survives a Config → genesis.ConfigOutput → JSON → genesis.ConfigOutput round
// trip. Closes the wire-form half of F102.
func TestConfig_SecurityProfile_JSONRoundtrip(t *testing.T) {
	canonical := consensusconfig.StrictPQ()
	live, err := canonical.ComputeHash()
	if err != nil {
		t.Fatalf("ComputeHash: %v", err)
	}
	pin := &genesis.SecurityProfile{
		ProfileID:      uint8(consensusconfig.ProfileStrictPQ),
		ProfileHashHex: hex.EncodeToString(live[:]),
	}

	out := &genesis.ConfigOutput{
		NetworkID:       12345,
		Message:         "test",
		SecurityProfile: pin,
	}
	raw, err := json.Marshal(out)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if !strings.Contains(string(raw), `"securityProfile"`) {
		t.Fatalf("marshalled output missing securityProfile field: %s", raw)
	}

	var got genesis.ConfigOutput
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if got.SecurityProfile == nil {
		t.Fatal("SecurityProfile was nil after round-trip")
	}
	if got.SecurityProfile.ProfileID != pin.ProfileID {
		t.Errorf("ProfileID = %d; want %d", got.SecurityProfile.ProfileID, pin.ProfileID)
	}
	if got.SecurityProfile.ProfileHashHex != pin.ProfileHashHex {
		t.Errorf("ProfileHashHex = %q; want %q", got.SecurityProfile.ProfileHashHex, pin.ProfileHashHex)
	}
	if _, err := ResolveProfile(got.SecurityProfile); err != nil {
		t.Errorf("ResolveProfile() after round-trip returned %v", err)
	}
}
