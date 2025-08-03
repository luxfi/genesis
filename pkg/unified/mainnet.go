package unified

import (
	"fmt"
	"time"
	
	"github.com/luxfi/genesis/pkg/core"
)

const (
	// Total supply: 111B LUX (with 9 decimals)
	TotalSupply = uint64(111_000_000_000) * 1e9
	
	// Allocations
	NumAddresses     = 111
	AmountPerAddress = uint64(1_000_000_000) * 1e9 // 1B LUX per address
	
	// Vesting
	VestingStartYear = 2020
	VestingYears     = 100
	
	// Network IDs
	MainnetID = 96369
	TestnetID = 96368
	LocalID   = 12345
)

// MainnetAllocations creates the 111 addresses with proper allocations
func MainnetAllocations() map[string]uint64 {
	allocations := make(map[string]uint64)
	
	// Generate 111 addresses with 1B LUX each
	for i := 1; i <= NumAddresses; i++ {
		addr := fmt.Sprintf("X-lux1%039d", i)
		allocations[addr] = AmountPerAddress
	}
	
	return allocations
}

// MainnetValidators returns the 21 mainnet validators
func MainnetValidators() []core.Validator {
	// These would be loaded from the validators.json file
	// For now, returning a placeholder
	validators := make([]core.Validator, 21)
	for i := 0; i < 21; i++ {
		validators[i] = core.Validator{
			NodeID:        fmt.Sprintf("NodeID-%d", i),
			RewardAddress: fmt.Sprintf("X-lux1%039d", i+1),
			DelegationFee: 20000, // 2%
			Weight:        TotalSupply / 21,
		}
	}
	return validators
}

// TestnetValidators returns the 11 testnet validators
func TestnetValidators() []core.Validator {
	validators := make([]core.Validator, 11)
	for i := 0; i < 11; i++ {
		validators[i] = core.Validator{
			NodeID:        fmt.Sprintf("NodeID-Test%d", i),
			RewardAddress: fmt.Sprintf("X-lux1test%034d", i+1),
			DelegationFee: 20000, // 2%
			Weight:        TotalSupply / 11,
		}
	}
	return validators
}

// VestingTransform creates a transform that adds 100-year vesting to P-Chain allocations
func VestingTransform() TransformFunc[*PChainGenesis] {
	return func(genesis *PChainGenesis) (*PChainGenesis, error) {
		startTime := time.Date(VestingStartYear, 1, 1, 0, 0, 0, 0, time.UTC)
		
		// Update all allocations with vesting schedule
		for i := range genesis.Allocations {
			alloc := &genesis.Allocations[i]
			
			// Create 100-year monthly vesting
			totalAmount := alloc.InitialAmount
			monthlyAmount := totalAmount / (VestingYears * 12)
			remainder := totalAmount % (VestingYears * 12)
			
			alloc.UnlockSchedule = make([]core.UnlockSchedule, 0, VestingYears*12)
			
			for year := 0; year < VestingYears; year++ {
				for month := 0; month < 12; month++ {
					unlockTime := startTime.AddDate(year, month, 0)
					amount := monthlyAmount
					
					// Add remainder to last unlock
					if year == VestingYears-1 && month == 11 {
						amount += remainder
					}
					
					alloc.UnlockSchedule = append(alloc.UnlockSchedule, core.UnlockSchedule{
						Amount:   amount,
						Locktime: uint64(unlockTime.Unix()),
					})
				}
			}
			
			// Set initial amount to 0 since everything is vesting
			alloc.InitialAmount = 0
		}
		
		return genesis, nil
	}
}

// NFTAirdropTransform adds NFT-based airdrops
func NFTAirdropTransform(nftData map[string][]string) TransformFunc[*XChainGenesis] {
	return func(genesis *XChainGenesis) (*XChainGenesis, error) {
		// Add allocations for:
		// - Lux Genesis NFT validators
		// - Lux Coin holders
		// - Lux Card NFT holders
		// - ZOO NFT holders (4.2M ZOO per EGG)
		
		// This would process the NFT data and add appropriate allocations
		// For now, this is a placeholder
		
		return genesis, nil
	}
}

// LocalNetworkTransform converts addresses for local network using mnemonic
func LocalNetworkTransform(mnemonic string) TransformFunc[*UnifiedGenesis] {
	return func(genesis *UnifiedGenesis) (*UnifiedGenesis, error) {
		// Derive 111 addresses from mnemonic
		// Update all allocations to use these addresses
		// This would use the HD wallet derivation
		
		return genesis, nil
	}
}