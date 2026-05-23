// Copyright (C) 2019-2025, Lux Partners Limited. All rights reserved.
// See the file LICENSE for licensing terms.

package genesis

import (
	"encoding/hex"
	"testing"

	"github.com/luxfi/address"
	"github.com/luxfi/ids"
)

func TestChainPrefixFormat_KnownVectors(t *testing.T) {
	// Known test addresses from mainnet genesis
	testCases := []struct {
		name        string
		addrHex  string
		chainPrefix string
		hrp         string
		expected    string
	}{
		{
			name:        "node1_mainnet",
			addrHex:  "9011E888251AB053B7bD1cdB598Db4f9DEd94714",
			chainPrefix: "P",
			hrp:         "lux",
			expected:    "P-lux1jqg73zp9r2c98daarnd4nrd5l80dj3c5eha5fl",
		},
		{
			name:        "node2_mainnet",
			addrHex:  "EAbCC110fAcBfebabC66Ad6f9E7B67288e720B59",
			chainPrefix: "P",
			hrp:         "lux",
			expected:    "P-lux1a27vzy86e0lt40rx44heu7m89z88yz6ey7av5e",
		},
		{
			name:        "node3_mainnet",
			addrHex:  "8d5081153aE1cfb41f5c932fe0b6Beb7E159cF84",
			chainPrefix: "P",
			hrp:         "lux",
			expected:    "P-lux134ggz9f6u88mg86ujvh7pd47kls4nnuy3hx4yp",
		},
		{
			name:        "node4_mainnet",
			addrHex:  "f8f12D0592e6d1bFe92ee16CaBCC4a6F26dAAe23",
			chainPrefix: "P",
			hrp:         "lux",
			expected:    "P-lux1lrcj6pvjumgml6fwu9k2hnz2dund4t3rpsjuxu",
		},
		{
			name:        "node5_mainnet",
			addrHex:  "Fb66808f708e1d4D7D43a8c75596e84f94e06806",
			chainPrefix: "P",
			hrp:         "lux",
			expected:    "P-lux1ldngprms3cw56l2r4rr4t9hgf72wq6qx057vd2",
		},
		{
			name:        "x_chain_testnet",
			addrHex:  "9011E888251AB053B7bD1cdB598Db4f9DEd94714",
			chainPrefix: "X",
			hrp:         "test",
			expected:    "X-test1jqg73zp9r2c98daarnd4nrd5l80dj3c5644e09",
		},
		{
			name:        "local_dev",
			addrHex:  "9011E888251AB053B7bD1cdB598Db4f9DEd94714",
			chainPrefix: "P",
			hrp:         "local",
			expected:    "P-local1jqg73zp9r2c98daarnd4nrd5l80dj3c56acgey",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			addrBytes, err := hex.DecodeString(tc.addrHex)
			if err != nil {
				t.Fatalf("failed to decode eth address: %v", err)
			}

			var shortID ids.ShortID
			copy(shortID[:], addrBytes)

			// Test ChainPrefix.Format
			result, err := ChainPrefix(tc.chainPrefix).Format(tc.hrp, shortID[:])
			if err != nil {
				t.Fatalf("ChainPrefix.Format: %v", err)
			}

			if result != tc.expected {
				t.Errorf("ChainPrefix.Format = %s, want %s", result, tc.expected)
			}

			// Verify the address can be parsed back
			chainID, hrp, parsedAddr, err := address.Parse(result)
			if err != nil {
				t.Errorf("failed to parse generated address: %v", err)
			}
			if chainID != tc.chainPrefix {
				t.Errorf("parsed chain ID = %s, want %s", chainID, tc.chainPrefix)
			}
			if hrp != tc.hrp {
				t.Errorf("parsed hrp = %s, want %s", hrp, tc.hrp)
			}
			if !equalBytes(parsedAddr, addrBytes) {
				t.Errorf("parsed address doesn't match: got %x, want %x", parsedAddr, addrBytes)
			}
		})
	}
}

func TestInvalidAddressChecksum(t *testing.T) {
	// Test that the broken addresses from the old genesis are rejected
	invalidAddresses := []string{
		"P-lux18jma8ppw3nhx5r4ap8clazz0dps7rv5u00z96u", // Invalid checksum
	}

	for _, addr := range invalidAddresses {
		_, _, _, err := address.Parse(addr)
		if err == nil {
			t.Errorf("expected error parsing invalid address %s, got nil", addr)
		}
	}
}

func TestBech32ChecksumConsistency(t *testing.T) {
	// Verify that our implementation produces the same result as the node's address package
	testAddrs := []string{
		"9011E888251AB053B7bD1cdB598Db4f9DEd94714",
		"EAbCC110fAcBfebabC66Ad6f9E7B67288e720B59",
		"8d5081153aE1cfb41f5c932fe0b6Beb7E159cF84",
		"f8f12D0592e6d1bFe92ee16CaBCC4a6F26dAAe23",
		"Fb66808f708e1d4D7D43a8c75596e84f94e06806",
	}

	for _, addr := range testAddrs {
		addrBytes, err := hex.DecodeString(addr)
		if err != nil {
			t.Fatalf("failed to decode eth address: %v", err)
		}

		var shortID ids.ShortID
		copy(shortID[:], addrBytes)

		// Our implementation
		ourAddr, err := PChainPrefix.Format("lux", shortID[:])
		if err != nil {
			t.Fatalf("PChainPrefix.Format: %v", err)
		}

		// Node's implementation
		nodeAddr, err := address.Format("P", "lux", addrBytes)
		if err != nil {
			t.Fatalf("node's Format failed: %v", err)
		}

		if ourAddr != nodeAddr {
			t.Errorf("address mismatch for %s:\n  ours: %s\n  node: %s", addr, ourAddr, nodeAddr)
		}
	}
}

func equalBytes(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
