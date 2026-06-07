// Copyright (C) 2019-2025, Lux Partners Limited. All rights reserved.
// See the file LICENSE for licensing terms.

package genesis

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/luxfi/constants"
	// ML-DSA-65 (FIPS 204) deterministic keygen — canonical lux/crypto
	// package. NewKeyFromSeed accepts the 32-byte ξ FIPS 204 §5.1 KeyGen
	// consumes; the HIP-0077 SHAKE-256(label || child_seed) expansion
	// happens at the call site below (mldsaKeygenFromChildSeed) so the
	// derivation is byte-for-byte reproducible against the prior CIRCL
	// stop-gap.
	"github.com/luxfi/crypto"
	"github.com/luxfi/crypto/bls"
	"github.com/luxfi/crypto/keccak256"
	"github.com/luxfi/crypto/pq/mldsa/mldsa65"
	"github.com/luxfi/crypto/secp256k1"
	"github.com/luxfi/go-bip32"
	"github.com/luxfi/go-bip39"
	"github.com/luxfi/ids"
	"github.com/luxfi/log"
	luxtls "github.com/luxfi/tls"
	"golang.org/x/crypto/sha3"
)

const (
	// DefaultAllocationPerAccount is the canonical fresh-genesis amount:
	// 50M LUX per derived account, spendable immediately on both X-Chain
	// and P-Chain (same address — bech32(ripemd160(sha256(secp256k1_pubkey)))).
	//
	// 1000 accounts × 50M × 2 chains = 100B LUX of UTXOs in genesis.
	// Per-chain SupplyCap is 2T LUX, so each chain individually clears.
	//
	// Address scheme — Bitcoin-UTXO-style (more quantum-resistant):
	//   P-Chain / X-Chain addresses are bech32(ripemd160(sha256(pubkey))).
	//   The public key is hidden behind two hash layers until first spend.
	//   A future quantum adversary cannot recover the private key from the
	//   address alone — Shor's works on the pubkey, not on sha256+ripemd160.
	//   C-Chain (Ethereum) addresses expose the pubkey via ECDSA recovery
	//   on every signed tx; long-term holds belong on P/X, not C.
	DefaultAllocationPerAccount = 50_000_000 * Lux

	// DefaultAllocationPerValidator is kept for backward compatibility
	DefaultAllocationPerValidator = DefaultAllocationPerAccount

	// DefaultNumAccounts is the default number of mnemonic-derived accounts.
	// Funds 1000 BIP44 wallet keys at m/44'/9000'/0'/0/i so that any
	// canonical-BIP44 wallet (Lux Wallet, MetaMask, `lux key derive`) sees
	// a fundable address on both P and X under the same mnemonic.
	// Per-network isolation comes from a DIFFERENT mnemonic per env
	// (loaded from KMS), not from path divergence.
	DefaultNumAccounts = 1000

	// TreasuryAddress is the C-Chain treasury (no per-account C-Chain alloc).
	TreasuryAddress = "0x9011E888251AB053B7bD1cdB598Db4f9DEd94714"

	// TreasuryAmount is 2 trillion LUX in microLUX (2T * 10^6).
	TreasuryAmount = 2_000_000_000_000 * Lux

	// PChainFeeReserve is 10,000 LUX per validator earmarked for P-Chain fees.
	PChainFeeReserve = 10_000 * Lux

	// LightMnemonic is the well-known public dev seed. Pass this as the
	// value of LUX_MNEMONIC to bootstrap a local network (network ID >=
	// 1337). RefuseLightMnemonicOnProduction enforces it cannot be used
	// on mainnet/testnet/devnet (network IDs 1/2/3).
	LightMnemonic = "light light light light light light light light light light light energy"
)

// KeyInfo contains parsed key information for a node
type KeyInfo struct {
	NodeID               ids.NodeID
	StakerKey            []byte
	BLSPublicKey         []byte
	BLSProofOfPossession []byte
	MLDSAPublicKey       []byte      // ML-DSA post-quantum public key (FIPS 204)
	CoronaPublicKey      []byte      // Corona ring signature public key
	StakingAddr          ids.ShortID // P-chain address derived from staker key
	EVMAddr              ids.ShortID // C-chain (and other EVM chain) H160 address
}

// LoadKeysFromDir loads all node keys from a directory
// Expected structure: keysDir/{node1,node2,...}/staker.key, staker.crt, signer.key
func LoadKeysFromDir(keysDir string) ([]KeyInfo, error) {
	if keysDir == "" {
		home, _ := os.UserHomeDir()
		keysDir = filepath.Join(home, ".lux", "keys")
	}

	entries, err := os.ReadDir(keysDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read keys directory %s: %w", keysDir, err)
	}

	var keys []KeyInfo
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		nodeDir := filepath.Join(keysDir, entry.Name())
		keyInfo, err := loadNodeKey(nodeDir)
		if err != nil {
			// Skip nodes with incomplete keys
			continue
		}
		keys = append(keys, *keyInfo)
	}

	return keys, nil
}

