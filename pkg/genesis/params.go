// Copyright (C) 2019-2025, Lux Industries Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package genesis

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/luxfi/address"
	"github.com/luxfi/constants"
	"github.com/luxfi/ids"
)

// Decimal places for different chains:
// - P-chain/X-chain: 6 decimals (microLux) - smallest unit is 0.000001 LUX
// - C-chain: 18 decimals (wei) - compatible with Ethereum tooling
//
// Total Supply: 2 trillion LUX (2T)

// P-chain/X-chain units (6 decimals)
const (
	MicroLux uint64 = 1                         // Base unit (6 decimals) - 0.000001 LUX
	MilliLux uint64 = 1_000                     // 0.001 LUX
	Lux      uint64 = 1_000_000                 // 1 LUX = 10^6 microLUX
	KiloLux  uint64 = 1_000_000_000             // 1,000 LUX
	MegaLux  uint64 = 1_000_000_000_000         // 1,000,000 LUX (1M)
	GigaLux  uint64 = 1_000_000_000_000_000     // 1 billion LUX (1B)
	TeraLux  uint64 = 1_000_000_000_000_000_000 // 1 trillion LUX (1T)

	// TotalSupply is the maximum supply of LUX (2 trillion)
	TotalSupply uint64 = 2 * TeraLux // 2T LUX
)

// C-chain decimal conversion factor
const (
	// CChainDecimalShift is 10^12 to convert from microLux (6 dec) to wei (18 dec)
	CChainDecimalShift = 1_000_000_000_000
)

// Params combines staking and fee parameters for a network
type Params struct {
	TxFee             uint64
	CreateAssetTxFee  uint64
	UptimeRequirement float64
	MinValidatorStake uint64
	MaxValidatorStake uint64
	MinDelegatorStake uint64
	MinDelegationFee  uint32
	MinStakeDuration  uint64
	MaxStakeDuration  uint64
	RewardConfig      RewardConfig
}

// LocalParams contains default parameters for local networks
var LocalParams = Params{
	TxFee:             MilliLux,
	CreateAssetTxFee:  10 * MilliLux,
	UptimeRequirement: 0.2,
	MinValidatorStake: 1_000_000 * Lux,
	MaxValidatorStake: 3000000 * Lux,
	MinDelegatorStake: 1 * Lux,
	MinDelegationFee:  20000, // 2%
	MinStakeDuration:  60,    // 1 minute
	MaxStakeDuration:  365 * 24 * 60 * 60,
	RewardConfig: RewardConfig{
		MaxConsumptionRate: 120000,
		MinConsumptionRate: 100000,
		MintingPeriod:      365 * 24 * 60 * 60,
		SupplyCap:          2 * TeraLux, // 2 trillion LUX max supply
	},
}

// Activation is the one instant every mainnet rule turns on:
// 2025-12-25 16:20 America/Los_Angeles. The precompile schedule and the staking
// policy both read it, so no rule can activate at a moment its neighbours do
// not know.
const Activation int64 = 1766708400

// MainnetParams contains default parameters for mainnet
var MainnetParams = Params{
	TxFee:             MilliLux,
	CreateAssetTxFee:  10 * MilliLux,
	UptimeRequirement: 0.8,
	MinValidatorStake: 1_000_000 * Lux,
	MaxValidatorStake: 5 * GigaLux,
	MinDelegatorStake: 25 * Lux,
	MinDelegationFee:  20000,
	MinStakeDuration:  2 * 7 * 24 * 60 * 60, // 2 weeks
	MaxStakeDuration:  365 * 24 * 60 * 60,
	RewardConfig: RewardConfig{
		MaxConsumptionRate: 120000,
		MinConsumptionRate: 100000,
		MintingPeriod:      365 * 24 * 60 * 60,
		SupplyCap:          2 * TeraLux, // 2 trillion LUX max supply
	},
}

