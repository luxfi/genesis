// Copyright (C) 2019-2025, Lux Partners Limited. All rights reserved.
// See the file LICENSE for licensing terms.

// Package configs provides genesis configuration loading for Lux networks.
// This package is used by CLI, netrunner, and node to obtain genesis JSON
// for different network IDs.
//
// Dynamic P-Chain Allocations:
// P-Chain allocations can be specified dynamically at runtime via:
//   - LUX_PCHAIN_ALLOCS: JSON string of allocations
//   - LUX_PCHAIN_ALLOCS_FILE: Path to allocations JSON file
//   - ~/.lux/genesis/{network}/pchain.json: Standard override location
//
// The C-Chain genesis remains embedded and immutable.
package configs

import (
	"embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/luxfi/genesis/pkg/genesis"
)

// Network ID constants (P-Chain)
// mainnet, testnet, devnet: proper public networks
// localnet: local development with LIGHT mnemonic (networkID=1337, EVM chainID=31337)
// anything else is custom (override via --genesis-file)
const (
	MainnetID  = 1
	TestnetID  = 2
	DevnetID   = 3
	LocalnetID = 1337

	// Chain ID constants (C-Chain EVM)
	MainnetChainID  = 96369
	TestnetChainID  = 96368
	DevnetChainID   = 96370
	LocalnetChainID = 31337
)

// Liquid L2 Chain IDs (EVM-compatible, for wallets/dApps).
// These are the chain IDs used by the Liquid settlement chains running
// as L2s on the Lux Network. They are NOT the primary network C-Chain IDs.
//
//	Network    NetworkID    EVM          DEX          FHE
//	─────────  ─────────    ───────      ───────      ───────
//	Mainnet    1            8675309      8675313      8675317
//	Testnet    2            8675310      8675314      8675318
//	Devnet     3            8675311      8675315      8675319
//	Localnet   1337         31337        31338        31339
const (
	//  — primary settlement chain
	LiquidEVMMainnet  = 8675309
	LiquidEVMTestnet  = 8675310
	LiquidEVMDevnet   = 8675311
	LiquidEVMLocalnet = 31337

	//  — native orderbook chain
	LiquidDEXMainnet  = 8675313
	LiquidDEXTestnet  = 8675314
	LiquidDEXDevnet   = 8675315
	LiquidDEXLocalnet = 31338

	// Liquid FHE — confidential compute chain
	LiquidFHEMainnet  = 8675317
	LiquidFHETestnet  = 8675318
	LiquidFHEDevnet   = 8675319
	LiquidFHELocalnet = 31339
)

//go:embed mainnet testnet devnet localnet
var embeddedGenesis embed.FS

// GetGenesis returns the genesis JSON bytes for a network ID.
// It supports dynamic P-Chain allocations via environment variables or files:
//   - LUX_PCHAIN_ALLOCS: JSON string of allocations
//   - LUX_PCHAIN_ALLOCS_FILE: Path to allocations JSON file
//   - ~/.lux/genesis/{network}/pchain.json: Standard override location
//
// C-Chain genesis remains embedded and immutable.
func GetGenesis(networkID uint32) ([]byte, error) {
	networkName := networkNameFromID(networkID)

	// Check for dynamic P-Chain allocations
	dynamicPChain := loadDynamicPChainAllocations(networkName)

	if networkName != "" {
		// Try to load from embedded FS with optional dynamic allocations
		data, err := loadEmbeddedGenesisWithDynamic(networkName, dynamicPChain)
		if err == nil {
			return data, nil
		}
	}

	// Fall back to file system locations
	return loadGenesisFromFS(networkID)
}

// GetGenesisWithAllocations returns genesis with custom P-Chain allocations.
// This allows booting networks with custom validator allocations.
func GetGenesisWithAllocations(networkID uint32, allocations []genesis.AllocationJSON) ([]byte, error) {
	networkName := networkNameFromID(networkID)

	// Convert allocations to PChainConfig
	pchain := &genesis.PChainConfig{
		Allocations: allocations,
	}

	if networkName != "" {
		data, err := loadEmbeddedGenesisWithDynamic(networkName, pchain)
		if err == nil {
			return data, nil
		}
	}

	return nil, fmt.Errorf("failed to load genesis for network %d", networkID)
}