// loadNodeKey loads key info from a single node directory
// Supports two directory structures:
// 1. Modern: nodeDir/staking/staker.crt, nodeDir/bls/signer.key
// 2. Legacy: nodeDir/staker.crt, nodeDir/signer.key
func loadNodeKey(nodeDir string) (*KeyInfo, error) {
	// Try modern path structure first (staking/ subdirectory)
	certPath := filepath.Join(nodeDir, "staking", "staker.crt")
	certPEM, err := os.ReadFile(certPath)
	if err != nil {
		// Fall back to legacy path (direct in node dir)
		certPath = filepath.Join(nodeDir, "staker.crt")
		certPEM, err = os.ReadFile(certPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read staker.crt: %w", err)
		}
	}

	// Load staker key - try modern path first
	keyPath := filepath.Join(nodeDir, "staking", "staker.key")
	keyPEM, err := os.ReadFile(keyPath)
	if err != nil {
		// Fall back to legacy path
		keyPath = filepath.Join(nodeDir, "staker.key")
		keyPEM, err = os.ReadFile(keyPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read staker.key: %w", err)
		}
	}

	// Use github.com/luxfi/tls to correctly derive node ID (same method as luxd)
	tlsCert, err := luxtls.LoadTLSCertFromBytes(keyPEM, certPEM)
	if err != nil {
		return nil, fmt.Errorf("failed to load TLS cert: %w", err)
	}

	stakingCert := &ids.Certificate{
		Raw:       tlsCert.Leaf.Raw,
		PublicKey: tlsCert.Leaf.PublicKey,
	}
	nodeID := ids.NodeIDFromCert(stakingCert)

	keyInfo := &KeyInfo{
		NodeID:    nodeID,
		StakerKey: keyPEM,
	}

	// Try to load BLS signer key (optional) - try modern path first
	signerPath := filepath.Join(nodeDir, "bls", "signer.key")
	signerPEM, signerErr := os.ReadFile(signerPath)
	if signerErr != nil {
		// Fall back to legacy path
		signerPath = filepath.Join(nodeDir, "signer.key")
		signerPEM, signerErr = os.ReadFile(signerPath)
	}
	if signerErr == nil {
		// Parse BLS key and get public key + proof of possession
		sk, err := bls.SecretKeyFromBytes(signerPEM)
		if err == nil {
			pk := bls.PublicFromSecretKey(sk)
			keyInfo.BLSPublicKey = bls.PublicKeyToCompressedBytes(pk)
			// Sign public key for proof of possession (uses PoP domain separation tag)
			sig := bls.SignProofOfPossession(sk, keyInfo.BLSPublicKey)
			keyInfo.BLSProofOfPossession = bls.SignatureToBytes(sig)
		}
	}

	// Try to load EC private key for proper ETH/P-chain address derivation
	// Look in ec/private.key subdirectory first (modern structure)
	ecKeyPath := filepath.Join(nodeDir, "ec", "private.key")
	ecKeyHex, ecErr := os.ReadFile(ecKeyPath)
	if ecErr != nil {
		// Fall back to legacy path
		ecKeyPath = filepath.Join(nodeDir, "private.key")
		ecKeyHex, ecErr = os.ReadFile(ecKeyPath)
	}

	if ecErr == nil {
		// Parse EC key and derive proper addresses
		privKeyHex := strings.TrimSpace(string(ecKeyHex))
		privKeyBytes, err := hex.DecodeString(privKeyHex)
		if err == nil {
			// Get EVM address
			evmPrivKey, err := crypto.ToECDSA(privKeyBytes)
			if err == nil {
				evmAddr := keccakAddr(evmPrivKey.PublicKey.X, evmPrivKey.PublicKey.Y)
				copy(keyInfo.EVMAddr[:], evmAddr[:])
			}

			// Get Lux ShortID (for X/P chain addresses)
			utxoPrivKey, err := secp256k1.ToPrivateKey(privKeyBytes)
			if err == nil {
				pubKey := utxoPrivKey.PublicKey()
				shortID := ids.ShortID(pubKey.Address())
				copy(keyInfo.StakingAddr[:], shortID[:])
			}
		}
	} else {
		// Fallback: derive from node ID (NOT correct but backward compatible)
		// WARNING: These addresses won't have usable private keys!
		copy(keyInfo.StakingAddr[:], nodeID[:])
		copy(keyInfo.EVMAddr[:], nodeID[:])
	}

	// Load ML-DSA public key (post-quantum identity)
	mldsaPath := filepath.Join(nodeDir, "mldsa", "public.key")
	if data, err := os.ReadFile(mldsaPath); err == nil {
		keyInfo.MLDSAPublicKey = data
	}

	// Load Corona public key (ring signatures)
	rtPath := filepath.Join(nodeDir, "rt", "public.key")
	if data, err := os.ReadFile(rtPath); err == nil {
		keyInfo.CoronaPublicKey = data
	}

	return keyInfo, nil
}

// BuildConfigFromKeys creates a genesis config from local keys
// Validators get fee reserve on P-Chain, all keys get X-Chain allocation with vesting
func BuildConfigFromKeys(networkID uint32, keysDir string, allocationPerKey uint64) (*Config, error) {
	keys, err := LoadKeysFromDir(keysDir)
	if err != nil {
		return nil, fmt.Errorf("failed to load keys: %w", err)
	}

	if len(keys) == 0 {
		return nil, fmt.Errorf("no keys found in %s", keysDir)
	}

	// All loaded keys are both validators and account holders
	return buildConfigFromKeyInfos(networkID, keys, keys, allocationPerKey)
}

// deriveFeeKey derives a fee reserve key from a validator's EC private key
// The fee key is keccakBytes("fee-reserve:" || ecPrivKey) which gives a different
// secp256k1 private key with a different P-chain address
func deriveFeeKey(keysDir string, validatorKey KeyInfo, index int) (*KeyInfo, error) {
	// Read the validator's EC private key
	ecKeyPath := filepath.Join(keysDir, fmt.Sprintf("node%d", index), "ec", "private.key")
	ecKeyHex, err := os.ReadFile(ecKeyPath)
	if err != nil {
		return nil, fmt.Errorf("no EC key at %s: %w", ecKeyPath, err)
	}

	privKeyBytes, err := hex.DecodeString(strings.TrimSpace(string(ecKeyHex)))
	if err != nil {
		return nil, fmt.Errorf("invalid EC key hex: %w", err)
	}

	// Derive fee private key: keccakBytes("fee-reserve:" || ecPrivKey)
	feePrivBytes := keccakBytes(append([]byte("fee-reserve:"), privKeyBytes...))

	// Derive proper P-chain address using secp256k1
	feePrivKey, err := secp256k1.ToPrivateKey(feePrivBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to create fee private key: %w", err)
	}
	feePubKey := feePrivKey.PublicKey()
	feeAddr := ids.ShortID(feePubKey.Address())

	// Derive EVM address
	evmPrivKey, err := crypto.ToECDSA(feePrivBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to create fee ETH key: %w", err)
	}
	evmAddr := keccakAddr(evmPrivKey.PublicKey.X, evmPrivKey.PublicKey.Y)
	var evmShortID ids.ShortID
	copy(evmShortID[:], evmAddr[:])

	// Save fee private key for later use by deploy tools
	feeKeyDir := filepath.Join(keysDir, fmt.Sprintf("fee%d", index))
	os.MkdirAll(filepath.Join(feeKeyDir, "ec"), 0700)
	feeKeyHex := hex.EncodeToString(feePrivBytes)
	os.WriteFile(filepath.Join(feeKeyDir, "ec", "private.key"), []byte(feeKeyHex), 0600)

	fmt.Fprintf(os.Stderr, "Fee key %d: addr=%s evmAddr=0x%x saved to %s\n",
		index, feeAddr, evmAddr, feeKeyDir)

	return &KeyInfo{
		StakingAddr: feeAddr,
		EVMAddr:     evmShortID,
	}, nil
}