// TestnetParams contains default parameters for testnet
var TestnetParams = Params{
	TxFee:             MilliLux,
	CreateAssetTxFee:  10 * MilliLux,
	UptimeRequirement: 0.8,
	MinValidatorStake: 1_000_000 * Lux,
	MaxValidatorStake: 3000000 * Lux,
	MinDelegatorStake: 1 * Lux,
	MinDelegationFee:  20000,
	MinStakeDuration:  24 * 60 * 60, // 1 day
	MaxStakeDuration:  365 * 24 * 60 * 60,
	RewardConfig: RewardConfig{
		MaxConsumptionRate: 120000,
		MinConsumptionRate: 100000,
		MintingPeriod:      365 * 24 * 60 * 60,
		SupplyCap:          2 * TeraLux, // 2 trillion LUX max supply
	},
}

// GetParams returns network parameters for a network ID
func GetParams(networkID uint32) Params {
	switch networkID {
	case constants.MainnetID:
		return MainnetParams
	case constants.TestnetID:
		return TestnetParams
	default:
		return LocalParams
	}
}


// Default tx fee configs per network
var (
	MainnetTxFeeConfig = TxFeeConfig{
		TxFee:            MilliLux,
		CreateAssetTxFee: 10 * MilliLux,
	}

	TestnetTxFeeConfig = TxFeeConfig{
		TxFee:            MilliLux,
		CreateAssetTxFee: 10 * MilliLux,
	}

	LocalTxFeeConfig = TxFeeConfig{
		TxFee:            MilliLux,
		CreateAssetTxFee: 10 * MilliLux,
	}
)

// GetStakingConfig is the staking half of a network's parameters. It is a
// projection, not a second copy: there was one, and its mainnet floor disagreed
// with the parameters by a thousandfold while both compiled and every test
// passed. The node reads this at boot, so this is the number a chain enforces.
func GetStakingConfig(networkID uint32) StakingConfig {
	p := GetParams(networkID)
	return StakingConfig{
		UptimeRequirement: p.UptimeRequirement,
		MinValidatorStake: p.MinValidatorStake,
		MaxValidatorStake: p.MaxValidatorStake,
		MinDelegatorStake: p.MinDelegatorStake,
		MinDelegationFee:  p.MinDelegationFee,
		MinStakeDuration:  p.MinStakeDuration,
		MaxStakeDuration:  p.MaxStakeDuration,
		RewardConfig:      p.RewardConfig,
	}
}

// GetTxFeeConfig returns tx fee config for a network
func GetTxFeeConfig(networkID uint32) TxFeeConfig {
	switch networkID {
	case constants.MainnetID:
		return MainnetTxFeeConfig
	case constants.TestnetID:
		return TestnetTxFeeConfig
	default:
		return LocalTxFeeConfig
	}
}

// GetBootstrappers loads bootstrappers dynamically from config or keys
func GetBootstrappers(networkID uint32) []Bootstrapper {
	// Try to load from bootstrappers.json in genesis directory
	home, _ := os.UserHomeDir()

	var networkName string
	switch networkID {
	case constants.MainnetID:
		networkName = "mainnet"
	case constants.TestnetID:
		networkName = "testnet"
	case constants.DevnetID:
		networkName = "devnet"
	default:
		// For custom/local networks, check environment variable for bootstrappers path
		if envPath := os.Getenv("BOOTSTRAPPERS_FILE"); envPath != "" {
			data, err := os.ReadFile(envPath)
			if err == nil {
				var bootstrappers []Bootstrapper
				if err := json.Unmarshal(data, &bootstrappers); err == nil {
					return bootstrappers
				}
			}
		}
		return nil // Local networks don't need bootstrappers by default
	}

	// Check standard locations for bootstrappers.json
	candidates := []string{
		filepath.Join(home, "work/lux/genesis/configs", networkName, "bootstrappers.json"),
		filepath.Join(home, ".lux/genesis/configs", networkName, "bootstrappers.json"),
		filepath.Join("/etc/lux/genesis/configs", networkName, "bootstrappers.json"),
	}

	for _, path := range candidates {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var bootstrappers []Bootstrapper
		if err := json.Unmarshal(data, &bootstrappers); err != nil {
			continue
		}
		return bootstrappers
	}

	return nil
}

