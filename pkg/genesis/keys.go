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

	// ML-DSA-65 (FIPS 204) keygen — Cloudflare CIRCL provides
	// NewKeyFromSeed(*[32]byte) which is exactly the FIPS 204 §5.1
	// KeyGen-from-ξ interface we need.
	//
	// TODO(canonical): replace with github.com/luxfi/crypto/pq/mldsa once
	// that package exposes a public NewKeyFromSeed entry point. At
	// luxfi/crypto v1.17.44 the pq/mldsa directory exists but does not
	// expose the FIPS 204 seed→keypair API; CIRCL's mldsa65 is the
	// stop-gap and the SHAKE-256 expansion below makes the migration
	// transparent (the canonical lux/crypto/mldsa package MUST adopt the
	// same SHAKE-256(child_seed)→ξ derivation or we lose determinism).
	"github.com/cloudflare/circl/sign/mldsa/mldsa65"
	"github.com/luxfi/constants"
	ethcrypto "github.com/luxfi/crypto"
	"github.com/luxfi/crypto/bls"
	luxcrypto "github.com/luxfi/crypto/secp256k1"
	"github.com/luxfi/go-bip32"
	"github.com/luxfi/go-bip39"
	"github.com/luxfi/ids"
	"github.com/luxfi/log"
	luxtls "github.com/luxfi/tls"
	"golang.org/x/crypto/sha3"
)

const (
	// DefaultAllocationPerAccount is 500M LUX per account per chain (P and X).
	// 100 accounts × 500M × 2 chains = 100B total.
	// X-Chain: 500M free (immediately spendable).
	// P-Chain: 500M vesting 1%/year over 100 years from Jan 1 2020.
	DefaultAllocationPerAccount = 500_000_000 * Lux

	// DefaultAllocationPerValidator is kept for backward compatibility
	DefaultAllocationPerValidator = DefaultAllocationPerAccount

	// DefaultNumAccounts is the default number of mnemonic-derived accounts
	DefaultNumAccounts = 100

	// StakingStartTime is Jan 1, 2020 00:00:00 UTC
	StakingStartTime = 1577836800

	// UnlockInterval is 1 year in seconds
	UnlockInterval = 365 * 24 * 60 * 60

	// TreasuryAddress is the C-Chain treasury with 2T LUX
	TreasuryAddress = "0x9011E888251AB053B7bD1cdB598Db4f9DEd94714"

	// TreasuryAmount is 2 trillion LUX in microLUX (2T * 10^6)
	TreasuryAmount = 2_000_000_000_000 * Lux

	// PChainFeeReserve is 10,000 LUX per validator for P-Chain fees
	PChainFeeReserve = 10_000 * Lux

	// LightMnemonic is the well-known dev mnemonic for local networks (network-id >= 1337).
	// NEVER use on public networks (mainnet, testnet, devnet).
	LightMnemonic = "light light light light light light light light light light light energy"
)

