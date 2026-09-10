// Copyright (C) 2019-2026, Lux Industries Inc. All rights reserved.
// See the file LICENSE for licensing terms.

// Command staker derives a network's validator staking identities from the
// mnemonic, so a validator set can be rebuilt from one seed instead of
// recovered from eight scattered key files.
//
//	staker --network 36963 --count 5              the identities, as JSON
//	staker --network 36963 --count 5 --stakers    the genesis initialStakers array
//	staker --network 36963 --count 5 --out DIR    staker-i.crt/.key, signer-i.key
//
// WHY THIS EXISTS. Staking identities used to be minted with crypto/rand, one
// invocation per validator, and kept only as key files. That makes the set
// unrecoverable the moment the files are: a genesis names NodeIDs, and a NodeID
// with no private key behind it is a validator nobody can run. Hanzo mainnet was
// exactly that — five names in a genesis whose keys were on a cluster that is
// gone, and no seed to rebuild them from.
//
// Derived from the mnemonic, the set is a function of the seed and the network,
// so it can be recomputed on any machine that can read the seed and nothing has
// to survive except the seed.
//
// THE DERIVATION. HKDF-SHA256 over the BIP-39 seed, domain-separated by network
// and role:
//
//	material(role, net, i) = HKDF(seed, salt "lux/staking/v1", info "<net>/<role>/<i>")
//
// The network is part of the info string because a primary networkID equals its
// EVM chainID and is unique per L1 per environment; that is what keeps one
// seed's Hanzo validators disjoint from its Zoo validators. The role separates
// the TLS identity from the BLS one, so neither can ever be the other. Nothing
// here shares material with the account path m/44'/9000'/0'/0/<i> that funds the
// genesis alloc, so a validator key is never also a funded account.
//
// WHICH NodeID IS REBUILDABLE, AND WHICH IS NOT. A post-quantum profile derives
// the NodeID from the ML-DSA public key, and FIPS 204 key generation consumes a
// fixed seed, so that NodeID is a function of the mnemonic and nothing else.
//
// A classical profile derives it from the TLS CERTIFICATE, and that is not
// reproducible from a seed. staking.NewCertAndKeyBytesFromKey pins the serial and
// the validity window and seeds its randomness from the key, but Go signs ECDSA
// with a hedged nonce and randutil.MaybeReadByte reads a byte from the entropy
// source only some of the time — deliberately, to stop anyone depending on the
// bytes. Measured: same key, same template, different signature, different DER,
// different NodeID on every run.
//
// So a classical validator set cannot be rebuilt from the mnemonic. Its keys can
// be, but its NAMES cannot, and a genesis names NodeIDs. The classical value is
// reported here for the profile that still uses it; the mnemonic is a complete
// backup only on the PQ path.
//
// The BLS key goes through localsigner.FromSeed, the node's own seed-to-key
// derivation.
//
// This tool rolls no crypto. The scalar reduction is the only arithmetic here,
// and it is the standard wide-reduction: 48 bytes into a 256-bit field leaves a
// bias below 2^-128.
package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"flag"
	"fmt"
	"io"
	"math/big"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/hkdf"

	"bytes"

	"github.com/luxfi/crypto/bls"
	"github.com/luxfi/crypto/bls/signer/localsigner"
	"github.com/luxfi/crypto/mldsa"
	luxbip39 "github.com/luxfi/go-bip39"
	"github.com/luxfi/ids"
	"github.com/luxfi/node/staking"
)

// salt fixes this derivation for all time. Changing it changes every NodeID
// every network would derive, which is a new validator set, not a new version.
const salt = "lux/staking/v1"

// The roles one seed derives. Two, and they must never collide: a validator
// proves its name with the TLS key and its stake with the BLS one.
const (
	roleTLS = "tls"
	roleBLS = "bls"
	rolePQ  = "pq"
)

// A derived identity, in the shapes its three consumers read: the node reads
// the files, a genesis reads the staker entry, KMS reads the JSON.
type identity struct {
	Index int `json:"index"`
	// The NodeID a validator on a post-quantum profile answers to, derived from
	// the ML-DSA public key. This one is a function of the seed.
	NodeID string `json:"node_id"`
	// The NodeID a classical-profile node answers to, derived from the TLS
	// CERTIFICATE. Reported for the profile that still uses it, and it is NOT a
	// function of the seed alone — see the note on this type in the package doc.
	NodeIDClassical string `json:"node_id_classical"`
	StakerCert      string `json:"staker_crt"`
	StakerKey       string `json:"staker_key"`
	SignerKey       string `json:"signer_key_hex"`
	PublicKey       string `json:"bls_public_key"`
	ProofOfPos      string `json:"bls_proof_of_possession"`
	MLDSAKey        string `json:"mldsa_key_hex"`
	MLDSAPub        string `json:"mldsa_pub_hex"`
}

