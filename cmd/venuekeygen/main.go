// Copyright (C) 2026, Lux Industries Inc. All rights reserved.
// See the file LICENSE for licensing terms.

// Command venuekeygen generates the D-Chain VENUE validator staking key set
// for one network and emits it either as a KMS-ready JSON payload (default,
// stdout) or as individual on-disk artifacts under --out (local inspection
// only).
//
// The venue is a SINGLE-OPERATOR D-Chain node — exactly ONE staking identity
// per network, DISTINCT from the 5 public luxd validators. A staking identity
// here is three keys, all generated with the node's own audited primitives
// (this tool rolls NO crypto of its own):
//
//   - TLS staking cert+key (staker.crt / staker.key): P-256 ECDSA self-signed
//     cert, via github.com/luxfi/node/staking.NewCertAndKeyBytes. PEM block
//     types "CERTIFICATE" / "PRIVATE KEY" (PKCS#8). This is what luxd loads
//     from --staking-tls-cert-file / --staking-tls-key-file.
//
//   - BLS signer key (signer.key): a fresh localsigner secret key, serialized
//     with LocalSigner.ToBytes() (raw big-endian secret-key bytes). luxd's
//     getStakingSigner reads the FILE form as RAW bytes and the KMS form as a
//     HEX string, both fed to localsigner.FromBytes. So:
//   - the --out signer.key file is written as RAW bytes (file-loader shape)
//   - the stdout JSON `signer_key` field is HEX (KMS-loader shape)
//     Both round-trip through the same localsigner.FromBytes.
//
//   - PQ ML-DSA-65 key (mldsa.key / mldsa.pub): NIST Level 3 strict-PQ
//     identity, via staking.NewPQKeyPair, PEM block types
//     "ML-DSA-65 PRIVATE KEY" / "ML-DSA-65 PUBLIC KEY" over
//     PrivateKeyBytes()/PublicKeyBytes() — replicating staking.InitNodePQKeyPair
//     byte-for-byte so staking.LoadPQKeyPair consumes them.
//
// The venue's primary NodeID (node_id / --node-id) is the STRICT-PQ NodeID:
// the venue boots under the PQ securityProfile (devnet profileID=2,
// testnet/mainnet=1; the operator manifests pass --staking-mldsa-key-file +
// --staking-mldsa-pub-key-file), so luxd derives the primary NodeID from the
// ML-DSA-65 pubkey, NOT the TLS cert, via
// ids.NodeIDSchemeMLDSA65.DeriveMLDSA(ids.Empty, mldsaPubRaw) — the same seam
// node/node.go:143 → config/node StakingConfig.DeriveNodeID(ids.Empty) runs at
// boot. THAT NodeID is what goes in the venue's bootstrapper config. The legacy
// cert-derived value (ids.NodeIDFromCert) is still emitted as node_id_classical
// for non-PQ / classical-compat profiles, so nothing is lost.
//
// Every invocation generates FRESH random keys (crypto/rand via the node
// primitives). Before any output, the tool round-trips each generated key
// back through the node's own loaders and fails hard if any does not parse —
// proving the artifact is consumable by a real luxd.
//
// Build:
//
//	cd ~/work/lux/genesis && GOWORK=off go build -o /tmp/venuekeygen ./cmd/venuekeygen/
//
// Usage:
//
//	venuekeygen --net <mainnet|testnet|devnet|localnet> [--out <dir>] [--node-id]
//
//	# Default: print the KMS-ready JSON payload to stdout, kms-put commands to stderr
//	venuekeygen --net devnet --node-id
//
//	# Also write local PEM/raw artifacts for inspection (NOT for git)
//	venuekeygen --net devnet --out /tmp/venue-devnet
//
// The stdout JSON object embeds the node's KMSStakingKeys shape
// (tls_key / tls_cert / signer_key) PLUS the ML-DSA PEMs and the derived
// NodeID, so the whole object is the exact value an operator stores under the
// venue secret. The per-artifact `kms put` commands printed to stderr store
// each individual key under org=lux, path <net>/dchain-venue/<artifact>.
package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/luxfi/crypto/bls"
	"github.com/luxfi/crypto/bls/signer/localsigner"
	"github.com/luxfi/ids"
	"github.com/luxfi/node/staking"
)

// PEM block types for the ML-DSA-65 strict-PQ identity. These MUST match
// staking.InitNodePQKeyPair / staking.LoadPQKeyPair exactly, or luxd will
// refuse the artifact at config-load. Mirrored here (not imported) because
// the node package does not export them, and we replicate the exact encoding.
const (
	mldsaPrivPEMType = "ML-DSA-65 PRIVATE KEY"
	mldsaPubPEMType  = "ML-DSA-65 PUBLIC KEY"
)

