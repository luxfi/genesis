// Package commands contains all genesis command implementations
package commands

import (
	"fmt"
	"os"
	"path/filepath"
	
	"github.com/luxfi/genesis/pkg/core"
	"github.com/luxfi/genesis/pkg/unified"
	"github.com/spf13/cobra"
)

// GenerateOptions contains options for the generate command
type GenerateOptions struct {
	Network      string
	ChainType    string
	OutputDir    string
	ChainID      uint64
	ValidatorCount int
}

// NewGenerateCommand creates the generate command
func NewGenerateCommand() *cobra.Command {
	opts := &GenerateOptions{}
	
	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate genesis files",
		Long:  `Generate genesis files for P, C, and X chains using the unified system`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return RunGenerate(opts)
		},
	}
	
	cmd.Flags().StringVar(&opts.Network, "network", "mainnet", "Network name (mainnet, testnet, local)")
	cmd.Flags().StringVar(&opts.ChainType, "type", "unified", "Chain type (unified, p, c, x)")
	cmd.Flags().StringVar(&opts.OutputDir, "output", "./configs", "Output directory")
	cmd.Flags().Uint64Var(&opts.ChainID, "chain-id", 0, "Chain ID (overrides network default)")
	cmd.Flags().IntVar(&opts.ValidatorCount, "validators", 0, "Number of validators (overrides network default)")
	
	return cmd
}

// RunGenerate executes the generate command
func RunGenerate(opts *GenerateOptions) error {
	// Determine network configuration
	network := resolveNetwork(opts.Network)
	if opts.ChainID != 0 {
		network.ChainID = opts.ChainID
		network.NetworkID = opts.ChainID
	}
	
	// Select appropriate allocations and validators
	var allocations map[string]uint64
	var validators []core.Validator
	
	switch opts.Network {
	case "mainnet":
		allocations = unified.MainnetAllocations()
		validators = unified.MainnetValidators()
		if opts.ValidatorCount == 21 {
			// Use 21 validators
		} else if opts.ValidatorCount == 11 {
			// Reduce to 11 for testing
			validators = validators[:11]
		}
		
	case "testnet":
		allocations = unified.MainnetAllocations() // Same allocations
		validators = unified.TestnetValidators()
		
	case "local":
		// Use mnemonic-derived addresses for local
		transform := unified.LocalNetworkTransform("light light light light light light light light light light light energy")
		// Apply transform later
		allocations = unified.MainnetAllocations()
		validators = []core.Validator{
			{
				NodeID:        "NodeID-7Xhw2mDxuDS44j42TCB6U5579esbSt3Lg",
				RewardAddress: "X-local1q9c6ltuxpsqz7ul8j0h0d0ha439qt70sr3x2z0",
				DelegationFee: 20000,
			},
		}
		
	default:
		return fmt.Errorf("unknown network: %s", opts.Network)
	}
	
	// Override validator count if specified
	if opts.ValidatorCount > 0 && opts.ValidatorCount < len(validators) {
		validators = validators[:opts.ValidatorCount]
	}
	
	// Create output directory
	outputPath := filepath.Join(opts.OutputDir, fmt.Sprintf("lux-%s-%d", opts.Network, network.NetworkID))
	
	// Build genesis based on chain type
	switch opts.ChainType {
	case "unified":
		return generateUnified(network, outputPath, allocations, validators)
	case "p":
		return generatePChain(network, outputPath, allocations, validators)
	case "c":
		return generateCChain(network, outputPath, allocations)
	case "x":
		return generateXChain(network, outputPath, allocations)
	default:
		return fmt.Errorf("unknown chain type: %s", opts.ChainType)
	}
}

func generateUnified(network core.Network, outputPath string, allocations map[string]uint64, validators []core.Validator) error {
	builder := unified.NewUnifiedBuilder(network).
		With(
			unified.WithAllocations[*unified.UnifiedGenesis](allocations),
			unified.WithValidators[*unified.UnifiedGenesis](validators),
		)
	
	// Add vesting for mainnet/testnet
	if network.Name == "mainnet" || network.Name == "testnet" {
		builder = builder.Transform(func(genesis *unified.UnifiedGenesis) (*unified.UnifiedGenesis, error) {
			// Apply vesting to P-Chain
			pTransformed, err := unified.VestingTransform()(genesis.P)
			if err != nil {
				return nil, err
			}
			genesis.P = pTransformed
			return genesis, nil
		})
	}
	
	// Generate
	genesis, err := builder.Generate()
	if err != nil {
		return fmt.Errorf("failed to generate unified genesis: %w", err)
	}
	
	// Validate
	if err := builder.Validate(genesis); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}
	
	// Export
	if err := builder.Export(genesis, outputPath); err != nil {
		return fmt.Errorf("failed to export genesis: %w", err)
	}
	
	fmt.Printf("✅ Generated unified genesis in %s\n", outputPath)
	return nil
}