// buildCChainGenesisTreasury creates C-chain genesis JSON with only the treasury allocation.
// Treasury: 0x9011E888251AB053B7bD1cdB598Db4f9DEd94714 gets 2T LUX.
// No mnemonic-derived account allocations on C-Chain.
func buildCChainGenesisTreasury(networkID uint32) (string, error) {
	// C-Chain uses wei (18 decimals), TreasuryAmount is in microLUX (6 decimals)
	// Multiply by 1e12 to convert microLUX → wei
	treasuryWei := new(big.Int).Mul(
		big.NewInt(int64(TreasuryAmount)),
		new(big.Int).Exp(big.NewInt(10), big.NewInt(12), nil),
	)
	alloc := map[string]Balance{
		TreasuryAddress: {
			Balance: fmt.Sprintf("0x%x", treasuryWei),
		},
	}

	return marshalCChainGenesis(networkID, alloc)
}

// buildCChainGenesis creates C-chain genesis JSON with per-key allocations (legacy).
func buildCChainGenesis(networkID uint32, keys []KeyInfo, allocationPerKey uint64) (string, error) {
	alloc := make(map[string]Balance)
	for _, key := range keys {
		addr := fmt.Sprintf("0x%s", key.EVMAddr.Hex())
		alloc[addr] = Balance{
			Balance: fmt.Sprintf("0x%x", allocationPerKey),
		}
	}

	return marshalCChainGenesis(networkID, alloc)
}

// marshalCChainGenesis marshals a C-chain genesis config with the given allocations.
func marshalCChainGenesis(networkID uint32, alloc map[string]Balance) (string, error) {
	cchain := CChainConfig{
		Config: CChainParams{
			ChainID:             uint64(networkID),
			HomesteadBlock:      0,
			EIP150Block:         0,
			EIP155Block:         0,
			EIP158Block:         0,
			ByzantiumBlock:      0,
			ConstantinopleBlock: 0,
			PetersburgBlock:     0,
			IstanbulBlock:       0,
			MuirGlacierBlock:    0,
			BerlinBlock:         0,
			LondonBlock:         0,
			FeeConfig: FeeConfig{
				GasLimit:                 12000000,
				TargetBlockRate:          2,
				MinBaseFee:               25000000000,
				TargetGas:                60000000,
				BaseFeeChangeDenominator: 36,
				MinBlockGasCost:          0,
				MaxBlockGasCost:          1000000,
				BlockGasCostStep:         200000,
			},
		},
		Nonce:      "0x0",
		Timestamp:  fmt.Sprintf("0x%x", time.Now().Unix()),
		ExtraData:  "0x",
		GasLimit:   "0xb71b00",
		Difficulty: "0x0",
		MixHash:    "0x0000000000000000000000000000000000000000000000000000000000000000",
		Coinbase:   "0x0000000000000000000000000000000000000000",
		Alloc:      alloc,
		Number:     "0x0",
		GasUsed:    "0x0",
		ParentHash: "0x0000000000000000000000000000000000000000000000000000000000000000",
	}

	data, err := json.Marshal(cchain)
	if err != nil {
		return "", err
	}

	return string(data), nil
}

// LoadKeyFromEnv loads a single key from environment variables
// Env vars: PRIVATE_KEY (hex), NODE_ID
func LoadKeyFromEnv() (*KeyInfo, error) {
	privKeyHex := os.Getenv("PRIVATE_KEY")
	if privKeyHex == "" {
		return nil, fmt.Errorf("PRIVATE_KEY not set")
	}

	privKeyHex = strings.TrimPrefix(privKeyHex, "0x")
	privKeyBytes, err := hex.DecodeString(privKeyHex)
	if err != nil {
		return nil, fmt.Errorf("invalid private key hex: %w", err)
	}

	return keyInfoFromPrivateKey(privKeyBytes)
}

