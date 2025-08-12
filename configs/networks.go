package configs

import (
	_ "embed"
)

// Network genesis configurations for all supported networks
// These are embedded at compile time for reliable access

var (
	//go:embed genesis_96369.json
	Genesis96369 []byte // Lux Mainnet

	//go:embed genesis_96368.json
	Genesis96368 []byte // Lux Testnet

	//go:embed genesis_7777.json
	Genesis7777 []byte // Lux Genesis/Local

	//go:embed genesis_200200.json
	Genesis200200 []byte // Zoo Mainnet

	//go:embed genesis_200201.json
	Genesis200201 []byte // Zoo Testnet

	//go:embed genesis_36911.json
	Genesis36911 []byte // SPC Mainnet
)

// NetworkConfig represents a network's configuration
type NetworkConfig struct {
	NetworkID   int    `json:"networkId"`
	ChainID     int64  `json:"chainId"`
	Name        string `json:"name"`
	GenesisJSON []byte `json:"-"`
}

// Networks maps chain IDs to their configurations
var Networks = map[int64]*NetworkConfig{
	96369: {
		NetworkID:   96369,
		ChainID:     96369,
		Name:        "Lux Mainnet",
		GenesisJSON: Genesis96369,
	},
	96368: {
		NetworkID:   96368,
		ChainID:     96368,
		Name:        "Lux Testnet",
		GenesisJSON: Genesis96368,
	},
	7777: {
		NetworkID:   7777,
		ChainID:     7777,
		Name:        "Lux Genesis/Local",
		GenesisJSON: Genesis7777,
	},
	200200: {
		NetworkID:   200200,
		ChainID:     200200,
		Name:        "Zoo Mainnet",
		GenesisJSON: Genesis200200,
	},
	200201: {
		NetworkID:   200201,
		ChainID:     200201,
		Name:        "Zoo Testnet",
		GenesisJSON: Genesis200201,
	},
	36911: {
		NetworkID:   36911,
		ChainID:     36911,
		Name:        "SPC Mainnet",
		GenesisJSON: Genesis36911,
	},
}

// GetGenesisForChainID returns the genesis configuration for a given chain ID
func GetGenesisForChainID(chainID int64) ([]byte, bool) {
	if config, exists := Networks[chainID]; exists {
		return config.GenesisJSON, true
	}
	return nil, false
}

// GetGenesisJSONForChainID returns the genesis as a JSON string
func GetGenesisJSONForChainID(chainID int64) (string, bool) {
	if genesis, exists := GetGenesisForChainID(chainID); exists {
		return string(genesis), true
	}
	return "", false
}