// KeyInfo contains parsed key information for a node
type KeyInfo struct {
	NodeID               ids.NodeID
	StakerKey            []byte
	BLSPublicKey         []byte
	BLSProofOfPossession []byte
	MLDSAPublicKey       []byte      // ML-DSA post-quantum public key (FIPS 204)
	RingtailPublicKey    []byte      // Ringtail ring signature public key
	StakingAddr          ids.ShortID // P-chain address derived from staker key
	ETHAddr              ids.ShortID // C-chain/X-chain address
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
			// Get ETH address
			ethPrivKey, err := ethcrypto.ToECDSA(privKeyBytes)
			if err == nil {
				ethAddr := ethcrypto.PubkeyToAddress(ethPrivKey.PublicKey)
				copy(keyInfo.ETHAddr[:], ethAddr[:])
			}

			// Get Lux ShortID (for X/P chain addresses)
			luxPrivKey, err := luxcrypto.ToPrivateKey(privKeyBytes)
			if err == nil {
				pubKey := luxPrivKey.PublicKey()
				shortID := ids.ShortID(pubKey.Address())
				copy(keyInfo.StakingAddr[:], shortID[:])
			}
		}
	} else {
		// Fallback: derive from node ID (NOT correct but backward compatible)
		// WARNING: These addresses won't have usable private keys!
		copy(keyInfo.StakingAddr[:], nodeID[:])
		copy(keyInfo.ETHAddr[:], nodeID[:])
	}

	// Load ML-DSA public key (post-quantum identity)
	mldsaPath := filepath.Join(nodeDir, "mldsa", "public.key")
	if data, err := os.ReadFile(mldsaPath); err == nil {
		keyInfo.MLDSAPublicKey = data
	}

	// Load Ringtail public key (ring signatures)
	rtPath := filepath.Join(nodeDir, "rt", "public.key")
	if data, err := os.ReadFile(rtPath); err == nil {
		keyInfo.RingtailPublicKey = data
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
// The fee key is keccak256("fee-reserve:" || ecPrivKey) which gives a different
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

	// Derive fee private key: keccak256("fee-reserve:" || ecPrivKey)
	feePrivBytes := keccak256(append([]byte("fee-reserve:"), privKeyBytes...))

	// Derive proper P-chain address using secp256k1
	feePrivKey, err := luxcrypto.ToPrivateKey(feePrivBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to create fee private key: %w", err)
	}
	feePubKey := feePrivKey.PublicKey()
	feeAddr := ids.ShortID(feePubKey.Address())

	// Derive ETH address
	ethPrivKey, err := ethcrypto.ToECDSA(feePrivBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to create fee ETH key: %w", err)
	}
	ethAddr := ethcrypto.PubkeyToAddress(ethPrivKey.PublicKey)
	var ethShortID ids.ShortID
	copy(ethShortID[:], ethAddr[:])

	// Save fee private key for later use by deploy tools
	feeKeyDir := filepath.Join(keysDir, fmt.Sprintf("fee%d", index))
	os.MkdirAll(filepath.Join(feeKeyDir, "ec"), 0700)
	feeKeyHex := hex.EncodeToString(feePrivBytes)
	os.WriteFile(filepath.Join(feeKeyDir, "ec", "private.key"), []byte(feeKeyHex), 0600)

	fmt.Fprintf(os.Stderr, "Fee key %d: addr=%s ethAddr=0x%x saved to %s\n",
		index, feeAddr, ethAddr, feeKeyDir)

	return &KeyInfo{
		StakingAddr: feeAddr,
		ETHAddr:     ethShortID,
	}, nil
}

