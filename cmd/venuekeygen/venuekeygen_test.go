// Copyright (C) 2026, Lux Industries Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package main

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/luxfi/crypto/bls/signer/localsigner"
	"github.com/luxfi/ids"
	"github.com/luxfi/node/staking"
)

// TestGenerateAllArtifactsConsumable generates a devnet venue identity and
// proves every artifact parses back through the node's own loaders.
func TestGenerateAllArtifactsConsumable(t *testing.T) {
	ks, err := generate()
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	// All five artifacts must be non-empty.
	if len(ks.stakerCertPEM) == 0 {
		t.Error("staker.crt (stakerCertPEM) is empty")
	}
	if len(ks.stakerKeyPEM) == 0 {
		t.Error("staker.key (stakerKeyPEM) is empty")
	}
	if len(ks.signerKeyRaw) == 0 {
		t.Error("signer.key (signerKeyRaw) is empty")
	}
	if len(ks.mldsaKeyPEM) == 0 {
		t.Error("mldsa.key (mldsaKeyPEM) is empty")
	}
	if len(ks.mldsaPubPEM) == 0 {
		t.Error("mldsa.pub (mldsaPubPEM) is empty")
	}

	// selfCheck performs every node-loader round-trip; if it passes, the
	// artifacts are luxd-consumable.
	if err := selfCheck(ks); err != nil {
		t.Fatalf("selfCheck: %v", err)
	}

	// Explicit loader round-trips (independent of selfCheck) for clarity.
	tlsCert, err := staking.LoadTLSCertFromBytes(ks.stakerKeyPEM, ks.stakerCertPEM)
	if err != nil {
		t.Fatalf("LoadTLSCertFromBytes: %v", err)
	}
	if tlsCert.Leaf == nil {
		t.Fatal("LoadTLSCertFromBytes: nil leaf")
	}
	if _, err := localsigner.FromBytes(ks.signerKeyRaw); err != nil {
		t.Fatalf("localsigner.FromBytes (raw file form): %v", err)
	}
	rawFromHex, err := hex.DecodeString(hex.EncodeToString(ks.signerKeyRaw))
	if err != nil {
		t.Fatalf("hex decode signer: %v", err)
	}
	if _, err := localsigner.FromBytes(rawFromHex); err != nil {
		t.Fatalf("localsigner.FromBytes (KMS hex form): %v", err)
	}
}

// TestPayloadUnmarshalsIntoKMSStakingKeys proves the stdout JSON document
// unmarshals into the node's KMSStakingKeys struct with all three fields
// populated, AND that the signer_key field is valid hex consumable by the
// node's KMS signer path.
func TestPayloadUnmarshalsIntoKMSStakingKeys(t *testing.T) {
	ks, err := generate()
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	doc := buildPayload(ks)
	body, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	// The exact node consumer: unmarshal into staking.KMSStakingKeys.
	var keys staking.KMSStakingKeys
	if err := json.Unmarshal(body, &keys); err != nil {
		t.Fatalf("unmarshal into KMSStakingKeys: %v", err)
	}
	if keys.TLSKey == "" {
		t.Error("KMSStakingKeys.TLSKey (json tls_key) is empty after unmarshal")
	}
	if keys.TLSCert == "" {
		t.Error("KMSStakingKeys.TLSCert (json tls_cert) is empty after unmarshal")
	}
	if keys.SignerKey == "" {
		t.Error("KMSStakingKeys.SignerKey (json signer_key) is empty after unmarshal")
	}

	// signer_key must decode as hex and parse via localsigner — the node's
	// KMS signer branch does exactly hex.DecodeString → localsigner.FromBytes.
	rawSigner, err := hex.DecodeString(keys.SignerKey)
	if err != nil {
		t.Fatalf("signer_key not valid hex: %v", err)
	}
	if _, err := localsigner.FromBytes(rawSigner); err != nil {
		t.Fatalf("signer_key hex not consumable by localsigner.FromBytes: %v", err)
	}

	// Round-trip the TLS PEMs straight out of the JSON too.
	if _, err := staking.LoadTLSCertFromBytes([]byte(keys.TLSKey), []byte(keys.TLSCert)); err != nil {
		t.Fatalf("TLS PEMs from JSON not loadable: %v", err)
	}

	// The ML-DSA fields and NodeID must also be present in the full payload.
	var full payload
	if err := json.Unmarshal(body, &full); err != nil {
		t.Fatalf("unmarshal into payload: %v", err)
	}
	if full.MLDSAKey == "" || full.MLDSAPub == "" {
		t.Error("payload mldsa_key / mldsa_pub empty after unmarshal")
	}
	if full.NodeID == "" {
		t.Error("payload node_id (strict-PQ) empty after unmarshal")
	}
	if full.NodeIDClassical == "" {
		t.Error("payload node_id_classical empty after unmarshal")
	}
	// node_id (strict-PQ) and node_id_classical MUST be different values.
	if full.NodeID == full.NodeIDClassical {
		t.Errorf("node_id == node_id_classical (%s); strict-PQ derivation not applied", full.NodeID)
	}
	// node_id MUST be the strict-PQ ML-DSA-65 NodeID derived from the generated
	// ML-DSA pubkey via the canonical node seam.
	wantPQ, _, err := ids.NodeIDSchemeMLDSA65.DeriveMLDSA(ids.Empty, ks.mldsaPubRaw)
	if err != nil {
		t.Fatalf("DeriveMLDSA: %v", err)
	}
	if full.NodeID != wantPQ.String() {
		t.Errorf("node_id = %s, want strict-PQ %s", full.NodeID, wantPQ.String())
	}
}

