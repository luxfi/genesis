// Copyright (C) 2019-2025, Lux Partners Limited. All rights reserved.
// See the file LICENSE for licensing terms.

package genesis

import (
	"fmt"
	"math/big"
)

// ChainAllocations holds genesis allocations for all chains.
// Use NewAllocations() to create.
//
// All allocations are immediate (no vesting). The long-locked validator
// stake bucket is attached separately by buildConfigFromKeyInfos and is
// the only place an UnlockSchedule appears in the canonical genesis path.
type ChainAllocations struct {
	keys   []ValidatorKeyInfo
	hrp    string
	amount uint64
}

// NewAllocations creates a ChainAllocations for the given validator keys.
// Default amount is DefaultValidatorAllocation per key, immediate spend.
func NewAllocations(keys []ValidatorKeyInfo, hrp string) *ChainAllocations {
	return &ChainAllocations{
		keys:   keys,
		hrp:    hrp,
		amount: DefaultValidatorAllocation,
	}
}

// WithAmount sets the allocation amount per key.
func (a *ChainAllocations) WithAmount(amount uint64) *ChainAllocations {
	a.amount = amount
	return a
}

// PChain returns P-chain allocations in the standard AllocationJSON format.
// All entries are immediately spendable (locktime=0).
func (a *ChainAllocations) PChain() ([]AllocationJSON, error) {
	allocations := make([]AllocationJSON, len(a.keys))

	for i, key := range a.keys {
		utxoAddr, err := FormatChainAddress("P", a.hrp, key.ShortID)
		if err != nil {
			return nil, fmt.Errorf("failed to format P-chain address for key %d: %w", i, err)
		}

		allocations[i] = AllocationJSON{
			EVMAddr:        key.EVMAddr,
			UTXOAddr:       utxoAddr,
			InitialAmount:  a.amount,
			UnlockSchedule: []LockedAmount{{Amount: a.amount, Locktime: 0}},
		}
	}
	return allocations, nil
}

// CChain returns C-chain allocations as a Balance map.
// C-chain uses 18 decimals (wei), P-chain uses 6 decimals (microLux).
// This converts the P-chain amount to C-chain wei automatically.
func (a *ChainAllocations) CChain() map[string]Balance {
	alloc := make(map[string]Balance, len(a.keys))
	// Convert from microLux (6 decimals) to wei (18 decimals)
	// Multiply by 10^12 to go from 6 to 18 decimals
	cchainAmount := new(big.Int).Mul(
		new(big.Int).SetUint64(a.amount),
		new(big.Int).Exp(big.NewInt(10), big.NewInt(12), nil),
	)
	for _, key := range a.keys {
		alloc[key.EVMAddr] = Balance{
			Balance: fmt.Sprintf("0x%x", cchainAmount),
		}
	}
	return alloc
}

// CChainMap returns C-chain allocations as a simple string map (for netrunner compatibility).
// C-chain uses 18 decimals (wei), P-chain uses 6 decimals (microLux).
func (a *ChainAllocations) CChainMap() map[string]map[string]string {
	alloc := make(map[string]map[string]string, len(a.keys))
	// Convert from microLux (6 decimals) to wei (18 decimals)
	cchainAmount := new(big.Int).Mul(
		new(big.Int).SetUint64(a.amount),
		new(big.Int).Exp(big.NewInt(10), big.NewInt(12), nil),
	)
	balanceHex := fmt.Sprintf("0x%x", cchainAmount)
	for _, key := range a.keys {
		alloc[key.EVMAddr] = map[string]string{"balance": balanceHex}
	}
	return alloc
}

// PChainMap returns P-chain allocations as interface maps (for netrunner compatibility).
// All entries are immediately spendable (locktime=0).
func (a *ChainAllocations) PChainMap() ([]map[string]interface{}, error) {
	allocations := make([]map[string]interface{}, len(a.keys))

	for i, key := range a.keys {
		utxoAddr, err := FormatChainAddress("P", a.hrp, key.ShortID)
		if err != nil {
			return nil, fmt.Errorf("failed to format P-chain address for key %d: %w", i, err)
		}

		allocations[i] = map[string]interface{}{
			"evmAddr":       key.EVMAddr,
			"utxoAddr":      utxoAddr,
			"initialAmount": a.amount,
			"unlockSchedule": []map[string]interface{}{
				{"amount": a.amount, "locktime": uint64(0)},
			},
		}
	}
	return allocations, nil
}

// XChain returns X-chain allocations (same format as P-chain for cross-chain compatibility).
func (a *ChainAllocations) XChain() ([]AllocationJSON, error) {
	return a.PChain() // X-chain uses same allocation format
}

// All returns allocations for all chains in a combined struct.
func (a *ChainAllocations) All() (*AllChainAllocations, error) {
	pchain, err := a.PChain()
	if err != nil {
		return nil, err
	}

	return &AllChainAllocations{
		PChain: pchain,
		CChain: a.CChain(),
		XChain: pchain, // X uses same format as P
	}, nil
}

// AllChainAllocations contains allocations for all chains.
type AllChainAllocations struct {
	PChain []AllocationJSON
	CChain map[string]Balance
	XChain []AllocationJSON
}

// --- Convenience functions for simple use cases ---

// QuickAllocations creates immediate allocations (no vesting) for all chains.
// This is the simplest way to set up a local/test network.
func QuickAllocations(keys []ValidatorKeyInfo, hrp string, amount uint64) (*AllChainAllocations, error) {
	return NewAllocations(keys, hrp).WithAmount(amount).All()
}

// MainnetAllocations creates immediate-spend mainnet allocations (1B LUX per key).
func MainnetAllocations(keys []ValidatorKeyInfo, hrp string) (*AllChainAllocations, error) {
	return NewAllocations(keys, hrp).
		WithAmount(OneBillionLUX).
		All()
}

// TestnetAllocations creates immediate-spend testnet allocations (100M LUX per key).
func TestnetAllocations(keys []ValidatorKeyInfo, hrp string) (*AllChainAllocations, error) {
	return NewAllocations(keys, hrp).
		WithAmount(OneHundredMillionLUX).
		All()
}