// The genesis form. Field names are the genesis document's, not ours.
type staker struct {
	NodeID        string `json:"nodeID"`
	RewardAddress string `json:"rewardAddress"`
	DelegationFee int    `json:"delegationFee"`
	Signer        signer `json:"signer"`
}

type signer struct {
	PublicKey         string `json:"publicKey"`
	ProofOfPossession string `json:"proofOfPossession"`
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "staker: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	var (
		network = flag.Uint64("network", 0, "the primary networkID these validators serve (equals the EVM chainID)")
		count   = flag.Int("count", 0, "how many identities to derive")
		base    = flag.Int("index-base", 0, "the index the first identity derives at")
		out     = flag.String("out", "", "write staker-i.crt/.key and signer-i.key under this directory")
		reward  = flag.String("reward-address", "", "the P-chain address genesis stakers are rewarded to")
		fee     = flag.Int("delegation-fee", 20000, "the delegation fee genesis stakers carry")
		stakers = flag.Bool("stakers", false, "print the genesis initialStakers array instead of the identities")
		mnFile  = flag.String("mnemonic-file", "", "read the BIP-39 mnemonic from this file instead of LUX_MNEMONIC")
	)
	flag.Parse()

	if *network == 0 {
		return errors.New("--network names the primary networkID these validators serve")
	}
	if *count <= 0 {
		return errors.New("--count is how many identities to derive")
	}
	if *stakers && *reward == "" {
		return errors.New("--stakers needs --reward-address: a genesis staker is paid somewhere")
	}

	seed, err := loadSeed(*mnFile)
	if err != nil {
		return err
	}

	set := make([]identity, 0, *count)
	for i := *base; i < *base+*count; i++ {
		id, err := derive(seed, *network, i)
		if err != nil {
			return fmt.Errorf("index %d: %w", i, err)
		}
		set = append(set, *id)
	}

	if *out != "" {
		if err := write(*out, set); err != nil {
			return err
		}
	}
	if *stakers {
		return emit(genesisStakers(set, *reward, *fee))
	}
	return emit(set)
}

// derive produces the identity this seed gives network `net` at `index`.
func derive(seed []byte, net uint64, index int) (*identity, error) {
	tlsKey, err := p256(material(seed, net, roleTLS, index, 48))
	if err != nil {
		return nil, err
	}
	certPEM, keyPEM, err := staking.NewCertAndKeyBytesFromKey(tlsKey)
	if err != nil {
		return nil, fmt.Errorf("staking cert: %w", err)
	}
	cert, err := staking.ParseCertificate(pemBody(certPEM))
	if err != nil {
		return nil, fmt.Errorf("parse own cert: %w", err)
	}

	sk, err := localsigner.FromSeed(material(seed, net, roleBLS, index, 32))
	if err != nil {
		return nil, fmt.Errorf("bls key: %w", err)
	}
	pub := bls.PublicKeyToCompressedBytes(sk.PublicKey())
	pop, err := sk.SignProofOfPossession(pub)
	if err != nil {
		return nil, fmt.Errorf("proof of possession: %w", err)
	}

	// FIPS 204 key generation consumes a fixed-size seed, so a reader over
	// derived material makes the key — and therefore the NodeID — a function of
	// the mnemonic. This is the identity that survives losing every file.
	pq, err := mldsa.GenerateKey(bytes.NewReader(material(seed, net, rolePQ, index, 64)), mldsa.MLDSA65)
	if err != nil {
		return nil, fmt.Errorf("ml-dsa key: %w", err)
	}
	pqPub := pq.PublicKey.Bytes()
	nodeID, _, err := ids.NodeIDSchemeMLDSA65.DeriveMLDSA(ids.Empty, pqPub)
	if err != nil {
		return nil, fmt.Errorf("derive node id: %w", err)
	}

	return &identity{
		Index:           index,
		NodeID:          nodeID.String(),
		NodeIDClassical: ids.NodeIDFromCert(cert).String(),
		StakerCert:      string(certPEM),
		StakerKey:       string(keyPEM),
		SignerKey:       hex.EncodeToString(sk.ToBytes()),
		PublicKey:       "0x" + hex.EncodeToString(pub),
		ProofOfPos:      "0x" + hex.EncodeToString(bls.SignatureToBytes(pop)),
		MLDSAKey:        hex.EncodeToString(pq.Bytes()),
		MLDSAPub:        hex.EncodeToString(pqPub),
	}, nil
}