// venueSecretBase is the per-artifact KMS path segment under which the venue
// stores its keys. The full path is "<net>/dchain-venue/<artifact>" — the
// network is the leading segment because the kms CLI has no --env flag (env
// is encoded in the path); org=lux selects the Lux KMS tenant via --org.
const venueSecretBase = "dchain-venue"

// venueArtifacts are the on-disk filenames written under --out and the
// trailing KMS key names, in stable order.
var venueArtifacts = []string{
	"staker.crt",
	"staker.key",
	"signer.key",
	"mldsa.key",
	"mldsa.pub",
}

// payload is the stdout JSON document. It embeds the node's KMSStakingKeys
// json shape verbatim (tls_key / tls_cert / signer_key) so the node's KMS
// loader (staking.FetchFromKMS → json.Unmarshal into KMSStakingKeys) can
// consume the same bytes, and adds the strict-PQ ML-DSA material plus the
// derived classical NodeID.
//
// signer_key is HEX of LocalSigner.ToBytes() — the exact form
// getStakingSigner expects on the KMS path (hex.DecodeString →
// localsigner.FromBytes).
type payload struct {
	staking.KMSStakingKeys        // tls_key, tls_cert, signer_key (json:"...")
	MLDSAKey               string `json:"mldsa_key"` // PEM "ML-DSA-65 PRIVATE KEY"
	MLDSAPub               string `json:"mldsa_pub"` // PEM "ML-DSA-65 PUBLIC KEY"
	// NodeID is the strict-PQ ML-DSA-65 NodeID
	// (ids.NodeIDSchemeMLDSA65.DeriveMLDSA(ids.Empty, mldsaPubRaw)) — the value
	// luxd computes for this venue under the PQ securityProfile. This is the
	// NodeID that belongs in the venue's bootstrapper config.
	NodeID string `json:"node_id"`
	// NodeIDClassical is the legacy cert-derived NodeID (ids.NodeIDFromCert),
	// retained for non-PQ / classical-compat profiles. NOT used by this venue.
	NodeIDClassical string `json:"node_id_classical"`
}

// keyset holds the raw generated artifact bytes for one venue identity,
// before they are shaped into JSON or written to disk.
type keyset struct {
	stakerCertPEM []byte // PEM "CERTIFICATE"
	stakerKeyPEM  []byte // PEM "PRIVATE KEY" (PKCS#8 P-256)
	signerKeyRaw  []byte // raw LocalSigner.ToBytes() (file form)
	mldsaKeyPEM   []byte // PEM "ML-DSA-65 PRIVATE KEY"
	mldsaPubPEM   []byte // PEM "ML-DSA-65 PUBLIC KEY"
	mldsaPubRaw   []byte // raw ML-DSA-65 pubkey bytes (the strict-PQ NodeID seed)
	// nodeID is the venue's REAL primary NodeID: under the strict-PQ
	// securityProfile (all four Lux networks; the operator manifests pass
	// --staking-mldsa-key-file/--staking-mldsa-pub-key-file), luxd derives the
	// primary NodeID from the ML-DSA-65 pubkey, NOT the TLS cert. See
	// nodeIDStrictPQ for the exact derivation + node-source citation.
	nodeID ids.NodeID
	// nodeIDClassical is the legacy cert-derived NodeID (ids.NodeIDFromCert),
	// kept for non-PQ profiles / classical-compat. It is NOT the value luxd
	// uses for this venue.
	nodeIDClassical ids.NodeID
	blsPubHex       string // compressed BLS public key, hex (logged only)
}

