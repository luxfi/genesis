// Copyright (C) 2019-2026, Lux Industries Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package genesis

import (
	"encoding/hex"
	"testing"

	"github.com/luxfi/address"
)

// TestChainPrefixFormat covers the decomplected entry point: ChainPrefix carries
// the per-chain identity, hrp carries the per-network identity, and the two
// compose at the call site. Output must be byte-identical to upstream
// address.Format directly.
func TestChainPrefixFormat(t *testing.T) {
	cases := []struct {
		name     string
		prefix   ChainPrefix
		hrp      string
		hex      string
		expected string
	}{
		{
			name:     "P_mainnet_lux",
			prefix:   PChainPrefix,
			hrp:      "lux",
			hex:      "9011E888251AB053B7bD1cdB598Db4f9DEd94714",
			expected: "P-lux1jqg73zp9r2c98daarnd4nrd5l80dj3c5eha5fl",
		},
		{
			name:     "X_testnet_test",
			prefix:   XChainPrefix,
			hrp:      "test",
			hex:      "9011E888251AB053B7bD1cdB598Db4f9DEd94714",
			expected: "X-test1jqg73zp9r2c98daarnd4nrd5l80dj3c5644e09",
		},
		{
			name:     "P_local",
			prefix:   PChainPrefix,
			hrp:      "local",
			hex:      "9011E888251AB053B7bD1cdB598Db4f9DEd94714",
			expected: "P-local1jqg73zp9r2c98daarnd4nrd5l80dj3c56acgey",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			raw, err := hex.DecodeString(tc.hex)
			if err != nil {
				t.Fatalf("decode hex: %v", err)
			}

			got, err := tc.prefix.Format(tc.hrp, raw)
			if err != nil {
				t.Fatalf("ChainPrefix.Format: %v", err)
			}
			if got != tc.expected {
				t.Errorf("ChainPrefix.Format = %s, want %s", got, tc.expected)
			}

			// Must match upstream address.Format byte-for-byte.
			up, err := address.Format(string(tc.prefix), tc.hrp, raw)
			if err != nil {
				t.Fatalf("address.Format: %v", err)
			}
			if got != up {
				t.Errorf("ChainPrefix.Format diverges from address.Format:\n  got:  %s\n  want: %s", got, up)
			}

		})
	}
}

// TestChainPrefixOrthogonal asserts the decomplection invariant: a prefix
// instance produces correctly-prefixed output across every HRP, and an HRP
// passes through unchanged across every prefix. Prefix and HRP do not braid.
func TestChainPrefixOrthogonal(t *testing.T) {
	addr, err := hex.DecodeString("9011E888251AB053B7bD1cdB598Db4f9DEd94714")
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	prefixes := []ChainPrefix{PChainPrefix, XChainPrefix}
	hrps := []string{"lux", "test", "local"}

	for _, p := range prefixes {
		for _, h := range hrps {
			out, err := p.Format(h, addr)
			if err != nil {
				t.Fatalf("%s/%s: %v", p, h, err)
			}
			// The output begins with "<prefix>-<hrp>1" — chain and HRP are
			// independently composed into the textual envelope.
			want := string(p) + "-" + h + "1"
			if len(out) < len(want) || out[:len(want)] != want {
				t.Errorf("ChainPrefix(%q).Format(%q, _) = %q; want prefix %q", p, h, out, want)
			}
		}
	}
}