// loadDynamicPChainAllocations loads P-Chain allocations from environment or files.
func loadDynamicPChainAllocations(networkName string) *genesis.PChainConfig {
	// First, check LUX_PCHAIN_ALLOCS environment variable (JSON string)
	if allocsJSON := os.Getenv("LUX_PCHAIN_ALLOCS"); allocsJSON != "" {
		var pchain genesis.PChainConfig
		if err := json.Unmarshal([]byte(allocsJSON), &pchain); err == nil {
			return &pchain
		}
	}

	// Second, check LUX_PCHAIN_ALLOCS_FILE environment variable
	if allocsFile := os.Getenv("LUX_PCHAIN_ALLOCS_FILE"); allocsFile != "" {
		data, err := os.ReadFile(allocsFile)
		if err == nil {
			var pchain genesis.PChainConfig
			if err := json.Unmarshal(data, &pchain); err == nil {
				return &pchain
			}
		}
	}

	// Third, check standard override location ~/.lux/genesis/{network}/pchain.json
	home, _ := os.UserHomeDir()
	overridePath := filepath.Join(home, ".lux/genesis", networkName, "pchain.json")
	data, err := os.ReadFile(overridePath)
	if err == nil {
		var pchain genesis.PChainConfig
		if err := json.Unmarshal(data, &pchain); err == nil {
			return &pchain
		}
	}

	return nil
}

// loadEmbeddedGenesisWithDynamic loads genesis with optional dynamic P-Chain allocations.
func loadEmbeddedGenesisWithDynamic(networkName string, dynamicPChain *genesis.PChainConfig) ([]byte, error) {
	// Load network.json from embedded
	networkData, err := embeddedGenesis.ReadFile(filepath.Join(networkName, "network.json"))
	if err != nil {
		// Fall back to single genesis.json file
		return embeddedGenesis.ReadFile(filepath.Join(networkName, "genesis.json"))
	}
	var network genesis.NetworkConfig
	if err := json.Unmarshal(networkData, &network); err != nil {
		return nil, fmt.Errorf("failed to parse network.json: %w", err)
	}

	// Check if split pchain.json/cchain.json exist - if not, fall back to genesis.json
	_, pchainErr := embeddedGenesis.ReadFile(filepath.Join(networkName, "pchain.json"))
	_, cchainErr := embeddedGenesis.ReadFile(filepath.Join(networkName, "cchain.json"))
	if pchainErr != nil || cchainErr != nil {
		// No split files, fall back to combined genesis.json (devnet case)
		return embeddedGenesis.ReadFile(filepath.Join(networkName, "genesis.json"))
	}

	// Load P-Chain config - use dynamic if provided, otherwise embedded
	var pchain genesis.PChainConfig
	if dynamicPChain != nil && len(dynamicPChain.Allocations) > 0 {
		pchain = *dynamicPChain
		// If dynamic allocations don't have staking config, load from embedded
		if pchain.InitialStakeDuration == 0 {
			embeddedPChain, _ := loadEmbeddedPChainConfig(networkName)
			if embeddedPChain != nil {
				pchain.InitialStakeDuration = embeddedPChain.InitialStakeDuration
				pchain.InitialStakeDurationOffset = embeddedPChain.InitialStakeDurationOffset
				pchain.InitialStakedFunds = embeddedPChain.InitialStakedFunds
				pchain.InitialStakers = embeddedPChain.InitialStakers
			}
		}
	} else {
		pchainData, err := embeddedGenesis.ReadFile(filepath.Join(networkName, "pchain.json"))
		if err != nil {
			return nil, fmt.Errorf("failed to read pchain.json: %w", err)
		}
		if err := json.Unmarshal(pchainData, &pchain); err != nil {
			return nil, fmt.Errorf("failed to parse pchain.json: %w", err)
		}
	}

	// Load C-Chain genesis (always embedded, immutable)
	cchainData, err := embeddedGenesis.ReadFile(filepath.Join(networkName, "cchain.json"))
	if err != nil {
		return nil, fmt.Errorf("failed to read cchain.json: %w", err)
	}

	// Build combined genesis config
	config := genesis.ConfigOutput{
		NetworkID:                  network.NetworkID,
		Allocations:                pchain.Allocations,
		StartTime:                  network.StartTime,
		InitialStakeDuration:       pchain.InitialStakeDuration,
		InitialStakeDurationOffset: pchain.InitialStakeDurationOffset,
		InitialStakedFunds:         pchain.InitialStakedFunds,
		InitialStakers:             pchain.InitialStakers,
		CChainGenesis:              string(cchainData),
		Message:                    network.Message,
	}

	return json.Marshal(config)
}