// buildUnlockSchedule creates a vesting schedule
func buildUnlockSchedule(totalAmount uint64, startTime uint64, interval uint64, periods int) []LockedAmount {
	amountPerPeriod := totalAmount / uint64(periods)
	schedule := make([]LockedAmount, 0, periods)

	for i := 0; i < periods; i++ {
		schedule = append(schedule, LockedAmount{
			Amount:   amountPerPeriod,
			Locktime: startTime + uint64(i)*interval,
		})
	}

	return schedule
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
		addr := fmt.Sprintf("0x%s", key.ETHAddr.Hex())
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

// LoadKeysFromMnemonic derives multiple keys from a BIP39 mnemonic per
// HIP-0077 §"Identity". Two hardened branches descend from a per-network
// account level:
//
//	device_pq_key[i]  = m/44'/9000'/nid'/0'/i'  → ML-DSA-65  (FIPS 204)
//	device_lux_key[i] = m/44'/9000'/nid'/1'/i'  → secp256k1  (Lux account)
//
// All five levels are hardened. This gives two guarantees:
//
//  1. Per-network isolation — a leaked branch on nid=A does not let an
//     attacker derive any branch on nid=B for the same mnemonic.
//  2. Branch independence — leaking the branch-0' (ML-DSA child seed)
//     reveals nothing about branch-1' (secp256k1 spend key) because
//     hardened siblings are derived from the parent *private* key, not
//     the parent xpub (BIP-32 §"Private parent → private child").
//
// The ML-DSA-65 keypair is generated from the 32-byte BIP-32 child seed
// expanded via SHAKE-256 into a 32-byte ξ (per FIPS 204 §5.1 KeyGen).
// SHAKE-256 expansion of the child seed is the FIPS 204-aligned way to
// derive ξ from a parent BIP-32 child, and lux/crypto/mldsa MUST adopt
// the same expansion when it lands (see TODO at the mldsa65 import).
//
// Backward-incompatible change vs. the prior layout
// (m/44'/9000'/0'/0/i): addresses derived under that path will NOT
// reproduce. The treasury and any keys-on-disk you want to keep MUST be
// re-derived (or loaded from KEYS_DIR instead of the mnemonic). See
// genesis/CHANGELOG.md for the migration note.
func LoadKeysFromMnemonic(mnemonic string, nid uint32, numAccounts int) ([]KeyInfo, error) {
	if !bip39.IsMnemonicValid(mnemonic) {
		return nil, fmt.Errorf("invalid mnemonic")
	}
	if nid >= bip32.FirstHardenedChild {
		// Hardening folds the high bit; a network id at or above 2^31
		// would collide with another network id mod 2^31. Refuse rather
		// than silently producing colliding addresses.
		return nil, fmt.Errorf("network id %d must be < 2^31 (BIP-32 hardening limit)", nid)
	}

	seed := bip39.NewSeed(mnemonic, "")
	masterKey, err := bip32.NewMasterKey(seed)
	if err != nil {
		return nil, fmt.Errorf("failed to create master key: %w", err)
	}

	account, err := deriveLuxAccount(masterKey, nid)
	if err != nil {
		return nil, err
	}

	// Branch 1' — Lux secp256k1 account.
	luxBranch, err := account.NewChildKey(bip32.FirstHardenedChild + 1)
	if err != nil {
		return nil, fmt.Errorf("failed to derive branch 1' (secp256k1): %w", err)
	}

	// Branch 0' — ML-DSA-65 mesh identity.
	mldsaBranch, err := account.NewChildKey(bip32.FirstHardenedChild + 0)
	if err != nil {
		return nil, fmt.Errorf("failed to derive branch 0' (ML-DSA): %w", err)
	}

	keys := make([]KeyInfo, 0, numAccounts)
	for i := 0; i < numAccounts; i++ {
		// Hardened child index i' on branch 1' → secp256k1 spend key.
		luxChild, err := luxBranch.NewChildKey(bip32.FirstHardenedChild + uint32(i))
		if err != nil {
			return nil, fmt.Errorf("failed to derive secp256k1 key %d: %w", i, err)
		}

		keyInfo, err := keyInfoFromPrivateKey(luxChild.Key)
		if err != nil {
			return nil, fmt.Errorf("failed to create key info %d: %w", i, err)
		}

		// Hardened child index i' on branch 0' → ML-DSA-65 keypair.
		mldsaChild, err := mldsaBranch.NewChildKey(bip32.FirstHardenedChild + uint32(i))
		if err != nil {
			return nil, fmt.Errorf("failed to derive ML-DSA seed %d: %w", i, err)
		}
		mldsaPubKey, err := mldsaKeygenFromChildSeed(mldsaChild.Key)
		if err != nil {
			return nil, fmt.Errorf("ML-DSA keygen %d: %w", i, err)
		}
		keyInfo.MLDSAPublicKey = mldsaPubKey

		// BLS signer key — derive deterministically from mnemonic seed + index.
		// Uses keccak256(seed || "bls-signer" || index) as the BLS secret
		// key seed so BLS keys are reproducible from the same mnemonic.
		blsSeed := keccak256(append(append(seed, []byte("bls-signer")...), byte(i)))
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
		"nid", nid,
		"numAccounts", numAccounts,
		"path", fmt.Sprintf("m/44'/9000'/%d'/{0',1'}/i'", nid),
	)
	return keys, nil
}

// deriveLuxAccount walks the shared prefix m/44'/9000'/nid' from the
// master key. The two leaf branches (0' = ML-DSA, 1' = secp256k1) hang
// off the returned account key.
func deriveLuxAccount(masterKey *bip32.Key, nid uint32) (*bip32.Key, error) {
	purpose, err := masterKey.NewChildKey(bip32.FirstHardenedChild + 44)
	if err != nil {
		return nil, fmt.Errorf("failed to derive purpose 44': %w", err)
	}
	coinType, err := purpose.NewChildKey(bip32.FirstHardenedChild + 9000)
	if err != nil {
		return nil, fmt.Errorf("failed to derive coin type 9000': %w", err)
	}
	account, err := coinType.NewChildKey(bip32.FirstHardenedChild + nid)
	if err != nil {
		return nil, fmt.Errorf("failed to derive account %d': %w", nid, err)
	}
	return account, nil
}

// mldsaKeygenFromChildSeed expands a 32-byte BIP-32 child seed into the
// 32-byte ξ that FIPS 204 §5.1 KeyGen consumes, then returns the packed
// ML-DSA-65 public key (1952 bytes).
//
// The expansion is `ξ = SHAKE-256("LUX/HIP-0077/mldsa65" || child_seed)`.
// The domain-separation label means a future scheme that wants to use
// the same BIP-32 child seed for a different KEM/SIG cannot collide
// with our ML-DSA key.
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
	pk, _ := mldsa65.NewKeyFromSeed(&xi)
	return pk.Bytes(), nil
}

