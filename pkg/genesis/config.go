// Copyright (C) 2019-2025, Lux Partners Limited. All rights reserved.
// See the file LICENSE for licensing terms.

package genesis

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/luxfi/constants"
	"github.com/luxfi/ids"
)

// GetConfig returns the genesis config for a network, loading from files
func GetConfig(networkID uint32) *Config {
	// Try to load from standard locations based on network
	var networkName string
	switch networkID {
	case constants.MainnetID, constants.MainnetChainID:
		networkName = "mainnet"
	case constants.TestnetID, constants.TestnetChainID:
		networkName = "testnet"
	case constants.DevnetID, constants.DevnetChainID:
		networkName = "devnet"
	case constants.LocalID:
		networkName = "custom"
	default:
		networkName = "custom"
	}

	// Try standard genesis locations (user overrides checked first)
	home, _ := os.UserHomeDir()
	candidates := []string{
		// User override (highest priority)
		filepath.Join(home, ".lux/genesis", networkName),
		// System paths
		filepath.Join("/etc/lux/genesis", networkName),
		// Docker container paths
		filepath.Join("/app/genesis", networkName),
		filepath.Join("/app/configs/genesis", networkName),
		// Source tree (lowest priority)
		filepath.Join(home, "work/lux/genesis/configs", networkName),
	}

	for _, dir := range candidates {
		if config, err := GetConfigFromDir(dir); err == nil {
			return config
		}
	}

	// Return empty config - must be built dynamically
	return &Config{
		NetworkID: networkID,
		Message:   fmt.Sprintf("Lux %s Genesis", networkName),
	}
}

// GetConfigFile loads genesis config from a single JSON file
// The JSON file should be in ConfigOutput format (string-encoded addresses)
// which is the standard serialization format for genesis files.
func GetConfigFile(filepath string) (*Config, error) {
	data, err := os.ReadFile(filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to read genesis file: %w", err)
	}

	// Unmarshal into ConfigOutput first (uses string fields for addresses)
	var output ConfigOutput
	if err := json.Unmarshal(data, &output); err != nil {
		return nil, fmt.Errorf("failed to parse genesis config: %w", err)
	}

	// Parse allocations from string format
	allocations, err := parseAllocations(output.Allocations)
	if err != nil {
		return nil, fmt.Errorf("failed to parse allocations: %w", err)
	}

	// Parse stakers from string format
	stakers, err := parseStakers(output.InitialStakers)
	if err != nil {
		return nil, fmt.Errorf("failed to parse stakers: %w", err)
	}

	// Parse staked funds addresses
	stakedFunds, err := parseAddresses(output.InitialStakedFunds)
	if err != nil {
		return nil, fmt.Errorf("failed to parse staked funds: %w", err)
	}

	return &Config{
		NetworkID:                  output.NetworkID,
		Allocations:                allocations,
		StartTime:                  output.StartTime,
		InitialStakeDuration:       output.InitialStakeDuration,
		InitialStakeDurationOffset: output.InitialStakeDurationOffset,
		InitialStakedFunds:         stakedFunds,
		InitialStakers:             stakers,
		CChainGenesis:              output.CChainGenesis,
		DChainGenesis:              output.DChainGenesis,
		QChainGenesis:              output.QChainGenesis,
		AChainGenesis:              output.AChainGenesis,
		BChainGenesis:              output.BChainGenesis,
		TChainGenesis:              output.TChainGenesis,
		ZChainGenesis:              output.ZChainGenesis,
		GChainGenesis:              output.GChainGenesis,
		KChainGenesis:              output.KChainGenesis,
		Message:                    output.Message,
	}, nil
}