// loadEmbeddedPChainConfig loads only the P-Chain config from embedded.
func loadEmbeddedPChainConfig(networkName string) (*genesis.PChainConfig, error) {
	pchainData, err := embeddedGenesis.ReadFile(filepath.Join(networkName, "pchain.json"))
	if err != nil {
		return nil, err
	}
	var pchain genesis.PChainConfig
	if err := json.Unmarshal(pchainData, &pchain); err != nil {
		return nil, err
	}
	return &pchain, nil
}

// GetConfig returns the parsed genesis Config for a network ID.
func GetConfig(networkID uint32) (*genesis.Config, error) {
	data, err := GetGenesis(networkID)
	if err != nil {
		return nil, err
	}

	var config genesis.Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse genesis config: %w", err)
	}

	return &config, nil
}

// networkNameFromID returns the network directory name for a network ID.
// Accepts both network IDs (1, 2, 3, 1337) and chain IDs (96369, 96368, 96370) as aliases.
func networkNameFromID(networkID uint32) string {
	switch networkID {
	case MainnetID, MainnetChainID:
		return "mainnet"
	case TestnetID, TestnetChainID:
		return "testnet"
	case DevnetID, DevnetChainID:
		return "devnet"
	case LocalnetID: // LocalnetChainID == LocalnetID (both 1337)
		return "localnet"
	default:
		return ""
	}
}

// GetCanonicalGenesisBytes returns the canonical genesis bytes for a network.
// This function builds the genesis from split files (network.json, pchain.json, cchain.json)
// to ensure cChainGenesis is properly serialized as a JSON string.
//
// CRITICAL: The embedded genesis.json stores cChainGenesis as an object for easy editing,
// but luxd requires it to be a JSON-encoded string. This function handles the conversion.
func GetCanonicalGenesisBytes(networkID uint32) ([]byte, error) {
	networkName := networkNameFromID(networkID)
	if networkName == "" {
		return nil, fmt.Errorf("unknown network ID: %d", networkID)
	}

	// First, try to build from split files (network.json + pchain.json + cchain.json)
	// This properly stringifies cChainGenesis
	data, err := buildCanonicalGenesisFromSplitFiles(networkName)
	if err == nil {
		return data, nil
	}

	// Fall back to file system locations with split files
	home, _ := os.UserHomeDir()
	candidates := []string{
		filepath.Join(home, "work/lux/genesis/configs", networkName),
		filepath.Join(home, ".lux/genesis", networkName),
		filepath.Join("/etc/lux/genesis", networkName),
	}

	for _, dir := range candidates {
		if data, err := buildGenesisFromDir(dir); err == nil {
			return data, nil
		}
	}

	return nil, fmt.Errorf("canonical genesis not found for network %s (need split files: network.json, pchain.json, cchain.json)", networkName)
}