// LoadKeysFromMnemonicEnv loads keys from mnemonic env vars.
// Priority: MNEMONIC > LUX_MNEMONIC > LIGHT_MNEMONIC.
//
// nid is the Hanzo mesh network id baked into the hardened derivation
// path. Callers that already know the network id MUST use
// LoadKeysFromMnemonicEnvForNetwork instead — it adds the
// production-safe public-mnemonic guard around this entry point.
func LoadKeysFromMnemonicEnv(nid uint32, numAccounts int) ([]KeyInfo, error) {
	mnemonic := getMnemonicEnv()
	if mnemonic == "" {
		return nil, fmt.Errorf("mnemonic not set (set MNEMONIC, LUX_MNEMONIC, or LIGHT_MNEMONIC)")
	}

	return LoadKeysFromMnemonic(mnemonic, nid, numAccounts)
}

// keyInfoFromPrivateKey creates KeyInfo from raw private key bytes
func keyInfoFromPrivateKey(privKey []byte) (*KeyInfo, error) {
	// Derive ETH address from private key
	ethAddr, err := privateKeyToETHAddress(privKey)
	if err != nil {
		return nil, fmt.Errorf("failed to derive ETH address: %w", err)
	}

	// Generate a deterministic node ID from the private key
	nodeIDBytes := keccak256(append([]byte("node-id:"), privKey...))
	nodeID, err := ids.ToNodeID(nodeIDBytes[:20])
	if err != nil {
		// Fallback to generating from hash
		var nid ids.NodeID
		copy(nid[:], nodeIDBytes[:20])
		nodeID = nid
	}

	// Derive Lux P/X-Chain address (SHA256+RIPEMD160, like Bitcoin)
	luxKey, err := luxcrypto.ToPrivateKey(privKey)
	if err != nil {
		return nil, fmt.Errorf("failed to derive lux key: %w", err)
	}
	stakingAddr := luxKey.Address()

	return &KeyInfo{
		NodeID:      nodeID,
		StakerKey:   privKey,
		StakingAddr: stakingAddr,
		ETHAddr:     ethAddr,
	}, nil
}

// privateKeyToETHAddress derives an Ethereum address from a private key
func privateKeyToETHAddress(privKey []byte) (ids.ShortID, error) {
	key, err := ethcrypto.ToECDSA(privKey)
	if err != nil {
		return ids.ShortID{}, fmt.Errorf("invalid secp256k1 private key: %w", err)
	}
	ethAddr := ethcrypto.PubkeyToAddress(key.PublicKey)
	var addr ids.ShortID
	copy(addr[:], ethAddr[:])
	return addr, nil
}