func main() {
	net := flag.String("net", "", "network: mainnet|testnet|devnet|localnet (required)")
	out := flag.String("out", "", "if set, ALSO write staker.crt/staker.key/signer.key/mldsa.key/mldsa.pub into this dir (0600, local inspection only — NOT for git)")
	nodeID := flag.Bool("node-id", false, "also print the venue's strict-PQ ML-DSA-65 NodeID in bare bootstrap form")
	flag.Parse()

	netSlug, err := normalizeNet(*net)
	if err != nil {
		fmt.Fprintf(os.Stderr, "venuekeygen: %v\n", err)
		os.Exit(1)
	}

	ks, err := generate()
	if err != nil {
		fmt.Fprintf(os.Stderr, "venuekeygen: generate: %v\n", err)
		os.Exit(1)
	}

	// Self-check: prove every artifact round-trips through the node's own
	// loaders before we emit anything. A failure here means the artifact
	// would not be consumable by a real luxd, so we must not ship it.
	if err := selfCheck(ks); err != nil {
		fmt.Fprintf(os.Stderr, "venuekeygen: self-check failed (artifact not luxd-consumable): %v\n", err)
		os.Exit(1)
	}

	if *out != "" {
		if err := writeArtifacts(*out, ks); err != nil {
			fmt.Fprintf(os.Stderr, "venuekeygen: write artifacts: %v\n", err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "venuekeygen: wrote local artifacts to %s (0600) — these are SECRETS, do NOT commit / gitignore this dir\n", *out)
	}

	doc := buildPayload(ks)
	body, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "venuekeygen: marshal payload: %v\n", err)
		os.Exit(1)
	}
	// stdout stays clean JSON — the exact value to store in KMS.
	fmt.Println(string(body))

	// Everything operator-facing goes to stderr so stdout is pipeable JSON.
	fmt.Fprintf(os.Stderr, "\nvenuekeygen: D-Chain venue staking identity for net=%s\n", netSlug)
	fmt.Fprintf(os.Stderr, "venuekeygen: NodeID=%s\n", ks.nodeID)
	fmt.Fprintf(os.Stderr, "venuekeygen: node_id is the strict-PQ ML-DSA-65 NodeID (DeriveMLDSA(ids.Empty,pub)) — the value luxd computes under the PQ securityProfile; node_id_classical=%s is the legacy cert-derived value for non-PQ profiles.\n", ks.nodeIDClassical)
	fmt.Fprintf(os.Stderr, "venuekeygen: BLS pubkey (compressed, hex)=%s\n", ks.blsPubHex)
	if *nodeID {
		// Also print the NodeID to stdout-adjacent stderr in the bare form a
		// bootstrapper config wants. (stdout itself carries it in JSON.) This is
		// the strict-PQ NodeID — the one that matters because the venue runs
		// under the PQ securityProfile.
		fmt.Fprintf(os.Stderr, "venuekeygen: bootstrap NodeID -> %s\n", ks.nodeID)
	}
	printKMSCommands(netSlug)
}

// generate produces one fresh venue staking identity using only the node's
// audited primitives. crypto/rand is the entropy source inside each.
func generate() (*keyset, error) {
	// TLS staking cert + key (P-256 ECDSA), node's canonical generator.
	certPEM, keyPEM, err := staking.NewCertAndKeyBytes()
	if err != nil {
		return nil, fmt.Errorf("new TLS cert/key: %w", err)
	}

	// BLS signer key — node's localsigner. ToBytes() is the on-disk file form.
	signer, err := localsigner.New()
	if err != nil {
		return nil, fmt.Errorf("new BLS signer: %w", err)
	}
	signerRaw := signer.ToBytes()
	blsPubHex := hex.EncodeToString(bls.PublicKeyToCompressedBytes(signer.PublicKey()))

	// PQ ML-DSA-65 key pair — node's strict-PQ generator.
	pq, err := staking.NewPQKeyPair()
	if err != nil {
		return nil, fmt.Errorf("new ML-DSA-65 key pair: %w", err)
	}
	mldsaPubRaw := pq.PublicKeyBytes()
	mldsaKeyPEM, err := encodePEM(mldsaPrivPEMType, pq.PrivateKeyBytes())
	if err != nil {
		return nil, fmt.Errorf("encode ML-DSA private PEM: %w", err)
	}
	mldsaPubPEM, err := encodePEM(mldsaPubPEMType, mldsaPubRaw)
	if err != nil {
		return nil, fmt.Errorf("encode ML-DSA public PEM: %w", err)
	}

	// Strict-PQ NodeID — the venue's REAL primary NodeID. Derived from the
	// SAME raw ML-DSA-65 pubkey bytes that are PEM-encoded into mldsa.pub.
	pqNID, err := nodeIDStrictPQ(mldsaPubRaw)
	if err != nil {
		return nil, fmt.Errorf("derive strict-PQ NodeID: %w", err)
	}

	// Classical cert-derived NodeID — kept for non-PQ / classical-compat
	// profiles only. Not the value luxd uses for this venue.
	classicalNID, err := nodeIDFromCertPEM(certPEM, keyPEM)
	if err != nil {
		return nil, fmt.Errorf("derive classical NodeID: %w", err)
	}

	return &keyset{
		stakerCertPEM:   certPEM,
		stakerKeyPEM:    keyPEM,
		signerKeyRaw:    signerRaw,
		mldsaKeyPEM:     mldsaKeyPEM,
		mldsaPubPEM:     mldsaPubPEM,
		mldsaPubRaw:     mldsaPubRaw,
		nodeID:          pqNID,
		nodeIDClassical: classicalNID,
		blsPubHex:       blsPubHex,
	}, nil
}