// LoadKeysFromMnemonic derives keys from a BIP39 mnemonic using the canonical
// Lux BIP44 path m/44'/9000'/0'/0/i. The same mnemonic produces the same keys
// across every Lux network — addresses are stable in MetaMask, Lux Wallet,
// and `lux key derive`. Per-network isolation comes from using a different
// mnemonic per environment, not from path divergence.
func LoadKeysFromMnemonic(mnemonic string, numAccounts int) ([]KeyInfo, error) {
	if !bip39.IsMnemonicValid(mnemonic) {
		return nil, fmt.Errorf("invalid mnemonic")
	}

	seed := bip39.NewSeed(mnemonic, "")
	masterKey, err := bip32.NewMasterKey(seed)
	if err != nil {
		return nil, fmt.Errorf("failed to create master key: %w", err)
	}

	purpose, err := masterKey.NewChildKey(bip32.FirstHardenedChild + 44)
	if err != nil {
		return nil, fmt.Errorf("failed to derive purpose 44': %w", err)
	}
	coinType, err := purpose.NewChildKey(bip32.FirstHardenedChild + 9000)
	if err != nil {
		return nil, fmt.Errorf("failed to derive coin type 9000': %w", err)
	}
	account, err := coinType.NewChildKey(bip32.FirstHardenedChild + 0)
	if err != nil {
		return nil, fmt.Errorf("failed to derive account 0': %w", err)
	}
	change, err := account.NewChildKey(0)
	if err != nil {
		return nil, fmt.Errorf("failed to derive change 0: %w", err)
	}

	keys := make([]KeyInfo, 0, numAccounts)
	for i := 0; i < numAccounts; i++ {
		// Standard Lux secp256k1 child: m/44'/9000'/0'/0/i
		// (non-hardened index — matches Lux Wallet, MetaMask, cast).
		luxChild, err := change.NewChildKey(uint32(i)) //nolint:gosec
		if err != nil {
			return nil, fmt.Errorf("failed to derive secp256k1 key %d: %w", i, err)
		}

		keyInfo, err := keyInfoFromPrivateKey(luxChild.Key)
		if err != nil {
			return nil, fmt.Errorf("failed to create key info %d: %w", i, err)
		}

		// ML-DSA-65 mesh identity — derive from a separate label so
		// no collision with the secp256k1 path. Still reproducible
		// from the same mnemonic + index.
		mldsaSeed := keccakBytes(append(append(seed, []byte("LUX/HIP-0077/mldsa65")...), byte(i)))
		mldsaPubKey, err := mldsaKeygenFromChildSeed(mldsaSeed[:32])
		if err != nil {
			return nil, fmt.Errorf("ML-DSA keygen %d: %w", i, err)
		}
		keyInfo.MLDSAPublicKey = mldsaPubKey

		// BLS signer key — derive deterministically from mnemonic seed + index.
		// Uses keccakBytes(seed || "bls-signer" || index) as the BLS secret
		// key seed so BLS keys are reproducible from the same mnemonic.
		blsSeed := keccakBytes(append(append(seed, []byte("bls-signer")...), byte(i)))
		blsSK, blsErr := bls.SecretKeyFromSeed(blsSeed)
		if blsErr == nil {
			blsPK := bls.PublicFromSecretKey(blsSK)
			keyInfo.BLSPublicKey = bls.PublicKeyToCompressedBytes(blsPK)
			sig := bls.SignProofOfPossession(blsSK, keyInfo.BLSPublicKey)
			keyInfo.BLSProofOfPossession = bls.SignatureToBytes(sig)
		}

		keys = append(keys, *keyInfo)
	}

	log.Debug("derived HD keys",
		"numAccounts", numAccounts,
		"path", "m/44'/9000'/0'/0/i",
	)
	return keys, nil
}

// mldsaKeygenFromChildSeed expands a 32-byte BIP-32 child seed into the
// 32-byte ξ that FIPS 204 §5.1 KeyGen consumes, then returns the packed
// ML-DSA-65 public key (1952 bytes).
//
// The expansion is `ξ = SHAKE-256("LUX/HIP-0077/mldsa65" || child_seed)`.
// The domain-separation label means a future scheme that wants to use
// the same BIP-32 child seed for a different KEM/SIG cannot collide
// with our ML-DSA key.
//
// The 32-byte ξ is then handed verbatim to
// github.com/luxfi/crypto/pq/mldsa/mldsa65.NewKeyFromSeed: at len == 32
// the package wires the seed straight into the FIPS 204 §5.1 KeyGen, so
// the keypair is byte-for-byte reproducible against the prior CIRCL
// stop-gap (which the canonical package wraps unchanged).
func mldsaKeygenFromChildSeed(childSeed []byte) ([]byte, error) {
	if len(childSeed) != 32 {
		return nil, fmt.Errorf("child seed must be 32 bytes, got %d", len(childSeed))
	}
	var xi [mldsa65.SeedSize]byte // SeedSize == 32 per FIPS 204
	h := sha3.NewShake256()
	if _, err := h.Write([]byte("LUX/HIP-0077/mldsa65")); err != nil {
		return nil, fmt.Errorf("shake write label: %w", err)
	}
	if _, err := h.Write(childSeed); err != nil {
		return nil, fmt.Errorf("shake write seed: %w", err)
	}
	if _, err := h.Read(xi[:]); err != nil {
		return nil, fmt.Errorf("shake read xi: %w", err)
	}
	pk, _, err := mldsa65.NewKeyFromSeed(xi[:])
	if err != nil {
		return nil, fmt.Errorf("mldsa65 keygen: %w", err)
	}
	return pk.Bytes(), nil
}

// LoadKeysFromMnemonicEnv loads keys from the LUX_MNEMONIC env var.
// Callers that already know the network id MUST use
// LoadKeysFromMnemonicEnvForNetwork instead — it adds the production-safe
// public-mnemonic guard around this entry point.
func LoadKeysFromMnemonicEnv(numAccounts int) ([]KeyInfo, error) {
	mnemonic := getMnemonicEnv()
	if mnemonic == "" {
		return nil, fmt.Errorf("mnemonic not set (set %s)", MnemonicEnvVar)
	}
	return LoadKeysFromMnemonic(mnemonic, numAccounts)
}

// keyInfoFromPrivateKey creates KeyInfo from raw private key bytes
func keyInfoFromPrivateKey(privKey []byte) (*KeyInfo, error) {
	// Derive EVM address from private key
	evmAddr, err := privateKeyToEVMAddress(privKey)
	if err != nil {
		return nil, fmt.Errorf("failed to derive ETH address: %w", err)
	}

	// Generate a deterministic node ID from the private key
	nodeIDBytes := keccakBytes(append([]byte("node-id:"), privKey...))
	nodeID, err := ids.ToNodeID(nodeIDBytes[:20])
	if err != nil {
		// Fallback to generating from hash
		var nid ids.NodeID
		copy(nid[:], nodeIDBytes[:20])
		nodeID = nid
	}

	// Derive Lux P/X-Chain address (SHA256+RIPEMD160, like Bitcoin)
	secpKey, err := secp256k1.ToPrivateKey(privKey)
	if err != nil {
		return nil, fmt.Errorf("failed to derive lux key: %w", err)
	}
	stakingAddr := secpKey.Address()

	return &KeyInfo{
		NodeID:      nodeID,
		StakerKey:   privKey,
		StakingAddr: stakingAddr,
		EVMAddr:     evmAddr,
	}, nil
}

