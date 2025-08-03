package launch

import "github.com/luxfi/genesis/pkg/core"

// Presets contains predefined network configurations
var Presets = map[string]core.Network{
	"mainnet": {
		Name:      "mainnet",
		NetworkID: 96369,
		ChainID:   96369,
		Nodes:     1,
		Genesis: core.GenesisConfig{
			Source:     "import",
			ImportPath: "/home/z/work/lux/genesis/state/chaindata",
		},
		// Single node 1/1 consensus for mainnet replay
		Consensus: core.ConsensusConfig{K: 1, Alpha: 1, Beta: 1},
	},
	"dev": {
		Name:      "dev",
		NetworkID: 12345,
		ChainID:   12345,
		Nodes:     1,
		Genesis: core.GenesisConfig{
			Source: "fresh",
			Allocations: map[string]uint64{
				"P-lux1q9c6ltuxpsqz7ul8j0h0d0ha439qt70sr3x2z0": 100_000_000_000_000,
			},
		},
		// Consensus will be auto-configured as 1/1/1
	},
	"local": {
		Name:      "local",
		NetworkID: 12345,
		ChainID:   12345,
		Nodes:     5, // Default 5 nodes
		Genesis: core.GenesisConfig{
			Source: "fresh",
			Allocations: map[string]uint64{
				"P-lux1q9c6ltuxpsqz7ul8j0h0d0ha439qt70sr3x2z0": 100_000_000_000_000,
			},
		},
		// Consensus will be auto-configured based on node count
	},
	"zoo": {
		Name:      "zoo",
		NetworkID: 200200,
		ChainID:   200200,
		Nodes:     1,
		Genesis:   core.GenesisConfig{Source: "fresh"},
		Consensus: core.ConsensusConfig{K: 3, Alpha: 3, Beta: 5},
		Metadata: map[string]interface{}{
			"l2-base-chain": "lux",
			"token-symbol":  "ZOO",
		},
	},
	"spc": {
		Name:      "spc",
		NetworkID: 36911,
		ChainID:   36911,
		Nodes:     1,
		Genesis:   core.GenesisConfig{Source: "fresh"},
		Consensus: core.ConsensusConfig{K: 3, Alpha: 3, Beta: 5},
		Metadata: map[string]interface{}{
			"l2-base-chain": "lux",
			"token-symbol":  "SPC",
		},
	},
	"hanzo": {
		Name:      "hanzo",
		NetworkID: 36963,
		ChainID:   36963,
		Nodes:     1,
		Genesis:   core.GenesisConfig{Source: "fresh"},
		Consensus: core.ConsensusConfig{K: 3, Alpha: 3, Beta: 5},
		Metadata: map[string]interface{}{
			"l2-base-chain": "lux",
			"token-symbol":  "HANZO",
		},
	},
}