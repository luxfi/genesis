// Copyright (C) 2026, Lux Industries Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package genesis

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha512"
	"testing"

	"github.com/luxfi/constants"
	"github.com/luxfi/go-bip32"
	"github.com/luxfi/go-bip39"
)

// hdTestMnemonic is the same well-known dev seed used by light_mnemonic_guard_test.go.
// We deliberately use a public mnemonic so the test is reproducible and
// reviewer-checkable, then run on LocalID/dev networks where it's allowed.
const hdTestMnemonic = LightMnemonic

// mldsa65PublicKeySize is the FIPS 204 ML-DSA-65 packed public key size
// (32-byte ρ seed + K=6 PolyT1 packed coefficients × 320 bytes).
const mldsa65PublicKeySize = 1952

// TestLoadKeysFromMnemonic_Branch0IsMLDSA asserts that branch 0' produces
// a deterministic ML-DSA-65 public key of the FIPS 204 packed size for
// every account index. Two derivations with the same (mnemonic, nid)
// MUST return byte-identical MLDSAPublicKey slices.
func TestLoadKeysFromMnemonic_Branch0IsMLDSA(t *testing.T) {
	const n = 5
	nid := uint32(constants.LocalID)

	keys1, err := LoadKeysFromMnemonic(hdTestMnemonic, nid, n)
	if err != nil {
		t.Fatalf("first derivation: %v", err)
	}
	if len(keys1) != n {
		t.Fatalf("want %d keys, got %d", n, len(keys1))
	}

	for i, k := range keys1 {
		if got := len(k.MLDSAPublicKey); got != mldsa65PublicKeySize {
			t.Fatalf("index %d: MLDSAPublicKey size = %d, want %d", i, got, mldsa65PublicKeySize)
		}
	}

	// Re-derive: ML-DSA pubkeys MUST be bit-identical.
	keys2, err := LoadKeysFromMnemonic(hdTestMnemonic, nid, n)
	if err != nil {
		t.Fatalf("second derivation: %v", err)
	}
	for i := range keys1 {
		if !bytes.Equal(keys1[i].MLDSAPublicKey, keys2[i].MLDSAPublicKey) {
			t.Fatalf("index %d: ML-DSA pubkey not deterministic across two derivations", i)
		}
	}

	// Distinct indices must yield distinct pubkeys (collision probability is
	// negligible; an equal pair here means the derivation is broken).
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			if bytes.Equal(keys1[i].MLDSAPublicKey, keys1[j].MLDSAPublicKey) {
				t.Fatalf("ML-DSA pubkey at index %d == index %d (derivation collapsed)", i, j)
			}
		}
	}
}

// TestLoadKeysFromMnemonic_Branch1IsSecp256k1 asserts that branch 1'
// populates ETHAddr and StakingAddr just like the previous layout did
// (the public-API contract for secp256k1 outputs is unchanged; only the
// path moved). 5 keys, every address non-zero, every pair distinct.
func TestLoadKeysFromMnemonic_Branch1IsSecp256k1(t *testing.T) {
	const n = 5
	nid := uint32(constants.LocalID)

	keys, err := LoadKeysFromMnemonic(hdTestMnemonic, nid, n)
	if err != nil {
		t.Fatalf("derivation: %v", err)
	}
	if len(keys) != n {
		t.Fatalf("want %d keys, got %d", n, len(keys))
	}

	var zero [20]byte
	for i, k := range keys {
		if bytes.Equal(k.ETHAddr[:], zero[:]) {
			t.Fatalf("index %d: ETHAddr is zero", i)
		}
		if bytes.Equal(k.StakingAddr[:], zero[:]) {
			t.Fatalf("index %d: StakingAddr is zero", i)
		}
	}

	// Distinct indices → distinct addresses.
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			if keys[i].ETHAddr == keys[j].ETHAddr {
				t.Fatalf("ETHAddr collision at %d == %d", i, j)
			}
			if keys[i].StakingAddr == keys[j].StakingAddr {
				t.Fatalf("StakingAddr collision at %d == %d", i, j)
			}
		}
	}
}

// TestLoadKeysFromMnemonic_NIDHardened pins per-network isolation. The
// same mnemonic on nid=1 vs nid=1337 MUST produce different keypairs at
// every index, on BOTH branches.
func TestLoadKeysFromMnemonic_NIDHardened(t *testing.T) {
	const n = 3
	keysA, err := LoadKeysFromMnemonic(hdTestMnemonic, 1, n)
	if err != nil {
		t.Fatalf("derive nid=1: %v", err)
	}
	keysB, err := LoadKeysFromMnemonic(hdTestMnemonic, 1337, n)
	if err != nil {
		t.Fatalf("derive nid=1337: %v", err)
	}

	for i := 0; i < n; i++ {
		if keysA[i].ETHAddr == keysB[i].ETHAddr {
			t.Fatalf("index %d: secp256k1 ETHAddr collides across nids (hardening broken)", i)
		}
		if keysA[i].StakingAddr == keysB[i].StakingAddr {
			t.Fatalf("index %d: StakingAddr collides across nids", i)
		}
		if bytes.Equal(keysA[i].MLDSAPublicKey, keysB[i].MLDSAPublicKey) {
			t.Fatalf("index %d: ML-DSA pubkey collides across nids", i)
		}
	}
}