// privateKeyToEVMAddress derives an EVM address (H160 hex) from a private key
func privateKeyToEVMAddress(privKey []byte) (ids.ShortID, error) {
	key, err := crypto.ToECDSA(privKey)
	if err != nil {
		return ids.ShortID{}, fmt.Errorf("invalid secp256k1 private key: %w", err)
	}
	evmAddr := keccakAddr(key.PublicKey.X, key.PublicKey.Y)
	var addr ids.ShortID
	copy(addr[:], evmAddr[:])
	return addr, nil
}

// keccakBytes returns Keccak256(data) as a []byte slice (saves the
// two-step assignment-then-slice that Go requires for function-returned
// arrays).
func keccakBytes(data []byte) []byte {
	h := keccak256.Sum(data)
	return h[:]
}

// keccakAddr returns Keccak256(pub.X || pub.Y)[12:] — the EVM-format
// 20-byte H160 address used on C-Chain and other EVM letter chains.
func keccakAddr(pubX, pubY *big.Int) (out [20]byte) {
	buf := make([]byte, 64)
	pubX.FillBytes(buf[:32])
	pubY.FillBytes(buf[32:])
	h := keccak256.Sum(buf)
	copy(out[:], h[12:])
	return
}

// genesisMessage returns the Latin genesis message for a network.
func genesisMessage(networkID uint32) string {
	switch networkID {
	case constants.MainnetID:
		return "Lux et Libertas"
	case constants.TestnetID:
		return "Lux et Veritas"
	case constants.DevnetID:
		return "Lux ex Tenebris"
	default:
		return "Fiat Lux"
	}
}

// MnemonicEnvVar is the single canonical env var name for the genesis
// mnemonic. One env, one way:
//   - mainnet/testnet/devnet (IDs 1/2/3): a private hardware-RNG mnemonic
//     loaded from KMS; RefuseLightMnemonicOnProduction rejects all known
//     public mnemonics so the production guard is enforced at derivation.
//   - local (ID 1337): typically set to LightMnemonic for fast dev boot;
//     the same public-mnemonic blacklist still applies, but local IDs
//     are exempt by IsProductionNetwork.
const MnemonicEnvVar = "LUX_MNEMONIC"

// getMnemonicEnv returns the mnemonic from the canonical LUX_MNEMONIC env
// var. No fallback chain — one and only one env name.
func getMnemonicEnv() string {
	return os.Getenv(MnemonicEnvVar)
}

// IsLightMnemonic reports whether the given mnemonic is exactly the
// well-known LightMnemonic dev seed value. Compared in constant time so a
// timing attacker can't probe the running config from outside.
func IsLightMnemonic(mnemonic string) bool {
	return subtleConstantTimeEqual([]byte(mnemonic), []byte(LightMnemonic))
}

// knownPublicMnemonics is the curated set of seeds that anyone in the
// world can derive from. Production deployments MUST refuse all of them.
//
// LightMnemonic is one row; the rest are the most-frequently-cited public
// mnemonics from BIP-39 test vectors, common dev tooling defaults, and
// hardware-wallet demo seeds. Any of these on a production network →
// every derived child key is publicly enumerable.
//
// This list is deliberately not exhaustive — a strict whitelist of
// KMS-loaded mnemonics is the only complete defence. The blacklist
// stops the obvious public-mnemonic footguns; KMS handles the rest.
//
// Closes HIP-0077 red-review F31 (guard previously matched ONE mnemonic
// only; BIP-39 abandon-vector + Hardhat default + Trezor demo all passed).
var knownPublicMnemonics = []string{
	LightMnemonic,
	// BIP-39 test vector #1 (canonical "abandon" mnemonic) — used in
	// every BIP-39 reference implementation as the default test.
	"abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about",
	// BIP-39 test vector #5.
	"legal winner thank year wave sausage worth useful legal winner thank yellow",
	// Hardhat / Foundry default test mnemonic — millions of dev wallets.
	"test test test test test test test test test test test junk",
	// Trezor demo seed.
	"witch collapse practice feed shame open despair creek road again ice least",
	// Common "all 1s" BIP-39 vector.
	"abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon",
}

// IsKnownPublicMnemonic reports whether the given mnemonic appears in any
// well-known public list (LightMnemonic, BIP-39 test vectors, Hardhat
// default, hardware-wallet demos, etc.). Compared in constant time per
// entry so a timing attacker can't probe which entry matched.
//
// Production deployments MUST refuse any mnemonic for which this returns
// true. A real production seed comes from a hardware RNG and lives in
// KMS — never from a publicly-known list.
func IsKnownPublicMnemonic(mnemonic string) bool {
	matched := false
	mb := []byte(mnemonic)
	for _, candidate := range knownPublicMnemonics {
		// Constant-time per-entry compare so the loop reveals neither
		// which entry matched nor whether it short-circuited.
		if subtleConstantTimeEqual(mb, []byte(candidate)) {
			matched = true
		}
	}
	return matched
}

// IsProductionNetwork reports whether the given numeric network ID is on
// the list of *production* Lux networks. Local / primary-local meshes
// (network IDs >= 1337, including constants.LocalID = 1337) deliberately
// allow public mnemonics like LightMnemonic; mainnet, testnet and any
// other reserved low-ID network refuse them.
//
// Canonical network ID map (mirrors lux/constants):
//   - 1     mainnet (production — refuses public mnemonics)
//   - 2     testnet (production-grade staging — refuses public mnemonics)
//   - 3     devnet  (production-grade dev mesh — refuses public mnemonics)
//   - 1337  LocalID (free-form local dev — allows LightMnemonic)
//   - >= 1337 any tenant local / dev mesh (allows LightMnemonic)
//   - 4..1336 reserved; treated as production by default
func IsProductionNetwork(networkID uint32) bool {
	switch networkID {
	case constants.MainnetID, constants.TestnetID:
		return true
	}
	// Anything below 1337 that isn't an explicit dev ID is treated as
	// production. 1337+ is the dev mesh range per HIP-0077.
	return networkID < 1337
}

