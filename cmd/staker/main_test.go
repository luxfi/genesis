// Copyright (C) 2019-2026, Lux Industries Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package main

import (
	"testing"

	luxbip39 "github.com/luxfi/go-bip39"
)

// A published test vector, never a real seed. Every expectation below is a
// function of THIS mnemonic; changing it changes every value here.
const mnemonic = "test test test test test test test test test test test junk"

func seed(t *testing.T) []byte {
	t.Helper()
	if !luxbip39.IsMnemonicValid(mnemonic) {
		t.Fatal("the test vector is not a valid mnemonic")
	}
	return luxbip39.NewSeed(mnemonic, "")
}

// The whole point of the tool: the same seed gives the same validator, so a set
// can be rebuilt rather than recovered.
func a_seed_names_the_same_validator_every_time(t *testing.T) {
	s := seed(t)
	first, err := derive(s, 36963, 0)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 4; i++ {
		again, err := derive(s, 36963, 0)
		if err != nil {
			t.Fatal(err)
		}
		if again.NodeID != first.NodeID {
			t.Fatalf("run %d named %s, not %s", i, again.NodeID, first.NodeID)
		}
		if again.SignerKey != first.SignerKey || again.MLDSAKey != first.MLDSAKey {
			t.Fatalf("run %d derived different keys", i)
		}
	}
}

func TestASeedNamesTheSameValidatorEveryTime(t *testing.T) {
	a_seed_names_the_same_validator_every_time(t)
}

// One seed serves every network, so the networks must not serve each other.
func TestOneSeedGivesEachNetworkItsOwnValidators(t *testing.T) {
	s := seed(t)
	seen := map[string]uint64{}
	for _, net := range []uint64{36963, 200200, 96369, 1, 2} {
		id, err := derive(s, net, 0)
		if err != nil {
			t.Fatal(err)
		}
		if other, ok := seen[id.NodeID]; ok {
			t.Fatalf("network %d and %d derive the same validator %s", other, net, id.NodeID)
		}
		seen[id.NodeID] = net
	}
}

// Index separates validators within one network; a fleet of five must be five
// validators, not one repeated.
func TestEachIndexIsItsOwnValidator(t *testing.T) {
	s := seed(t)
	seen := map[string]int{}
	for i := 0; i < 8; i++ {
		id, err := derive(s, 36963, i)
		if err != nil {
			t.Fatal(err)
		}
		if other, ok := seen[id.NodeID]; ok {
			t.Fatalf("index %d and %d derive the same validator", other, i)
		}
		seen[id.NodeID] = i
	}
}

// The TLS identity and the BLS identity come from the same seed and must never
// come from the same material.
func TestTheRolesDoNotShareMaterial(t *testing.T) {
	s := seed(t)
	tls := material(s, 36963, roleTLS, 0, 32)
	sig := material(s, 36963, roleBLS, 0, 32)
	pq := material(s, 36963, rolePQ, 0, 32)
	for _, pair := range [][2][]byte{{tls, sig}, {tls, pq}, {sig, pq}} {
		if string(pair[0]) == string(pair[1]) {
			t.Fatal("two roles derived the same material")
		}
	}
}

// A different mnemonic is a different estate. Nothing about the derivation may
// leak one seed's validators into another's.
func TestADifferentSeedIsADifferentValidator(t *testing.T) {
	const other = "legal winner thank year wave sausage worth useful legal winner thank yellow"
	a, err := derive(seed(t), 36963, 0)
	if err != nil {
		t.Fatal(err)
	}
	b, err := derive(luxbip39.NewSeed(other, ""), 36963, 0)
	if err != nil {
		t.Fatal(err)
	}
	if a.NodeID == b.NodeID {
		t.Fatal("two mnemonics named one validator")
	}
}

// The classical NodeID is NOT reproducible, and this records that as a measured
// fact rather than a suspicion: Go signs ECDSA with a hedged nonce, so the same
// key and the same template still produce a different certificate. A future Go
// that made this stable would fail here, which is the moment to revisit whether
// the classical path can be seed-backed after all.
func TestTheClassicalNameIsNotRebuildableFromTheSeed(t *testing.T) {
	s := seed(t)
	first, err := derive(s, 36963, 0)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 8; i++ {
		again, err := derive(s, 36963, 0)
		if err != nil {
			t.Fatal(err)
		}
		// The KEY is stable even though the certificate is not.
		if again.StakerKey != first.StakerKey {
			t.Fatal("the TLS key must be a function of the seed")
		}
		if again.NodeIDClassical != first.NodeIDClassical {
			return // varied, as documented
		}
	}
	t.Fatal("the classical NodeID was stable across 8 runs; the package doc and the PQ-only backup claim need revisiting")
}
