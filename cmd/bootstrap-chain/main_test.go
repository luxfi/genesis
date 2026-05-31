// Unit tests for bootstrap-chain's defensive alloc key normalization.
//
// The whole point of normalizeAllocKey is to keep the load-and-validate
// path single-sourced: every consumer that touches an alloc key (the
// in-memory repair in main, future validation tooling, future on-disk
// rewriters) must route through this function. These tests pin the
// contract — a regression here would let a malformed key slip past the
// pre-flight check and burn LUX on a CreateChainTx that the node will
// later reject at genesis-load time.

package main

import "testing"

func TestNormalizeAllocKey_AlreadyCanonical(t *testing.T) {
	canonical, repaired, valid := normalizeAllocKey("0x1111111111111111111111111111111111111111")
	if !valid {
		t.Fatalf("canonical 0x... key must validate; got !valid")
	}
	if repaired {
		t.Error("canonical key must not flag repaired=true")
	}
	if canonical != "0x1111111111111111111111111111111111111111" {
		t.Errorf("canonical roundtrip mismatch: %q", canonical)
	}
}

func TestNormalizeAllocKey_AddsMissingPrefix(t *testing.T) {
	canonical, repaired, valid := normalizeAllocKey("1111111111111111111111111111111111111111")
	if !valid {
		t.Fatal("missing-prefix key must validate after repair")
	}
	if !repaired {
		t.Error("missing-prefix repair must flag repaired=true")
	}
	if canonical != "0x1111111111111111111111111111111111111111" {
		t.Errorf("expected 0x-prefixed form, got %q", canonical)
	}
}

func TestNormalizeAllocKey_NormalizesCapitalPrefix(t *testing.T) {
	// `0X` is non-standard for EVM address strings. We lowercase the
	// PREFIX while preserving body case (in case the caller is using
	// EIP-55 checksum casing on the address itself).
	canonical, repaired, valid := normalizeAllocKey("0XaBcDeF0123456789aBcDeF0123456789aBcDeF01")
	if !valid {
		t.Fatal("0X-prefixed key with valid body must validate")
	}
	if !repaired {
		t.Error("0X repair must flag repaired=true")
	}
	if canonical != "0xaBcDeF0123456789aBcDeF0123456789aBcDeF01" {
		t.Errorf("prefix should normalize to 0x while preserving body case; got %q", canonical)
	}
}

func TestNormalizeAllocKey_PreservesBodyCase(t *testing.T) {
	// EIP-55 checksum addresses encode the checksum in upper/lower case
	// of the hex digits. Stripping the case would destroy the checksum,
	// so we must preserve body case verbatim.
	canonical, _, valid := normalizeAllocKey("0xaBcDeF0123456789AbCdEf0123456789AbCdEf01")
	if !valid {
		t.Fatal("mixed-case body must validate")
	}
	if canonical != "0xaBcDeF0123456789AbCdEf0123456789AbCdEf01" {
		t.Errorf("body case must be preserved verbatim; got %q", canonical)
	}
}

func TestNormalizeAllocKey_RejectsShortBody(t *testing.T) {
	// 39 hex chars after 0x — one short of a valid address. Must fail.
	cases := []string{
		"0x111111111111111111111111111111111111111",  // 39
		"0x11111111111111111111111111111111111111",   // 38
		"0x",                                          // empty body
	}
	for _, k := range cases {
		_, _, valid := normalizeAllocKey(k)
		if valid {
			t.Errorf("short-body key %q must fail validation", k)
		}
	}
}

func TestNormalizeAllocKey_RejectsLongBody(t *testing.T) {
	// 41 hex chars — one over. Must fail.
	_, _, valid := normalizeAllocKey("0x11111111111111111111111111111111111111111")
	if valid {
		t.Error("long-body key must fail validation")
	}
}

func TestNormalizeAllocKey_RejectsNonHex(t *testing.T) {
	cases := []string{
		// `g` is not a hex char
		"0x111111111111111111111111111111111111111g",
		// Underscore
		"0x_aBcDeF0123456789aBcDeF0123456789aBcDeF0",
		// Spaces
		"0x 111111111111111111111111111111111111111",
	}
	for _, k := range cases {
		_, _, valid := normalizeAllocKey(k)
		if valid {
			t.Errorf("non-hex key %q must fail validation", k)
		}
	}
}

func TestNormalizeAllocKey_EmptyString(t *testing.T) {
	_, _, valid := normalizeAllocKey("")
	if valid {
		t.Error("empty string must fail validation")
	}
}

func TestNormalizeAllocKey_IsIdempotent(t *testing.T) {
	// Calling normalize twice in a row must produce the same result —
	// every consumer should be safe to defensive-call.
	first, _, valid := normalizeAllocKey("aBcDeF0123456789aBcDeF0123456789aBcDeF01")
	if !valid {
		t.Fatal("first call must validate")
	}
	second, repairedSecond, validSecond := normalizeAllocKey(first)
	if !validSecond {
		t.Error("second call must also validate (idempotent)")
	}
	if repairedSecond {
		t.Error("second call must report repaired=false; the input was already canonical")
	}
	if second != first {
		t.Errorf("second-call output differs from first: %q vs %q", second, first)
	}
}