// nodeIDStrictPQ derives the venue's primary NodeID the way luxd does under the
// strict-PQ securityProfile: from the raw ML-DSA-65 pubkey bytes, bound to the
// primary-network chain id (ids.Empty).
//
// This is byte-identical to what the node computes at boot. The single source
// of truth is the node's StakingConfig.DeriveNodeID seam:
//
//	config/node/config.go:298-301  (strict-PQ branch, when StakingMLDSAPub set):
//	    id, _, _ := ids.NodeIDSchemeMLDSA65.DeriveMLDSA(chainID, c.StakingMLDSAPub)
//	node/node.go:143  (boot call site):
//	    derivedNodeID, _ := config.StakingConfig.DeriveNodeID(ids.Empty)
//
// We call DeriveMLDSA directly (rather than constructing a config/node
// StakingConfig) because that package pulls the full node-config dependency
// surface (chains/network/server) — heavyweight and unnecessary for one
// derivation. The pubKey argument is the RAW ML-DSA pubkey bytes
// (PublicKeyBytes()); DeriveMLDSA applies its own SP 800-185 framing, so it
// must NOT be PEM-wrapped.
func nodeIDStrictPQ(mldsaPubRaw []byte) (ids.NodeID, error) {
	id, _, err := ids.NodeIDSchemeMLDSA65.DeriveMLDSA(ids.Empty, mldsaPubRaw)
	return id, err
}

// selfCheck loads every generated artifact back through the node's own
// loaders and asserts the NodeID is reproducible. Any failure means the
// artifact is not luxd-consumable.
func selfCheck(ks *keyset) error {
	// Strict-PQ NodeID — the venue's REAL primary NodeID. Must be non-empty and
	// STABLE: re-deriving from the same ML-DSA pubkey bytes yields the same id.
	if ks.nodeID == ids.EmptyNodeID {
		return errors.New("strict-PQ NodeID is empty")
	}
	rePQ, err := nodeIDStrictPQ(ks.mldsaPubRaw)
	if err != nil {
		return fmt.Errorf("strict-PQ NodeID re-derive: %w", err)
	}
	if rePQ != ks.nodeID {
		return fmt.Errorf("strict-PQ NodeID not reproducible: %s != %s", rePQ, ks.nodeID)
	}

	// TLS: parse via the node loader and re-derive the classical NodeID.
	tlsCert, err := staking.LoadTLSCertFromBytes(ks.stakerKeyPEM, ks.stakerCertPEM)
	if err != nil {
		return fmt.Errorf("TLS round-trip: %w", err)
	}
	if tlsCert.Leaf == nil {
		return errors.New("TLS round-trip: nil leaf")
	}
	reClassical := ids.NodeIDFromCert(&ids.Certificate{
		Raw:       tlsCert.Leaf.Raw,
		PublicKey: tlsCert.Leaf.PublicKey,
	})
	if reClassical != ks.nodeIDClassical {
		return fmt.Errorf("classical NodeID not reproducible: %s != %s", reClassical, ks.nodeIDClassical)
	}

	// The two derivations MUST differ: the strict-PQ NodeID comes from the
	// ML-DSA pubkey, the classical one from the TLS cert. If they collide,
	// something is wired wrong (e.g. the PQ branch fell back to the cert).
	if ks.nodeID == ks.nodeIDClassical {
		return errors.New("strict-PQ NodeID equals classical NodeID (PQ derivation not applied)")
	}

	// BLS: parse the raw file form back through localsigner.
	if _, err := localsigner.FromBytes(ks.signerKeyRaw); err != nil {
		return fmt.Errorf("BLS signer round-trip (file form): %w", err)
	}
	// BLS: parse the hex KMS form back through localsigner (same code path the
	// node uses on the KMS branch).
	rawFromHex, err := hex.DecodeString(hex.EncodeToString(ks.signerKeyRaw))
	if err != nil {
		return fmt.Errorf("BLS signer hex decode: %w", err)
	}
	if !bytes.Equal(rawFromHex, ks.signerKeyRaw) {
		return errors.New("BLS signer hex/raw mismatch")
	}
	if _, err := localsigner.FromBytes(rawFromHex); err != nil {
		return fmt.Errorf("BLS signer round-trip (KMS hex form): %w", err)
	}

	// ML-DSA: load both PEMs back through the node loader via temp files
	// (LoadPQKeyPair is the file-based API the node uses).
	tmp, err := os.MkdirTemp("", "venuekeygen-selfcheck-")
	if err != nil {
		return fmt.Errorf("temp dir: %w", err)
	}
	defer os.RemoveAll(tmp)
	keyPath := filepath.Join(tmp, "mldsa.key")
	pubPath := filepath.Join(tmp, "mldsa.pub")
	if err := os.WriteFile(keyPath, ks.mldsaKeyPEM, 0o600); err != nil {
		return fmt.Errorf("write temp ML-DSA key: %w", err)
	}
	if err := os.WriteFile(pubPath, ks.mldsaPubPEM, 0o600); err != nil {
		return fmt.Errorf("write temp ML-DSA pub: %w", err)
	}
	if _, err := staking.LoadPQKeyPair(keyPath, pubPath); err != nil {
		return fmt.Errorf("ML-DSA round-trip: %w", err)
	}
	return nil
}