// RefuseLightMnemonicOnProduction returns a non-nil error iff the running
// process is configured with any publicly-known mnemonic (LightMnemonic,
// BIP-39 test vectors, Hardhat / Trezor demos, …) AND the supplied
// networkID is a production network. Runtime guard required by HIP-0077
// §"Mnemonic exposure" / "Auto-funded blast radius".
//
// **Headline contract (HIP-0077 red-review F30):** every public callable
// that turns environment state into derived keys MUST invoke this guard
// itself. Callers may NOT rely on operator discipline to remember the
// call — `LoadKeysFromMnemonicEnv` and `BuildConfigFromEnv` invoke this
// directly so a misconfigured production deployment fails closed at the
// derivation point, never silently produces public-mnemonic-derived
// signing keys.
//
// The guard covers the full public-mnemonic blacklist (HIP-0077 red-review
// F31): LightMnemonic, BIP-39 abandon vector, Hardhat default, Trezor
// demo, etc. See knownPublicMnemonics.
func RefuseLightMnemonicOnProduction(networkID uint32) error {
	mnemonic := getMnemonicEnv()
	if mnemonic == "" {
		// No mnemonic at all → not our problem here; the caller will get
		// a "mnemonic not set" error from LoadKeysFromMnemonicEnv.
		return nil
	}
	if !IsKnownPublicMnemonic(mnemonic) {
		return nil
	}
	if !IsProductionNetwork(networkID) {
		return nil
	}
	return fmt.Errorf(
		"refusing to derive keys: a publicly-known mnemonic is set on production "+
			"network %d (mainnet/testnet/<1337). Public mnemonics (LightMnemonic, "+
			"BIP-39 test vectors, Hardhat/Trezor demos) are deterministic — anyone "+
			"can derive every child key. Set %s with a private hardware-RNG "+
			"mnemonic loaded from KMS, or run on a dev network ID (>= 1337)",
		networkID, MnemonicEnvVar,
	)
}

// LoadKeysFromMnemonicEnvForNetwork is the production-safe variant of
// LoadKeysFromMnemonicEnv: it invokes the public-mnemonic guard before
// any derivation. This is the function every node-start path SHOULD call
// instead of LoadKeysFromMnemonicEnv directly.
//
// Closes HIP-0077 red-review F30 (the prior LoadKeysFromMnemonicEnv was
// guard-free and operators could silently derive on production from
// the public LightMnemonic).
func LoadKeysFromMnemonicEnvForNetwork(networkID uint32, numAccounts int) ([]KeyInfo, error) {
	if err := RefuseLightMnemonicOnProduction(networkID); err != nil {
		return nil, err
	}
	return LoadKeysFromMnemonicEnv(numAccounts)
}

// subtleConstantTimeEqual is a thin wrapper so we don't pull crypto/subtle
// into this package's import surface for the one comparison.
func subtleConstantTimeEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	var v byte
	for i := range a {
		v |= a[i] ^ b[i]
	}
	return v == 0
}

// LoadBIP44WalletKeysFromMnemonic derives N spending keys from a BIP39
// mnemonic on the canonical BIP44 path m/44'/9000'/0'/0/i (purpose 44'
// / coin 9000' / account 0' hardened; change 0 / index i non-hardened).
// Returns KeyInfo entries with NodeID, StakingAddr (P/X bech32 base),
// and EVMAddr populated; no BLS/MLDSA — these are user spending keys,
// not validator node identities.
//
// This is the same derivation that BuildBIP44WalletAllocations uses,
// reshaped to return KeyInfo so callers (e.g. BuildConfigFromEnv) can
// flow these straight into buildConfigFromKeyInfos as account holders.
func LoadBIP44WalletKeysFromMnemonic(mnemonic string, numAccounts int) ([]KeyInfo, error) {
	if !bip39.IsMnemonicValid(mnemonic) {
		return nil, fmt.Errorf("invalid mnemonic")
	}
	seed := bip39.NewSeed(mnemonic, "")
	masterKey, err := bip32.NewMasterKey(seed)
	if err != nil {
		return nil, fmt.Errorf("failed to create master key: %w", err)
	}
	purpose, err := masterKey.NewChildKey(bip32.FirstHardenedChild + 44)
	if err != nil {
		return nil, fmt.Errorf("derive purpose 44': %w", err)
	}
	coinType, err := purpose.NewChildKey(bip32.FirstHardenedChild + 9000)
	if err != nil {
		return nil, fmt.Errorf("derive coin 9000': %w", err)
	}
	account, err := coinType.NewChildKey(bip32.FirstHardenedChild + 0)
	if err != nil {
		return nil, fmt.Errorf("derive account 0': %w", err)
	}
	change, err := account.NewChildKey(0) // non-hardened
	if err != nil {
		return nil, fmt.Errorf("derive change 0: %w", err)
	}

	keys := make([]KeyInfo, 0, numAccounts)
	for i := 0; i < numAccounts; i++ {
		child, err := change.NewChildKey(uint32(i)) // non-hardened
		if err != nil {
			return nil, fmt.Errorf("derive BIP44 wallet key %d: %w", i, err)
		}
		ki, err := keyInfoFromPrivateKey(child.Key)
		if err != nil {
			return nil, fmt.Errorf("keyInfo BIP44 wallet %d: %w", i, err)
		}
		keys = append(keys, *ki)
	}
	return keys, nil
}