// buildCanonicalGenesisFromSplitFiles builds genesis from embedded split files.
// This ensures cChainGenesis is properly serialized as a JSON string.
func buildCanonicalGenesisFromSplitFiles(networkName string) ([]byte, error) {
	// Load network.json
	networkData, err := embeddedGenesis.ReadFile(filepath.Join(networkName, "network.json"))
	if err != nil {
		return nil, fmt.Errorf("failed to read network.json: %w", err)
	}
	var network genesis.NetworkConfig
	if err := json.Unmarshal(networkData, &network); err != nil {
		return nil, fmt.Errorf("failed to parse network.json: %w", err)
	}

	// Load pchain.json
	pchainData, err := embeddedGenesis.ReadFile(filepath.Join(networkName, "pchain.json"))
	if err != nil {
		return nil, fmt.Errorf("failed to read pchain.json: %w", err)
	}
	var pchain genesis.PChainConfig
	if err := json.Unmarshal(pchainData, &pchain); err != nil {
		return nil, fmt.Errorf("failed to parse pchain.json: %w", err)
	}

	// Load cchain.json - this is the C-Chain genesis as a JSON object
	cchainData, err := embeddedGenesis.ReadFile(filepath.Join(networkName, "cchain.json"))
	if err != nil {
		return nil, fmt.Errorf("failed to read cchain.json: %w", err)
	}

	// Build combined genesis config with cChainGenesis as a JSON string
	config := genesis.ConfigOutput{
		NetworkID:                  network.NetworkID,
		Allocations:                pchain.Allocations,
		StartTime:                  network.StartTime,
		InitialStakeDuration:       pchain.InitialStakeDuration,
		InitialStakeDurationOffset: pchain.InitialStakeDurationOffset,
		InitialStakedFunds:         pchain.InitialStakedFunds,
		InitialStakers:             pchain.InitialStakers,
		CChainGenesis:              string(cchainData), // Properly convert object to string
		Message:                    network.Message,
	}

	return json.Marshal(config)
}

// loadGenesisFromFS loads genesis from file system locations.
func loadGenesisFromFS(networkID uint32) ([]byte, error) {
	networkName := networkNameFromID(networkID)
	if networkName == "" {
		networkName = "localnet"
	}

	home, _ := os.UserHomeDir()
	candidates := []string{
		filepath.Join(home, "work/lux/genesis", networkName),
		filepath.Join(home, ".lux/genesis", networkName),
		filepath.Join("/etc/lux/genesis", networkName),
	}

	for _, dir := range candidates {
		// Try component files first
		networkPath := filepath.Join(dir, "network.json")
		if _, err := os.Stat(networkPath); err == nil {
			return buildGenesisFromDir(dir)
		}

		// Try single genesis.json
		genesisPath := filepath.Join(dir, "genesis.json")
		if data, err := os.ReadFile(genesisPath); err == nil {
			return data, nil
		}

		// Try primary.json (our generated output)
		primaryPath := filepath.Join(dir, "primary.json")
		if data, err := os.ReadFile(primaryPath); err == nil {
			return data, nil
		}
	}

	return nil, fmt.Errorf("genesis not found for network %d", networkID)
}

// buildGenesisFromDir builds genesis from component files in a directory.
func buildGenesisFromDir(dir string) ([]byte, error) {
	// Load network.json
	networkData, err := os.ReadFile(filepath.Join(dir, "network.json"))
	if err != nil {
		return nil, fmt.Errorf("failed to read network.json: %w", err)
	}
	var network genesis.NetworkConfig
	if err := json.Unmarshal(networkData, &network); err != nil {
		return nil, fmt.Errorf("failed to parse network.json: %w", err)
	}

	// Load pchain.json
	pchainData, err := os.ReadFile(filepath.Join(dir, "pchain.json"))
	if err != nil {
		return nil, fmt.Errorf("failed to read pchain.json: %w", err)
	}
	var pchain genesis.PChainConfig
	if err := json.Unmarshal(pchainData, &pchain); err != nil {
		return nil, fmt.Errorf("failed to parse pchain.json: %w", err)
	}

	// Load cchain.json
	cchainData, err := os.ReadFile(filepath.Join(dir, "cchain.json"))
	if err != nil {
		return nil, fmt.Errorf("failed to read cchain.json: %w", err)
	}

	// Build combined genesis config
	config := genesis.ConfigOutput{
		NetworkID:                  network.NetworkID,
		Allocations:                pchain.Allocations,
		StartTime:                  network.StartTime,
		InitialStakeDuration:       pchain.InitialStakeDuration,
		InitialStakeDurationOffset: pchain.InitialStakeDurationOffset,
		InitialStakedFunds:         pchain.InitialStakedFunds,
		InitialStakers:             pchain.InitialStakers,
		CChainGenesis:              string(cchainData),
		Message:                    network.Message,
	}

	return json.Marshal(config)
}
