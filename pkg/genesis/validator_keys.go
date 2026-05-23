// Copyright (C) 2019-2025, Lux Partners Limited. All rights reserved.
// See the file LICENSE for licensing terms.

package genesis

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/luxfi/address"
	"github.com/luxfi/crypto"
	"github.com/luxfi/crypto/secp256k1"
	"github.com/luxfi/ids"
)

// Common allocation amounts
const (
	// OneBillionLUX is 1B LUX - each mainnet validator gets this amount
	OneBillionLUX uint64 = 1_000_000_000 * Lux // 10^15 microLUX

	// OneHundredMillionLUX is 100M LUX - smaller allocation for testing
	OneHundredMillionLUX uint64 = 100_000_000 * Lux // 10^14 microLUX

	// DefaultValidatorAllocation is 1B LUX per validator
	DefaultValidatorAllocation uint64 = OneBillionLUX

	// VestingPeriods is 100 years (1% unlocks per year)
	VestingPeriods = 100
)

// StakingStartTime and UnlockInterval are defined in keys.go

// ValidatorKeyInfo contains computed addresses from a validator's private key
type ValidatorKeyInfo struct {
	PrivKeyHex string      // Hex-encoded private key
	EVMAddr    string      // C-chain EVM address (H160 hex) (0x...)
	ShortID    ids.ShortID // P/X chain address as ShortID
}

// DefaultValidatorKeyPath returns the default path for validator keys
func DefaultValidatorKeyPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".lux", "keys")
}

// LoadOrGenerateValidatorKeys loads validator keys from the specified path or generates new ones if missing.
// Keys are stored as hex-encoded private keys in files named validator_XXX.pk
func LoadOrGenerateValidatorKeys(keyPath string, count int) ([]ValidatorKeyInfo, error) {
	if keyPath == "" {
		keyPath = DefaultValidatorKeyPath()
	}

	// Ensure key directory exists
	if err := os.MkdirAll(keyPath, 0700); err != nil {
		return nil, fmt.Errorf("failed to create key directory: %w", err)
	}

	keys := make([]ValidatorKeyInfo, count)
	for i := 0; i < count; i++ {
		keyFile := filepath.Join(keyPath, fmt.Sprintf("validator_%03d.pk", i))

		var privKeyHex string
		data, err := os.ReadFile(keyFile)
		if err != nil {
			// Key doesn't exist, generate new one
			privKey, genErr := generatePrivateKey()
			if genErr != nil {
				return nil, fmt.Errorf("failed to generate key %d: %w", i, genErr)
			}
			privKeyHex = hex.EncodeToString(privKey)

			// Save the new key
			if err := os.WriteFile(keyFile, []byte(privKeyHex+"\n"), 0600); err != nil {
				return nil, fmt.Errorf("failed to save key %d: %w", i, err)
			}
		} else {
			privKeyHex = strings.TrimSpace(string(data))
		}

		// Compute addresses from the key
		keyInfo, err := ComputeValidatorKeyInfo(privKeyHex)
		if err != nil {
			return nil, fmt.Errorf("failed to compute addresses for key %d: %w", i, err)
		}
		keys[i] = keyInfo
	}

	return keys, nil
}

// ComputeValidatorKeyInfo derives addresses from a hex-encoded private key
func ComputeValidatorKeyInfo(privKeyHex string) (ValidatorKeyInfo, error) {
	privKeyBytes, err := hex.DecodeString(privKeyHex)
	if err != nil {
		return ValidatorKeyInfo{}, fmt.Errorf("invalid hex key: %w", err)
	}

	// Get EVM address
	evmPrivKey, err := crypto.ToECDSA(privKeyBytes)
	if err != nil {
		return ValidatorKeyInfo{}, fmt.Errorf("invalid ECDSA key: %w", err)
	}
	evmAddr := keccakAddr(evmPrivKey.PublicKey.X, evmPrivKey.PublicKey.Y)

	// Get Lux ShortID (for X/P chain addresses)
	utxoPrivKey, err := secp256k1.ToPrivateKey(privKeyBytes)
	if err != nil {
		return ValidatorKeyInfo{}, fmt.Errorf("invalid Lux key: %w", err)
	}
	pubKey := utxoPrivKey.PublicKey()
	shortID := ids.ShortID(pubKey.Address())

	return ValidatorKeyInfo{
		PrivKeyHex: privKeyHex,
		EVMAddr:    fmt.Sprintf("0x%x", evmAddr[:]),
		ShortID:    shortID,
	}, nil
}

// FormatChainAddress formats a ShortID as an X or P chain address with the given HRP
func FormatChainAddress(chainID string, hrp string, shortID ids.ShortID) (string, error) {
	return address.Format(chainID, hrp, shortID[:])
}