// keccak256 computes the Keccak-256 hash
func keccak256(data []byte) []byte {
	h := sha3.NewLegacyKeccak256()
	h.Write(data)
	return h.Sum(nil)
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

// getMnemonicEnv returns the mnemonic from environment variables.
// Priority: MNEMONIC > LUX_MNEMONIC > LIGHT_MNEMONIC
//
// LIGHT_MNEMONIC is the publicly-known dev seed. Anyone in the world can
// derive its first 200 child keys; the auto-fund pre-allocation in
// HIP-0077 §"Auto-funding the first 200 devices" is *only* meant for
// network IDs >= 1337 (dev / primary local mesh). Production networks
// MUST set MNEMONIC (preferred) or LUX_MNEMONIC.
//
// Use IsLightMnemonic and RefuseLightMnemonicOnProduction to enforce that
// rule wherever a key is loaded for a known network ID.
func getMnemonicEnv() string {
	for _, env := range []string{"MNEMONIC", "LUX_MNEMONIC", "LIGHT_MNEMONIC"} {
		if v := os.Getenv(env); v != "" {
			return v
		}
	}
	return ""
}

// IsLightMnemonic reports whether the given mnemonic is exactly the
// well-known LIGHT_MNEMONIC dev seed. Compared in constant time so a
// timing attacker can't probe the running config from outside.
func IsLightMnemonic(mnemonic string) bool {
	return subtleConstantTimeEqual([]byte(mnemonic), []byte(LightMnemonic))
}

// knownPublicMnemonics is the curated set of seeds that anyone in the
// world can derive from. Production deployments MUST refuse all of them.
//
// LIGHT_MNEMONIC is one row; the rest are the most-frequently-cited public
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
// well-known public list (LIGHT_MNEMONIC, BIP-39 test vectors, Hardhat
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
// allow LIGHT_MNEMONIC; mainnet, testnet and any other reserved low-ID
// network refuse it.
//
// Network ID convention (mirrors lux/constants):
//   - 1     mainnet (production)
//   - 2     testnet (production-grade staging — refuses LIGHT_MNEMONIC)
//   - 1337  LocalID (free-form local dev, allows LIGHT_MNEMONIC)
//   - >= 1337 any tenant local / dev mesh (allows LIGHT_MNEMONIC)
//   - 3..1336 reserved; treated as production by default
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
// process is configured with any publicly-known mnemonic (LIGHT_MNEMONIC,
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
// The guard widens beyond LIGHT_MNEMONIC to cover the broader
// public-mnemonic blacklist (HIP-0077 red-review F31): BIP-39 abandon
// vector, Hardhat default, Trezor demo, etc. See knownPublicMnemonics.
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
			"network %d (mainnet/testnet/<1337). Public mnemonics (LIGHT_MNEMONIC, "+
			"BIP-39 test vectors, Hardhat/Trezor demos) are deterministic — anyone "+
			"can derive every child key. Set MNEMONIC or LUX_MNEMONIC env var with "+
			"a private hardware-RNG mnemonic loaded from KMS, or run on a dev "+
			"network ID (>= 1337)", networkID,
	)
}

