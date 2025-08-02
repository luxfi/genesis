package consensus

import (
	"time"
)

// ConsensusParams defines consensus parameters for each chain
type ConsensusParams struct {
	K                     int           `json:"k"`
	AlphaPreference       int           `json:"alphaPreference"`
	AlphaConfidence       int           `json:"alphaConfidence"`
	Beta                  int           `json:"beta"`
	MaxItemProcessingTime time.Duration `json:"maxItemProcessingTime"`
	ConcurrentRepolls     int           `json:"concurrentRepolls"`
	OptimalProcessing     int           `json:"optimalProcessing"`
	MaxOutstandingItems   int           `json:"maxOutstandingItems"`
	MaxItemProcessingTimeStr string     `json:"maxItemProcessingTimeString"`
}

// ChainConsensusParams holds consensus parameters for all supported chains
var ChainConsensusParams = map[string]ConsensusParams{
	// Lux Mainnet (21 validators - 9.63s consensus)
	"lux-mainnet": {
		K:                     21,
		AlphaPreference:       13,
		AlphaConfidence:       18,
		Beta:                  8,
		MaxItemProcessingTime: 9630 * time.Millisecond,
		ConcurrentRepolls:     4,
		OptimalProcessing:     10,
		MaxOutstandingItems:   256,
		MaxItemProcessingTimeStr: "9.63s",
	},
	
	// Lux Testnet (11 validators - 6.3s consensus)
	"lux-testnet": {
		K:                     11,
		AlphaPreference:       7,
		AlphaConfidence:       9,
		Beta:                  6,
		MaxItemProcessingTime: 6300 * time.Millisecond,
		ConcurrentRepolls:     4,
		OptimalProcessing:     10,
		MaxOutstandingItems:   256,
		MaxItemProcessingTimeStr: "6.3s",
	},
	
	// Lux Local (5 validators - 3.69s consensus)
	"lux-local": {
		K:                     5,
		AlphaPreference:       3,
		AlphaConfidence:       4,
		Beta:                  3,
		MaxItemProcessingTime: 3690 * time.Millisecond,
		ConcurrentRepolls:     4,
		OptimalProcessing:     10,
		MaxOutstandingItems:   256,
		MaxItemProcessingTimeStr: "3.69s",
	},
	
	// Zoo Mainnet (L2 on Lux)
	"zoo-mainnet": {
		K:                     15,
		AlphaPreference:       9,
		AlphaConfidence:       12,
		Beta:                  7,
		MaxItemProcessingTime: 7500 * time.Millisecond,
		ConcurrentRepolls:     4,
		OptimalProcessing:     10,
		MaxOutstandingItems:   256,
		MaxItemProcessingTimeStr: "7.5s",
	},
	
	// Zoo Testnet
	"zoo-testnet": {
		K:                     7,
		AlphaPreference:       5,
		AlphaConfidence:       6,
		Beta:                  4,
		MaxItemProcessingTime: 4200 * time.Millisecond,
		ConcurrentRepolls:     4,
		OptimalProcessing:     10,
		MaxOutstandingItems:   256,
		MaxItemProcessingTimeStr: "4.2s",
	},
	
	// SPC Mainnet (Specialized Processing Chain)
	"spc-mainnet": {
		K:                     19,
		AlphaPreference:       12,
		AlphaConfidence:       16,
		Beta:                  7,
		MaxItemProcessingTime: 8500 * time.Millisecond,
		ConcurrentRepolls:     4,
		OptimalProcessing:     10,
		MaxOutstandingItems:   256,
		MaxItemProcessingTimeStr: "8.5s",
	},
	
	// SPC Testnet
	"spc-testnet": {
		K:                     9,
		AlphaPreference:       6,
		AlphaConfidence:       8,
		Beta:                  5,
		MaxItemProcessingTime: 5400 * time.Millisecond,
		ConcurrentRepolls:     4,
		OptimalProcessing:     10,
		MaxOutstandingItems:   256,
		MaxItemProcessingTimeStr: "5.4s",
	},
	
	// Hanzo Mainnet (AI Chain)
	"hanzo-mainnet": {
		K:                     17,
		AlphaPreference:       11,
		AlphaConfidence:       14,
		Beta:                  7,
		MaxItemProcessingTime: 8100 * time.Millisecond,
		ConcurrentRepolls:     4,
		OptimalProcessing:     10,
		MaxOutstandingItems:   256,
		MaxItemProcessingTimeStr: "8.1s",
	},
	
	// Hanzo Testnet
	"hanzo-testnet": {
		K:                     9,
		AlphaPreference:       6,
		AlphaConfidence:       8,
		Beta:                  5,
		MaxItemProcessingTime: 5100 * time.Millisecond,
		ConcurrentRepolls:     4,
		OptimalProcessing:     10,
		MaxOutstandingItems:   256,
		MaxItemProcessingTimeStr: "5.1s",
	},
	
	// Quantum Mainnet
	"quantum-mainnet": {
		K:                     7,
		AlphaPreference:       5,
		AlphaConfidence:       6,
		Beta:                  4,
		MaxItemProcessingTime: 3690 * time.Millisecond,
		ConcurrentRepolls:     8, // Higher for quantum consensus
		OptimalProcessing:     20,
		MaxOutstandingItems:   512,
		MaxItemProcessingTimeStr: "3.69s",
	},
}

