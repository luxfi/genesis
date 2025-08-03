package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/luxfi/genesis/pkg/core"
	"github.com/luxfi/genesis/pkg/launch"
	"github.com/spf13/cobra"
)

// getLaunchCmd returns the unified launch command
func getLaunchCmd() *cobra.Command {
	var (
		preset    string
		nodes     int
		networkID uint64
		chainID   uint64
		dryRun    bool
		config    string
		baseDir   string
	)
	
	cmd := &cobra.Command{
		Use:   "launch",
		Short: "Launch any network with unified system",
		Long: `Launch networks using a unified, composable system.
		
Everything is just configuration - no special cases.

Examples:
  # Use a preset
  genesis launch --preset dev
  genesis launch --preset mainnet
  genesis launch --preset zoo
  
  # Override preset values
  genesis launch --preset dev --nodes 5
  
  # Custom network from config file
  genesis launch --config mynetwork.json
  
  # Fully custom
  genesis launch --name custom --network-id 99999 --chain-id 99999`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var network core.Network
			
			// Load configuration
			if config != "" {
				// Load from file
				data, err := os.ReadFile(config)
				if err != nil {
					return fmt.Errorf("failed to read config: %w", err)
				}
				if err := json.Unmarshal(data, &network); err != nil {
					return fmt.Errorf("invalid config: %w", err)
				}
			} else if preset != "" {
				// Use preset
				var ok bool
				network, ok = launch.Presets[preset]
				if !ok {
					return fmt.Errorf("unknown preset: %s\nAvailable: dev, local, mainnet, zoo, spc, hanzo", preset)
				}
			} else {
				// Build from flags
				name, _ := cmd.Flags().GetString("name")
				if name == "" {
					return fmt.Errorf("--name required without preset or config")
				}
				
				network = core.Network{
					Name:      name,
					NetworkID: networkID,
					ChainID:   chainID,
					Nodes:     nodes,
					Genesis: core.GenesisConfig{
						Source: "fresh",
						Allocations: map[string]uint64{
							"P-lux1q9c6ltuxpsqz7ul8j0h0d0ha439qt70sr3x2z0": 100_000_000_000_000,
						},
					},
				}
			}
			
			// Apply overrides
			if cmd.Flags().Changed("nodes") {
				network.Nodes = nodes
			}
			if cmd.Flags().Changed("network-id") {
				network.NetworkID = networkID
			}
			if cmd.Flags().Changed("chain-id") {
				network.ChainID = chainID
			}
			
			// Launch
			launcher := launch.New(network).WithDryRun(dryRun)
			if baseDir != "" {
				launcher = launcher.WithBaseDir(baseDir)
			}
			return launcher.Launch()
		},
	}
	
	// Flags
	cmd.Flags().StringVar(&preset, "preset", "", "Use a network preset (dev, local, mainnet, zoo, spc, hanzo)")
	cmd.Flags().StringVar(&config, "config", "", "Load network config from file")
	cmd.Flags().StringP("name", "n", "", "Network name")
	cmd.Flags().IntVar(&nodes, "nodes", 1, "Number of nodes")
	cmd.Flags().Uint64Var(&networkID, "network-id", 0, "Network ID")
	cmd.Flags().Uint64Var(&chainID, "chain-id", 0, "Chain ID")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be done")
	cmd.Flags().StringVar(&baseDir, "base-dir", "", "Base directory for network data")
	
	return cmd
}

// Backward compatibility commands

func getSetupNodeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "setup-node",
		Short: "Setup a single node (backward compatibility)",
		Long:  "Equivalent to: genesis launch --preset dev --dry-run",
		RunE: func(cmd *cobra.Command, args []string) error {
			network := launch.Presets["dev"]
			network.Nodes = 1
			launcher := launch.New(network).WithDryRun(true)
			return launcher.Launch()
		},
	}
}

func getSingleNodeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "single",
		Short: "Launch single node network (backward compatibility)",
		Long:  "Equivalent to: genesis launch --preset dev",
		RunE: func(cmd *cobra.Command, args []string) error {
			network := launch.Presets["dev"]
			launcher := launch.New(network)
			return launcher.Launch()
		},
	}
}