func generatePChain(network core.Network, outputPath string, allocations map[string]uint64, validators []core.Validator) error {
	builder := unified.NewPChainBuilder(network).
		With(
			unified.WithAllocations[*unified.PChainGenesis](allocations),
			unified.WithValidators[*unified.PChainGenesis](validators),
		)
	
	genesis, err := builder.Generate()
	if err != nil {
		return fmt.Errorf("failed to generate P-Chain genesis: %w", err)
	}
	
	if err := builder.Validate(genesis); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}
	
	outputFile := filepath.Join(outputPath, "P", "genesis.json")
	if err := builder.Export(genesis, outputFile); err != nil {
		return fmt.Errorf("failed to export genesis: %w", err)
	}
	
	fmt.Printf("✅ Generated P-Chain genesis in %s\n", outputFile)
	return nil
}

func generateCChain(network core.Network, outputPath string, allocations map[string]uint64) error {
	builder := unified.NewCChainBuilder(network).
		With(unified.WithAllocations[*unified.CChainGenesis](allocations))
	
	genesis, err := builder.Generate()
	if err != nil {
		return fmt.Errorf("failed to generate C-Chain genesis: %w", err)
	}
	
	if err := builder.Validate(genesis); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}
	
	outputFile := filepath.Join(outputPath, "C", "genesis.json")
	if err := builder.Export(genesis, outputFile); err != nil {
		return fmt.Errorf("failed to export genesis: %w", err)
	}
	
	fmt.Printf("✅ Generated C-Chain genesis in %s\n", outputFile)
	return nil
}

func generateXChain(network core.Network, outputPath string, allocations map[string]uint64) error {
	builder := unified.NewXChainBuilder(network).
		With(unified.WithAllocations[*unified.XChainGenesis](allocations))
	
	genesis, err := builder.Generate()
	if err != nil {
		return fmt.Errorf("failed to generate X-Chain genesis: %w", err)
	}
	
	if err := builder.Validate(genesis); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}
	
	outputFile := filepath.Join(outputPath, "X", "genesis.json")
	if err := builder.Export(genesis, outputFile); err != nil {
		return fmt.Errorf("failed to export genesis: %w", err)
	}
	
	fmt.Printf("✅ Generated X-Chain genesis in %s\n", outputFile)
	return nil
}

func resolveNetwork(name string) core.Network {
	switch name {
	case "mainnet":
		return core.Network{
			Name:      "mainnet",
			NetworkID: 96369,
			ChainID:   96369,
			Genesis: core.GenesisConfig{
				Source:  "fresh",
				Message: "lux mainnet genesis",
			},
			Consensus: core.ConsensusConfig{
				K:     20,
				Alpha: 15,
				Beta:  20,
			},
		}
	case "testnet":
		return core.Network{
			Name:      "testnet",
			NetworkID: 96368,
			ChainID:   96368,
			Genesis: core.GenesisConfig{
				Source:  "fresh",
				Message: "lux testnet genesis",
			},
			Consensus: core.ConsensusConfig{
				K:     20,
				Alpha: 15,
				Beta:  20,
			},
		}
	case "local":
		return core.Network{
			Name:      "local",
			NetworkID: 12345,
			ChainID:   12345,
			Genesis: core.GenesisConfig{
				Source:  "fresh",
				Message: "local test network",
			},
			Consensus: core.ConsensusConfig{
				K:     1,
				Alpha: 1,
				Beta:  1,
			},
		}
	default:
		// Custom network
		return core.Network{
			Name:      name,
			NetworkID: 99999,
			ChainID:   99999,
			Genesis: core.GenesisConfig{
				Source:  "fresh",
				Message: fmt.Sprintf("%s network genesis", name),
			},
			Consensus: core.ConsensusConfig{
				K:     5,
				Alpha: 4,
				Beta:  5,
			},
		}
	}
}