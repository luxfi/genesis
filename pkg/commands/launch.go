package commands

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/luxfi/genesis/pkg/core"
	"github.com/luxfi/genesis/pkg/launch"
	"github.com/spf13/cobra"
)

// LaunchOptions contains options for the launch command
type LaunchOptions struct {
	Preset    string
	Nodes     int
	NetworkID uint64
	ChainID   uint64
	DryRun    bool
	Config    string
	BaseDir   string
	Name      string
}

// NewLaunchCommand creates the launch command
func NewLaunchCommand() *cobra.Command {
	opts := &LaunchOptions{}
	
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
			return RunLaunch(opts)
		},
	}
	
	// Flags
	cmd.Flags().StringVar(&opts.Preset, "preset", "", "Use a network preset (dev, local, mainnet, zoo, spc, hanzo)")
	cmd.Flags().StringVar(&opts.Config, "config", "", "Load network config from file")
	cmd.Flags().StringVar(&opts.Name, "name", "", "Network name")
	cmd.Flags().IntVar(&opts.Nodes, "nodes", 1, "Number of nodes")
	cmd.Flags().Uint64Var(&opts.NetworkID, "network-id", 0, "Network ID")
	cmd.Flags().Uint64Var(&opts.ChainID, "chain-id", 0, "Chain ID")
	cmd.Flags().BoolVar(&opts.DryRun, "dry-run", false, "Show what would be done")
	cmd.Flags().StringVar(&opts.BaseDir, "base-dir", "", "Base directory for network data")

	return cmd
}

// RunLaunch executes the launch command
func RunLaunch(opts *LaunchOptions) error {
	var network core.Network
	
	// Load configuration
	if opts.Config != "" {
		// Load from file
		data, err := os.ReadFile(opts.Config)
		if err != nil {
			return fmt.Errorf("failed to read config: %w", err)
		}
		if err := json.Unmarshal(data, &network); err != nil {
			return fmt.Errorf("invalid config: %w", err)
		}
	} else if opts.Preset != "" {
		// Use preset
		var ok bool
		network, ok = launch.Presets[opts.Preset]
		if !ok {
			return fmt.Errorf("unknown preset: %s\nAvailable: dev, local, mainnet, zoo, spc, hanzo", opts.Preset)
		}
	} else {
		// Build from flags
		if opts.Name == "" {
			return fmt.Errorf("--name required without preset or config")
		}
		
		network = core.Network{
			Name:      opts.Name,
			NetworkID: opts.NetworkID,
			ChainID:   opts.ChainID,
			Nodes:     opts.Nodes,
			Genesis: core.GenesisConfig{
				Source: "fresh",
				Allocations: map[string]uint64{
					"P-lux1q9c6ltuxpsqz7ul8j0h0d0ha439qt70sr3x2z0": 100_000_000_000_000,
				},
			},
		}
	}
	
	// Apply overrides
	if opts.Nodes > 0 {
		network.Nodes = opts.Nodes
	}
	if opts.NetworkID > 0 {
		network.NetworkID = opts.NetworkID
	}
	if opts.ChainID > 0 {
		network.ChainID = opts.ChainID
	}
	
	// Launch
	launcher := launch.New(network).WithDryRun(opts.DryRun)
	if opts.BaseDir != "" {
		launcher = launcher.WithBaseDir(opts.BaseDir)
	}
	return launcher.Launch()
}

// Backward compatibility commands

// NewSetupNodeCommand creates the setup-node command (backward compatibility)
func NewSetupNodeCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "setup-node",
		Short: "Setup a single node (backward compatibility)",
		Long:  "Equivalent to: genesis launch --preset dev --dry-run",
		RunE: func(cmd *cobra.Command, args []string) error {
			opts := &LaunchOptions{
				Preset: "dev",
				Nodes:  1,
				DryRun: true,
			}
			return RunLaunch(opts)
		},
	}
}

// NewSingleNodeCommand creates the single command (backward compatibility)
func NewSingleNodeCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "single",
		Short: "Launch single node network (backward compatibility)",
		Long:  "Equivalent to: genesis launch --preset dev",
		RunE: func(cmd *cobra.Command, args []string) error {
			opts := &LaunchOptions{
				Preset: "dev",
			}
			return RunLaunch(opts)
		},
	}
}