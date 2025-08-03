package commands

import (
	"encoding/json"
	"fmt"

	"github.com/luxfi/genesis/pkg/consensus"
	"github.com/spf13/cobra"
)

// ConsensusOptions contains options for consensus commands
type ConsensusOptions struct {
	Network string
}

// NewConsensusCommand creates the consensus command
func NewConsensusCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "consensus",
		Short: "Manage consensus parameters",
		Long:  "View and manage consensus parameters for all supported chains",
	}

	cmd.AddCommand(
		NewConsensusListCommand(),
		NewConsensusShowCommand(),
		NewConsensusUpdateCommand(),
	)

	return cmd
}

// NewConsensusListCommand creates the consensus list command
func NewConsensusListCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List consensus parameters for all chains",
		RunE: func(cmd *cobra.Command, args []string) error {
			return RunListConsensus()
		},
	}
	
	return cmd
}

// NewConsensusShowCommand creates the consensus show command
func NewConsensusShowCommand() *cobra.Command {
	opts := &ConsensusOptions{}
	
	cmd := &cobra.Command{
		Use:   "show [network]",
		Short: "Show consensus parameters for a specific network",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Network = args[0]
			return RunShowConsensus(opts)
		},
	}
	
	return cmd
}

// NewConsensusUpdateCommand creates the consensus update command
func NewConsensusUpdateCommand() *cobra.Command {
	opts := &ConsensusOptions{}
	
	cmd := &cobra.Command{
		Use:   "update [network]",
		Short: "Update consensus parameters for a network",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Network = args[0]
			return RunUpdateConsensus(opts)
		},
	}
	
	return cmd
}

// NewMinimalConsensusCommand creates the minimal-consensus command (backward compatibility)
func NewMinimalConsensusCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "minimal-consensus",
		Short: "Generate minimal consensus configuration",
		Long:  "Generate a minimal consensus configuration for local testing (1/1 consensus)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return RunMinimalConsensus()
		},
	}
	
	return cmd
}

// Implementation functions

func RunListConsensus() error {
	fmt.Println("Consensus Parameters for All Chains:")
	fmt.Println("=====================================")
	
	for _, info := range consensus.AllChains {
		fmt.Printf("\n%s (Chain ID: %d)\n", info.Name, info.ChainID)
		fmt.Printf("  Type: %s\n", info.Type)
		if info.BaseChain != "" {
			fmt.Printf("  Base Chain: %s\n", info.BaseChain)
		}
		fmt.Printf("  Consensus:\n")
		fmt.Printf("    K: %d\n", info.Consensus.K)
		fmt.Printf("    Alpha Preference: %d\n", info.Consensus.AlphaPreference)
		fmt.Printf("    Alpha Confidence: %d\n", info.Consensus.AlphaConfidence)
		fmt.Printf("    Beta: %d\n", info.Consensus.Beta)
		fmt.Printf("    Max Processing Time: %s\n", info.Consensus.MaxItemProcessingTimeStr)
	}
	
	return nil
}

func RunShowConsensus(opts *ConsensusOptions) error {
	info, exists := consensus.GetChainInfo(opts.Network)
	if !exists {
		return fmt.Errorf("unknown network: %s", opts.Network)
	}
	
	// Output as JSON for easy parsing
	data, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal consensus info: %w", err)
	}
	
	fmt.Println(string(data))
	return nil
}

func RunUpdateConsensus(opts *ConsensusOptions) error {
	// TODO: Implement consensus parameter updates
	// This would read new params from flags and update the configuration
	fmt.Printf("Updating consensus parameters for %s...\n", opts.Network)
	fmt.Println("(Not implemented yet)")
	return nil
}

func RunMinimalConsensus() error {
	// Generate minimal consensus config for testing
	config := map[string]interface{}{
		"consensus": map[string]interface{}{
			"snow-sample-size": 1,
			"snow-quorum-size": 1,
			"snow-concurrent-repolls": 0,
			"snow-max-processing-time": "120s",
			"snow-max-time-processing": "120s",
			"k": 1,
			"alpha": 1,
			"beta": 1,
		},
		"bootstrap": map[string]interface{}{
			"max-time-getting-ancestors": "50ms",
			"ancestors-max-containers-sent": 2000,
			"ancestors-max-containers-received": 2000,
		},
		"network": map[string]interface{}{
			"max-reconnect-delay": "1s",
			"health-check-frequency": "2s",
			"peerlist-gossip-frequency": "250ms",
			"ping-frequency": "1s",
			"ping-timeout": "30s",
		},
	}
	
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}
	
	fmt.Println("Minimal consensus configuration:")
	fmt.Println(string(data))
	return nil
}