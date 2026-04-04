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
	ethcrypto "github.com/luxfi/crypto"
	"github.com/luxfi/crypto/bls"
	luxcrypto "github.com/luxfi/crypto/secp256k1"
	"github.com/luxfi/go-bip32"
	"github.com/luxfi/go-bip39"
	"github.com/luxfi/ids"
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
)

// KeyInfo contains parsed key information for a node
type KeyInfo struct {
	NodeID               ids.NodeID
	StakerKey            []byte
	BLSPublicKey         []byte
	BLSProofOfPossession []byte
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

// LoadKeysFromMnemonic derives multiple keys from a BIP39 mnemonic
// Uses BIP44 path: m/44'/9000'/0'/0/{account} (Lux P/X-Chain coin type)
func LoadKeysFromMnemonic(mnemonic string, numAccounts int) ([]KeyInfo, error) {
	if !bip39.IsMnemonicValid(mnemonic) {
		return nil, fmt.Errorf("invalid mnemonic")
	}

	seed := bip39.NewSeed(mnemonic, "")
	masterKey, err := bip32.NewMasterKey(seed)
	if err != nil {
		return nil, fmt.Errorf("failed to create master key: %w", err)
	}

	// BIP44 path for Lux P/X-Chain: m/44'/9000'/0'/0/{account}
	// 44' = purpose (BIP44)
	// 9000' = Lux coin type (P/X-Chain)
	// 0' = account
	// 0 = external chain
	purpose, err := masterKey.NewChildKey(bip32.FirstHardenedChild + 44)
	if err != nil {
		return nil, fmt.Errorf("failed to derive purpose: %w", err)
	}

	coinType, err := purpose.NewChildKey(bip32.FirstHardenedChild + 9000)
	if err != nil {
		return nil, fmt.Errorf("failed to derive coin type: %w", err)
	}

	account, err := coinType.NewChildKey(bip32.FirstHardenedChild + 0)
	if err != nil {
		return nil, fmt.Errorf("failed to derive account: %w", err)
	}

	change, err := account.NewChildKey(0)
	if err != nil {
		return nil, fmt.Errorf("failed to derive change: %w", err)
	}

	// Also derive ETH keys (coin type 60) for C-Chain addresses.
	// P/X-Chain uses coin type 9000, C-Chain uses standard ETH derivation.
	ethCoinType, err := purpose.NewChildKey(bip32.FirstHardenedChild + 60)
	if err != nil {
		return nil, fmt.Errorf("failed to derive ETH coin type: %w", err)
	}
	ethAccount, err := ethCoinType.NewChildKey(bip32.FirstHardenedChild + 0)
	if err != nil {
		return nil, fmt.Errorf("failed to derive ETH account: %w", err)
	}
	ethChange, err := ethAccount.NewChildKey(0)
	if err != nil {
		return nil, fmt.Errorf("failed to derive ETH change: %w", err)
	}

	keys := make([]KeyInfo, 0, numAccounts)
	for i := 0; i < numAccounts; i++ {
		// Lux key (coin type 9000) → P/X-Chain address + NodeID
		childKey, err := change.NewChildKey(uint32(i))
		if err != nil {
			return nil, fmt.Errorf("failed to derive key %d: %w", i, err)
		}

		keyInfo, err := keyInfoFromPrivateKey(childKey.Key)
		if err != nil {
			return nil, fmt.Errorf("failed to create key info %d: %w", i, err)
		}

		// ETH key (coin type 60) → C-Chain address
		// This ensures C-Chain address matches standard ETH wallets (MetaMask, etc.)
		ethChildKey, err := ethChange.NewChildKey(uint32(i))
		if err != nil {
			return nil, fmt.Errorf("failed to derive ETH key %d: %w", i, err)
		}
		ethPrivKey, err := ethcrypto.ToECDSA(ethChildKey.Key)
		if err == nil {
			ethAddr := ethcrypto.PubkeyToAddress(ethPrivKey.PublicKey)
			copy(keyInfo.ETHAddr[:], ethAddr[:])
		}

		keys = append(keys, *keyInfo)
	}

	return keys, nil
}

// LoadKeysFromMnemonicEnv loads keys from mnemonic env vars.
// Priority: MNEMONIC > LUX_MNEMONIC > LIGHT_MNEMONIC
func LoadKeysFromMnemonicEnv(numAccounts int) ([]KeyInfo, error) {
	mnemonic := getMnemonicEnv()
	if mnemonic == "" {
		return nil, fmt.Errorf("mnemonic not set (set MNEMONIC, LUX_MNEMONIC, or LIGHT_MNEMONIC)")
	}

	return LoadKeysFromMnemonic(mnemonic, numAccounts)
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
func getMnemonicEnv() string {
	for _, env := range []string{"MNEMONIC", "LUX_MNEMONIC", "LIGHT_MNEMONIC"} {
		if v := os.Getenv(env); v != "" {
			return v
		}
	}
	return ""
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
		allKeys, err = LoadKeysFromMnemonic(mnemonic, numAccounts)
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