// material is the derivation. Everything that makes one identity differ from
// another is in the info string, and nothing else reads the seed.
func material(seed []byte, net uint64, role string, index, n int) []byte {
	info := fmt.Sprintf("%d/%s/%d", net, role, index)
	out := make([]byte, n)
	if _, err := io.ReadFull(hkdf.New(sha256.New, seed, []byte(salt), []byte(info)), out); err != nil {
		panic(fmt.Sprintf("hkdf over a fixed-size read: %v", err)) // cannot fail
	}
	return out
}

// p256 turns derived bytes into a P-256 private key. Wide reduction: the input
// is 48 bytes against a 256-bit order, so the bias toward small scalars is
// below 2^-128, and the +1 keeps zero out of the field.
func p256(b []byte) (*ecdsa.PrivateKey, error) {
	curve := elliptic.P256()
	nMinus1 := new(big.Int).Sub(curve.Params().N, big.NewInt(1))
	d := new(big.Int).Mod(new(big.Int).SetBytes(b), nMinus1)
	d.Add(d, big.NewInt(1))

	key := &ecdsa.PrivateKey{D: d}
	key.PublicKey.Curve = curve
	key.PublicKey.X, key.PublicKey.Y = curve.ScalarBaseMult(d.Bytes())
	if key.PublicKey.X == nil {
		return nil, errors.New("derived scalar is not on the curve")
	}
	return key, nil
}

func genesisStakers(ids []identity, reward string, fee int) []staker {
	out := make([]staker, 0, len(ids))
	for _, id := range ids {
		out = append(out, staker{
			NodeID:        id.NodeID,
			RewardAddress: reward,
			DelegationFee: fee,
			Signer:        signer{PublicKey: id.PublicKey, ProofOfPossession: id.ProofOfPos},
		})
	}
	return out
}

// write lays the identities out the way the node reads them: the signer key as
// RAW bytes, because luxd reads that file as bytes and rejects the hex.
func write(dir string, ids []identity) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	for _, id := range ids {
		raw, err := hex.DecodeString(id.SignerKey)
		if err != nil {
			return err
		}
		for _, f := range []struct {
			name string
			body []byte
			mode os.FileMode
		}{
			{fmt.Sprintf("staker-%d.crt", id.Index), []byte(id.StakerCert), 0o644},
			{fmt.Sprintf("staker-%d.key", id.Index), []byte(id.StakerKey), 0o600},
			{fmt.Sprintf("signer-%d.key", id.Index), raw, 0o600},
			{fmt.Sprintf("mldsa-%d.key", id.Index), mustHex(id.MLDSAKey), 0o600},
			{fmt.Sprintf("mldsa-%d.pub", id.Index), mustHex(id.MLDSAPub), 0o644},
		} {
			if err := os.WriteFile(filepath.Join(dir, f.name), f.body, f.mode); err != nil {
				return err
			}
		}
	}
	return nil
}

func mustHex(s string) []byte {
	b, err := hex.DecodeString(s)
	if err != nil {
		panic("own hex is not hex: " + err.Error())
	}
	return b
}

func emit(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// loadSeed resolves the mnemonic the same three ways createchaintx does —
// --mnemonic-file, then LUX_MNEMONIC — and returns its BIP-39 seed. There is no
// fourth way and no default: a tool that invented a seed would mint a validator
// set nobody could reproduce.
func loadSeed(file string) ([]byte, error) {
	var mn string
	switch {
	case file != "":
		raw, err := os.ReadFile(file)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", file, err)
		}
		mn = strings.TrimSpace(string(raw))
	default:
		mn = strings.TrimSpace(os.Getenv("LUX_MNEMONIC"))
	}
	if mn == "" {
		return nil, errors.New("no mnemonic: set LUX_MNEMONIC or pass --mnemonic-file")
	}
	if !luxbip39.IsMnemonicValid(mn) {
		return nil, errors.New("invalid BIP-39 mnemonic")
	}
	return luxbip39.NewSeed(mn, ""), nil
}

// pemBody returns the DER inside a single PEM block.
func pemBody(b []byte) []byte {
	block, _ := pem.Decode(b)
	if block == nil {
		return nil
	}
	return block.Bytes
}