// ChainInfo contains all chain information
type ChainInfo struct {
	ChainID    uint64          `json:"chainId"`
	Name       string          `json:"name"`
	Network    string          `json:"network"`
	Type       string          `json:"type"`
	BaseChain  string          `json:"baseChain,omitempty"`
	Consensus  ConsensusParams `json:"consensus"`
}

// AllChains contains configuration for all 8 chains
var AllChains = map[string]ChainInfo{
	"lux-mainnet": {
		ChainID:   96369,
		Name:      "Lux Mainnet",
		Network:   "lux-mainnet",
		Type:      "l1",
		Consensus: ChainConsensusParams["lux-mainnet"],
	},
	"lux-testnet": {
		ChainID:   96368,
		Name:      "Lux Testnet",
		Network:   "lux-testnet",
		Type:      "l1",
		Consensus: ChainConsensusParams["lux-testnet"],
	},
	"lux-local": {
		ChainID:   1337,
		Name:      "Lux Local",
		Network:   "lux-local",
		Type:      "l1",
		Consensus: ChainConsensusParams["lux-local"],
	},
	"zoo-mainnet": {
		ChainID:   200200,
		Name:      "Zoo Mainnet",
		Network:   "zoo-mainnet",
		Type:      "l2",
		BaseChain: "lux",
		Consensus: ChainConsensusParams["zoo-mainnet"],
	},
	"zoo-testnet": {
		ChainID:   200201,
		Name:      "Zoo Testnet",
		Network:   "zoo-testnet",
		Type:      "l2",
		BaseChain: "lux",
		Consensus: ChainConsensusParams["zoo-testnet"],
	},
	"spc-mainnet": {
		ChainID:   36911,
		Name:      "SPC Mainnet",
		Network:   "spc-mainnet",
		Type:      "l1",
		Consensus: ChainConsensusParams["spc-mainnet"],
	},
	"spc-testnet": {
		ChainID:   36912,
		Name:      "SPC Testnet",
		Network:   "spc-testnet",
		Type:      "l1",
		Consensus: ChainConsensusParams["spc-testnet"],
	},
	"hanzo-mainnet": {
		ChainID:   36963,
		Name:      "Hanzo Mainnet",
		Network:   "hanzo-mainnet",
		Type:      "l1",
		Consensus: ChainConsensusParams["hanzo-mainnet"],
	},
	"hanzo-testnet": {
		ChainID:   36962,
		Name:      "Hanzo Testnet",
		Network:   "hanzo-testnet",
		Type:      "l1",
		Consensus: ChainConsensusParams["hanzo-testnet"],
	},
	"quantum-mainnet": {
		ChainID:   369369,
		Name:      "Quantum Mainnet",
		Network:   "quantum-mainnet",
		Type:      "quantum",
		Consensus: ChainConsensusParams["quantum-mainnet"],
	},
}

// GetChainInfo returns chain information by network name
func GetChainInfo(network string) (*ChainInfo, bool) {
	info, exists := AllChains[network]
	return &info, exists
}

// GetConsensusParams returns consensus parameters for a network
func GetConsensusParams(network string) (*ConsensusParams, bool) {
	params, exists := ChainConsensusParams[network]
	return &params, exists
}