// buildPayload shapes a verified keyset into the stdout JSON document.
func buildPayload(ks *keyset) payload {
	return payload{
		KMSStakingKeys: staking.KMSStakingKeys{
			TLSKey:    string(ks.stakerKeyPEM),
			TLSCert:   string(ks.stakerCertPEM),
			SignerKey: hex.EncodeToString(ks.signerKeyRaw), // KMS hex form
		},
		MLDSAKey:        string(ks.mldsaKeyPEM),
		MLDSAPub:        string(ks.mldsaPubPEM),
		NodeID:          ks.nodeID.String(),          // strict-PQ ML-DSA-65 NodeID
		NodeIDClassical: ks.nodeIDClassical.String(), // legacy cert-derived NodeID
	}
}

// writeArtifacts writes the five on-disk artifacts under dir with 0600 perms.
// signer.key is RAW bytes (the file-loader shape getStakingSigner reads); the
// rest are PEM. These are SECRETS for local inspection only.
func writeArtifacts(dir string, ks *keyset) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("mkdir %s: %w", dir, err)
	}
	files := map[string][]byte{
		"staker.crt": ks.stakerCertPEM,
		"staker.key": ks.stakerKeyPEM,
		"signer.key": ks.signerKeyRaw, // RAW, matches node file loader
		"mldsa.key":  ks.mldsaKeyPEM,
		"mldsa.pub":  ks.mldsaPubPEM,
	}
	for name, b := range files {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, b, 0o600); err != nil {
			return fmt.Errorf("write %s: %w", p, err)
		}
	}
	return nil
}

// printKMSCommands prints, to stderr, the exact `kms put` invocations the
// operator runs to store each artifact. The kms CLI has no --env flag, so the
// network is the leading path segment; --org lux selects the Lux KMS tenant.
// Operators pipe each artifact into --stdin to keep secret material off argv.
func printKMSCommands(netSlug string) {
	fmt.Fprintf(os.Stderr, "\nvenuekeygen: store each artifact in Lux KMS (org=lux), e.g. from an --out dir:\n")
	for _, art := range venueArtifacts {
		path := fmt.Sprintf("%s/%s/%s", netSlug, venueSecretBase, art)
		fmt.Fprintf(os.Stderr, "  kms put %s --stdin --org lux < %s\n", path, art)
	}
	fmt.Fprintf(os.Stderr, "venuekeygen: (note: the kms CLI has no --env flag; net=%s is the leading path segment)\n", netSlug)
}

// normalizeNet validates and canonicalizes the --net value.
func normalizeNet(net string) (string, error) {
	switch net {
	case "mainnet", "testnet", "devnet", "localnet":
		return net, nil
	case "":
		return "", errors.New("--net is required (mainnet|testnet|devnet|localnet)")
	default:
		return "", fmt.Errorf("unknown --net %q (expected mainnet|testnet|devnet|localnet)", net)
	}
}

// encodePEM wraps body in a PEM block of the given type.
func encodePEM(blockType string, body []byte) ([]byte, error) {
	var buf bytes.Buffer
	if err := pem.Encode(&buf, &pem.Block{Type: blockType, Bytes: body}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// nodeIDFromCertPEM parses a staking cert+key PEM pair via the node's TLS
// loader and derives the classical NodeID the same way luxd does.
func nodeIDFromCertPEM(certPEM, keyPEM []byte) (ids.NodeID, error) {
	cert, err := staking.LoadTLSCertFromBytes(keyPEM, certPEM)
	if err != nil {
		return ids.EmptyNodeID, err
	}
	if cert.Leaf == nil {
		return ids.EmptyNodeID, errors.New("nil cert leaf")
	}
	return ids.NodeIDFromCert(&ids.Certificate{
		Raw:       cert.Leaf.Raw,
		PublicKey: cert.Leaf.PublicKey,
	}), nil
}