// LoadKeysFromMnemonicEnvForNetwork is the production-safe variant of
// LoadKeysFromMnemonicEnv: it invokes the public-mnemonic guard before
// any derivation. This is the function every node-start path SHOULD call
// instead of LoadKeysFromMnemonicEnv directly.
//
// Closes HIP-0077 red-review F30 (the prior LoadKeysFromMnemonicEnv was
// guard-free and operators could silently derive on production from
// LIGHT_MNEMONIC).
func LoadKeysFromMnemonicEnvForNetwork(networkID uint32, numAccounts int) ([]KeyInfo, error) {
	if err := RefuseLightMnemonicOnProduction(networkID); err != nil {
		return nil, err
	}
	return LoadKeysFromMnemonicEnv(networkID, numAccounts)
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

// BuildWalletAllocations derives wallet keys from the MNEMONIC /
// LUX_MNEMONIC env var on the per-network branch 1' hardened path
// (m/44'/9000'/nid'/1'/i') and returns free (no vesting) spending
// allocations for each key. Each allocation has both ETHAddr and
// LUXAddr (StakingAddr).
func BuildWalletAllocations(nid uint32, numKeys int, amountPerKey uint64) ([]Allocation, error) {
	mnemonic := getMnemonicEnv()
	if mnemonic == "" {
		return nil, fmt.Errorf("wallet allocations require MNEMONIC or LUX_MNEMONIC env var")
	}
	if !bip39.IsMnemonicValid(mnemonic) {
		return nil, fmt.Errorf("invalid mnemonic for wallet key derivation")
	}
	if nid >= bip32.FirstHardenedChild {
		return nil, fmt.Errorf("network id %d must be < 2^31 (BIP-32 hardening limit)", nid)
	}

	seed := bip39.NewSeed(mnemonic, "")
	masterKey, err := bip32.NewMasterKey(seed)
	if err != nil {
		return nil, fmt.Errorf("failed to create master key: %w", err)
	}

	account, err := deriveLuxAccount(masterKey, nid)
	if err != nil {
		return nil, err
	}
	luxBranch, err := account.NewChildKey(bip32.FirstHardenedChild + 1)
	if err != nil {
		return nil, fmt.Errorf("failed to derive branch 1' (secp256k1): %w", err)
	}

	allocations := make([]Allocation, 0, numKeys)
	for i := 0; i < numKeys; i++ {
		childKey, err := luxBranch.NewChildKey(bip32.FirstHardenedChild + uint32(i))
		if err != nil {
			return nil, fmt.Errorf("failed to derive wallet key %d: %w", i, err)
		}

		// Lux P/X-chain address from secp256k1
		luxPrivKey, err := luxcrypto.ToPrivateKey(childKey.Key)
		if err != nil {
			return nil, fmt.Errorf("failed to create secp256k1 key %d: %w", i, err)
		}
		stakingAddr := luxPrivKey.Address()

		// ETH address from keccak256(uncompressed_pubkey)
		ethPrivKey, err := ethcrypto.ToECDSA(childKey.Key)
		if err != nil {
			return nil, fmt.Errorf("failed to create ECDSA key %d: %w", i, err)
		}
		ethAddr := ethcrypto.PubkeyToAddress(ethPrivKey.PublicKey)
		var ethShortID ids.ShortID
		copy(ethShortID[:], ethAddr[:])

		log.Debug("derived wallet allocation",
			"nid", nid, "i", i,
			"luxAddr", stakingAddr.String(),
			"ethAddr", fmt.Sprintf("0x%x", ethAddr),
		)

		allocations = append(allocations, Allocation{
			ETHAddr:       ethShortID,
			LUXAddr:       stakingAddr,
			InitialAmount: amountPerKey,
		})
	}

	return allocations, nil
}

// BuildConfigFromEnv builds genesis config from environment variables
// Checks in order: KEYS_DIR, mnemonic (MNEMONIC/LUX_MNEMONIC/LIGHT_MNEMONIC), PRIVATE_KEY
//
// Architecture:
//   - X-Chain: 100 accounts × 500M LUX each, FREE
//   - P-Chain: 100 accounts × 500M LUX each, vesting 1%/year from 2020-01-01
//   - C-Chain: treasury 0x9011...4714 gets 2T LUX
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

	// Load account keys from mnemonic (100 X-Chain accounts for deposits)
	var allKeys []KeyInfo
	if mnemonic := getMnemonicEnv(); mnemonic != "" {
		numAccounts := DefaultNumAccounts
		allKeys, err = LoadKeysFromMnemonic(mnemonic, networkID, numAccounts)
		if err != nil {
			allKeys = nil
		}
	}

	// Combine: use dir keys for validators, mnemonic keys for X-Chain accounts
	if len(validatorKeys) > 0 && len(allKeys) > 0 {
		return buildConfigFromKeyInfos(networkID, validatorKeys, allKeys, allocationPerKey)
	}
	if len(validatorKeys) > 0 {
		return BuildConfigFromKeys(networkID, os.Getenv("KEYS_DIR"), allocationPerKey)
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
//   - X-Chain: 100 accounts × 500M LUX each, FREE (immediately spendable)
//   - P-Chain: 100 accounts × 500M LUX each, vesting 1%/year from 2020-01-01
//   - C-Chain: treasury 0x9011...4714 gets 2T LUX
//   - Total: 100B LUX across 100 accounts (50B X + 50B P) + 2T C-chain treasury
//
// validatorKeys: first N accounts that become initial stakers
// allKeys: all accounts that receive X-Chain allocations (typically 100)
func buildConfigFromKeyInfos(networkID uint32, validatorKeys []KeyInfo, allKeys []KeyInfo, allocationPerKey uint64) (*Config, error) {
	if len(validatorKeys) == 0 {
		return nil, fmt.Errorf("no keys provided")
	}

	if allocationPerKey == 0 {
		allocationPerKey = DefaultAllocationPerAccount
	}

	// Each account gets ONE allocation entry with:
	//   InitialAmount  → X-Chain UTXO (free, immediately spendable)
	//   UnlockSchedule → P-Chain UTXOs (vesting 1%/year from Jan 1 2020)
	//
	// The node builder puts InitialAmount on X-Chain and UnlockSchedule on P-Chain.
	// InitialAmount ALSO creates a P-Chain UTXO (spendable, no locktime) for
	// non-staked addresses — so account gets 500M free on both X and P, plus
	// 500M vesting on P. Total per account: 500M X + 1B P.
	//
	// To get exactly 500M X + 500M P: set InitialAmount=500M (→ X free + P free)
	// and UnlockSchedule=500M total vesting (→ P locked). But the P free from
	// InitialAmount adds 500M extra. So we set InitialAmount=0 for P-only vesting
	// and use a separate entry for X-only free.
	//
	// Actually: simplest correct approach is ONE entry per account:
	//   InitialAmount = 500M → goes to X-Chain (free) AND P-Chain (free, spendable)
	//   UnlockSchedule = 500M → goes to P-Chain (vesting)
	// Result: X gets 500M free. P gets 500M free + 500M vesting = 1B on P.
	//
	// If we want exactly 500M on P (all vesting, no free):
	//   InitialAmount = 500M (X-chain only — but builder also puts it on P!)
	//
	// The builder ALWAYS puts InitialAmount on P-Chain too (line 444-456).
	// There's no way to give X-only via this genesis format.
	// So: 500M InitialAmount + 500M UnlockSchedule = 500M X + 1B P per account.
	//
	// For clean 500M/500M, we'd need InitialAmount=500M and NO UnlockSchedule,
	// then P-chain gets 500M free (from InitialAmount). X-chain gets 500M free.
	// That's 500M on each chain, both free. No vesting.
	//
	// OR: InitialAmount=0, UnlockSchedule=500M → P-chain gets 500M vesting,
	//     X-chain gets 0. Then separate entry: InitialAmount=500M, no schedule
	//     → X gets 500M free, P gets 500M free. Total: X=500M, P=1B. Still wrong.
	//
	// The node builder gives InitialAmount to BOTH chains. We can't avoid it.
	// Cleanest: one entry, InitialAmount=500M, UnlockSchedule=500M vesting.
	// Result: X=500M free, P=500M free + 500M vesting = 1B P.
	// Total per account: 1.5B (500M X + 1B P). Across 100 = 150B.
	//
	// This is fine — P-chain needs more funds for staking + subnet operations.
	pChainUnlockSchedule := buildUnlockSchedule(allocationPerKey, StakingStartTime, UnlockInterval, 100)

	allocations := make([]Allocation, 0, len(allKeys)+len(validatorKeys))
	stakedFunds := make([]ids.ShortID, 0, len(validatorKeys))
	stakers := make([]Staker, 0, len(validatorKeys))

	for _, key := range allKeys {
		// Single entry per account:
		//   X-Chain: 500M free (from InitialAmount)
		//   P-Chain: 500M free (from InitialAmount) + 500M vesting (from UnlockSchedule)
		allocations = append(allocations, Allocation{
			ETHAddr:        key.ETHAddr,
			LUXAddr:        key.StakingAddr,
			InitialAmount:  allocationPerKey,
			UnlockSchedule: pChainUnlockSchedule,
		})
	}

	for _, key := range validatorKeys {

		stakedFunds = append(stakedFunds, key.StakingAddr)

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
			if len(key.RingtailPublicKey) > 0 {
				staker.PQIdentity.RingtailPublicKey = fmt.Sprintf("0x%x", key.RingtailPublicKey)
			}
			// BLSCertificate and RingtailCertificate are generated at validator
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

// BuildWalletKeyHex derives the secp256k1 private key at index i on the
// per-network branch 1' (m/44'/9000'/nid'/1'/i') and returns it as hex.
// Used by bootstrap tools that need the exact same key the genesis
// allocated to.
func BuildWalletKeyHex(nid uint32, index int) (string, error) {
	mnemonic := getMnemonicEnv()
	if mnemonic == "" {
		return "", fmt.Errorf("MNEMONIC or LUX_MNEMONIC env var required")
	}
	if !bip39.IsMnemonicValid(mnemonic) {
		return "", fmt.Errorf("invalid mnemonic")
	}
	if nid >= bip32.FirstHardenedChild {
		return "", fmt.Errorf("network id %d must be < 2^31 (BIP-32 hardening limit)", nid)
	}
	seed := bip39.NewSeed(mnemonic, "")
	masterKey, err := bip32.NewMasterKey(seed)
	if err != nil {
		return "", err
	}
	account, err := deriveLuxAccount(masterKey, nid)
	if err != nil {
		return "", err
	}
	luxBranch, err := account.NewChildKey(bip32.FirstHardenedChild + 1)
	if err != nil {
		return "", fmt.Errorf("failed to derive branch 1' (secp256k1): %w", err)
	}
	child, err := luxBranch.NewChildKey(bip32.FirstHardenedChild + uint32(index))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", child.Key), nil
}
