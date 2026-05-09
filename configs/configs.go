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
	// Network IDs (P-Chain). These identify the PRIMARY network.
	MainnetID = 1
	TestnetID = 2
	DevnetID  = 3
	// LocalID is the canonical local single/multi-node dev network.
	// Pair with LocalChainID = 31337 on the C-Chain (Anvil convention).
	LocalID = 1337

	// CustomID is the sentinel for any network ID outside the well-known
	// {1, 2, 3, 1337} set — i.e. genuinely user-defined networks. It is
	// deliberately NOT 1337 so callers can distinguish "this is the local
	// dev network" (LocalID) from "this is some other custom network the
	// caller will configure via --genesis-file" (CustomID).
	CustomID uint32 = 0

	// LocalnetID is a deprecated alias for LocalID; existing callers
	// should migrate to LocalID. Kept here so older code keeps building
	// during the rollout.
	LocalnetID = LocalID

	// Chain ID constants (C-Chain EVM).
	MainnetChainID = 96369
	TestnetChainID = 96368
	DevnetChainID  = 96370
	// LocalChainID is the canonical local C-Chain EVM ID (Anvil convention).
	LocalChainID = 31337
	// CustomChainID is the sentinel C-Chain EVM ID for any chain outside
	// the well-known {96369, 96368, 96370, 31337} set. Mirrors CustomID
	// at the network-ID layer; the two should always be paired (a peer
	// presenting CustomID at the network layer also presents
	// CustomChainID at the EVM layer unless overridden via genesis-file).
	CustomChainID uint32 = 0
	// LocalnetChainID is a deprecated alias for LocalChainID.
	LocalnetChainID = LocalChainID
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

	// Load C-Chain genesis. Operator override:
	//   LUX_CCHAIN_GENESIS_FILE — absolute path to a JSON file that
	//   replaces the embedded cchain.json verbatim. Lets downstream
	//   networks (Liquidity at chainId 8675312, etc.) reuse the lqd
	//   binary without forking the embedded genesis tree.
	//
	// Unset → embedded default (per-network, immutable).
	var cchainData []byte
	if override := os.Getenv("LUX_CCHAIN_GENESIS_FILE"); override != "" {
		body, ferr := os.ReadFile(override)
		if ferr != nil {
			return nil, fmt.Errorf("read LUX_CCHAIN_GENESIS_FILE=%q: %w", override, ferr)
		}
		cchainData = body
	} else {
		cchainData, err = embeddedGenesis.ReadFile(filepath.Join(networkName, "cchain.json"))
		if err != nil {
			return nil, fmt.Errorf("failed to read cchain.json: %w", err)
		}
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

	// The embedded genesis files use string-encoded addresses (ConfigOutput format).
	// Parse as ConfigOutput first, then convert to Config.
	var output genesis.ConfigOutput
	if err := json.Unmarshal(data, &output); err != nil {
		return nil, fmt.Errorf("failed to parse genesis config: %w", err)
	}

	// Convert ConfigOutput → Config by parsing string addresses to binary.
	return genesis.ParseConfigOutput(&output, networkID)
}

// IsCustom reports whether the networkID is a user-defined network
// outside the well-known {Mainnet, Testnet, Devnet, Local + their
// C-Chain aliases} set. Mirrors luxfi/constants.IsCustom; both the
// genesis layer and the constants layer agree on the classification
// so a network can be classified consistently end-to-end.
func IsCustom(networkID uint32) bool {
	switch networkID {
	case MainnetID, MainnetChainID,
		TestnetID, TestnetChainID,
		DevnetID, DevnetChainID,
		LocalID, LocalChainID:
		return false
	}
	return true
}

// networkNameFromID returns the network directory name for a network ID.
// Accepts both network IDs (1, 2, 3, 1337) and chain IDs (96369, 96368, 96370, 31337)
// as aliases. User-defined custom networks return "" — callers must
// supply a genesis file (`--genesis-file`) for them since there are no
// embedded canonical configs to load from disk.
func networkNameFromID(networkID uint32) string {
	switch networkID {
	case MainnetID, MainnetChainID:
		return "mainnet"
	case TestnetID, TestnetChainID:
		return "testnet"
	case DevnetID, DevnetChainID:
		return "devnet"
	case LocalID, LocalChainID:
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