// GetBootstrappersFromKeys loads bootstrapper info from node keys directory
func GetBootstrappersFromKeys(keysDir string) ([]Bootstrapper, error) {
	if keysDir == "" {
		home, _ := os.UserHomeDir()
		keysDir = filepath.Join(home, ".lux/keys")
	}

	entries, err := os.ReadDir(keysDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read keys directory: %w", err)
	}

	var bootstrappers []Bootstrapper
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		nodeKeyPath := filepath.Join(keysDir, entry.Name(), "staker.key")
		if _, err := os.Stat(nodeKeyPath); err != nil {
			continue
		}
		// Node key exists, try to get node ID
		// This would need crypto package to properly derive node ID from key
		bootstrappers = append(bootstrappers, Bootstrapper{
			ID: entry.Name(), // Placeholder - should derive from key
			IP: "",           // Would need to be configured separately
		})
	}

	return bootstrappers, nil
}

// allowedHRPs is the closed set of bech32 HRPs that ParseAddress accepts.
// Any other HRP (notably "avax" from upstream Avalanche or arbitrary
// user HRPs) is a wrong-network event — the same 20 bytes silently re-encoded
// under a foreign HRP must not be admitted into a Lux genesis.
var allowedHRPs = map[string]struct{}{
	"lux":   {}, // mainnet:  P-lux1...   / X-lux1...
	"test":  {}, // testnet:  P-test1...  / X-test1...
	"dev":   {}, // devnet:   P-dev1...   / X-dev1...
	"local": {}, // localnet: P-local1... / X-local1...
}

// ParseAddress parses a bech32 address string to ShortID.
// Supports formats: P-lux1xxx, X-lux1xxx, lux1xxx, local1xxx.
// The HRP must be one of the canonical Lux HRPs (lux/test/dev/local).
func ParseAddress(addrStr string) (ids.ShortID, error) {
	if addrStr == "" {
		return ids.ShortID{}, fmt.Errorf("empty address")
	}

	var (
		hrp       string
		addrBytes []byte
		err       error
	)

	// Try full format first (P-lux1xxx, X-lux1xxx)
	if strings.Contains(addrStr, "-") {
		_, hrp, addrBytes, err = address.Parse(addrStr)
		if err != nil {
			return ids.ShortID{}, fmt.Errorf("failed to parse address %s: %w", addrStr, err)
		}
	} else {
		// Raw bech32 format (lux1xxx, local1xxx)
		hrp, addrBytes, err = address.ParseBech32(addrStr)
		if err != nil {
			return ids.ShortID{}, fmt.Errorf("failed to parse bech32 address %s: %w", addrStr, err)
		}
	}

	if _, ok := allowedHRPs[hrp]; !ok {
		return ids.ShortID{}, fmt.Errorf("unsupported HRP %q: only lux/test/dev/local are accepted", hrp)
	}

	var addr ids.ShortID
	copy(addr[:], addrBytes)
	return addr, nil
}

// ParseEVMAddress parses an Ethereum hex address to ShortID
func ParseEVMAddress(addrStr string) (ids.ShortID, error) {
	if addrStr == "" {
		return ids.ShortID{}, fmt.Errorf("empty address")
	}

	// Remove 0x prefix
	addrStr = strings.TrimPrefix(addrStr, "0x")

	// Decode hex
	bytes, err := hex.DecodeString(addrStr)
	if err != nil {
		return ids.ShortID{}, fmt.Errorf("invalid hex: %w", err)
	}

	if len(bytes) != 20 {
		return ids.ShortID{}, fmt.Errorf("invalid address length: %d", len(bytes))
	}

	var addr ids.ShortID
	copy(addr[:], bytes)
	return addr, nil
}

// ParseNodeID parses a node ID string
func ParseNodeID(nodeIDStr string) (ids.NodeID, error) {
	return ids.NodeIDFromString(nodeIDStr)
}

// FormatAddress formats a ShortID as bech32 address
func FormatAddress(hrp string, addr ids.ShortID) string {
	addrStr, err := address.FormatBech32(hrp, addr[:])
	if err != nil {
		return fmt.Sprintf("%s1%s", hrp, addr.String())
	}
	return addrStr
}