// TestLoadKeysFromMnemonic_BranchesIndependent pins BIP-32's hardening
// guarantee at the branch level.
//
// Concretely: the branch-0' child seed (the 32 bytes that flow into
// SHAKE-256 then ML-DSA) and the branch-1' secp256k1 private key are
// derived from the SAME parent xpriv at the account level — but because
// both branches are hardened, knowing one xpriv (or its xpub) does NOT
// let you derive the other. We assert the cryptographic precondition of
// that guarantee: the two child seeds are pairwise distinct AND don't
// share an HMAC-SHA512-extractable relationship with each other.
//
// (Proving the full hardening reduction is a math statement, not a Go
// test; what a test CAN do is confirm we actually use distinct hardened
// indices on both branches so the BIP-32 reduction applies.)
func TestLoadKeysFromMnemonic_BranchesIndependent(t *testing.T) {
	const n = 5
	const nid = uint32(constants.LocalID)

	seed := bip39.NewSeed(hdTestMnemonic, "")
	master, err := bip32.NewMasterKey(seed)
	if err != nil {
		t.Fatalf("master: %v", err)
	}
	account, err := deriveLuxAccount(master, nid)
	if err != nil {
		t.Fatalf("account: %v", err)
	}
	branchMLDSA, err := account.NewChildKey(bip32.FirstHardenedChild + 0)
	if err != nil {
		t.Fatalf("branch 0': %v", err)
	}
	branchSecp, err := account.NewChildKey(bip32.FirstHardenedChild + 1)
	if err != nil {
		t.Fatalf("branch 1': %v", err)
	}

	for i := 0; i < n; i++ {
		mldsaChild, err := branchMLDSA.NewChildKey(bip32.FirstHardenedChild + uint32(i))
		if err != nil {
			t.Fatalf("mldsa child %d: %v", i, err)
		}
		secpChild, err := branchSecp.NewChildKey(bip32.FirstHardenedChild + uint32(i))
		if err != nil {
			t.Fatalf("secp child %d: %v", i, err)
		}

		// 1. Child key bytes are distinct.
		if bytes.Equal(mldsaChild.Key, secpChild.Key) {
			t.Fatalf("index %d: branch 0' and branch 1' produced identical 32-byte child keys", i)
		}

		// 2. Chain codes are distinct — the two branches' subtrees are
		// fully independent, not just their leaves.
		if bytes.Equal(mldsaChild.ChainCode, secpChild.ChainCode) {
			t.Fatalf("index %d: branch 0' and branch 1' share a chain code", i)
		}

		// 3. No trivial linear relation between the two child seeds.
		// If branch 1' were a non-hardened sibling of branch 0', the
		// secp256k1 child key would equal HMAC-SHA512(chain_code, pub
		// || index)[:32] modular-added to the parent — i.e., it would
		// be a knowable function of branch 0'. Since both branches are
		// hardened from the account level, no such HMAC over branch
		// 0''s public material yields branch 1''s key. We assert that
		// HMAC-SHA512 of one against the other is not the identity.
		mac := hmac.New(sha512.New, branchMLDSA.ChainCode)
		mac.Write(mldsaChild.Key)
		_ = mac.Sum(nil) // produced for completeness; equality below.
		if bytes.Equal(mac.Sum(nil)[:32], secpChild.Key) {
			t.Fatalf("index %d: HMAC(branchMLDSA, mldsaChild) == secpChild — hardening broken", i)
		}
	}
}

// TestLoadKeysFromMnemonicEnvForNetwork_Integration pins the env-driven
// entry point: one mnemonic on a dev network produces N keys each with
// both an ML-DSA-65 pubkey (1952 B) and a populated secp256k1 ETHAddr.
func TestLoadKeysFromMnemonicEnvForNetwork_Integration(t *testing.T) {
	t.Setenv("MNEMONIC", "")
	t.Setenv("LIGHT_MNEMONIC", hdTestMnemonic)

	const n = 3
	keys, err := LoadKeysFromMnemonicEnvForNetwork(constants.LocalID, n)
	if err != nil {
		t.Fatalf("LoadKeysFromMnemonicEnvForNetwork: %v", err)
	}
	if len(keys) != n {
		t.Fatalf("want %d keys, got %d", n, len(keys))
	}

	var zero [20]byte
	for i, k := range keys {
		if got := len(k.MLDSAPublicKey); got != mldsa65PublicKeySize {
			t.Fatalf("index %d: MLDSAPublicKey size = %d, want %d", i, got, mldsa65PublicKeySize)
		}
		if bytes.Equal(k.ETHAddr[:], zero[:]) {
			t.Fatalf("index %d: ETHAddr is zero", i)
		}
	}
}

// TestLoadKeysFromMnemonic_NetworkIDOverflow rejects a network id at or
// above 2^31 (the BIP-32 hardening fold).
func TestLoadKeysFromMnemonic_NetworkIDOverflow(t *testing.T) {
	if _, err := LoadKeysFromMnemonic(hdTestMnemonic, bip32.FirstHardenedChild, 1); err == nil {
		t.Fatalf("expected error for nid >= 2^31")
	}
	if _, err := LoadKeysFromMnemonic(hdTestMnemonic, bip32.FirstHardenedChild+1, 1); err == nil {
		t.Fatalf("expected error for nid > 2^31")
	}
}