// TestNodeIDIsStrictPQAndStable asserts the venue's primary NodeID (ks.nodeID)
// is the strict-PQ ML-DSA-65 NodeID — non-empty, stable per ML-DSA pubkey, and
// DISTINCT from the legacy cert-derived NodeID. This is the value luxd computes
// under the PQ securityProfile (config/node DeriveNodeID strict-PQ branch).
func TestNodeIDIsStrictPQAndStable(t *testing.T) {
	ks, err := generate()
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if ks.nodeID == ids.EmptyNodeID {
		t.Fatal("strict-PQ NodeID is empty")
	}
	if ks.nodeIDClassical == ids.EmptyNodeID {
		t.Fatal("classical NodeID is empty")
	}

	// The strict-PQ NodeID and the classical cert-derived NodeID MUST differ —
	// they come from different keying material (ML-DSA pubkey vs TLS cert).
	if ks.nodeID == ks.nodeIDClassical {
		t.Fatalf("strict-PQ NodeID equals classical NodeID: %s", ks.nodeID)
	}

	// ks.nodeID MUST equal the canonical node-source derivation from the SAME
	// raw ML-DSA pubkey bytes: ids.NodeIDSchemeMLDSA65.DeriveMLDSA(ids.Empty, pub).
	wantPQ, _, err := ids.NodeIDSchemeMLDSA65.DeriveMLDSA(ids.Empty, ks.mldsaPubRaw)
	if err != nil {
		t.Fatalf("DeriveMLDSA: %v", err)
	}
	if ks.nodeID != wantPQ {
		t.Fatalf("ks.nodeID != DeriveMLDSA(ids.Empty,pub): %s != %s", ks.nodeID, wantPQ)
	}

	// Stability: re-deriving from the same pubkey yields the same NodeID.
	again, err := nodeIDStrictPQ(ks.mldsaPubRaw)
	if err != nil {
		t.Fatalf("re-derive strict-PQ NodeID: %v", err)
	}
	if again != ks.nodeID {
		t.Fatalf("strict-PQ NodeID not stable for same pub: %s != %s", again, ks.nodeID)
	}

	// A second, freshly generated identity must have a DIFFERENT NodeID
	// (fresh random ML-DSA keys per invocation).
	ks2, err := generate()
	if err != nil {
		t.Fatalf("second generate: %v", err)
	}
	if ks2.nodeID == ks.nodeID {
		t.Fatal("two fresh generations produced the same strict-PQ NodeID (keys not random)")
	}
}