// BuildBIP44WalletAllocations derives wallet keys on the canonical BIP44
// path m/44'/9000'/0'/0/i (purpose-44' / coin-9000' / account-0' hardened;
// change-0 / index-i NON-hardened). 9000 is the BIP44 coin type Lux uses
// by canonical convention; any BIP44-conformant wallet using that coin
// type derives the same key set. Returns free (no vesting) spending
// allocations for each key.
//
// Use this instead of BuildWalletAllocations when the receiving consumer
// (e.g. a chain-bootstrap CLI) expects classical BIP44 web-wallet
// addresses. BuildWalletAllocations uses a Lux-internal hardened layout
// (m/44'/9000'/nid'/1'/i') and will not match the web wallet.
//
// The networkID parameter is preserved for symmetry with
// BuildWalletAllocations but is unused — canonical BIP44 is a single
// tree shared across networks (the same seed produces the same addresses
// regardless of chain).
func BuildBIP44WalletAllocations(networkID uint32, numKeys int, amountPerKey uint64) ([]Allocation, error) {
	_ = networkID // see comment above
	mnemonic := getMnemonicEnv()
	if mnemonic == "" {
		return nil, fmt.Errorf("wallet allocations require %s env var", MnemonicEnvVar)
	}
	if !bip39.IsMnemonicValid(mnemonic) {
		return nil, fmt.Errorf("invalid mnemonic for wallet key derivation")
	}

	seed := bip39.NewSeed(mnemonic, "")
	masterKey, err := bip32.NewMasterKey(seed)
	if err != nil {
		return nil, fmt.Errorf("failed to create master key: %w", err)
	}
	purpose, err := masterKey.NewChildKey(bip32.FirstHardenedChild + 44)
	if err != nil {
		return nil, fmt.Errorf("derive purpose 44': %w", err)
	}
	coinType, err := purpose.NewChildKey(bip32.FirstHardenedChild + 9000)
	if err != nil {
		return nil, fmt.Errorf("derive coin 9000': %w", err)
	}
	account, err := coinType.NewChildKey(bip32.FirstHardenedChild + 0)
	if err != nil {
		return nil, fmt.Errorf("derive account 0': %w", err)
	}
	change, err := account.NewChildKey(0) // non-hardened
	if err != nil {
		return nil, fmt.Errorf("derive change 0: %w", err)
	}

	allocations := make([]Allocation, 0, numKeys)
	for i := 0; i < numKeys; i++ {
		childKey, err := change.NewChildKey(uint32(i)) // non-hardened
		if err != nil {
			return nil, fmt.Errorf("failed to derive wallet key %d: %w", i, err)
		}

		utxoPrivKey, err := secp256k1.ToPrivateKey(childKey.Key)
		if err != nil {
			return nil, fmt.Errorf("failed to create secp256k1 key %d: %w", i, err)
		}
		stakingAddr := utxoPrivKey.Address()

		evmPrivKey, err := crypto.ToECDSA(childKey.Key)
		if err != nil {
			return nil, fmt.Errorf("failed to create ECDSA key %d: %w", i, err)
		}
		evmAddr := keccakAddr(evmPrivKey.PublicKey.X, evmPrivKey.PublicKey.Y)
		var evmShortID ids.ShortID
		copy(evmShortID[:], evmAddr[:])

		log.Debug("derived BIP44 wallet allocation",
			"i", i,
			"utxoAddr", stakingAddr.String(),
			"evmAddr", fmt.Sprintf("0x%x", evmAddr),
		)

		allocations = append(allocations, Allocation{
			EVMAddr:       evmShortID,
			UTXOAddr:      stakingAddr,
			InitialAmount: amountPerKey,
		})
	}

	return allocations, nil
}

// BuildConfigFromEnv builds genesis config from environment variables.
// Checks in order: KEYS_DIR, LUX_MNEMONIC, PRIVATE_KEY.
//
// Architecture:
//   - X-Chain: DefaultNumAccounts × allocationPerKey, immediate spend
//   - P-Chain: DefaultNumAccounts × allocationPerKey, immediate spend
//   - C-Chain: treasury 0x9011...4714 gets 2T LUX (no per-account alloc)
func BuildConfigFromEnv(networkID uint32, numValidators int, allocationPerKey uint64) (*Config, error) {
	var err error

	if allocationPerKey == 0 {
		allocationPerKey = DefaultAllocationPerAccount
	}

	// Load validator keys from directory (if available)
	var validatorKeys []KeyInfo
	if keysDir := os.Getenv("KEYS_DIR"); keysDir != "" {
		validatorKeys, err = LoadKeysFromDir(keysDir)
		if err != nil {
			validatorKeys = nil
		}
	}

	// Load account keys from mnemonic at the canonical BIP44 path
	// m/44'/9000'/0'/0/i (the same path the genesis CLI's -bip44-wallet-keys
	// flag uses, and the same path the derive100 / luxfi wallet UIs use).
	// This is what every user-facing tool that scans the chain for funded
	// addresses will expect — derive100 against $LUX_MNEMONIC and the
	// addresses here must match byte-for-byte.
	//
	// NOTE: this is intentionally a SEPARATE concern from
	// LoadKeysFromMnemonic, which derives Lux-internal validator NODE
	// identities (NodeID + BLS + ML-DSA-65) on the hardened path
	// m/44'/9000'/nid'/1'/i'. Wallet keys are user-spending keys;
	// validator keys are node-signing identities. Don't conflate.
	var allKeys []KeyInfo
	if mnemonic := getMnemonicEnv(); mnemonic != "" {
		numAccounts := DefaultNumAccounts
		allKeys, err = LoadBIP44WalletKeysFromMnemonic(mnemonic, numAccounts)
		if err != nil {
			allKeys = nil
		}
	}

	// Validator NODE identities use the Lux-internal hardened branch
	// (NodeID + BLS + ML-DSA-65). Separate concern from wallet keys.
	var validatorNodeKeys []KeyInfo
	if mnemonic := getMnemonicEnv(); mnemonic != "" {
		if numValidators == 0 {
			numValidators = 3
		}
		validatorNodeKeys, _ = LoadKeysFromMnemonic(mnemonic, numValidators)
	}

	// Combine: use dir keys for validators when present, otherwise the
	// internal-branch mnemonic-derived node identities. Allocation set is
	// the canonical BIP44 wallet keys.
	if len(validatorKeys) > 0 && len(allKeys) > 0 {
		return buildConfigFromKeyInfos(networkID, validatorKeys, allKeys, allocationPerKey)
	}
	if len(validatorKeys) > 0 {
		return BuildConfigFromKeys(networkID, os.Getenv("KEYS_DIR"), allocationPerKey)
	}
	if len(validatorNodeKeys) > 0 && len(allKeys) > 0 {
		return buildConfigFromKeyInfos(networkID, validatorNodeKeys, allKeys, allocationPerKey)
	}
	if len(allKeys) > 0 {
		if numValidators == 0 {
			numValidators = 3
		}
		vKeys := allKeys[:numValidators]
		return buildConfigFromKeyInfos(networkID, vKeys, allKeys, allocationPerKey)
	}

	// Try single private key
	if privKey := os.Getenv("PRIVATE_KEY"); privKey != "" {
		keyInfo, err := LoadKeyFromEnv()
		if err == nil {
			return buildConfigFromKeyInfos(networkID, []KeyInfo{*keyInfo}, []KeyInfo{*keyInfo}, allocationPerKey)
		}
	}

	// Fall back to default keys directory (with fee key derivation)
	home, _ := os.UserHomeDir()
	keysDir := filepath.Join(home, ".lux", "keys")
	return BuildConfigFromKeys(networkID, keysDir, allocationPerKey)
}