// GetConfigFromDir builds genesis config from component files in a directory
// Expects: network.json, pchain.json, cchain.json
func GetConfigFromDir(dir string) (*Config, error) {
	// Load network config
	networkPath := filepath.Join(dir, "network.json")
	networkData, err := os.ReadFile(networkPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read network.json: %w", err)
	}
	var network NetworkConfig
	if err := json.Unmarshal(networkData, &network); err != nil {
		return nil, fmt.Errorf("failed to parse network.json: %w", err)
	}

	// Load P-Chain config
	pchainPath := filepath.Join(dir, "pchain.json")
	pchainData, err := os.ReadFile(pchainPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read pchain.json: %w", err)
	}
	var pchain PChainConfig
	if err := json.Unmarshal(pchainData, &pchain); err != nil {
		return nil, fmt.Errorf("failed to parse pchain.json: %w", err)
	}

	// Load C-Chain config (kept as raw JSON string)
	cchainPath := filepath.Join(dir, "cchain.json")
	cchainData, err := os.ReadFile(cchainPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read cchain.json: %w", err)
	}

	// Convert allocations from JSON format to internal format
	allocations, err := parseAllocations(pchain.Allocations)
	if err != nil {
		return nil, fmt.Errorf("failed to parse allocations: %w", err)
	}

	// Convert stakers from JSON format to internal format
	stakers, err := parseStakers(pchain.InitialStakers)
	if err != nil {
		return nil, fmt.Errorf("failed to parse stakers: %w", err)
	}

	// Parse initial staked funds addresses
	stakedFunds, err := parseAddresses(pchain.InitialStakedFunds)
	if err != nil {
		return nil, fmt.Errorf("failed to parse staked funds: %w", err)
	}

	return &Config{
		NetworkID:                  network.NetworkID,
		Allocations:                allocations,
		StartTime:                  network.StartTime,
		InitialStakeDuration:       pchain.InitialStakeDuration,
		InitialStakeDurationOffset: pchain.InitialStakeDurationOffset,
		InitialStakedFunds:         stakedFunds,
		InitialStakers:             stakers,
		CChainGenesis:              string(cchainData),
		Message:                    network.Message,
	}, nil
}

// GetConfigFromEnv builds genesis config using environment variables
// Environment variables:
//   - LUX_NETWORK_ID: network ID (default: custom)
//   - LUX_GENESIS_DIR: directory containing genesis files
//   - LUX_KEYS_DIR: directory containing node keys (default: ~/.lux/keys)
func GetConfigFromEnv() (*Config, error) {
	networkID := uint32(constants.CustomID)
	if envID := os.Getenv("LUX_NETWORK_ID"); envID != "" {
		var id uint32
		if _, err := fmt.Sscanf(envID, "%d", &id); err == nil {
			networkID = id
		}
	}

	// Get genesis directory from env
	genesisDir := os.Getenv("LUX_GENESIS_DIR")
	if genesisDir != "" {
		config, err := GetConfigFromDir(genesisDir)
		if err == nil {
			config.NetworkID = networkID
			return config, nil
		}
	}

	// Fall back to standard config
	return GetConfig(networkID), nil
}

// parseAllocations converts JSON allocations to internal format
func parseAllocations(jsonAllocs []AllocationJSON) ([]Allocation, error) {
	result := make([]Allocation, 0, len(jsonAllocs))
	for _, ja := range jsonAllocs {
		ethAddr, err := ParseETHAddress(ja.ETHAddr)
		if err != nil {
			return nil, fmt.Errorf("invalid eth address %s: %w", ja.ETHAddr, err)
		}
		luxAddr, err := ParseAddress(ja.LUXAddr)
		if err != nil {
			return nil, fmt.Errorf("invalid lux address %s: %w", ja.LUXAddr, err)
		}
		result = append(result, Allocation{
			ETHAddr:        ethAddr,
			LUXAddr:        luxAddr,
			InitialAmount:  ja.InitialAmount,
			UnlockSchedule: ja.UnlockSchedule,
		})
	}
	return result, nil
}

// parseStakers converts JSON stakers to internal format
func parseStakers(jsonStakers []StakerJSON) ([]Staker, error) {
	result := make([]Staker, 0, len(jsonStakers))
	for _, js := range jsonStakers {
		nodeID, err := ids.NodeIDFromString(js.NodeID)
		if err != nil {
			return nil, fmt.Errorf("invalid node ID %s: %w", js.NodeID, err)
		}
		rewardAddr, err := ParseAddress(js.RewardAddress)
		if err != nil {
			return nil, fmt.Errorf("invalid reward address %s: %w", js.RewardAddress, err)
		}
		result = append(result, Staker{
			NodeID:        nodeID,
			RewardAddress: rewardAddr,
			DelegationFee: js.DelegationFee,
			Signer:        js.Signer,
			Weight:        js.Weight,
			StartTime:     js.StartTime,
			EndTime:       js.EndTime,
		})
	}
	return result, nil
}

// parseAddresses converts string addresses to ShortIDs
func parseAddresses(addrs []string) ([]ids.ShortID, error) {
	result := make([]ids.ShortID, 0, len(addrs))
	for _, addr := range addrs {
		parsed, err := ParseAddress(addr)
		if err != nil {
			return nil, fmt.Errorf("invalid address %s: %w", addr, err)
		}
		result = append(result, parsed)
	}
	return result, nil
}
