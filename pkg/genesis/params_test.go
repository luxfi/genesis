// Copyright (C) 2019-2025, Lux Partners Limited. All rights reserved.
// See the file LICENSE for licensing terms.

package genesis

import (
	"strings"
	"testing"

	"github.com/luxfi/ids"
)

// TestParseAddress_HRPWhitelist locks in that ParseAddress accepts only the
// canonical Lux HRPs (lux/test/dev/local). Any other HRP — notably an upstream
// "avax"/"fuji" address copy-pasted into a Lux genesis — must be rejected with
// a clear error rather than silently re-encoded into a lux1... allocation.
func TestParseAddress_HRPWhitelist(t *testing.T) {
	tests := []struct {
		name    string
		addr    string
		wantErr string // substring; empty means must succeed
	}{
		// Accept: canonical Lux HRPs in the chain-prefixed P-/X- form.
		{
			name: "accept P-lux mainnet validator",
			addr: "P-lux1ck0t9h5u7jvvzhx29n99guqjsfkpzt67wgx7wg",
		},
		{
			name: "accept X-test testnet",
			addr: "X-test1axv78l5zrj7vf3x5h33c8nrq2eslg5vj3zyat4",
		},

		// Reject: upstream Avalanche HRPs synthesized from the canonical
		// mainnet validator (same 20 bytes, foreign HRP) — exactly the
		// footgun this guard exists to catch.
		{
			name:    "reject P-avax (avax HRP not allowed)",
			addr:    "P-avax1ck0t9h5u7jvvzhx29n99guqjsfkpzt67537yam",
			wantErr: `unsupported HRP "avax"`,
		},
		{
			name:    "reject P-fuji (fuji HRP not allowed)",
			addr:    "P-fuji1ck0t9h5u7jvvzhx29n99guqjsfkpzt67cr6m3y",
			wantErr: `unsupported HRP "fuji"`,
		},

		// Reject: empty.
		{
			name:    "reject empty",
			addr:    "",
			wantErr: "empty address",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseAddress(tc.addr)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("ParseAddress(%q) unexpected error: %v", tc.addr, err)
				}
				if got == ids.ShortEmpty {
					t.Fatalf("ParseAddress(%q) returned zero ShortID", tc.addr)
				}
				return
			}
			if err == nil {
				t.Fatalf("ParseAddress(%q) succeeded; want error containing %q", tc.addr, tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("ParseAddress(%q) error = %v; want substring %q", tc.addr, err, tc.wantErr)
			}
		})
	}
}

// TestParseAddress_RawBech32HRPWhitelist exercises the non-chain-prefixed code
// path (raw bech32 without the leading "P-"/"X-") to ensure both paths enforce
// the whitelist.
func TestParseAddress_RawBech32HRPWhitelist(t *testing.T) {
	// Raw bech32 (no chain prefix). Accept lux1..., reject avax1...
	const luxRaw = "lux1ck0t9h5u7jvvzhx29n99guqjsfkpzt67wgx7wg"
	const avaxRaw = "avax1ck0t9h5u7jvvzhx29n99guqjsfkpzt67537yam"

	if _, err := ParseAddress(luxRaw); err != nil {
		t.Fatalf("ParseAddress(%q) unexpected error: %v", luxRaw, err)
	}
	_, err := ParseAddress(avaxRaw)
	if err == nil {
		t.Fatalf("ParseAddress(%q) succeeded; want HRP rejection", avaxRaw)
	}
	if !strings.Contains(err.Error(), `unsupported HRP "avax"`) {
		t.Fatalf("ParseAddress(%q) error = %v; want avax HRP rejection", avaxRaw, err)
	}
}