// TestNodeIDDerivesFromMLDSAPubNotCert proves the strict-PQ NodeID is a pure
// function of the ML-DSA pubkey: a different ML-DSA pubkey changes node_id,
// while the TLS cert (which only drives node_id_classical) has NO effect on it.
func TestNodeIDDerivesFromMLDSAPubNotCert(t *testing.T) {
	// Two independent ML-DSA pubkeys -> two different strict-PQ NodeIDs.
	kpA, err := staking.NewPQKeyPair()
	if err != nil {
		t.Fatalf("new ML-DSA pair A: %v", err)
	}
	kpB, err := staking.NewPQKeyPair()
	if err != nil {
		t.Fatalf("new ML-DSA pair B: %v", err)
	}
	pubA, pubB := kpA.PublicKeyBytes(), kpB.PublicKeyBytes()

	nidA, err := nodeIDStrictPQ(pubA)
	if err != nil {
		t.Fatalf("derive A: %v", err)
	}
	nidB, err := nodeIDStrictPQ(pubB)
	if err != nil {
		t.Fatalf("derive B: %v", err)
	}
	if nidA == nidB {
		t.Fatal("different ML-DSA pubkeys produced the same strict-PQ NodeID")
	}

	// Same pubkey -> same NodeID regardless of any cert: the derivation never
	// reads cert material. Re-deriving from pubA twice is stable.
	nidA2, err := nodeIDStrictPQ(pubA)
	if err != nil {
		t.Fatalf("re-derive A: %v", err)
	}
	if nidA2 != nidA {
		t.Fatalf("strict-PQ NodeID not stable for pubA: %s != %s", nidA2, nidA)
	}

	// Cross-check against the canonical node-source seam for pubA.
	wantA, _, err := ids.NodeIDSchemeMLDSA65.DeriveMLDSA(ids.Empty, pubA)
	if err != nil {
		t.Fatalf("DeriveMLDSA A: %v", err)
	}
	if nidA != wantA {
		t.Fatalf("nodeIDStrictPQ(pubA) != DeriveMLDSA(ids.Empty,pubA): %s != %s", nidA, wantA)
	}
}

// TestWriteArtifactsToOutDir writes the five artifacts to a temp --out dir,
// asserts they are non-empty, and re-loads each via the node loaders from
// disk (the exact file forms luxd consumes).
func TestWriteArtifactsToOutDir(t *testing.T) {
	ks, err := generate()
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	dir := t.TempDir()
	if err := writeArtifacts(dir, ks); err != nil {
		t.Fatalf("writeArtifacts: %v", err)
	}

	for _, name := range venueArtifacts {
		p := filepath.Join(dir, name)
		info, err := os.Stat(p)
		if err != nil {
			t.Fatalf("stat %s: %v", p, err)
		}
		if info.Size() == 0 {
			t.Errorf("artifact %s is empty on disk", name)
		}
		// 0600 perms (secret material).
		if perm := info.Mode().Perm(); perm != 0o600 {
			t.Errorf("artifact %s perms = %#o, want 0600", name, perm)
		}
	}

	// TLS from files (the node's file loader).
	if _, err := staking.LoadTLSCertFromFiles(
		filepath.Join(dir, "staker.key"),
		filepath.Join(dir, "staker.crt"),
	); err != nil {
		t.Fatalf("LoadTLSCertFromFiles: %v", err)
	}

	// signer.key on disk is RAW bytes — parse via localsigner.
	rawSigner, err := os.ReadFile(filepath.Join(dir, "signer.key"))
	if err != nil {
		t.Fatalf("read signer.key: %v", err)
	}
	if _, err := localsigner.FromBytes(rawSigner); err != nil {
		t.Fatalf("signer.key (raw on disk) not consumable: %v", err)
	}

	// ML-DSA from files via the node loader.
	if _, err := staking.LoadPQKeyPair(
		filepath.Join(dir, "mldsa.key"),
		filepath.Join(dir, "mldsa.pub"),
	); err != nil {
		t.Fatalf("LoadPQKeyPair: %v", err)
	}
}

// TestNormalizeNet covers the --net validation.
func TestNormalizeNet(t *testing.T) {
	for _, ok := range []string{"mainnet", "testnet", "devnet", "localnet"} {
		if got, err := normalizeNet(ok); err != nil || got != ok {
			t.Errorf("normalizeNet(%q) = (%q, %v), want (%q, nil)", ok, got, err, ok)
		}
	}
	for _, bad := range []string{"", "mainet", "prod", "local"} {
		if _, err := normalizeNet(bad); err == nil {
			t.Errorf("normalizeNet(%q) = nil error, want error", bad)
		}
	}
}