// buildConfigFromKeyInfos creates config from KeyInfo slices.
//
// Architecture:
//   - X-Chain: N accounts × allocationPerKey LUX each, FREE (spendable at genesis)
//   - P-Chain: N accounts × allocationPerKey LUX each, FREE (spendable at genesis)
//   - C-Chain: treasury 0x9011...4714 gets 2T LUX
//   - Validators contribute a long-locked stake allocation so the
//     ProtocolVM can weight them; that locked-stake allocation is the
//     ONLY place an UnlockSchedule is attached. User wallets stay free.
//
// validatorKeys: first N accounts that become initial stakers
// allKeys: all accounts that receive X-Chain/P-Chain allocations
func buildConfigFromKeyInfos(networkID uint32, validatorKeys []KeyInfo, allKeys []KeyInfo, allocationPerKey uint64) (*Config, error) {
	if len(validatorKeys) == 0 {
		return nil, fmt.Errorf("no keys provided")
	}

	if allocationPerKey == 0 {
		allocationPerKey = DefaultAllocationPerAccount
	}

	// Wallet allocations are clean: InitialAmount only, no UnlockSchedule.
	// The node builder puts InitialAmount on BOTH X-Chain and P-Chain as a
	// free (no locktime) UTXO at the same address, so each account ends up
	// with `allocationPerKey` spendable on X AND `allocationPerKey`
	// spendable on P. No vesting decoration — keeps the genesis blob
	// small enough to fit a single zapdb batch (1000 keys × 100-period
	// vesting schedules overflows the batch-write limit and bricks boot).
	allocations := make([]Allocation, 0, len(allKeys)+len(validatorKeys))
	stakedFunds := make([]ids.ShortID, 0, len(validatorKeys))
	stakers := make([]Staker, 0, len(validatorKeys))

	for _, key := range allKeys {
		allocations = append(allocations, Allocation{
			EVMAddr:       key.EVMAddr,
			UTXOAddr:      key.StakingAddr,
			InitialAmount: allocationPerKey,
		})
	}

	// Track which staking addresses already have an allocation. Validator
	// addresses that aren't already in allKeys need their own stake
	// allocation; otherwise the ProtocolVM rejects the genesis
	// with "validator has not weight" because there's no UTXO at the
	// validator's stakedFunds address.
	allocByAddr := make(map[ids.ShortID]bool, len(allocations))
	for _, a := range allocations {
		allocByAddr[a.UTXOAddr] = true
	}
	// stakeAmount is the locktime-locked stake each validator
	// contributes. Matches the prior genesis behaviour (3B nLUX across
	// three future-locktime buckets), so the ProtocolVM sees a
	// non-zero stake for each initial staker.
	const stakeAmount = uint64(1_000_000_000)
	now := uint64(time.Now().Unix())
	stakeUnlockSchedule := []LockedAmount{
		{Amount: stakeAmount, Locktime: now + 5*365*24*60*60},  // ~5y
		{Amount: stakeAmount, Locktime: now + 10*365*24*60*60}, // ~10y
		{Amount: stakeAmount, Locktime: now + 20*365*24*60*60}, // ~20y
	}

	for _, key := range validatorKeys {

		stakedFunds = append(stakedFunds, key.StakingAddr)

		// Ensure the validator's staking address has a corresponding
		// allocation. If absent, add one with locked stake so the
		// ProtocolVM can weight the staker.
		if !allocByAddr[key.StakingAddr] {
			allocations = append(allocations, Allocation{
				EVMAddr:        key.EVMAddr,
				UTXOAddr:       key.StakingAddr,
				InitialAmount:  0,
				UnlockSchedule: stakeUnlockSchedule,
			})
			allocByAddr[key.StakingAddr] = true
		}

		staker := Staker{
			NodeID:        key.NodeID,
			RewardAddress: key.StakingAddr,
			DelegationFee: 20000,
		}

		if len(key.BLSPublicKey) > 0 {
			staker.Signer = &ProofOfPossession{
				PublicKey:         fmt.Sprintf("0x%x", key.BLSPublicKey),
				ProofOfPossession: fmt.Sprintf("0x%x", key.BLSProofOfPossession),
			}
		}

		if len(key.MLDSAPublicKey) > 0 {
			staker.PQIdentity = &PQIdentity{
				MLDSAPublicKey: fmt.Sprintf("0x%x", key.MLDSAPublicKey),
			}
			if len(key.CoronaPublicKey) > 0 {
				staker.PQIdentity.CoronaPublicKey = fmt.Sprintf("0x%x", key.CoronaPublicKey)
			}
			// BLSCertificate and CoronaCertificate are generated at validator
			// registration time (requires ML-DSA private key to sign).
		}

		stakers = append(stakers, staker)
	}

	// C-Chain: treasury-only allocation (2T LUX)
	cchainGenesis, err := buildCChainGenesisTreasury(networkID)
	if err != nil {
		return nil, fmt.Errorf("failed to build C-chain genesis: %w", err)
	}

	// Start time 1 hour ago: validators are active (for 1 year),
	// and vested funds with past locktimes (2020-2025) are spendable.
	startTime := uint64(time.Now().Unix()) - 3600

	return &Config{
		NetworkID:                  networkID,
		Allocations:                allocations,
		StartTime:                  startTime,
		InitialStakeDuration:       365 * 24 * 60 * 60,
		InitialStakeDurationOffset: 5400,
		InitialStakedFunds:         stakedFunds,
		InitialStakers:             stakers,
		CChainGenesis:              cchainGenesis,
		Message:                    genesisMessage(networkID),
	}, nil
}