// generatePrivateKey generates a new random 32-byte private key
func generatePrivateKey() ([]byte, error) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	return key, nil
}

// GeneratePChainAllocations creates P-chain genesis allocations for validator keys.
// Each key gets the specified amount immediately available (locktime=0) for spending.
func GeneratePChainAllocations(keys []ValidatorKeyInfo, hrp string, amountPerKey uint64) ([]AllocationJSON, error) {
	if amountPerKey == 0 {
		amountPerKey = DefaultValidatorAllocation
	}

	allocations := make([]AllocationJSON, len(keys))
	for i, key := range keys {
		utxoAddr, err := FormatChainAddress("P", hrp, key.ShortID)
		if err != nil {
			return nil, fmt.Errorf("failed to format address for key %d: %w", i, err)
		}

		// Simple allocation: all funds immediately available (no vesting)
		unlockSchedule := []LockedAmount{
			{
				Amount:   amountPerKey,
				Locktime: 0, // Immediately available
			},
		}

		allocations[i] = AllocationJSON{
			EVMAddr:        key.EVMAddr,
			UTXOAddr:       utxoAddr,
			InitialAmount:  0, // initialAmount is NOT immediately spendable
			UnlockSchedule: unlockSchedule,
		}
	}
	return allocations, nil
}

// GeneratePChainAllocationsWithVesting creates P-chain allocations with a vesting schedule.
// Each key gets the specified amount vested over the given number of periods.
func GeneratePChainAllocationsWithVesting(keys []ValidatorKeyInfo, hrp string, amountPerKey uint64, startTime uint64, interval uint64, periods int) ([]AllocationJSON, error) {
	if amountPerKey == 0 {
		amountPerKey = OneBillionLUX
	}
	if startTime == 0 {
		startTime = StakingStartTime
	}
	if interval == 0 {
		interval = UnlockInterval
	}
	if periods == 0 {
		periods = 100 // 100 years default
	}

	allocations := make([]AllocationJSON, len(keys))
	for i, key := range keys {
		utxoAddr, err := FormatChainAddress("P", hrp, key.ShortID)
		if err != nil {
			return nil, fmt.Errorf("failed to format address for key %d: %w", i, err)
		}

		// Build vesting schedule
		unlockSchedule := buildUnlockSchedule(amountPerKey, startTime, interval, periods)

		allocations[i] = AllocationJSON{
			EVMAddr:        key.EVMAddr,
			UTXOAddr:       utxoAddr,
			InitialAmount:  amountPerKey, // X-chain initial amount
			UnlockSchedule: unlockSchedule,
		}
	}
	return allocations, nil
}

// GenerateCChainAlloc creates C-chain genesis allocations for validator keys.
func GenerateCChainAlloc(keys []ValidatorKeyInfo, amount uint64) map[string]Balance {
	if amount == 0 {
		amount = DefaultValidatorAllocation
	}

	alloc := make(map[string]Balance)
	for _, key := range keys {
		alloc[key.EVMAddr] = Balance{
			Balance: fmt.Sprintf("0x%x", amount),
		}
	}
	return alloc
}

// GenerateCChainAllocMap creates C-chain genesis allocations as a simple map for netrunner.
func GenerateCChainAllocMap(keys []ValidatorKeyInfo, amount uint64) map[string]map[string]string {
	if amount == 0 {
		amount = DefaultValidatorAllocation
	}

	alloc := make(map[string]map[string]string)
	balanceHex := fmt.Sprintf("0x%x", amount)
	for _, key := range keys {
		alloc[key.EVMAddr] = map[string]string{"balance": balanceHex}
	}
	return alloc
}

// GenerateAllocationsMapForNetrunner creates allocations in the format netrunner expects.
// This is a convenience function for backwards compatibility.
func GenerateAllocationsMapForNetrunner(keys []ValidatorKeyInfo, hrp string, amountPerKey uint64) ([]map[string]interface{}, error) {
	if amountPerKey == 0 {
		amountPerKey = DefaultValidatorAllocation
	}

	allocations := make([]map[string]interface{}, len(keys))
	for i, key := range keys {
		utxoAddr, err := FormatChainAddress("P", hrp, key.ShortID)
		if err != nil {
			return nil, fmt.Errorf("failed to format address for key %d: %w", i, err)
		}

		// Simple allocation: all funds immediately available
		unlockSchedule := []map[string]interface{}{
			{
				"amount":   amountPerKey,
				"locktime": uint64(0),
			},
		}

		allocations[i] = map[string]interface{}{
			"evmAddr":        key.EVMAddr,
			"utxoAddr":        utxoAddr,
			"initialAmount":  uint64(0),
			"unlockSchedule": unlockSchedule,
		}
	}
	return allocations, nil
}
