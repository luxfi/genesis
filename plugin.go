package main

import (
	"github.com/spf13/cobra"
)

// Plugin exports the genesis plugin for lux-cli
type Plugin struct {
	Name        string
	Version     string
	Description string
	RootCmd     *cobra.Command
}

// GetPlugin returns the genesis plugin for lux-cli integration
func GetPlugin() *Plugin {
	return &Plugin{
		Name:        "genesis",
		Version:     Version,
		Description: "Genesis configuration and management for Lux ecosystem",
		RootCmd:     GetRootCommand(),
	}
}

// GetRootCommand returns the root command for lux-cli integration
func GetRootCommand() *cobra.Command {
	// Create a new root command for plugin mode
	pluginCmd := &cobra.Command{
		Use:   "genesis",
		Short: "Genesis configuration and management",
		Long: `Manage genesis configurations for all chains in the Lux ecosystem.

Supports:
- Lux Mainnet/Testnet/Local
- Zoo Mainnet/Testnet (L2)
- SPC Mainnet/Testnet
- Hanzo Mainnet/Testnet
- Quantum Chain

Features:
- Generate genesis configurations
- Manage consensus parameters
- Process and import ancient chain data
- Full pipeline from state repo to node import`,
	}

	// Add all subcommands
	pluginCmd.AddCommand(
		generateCmd,
		launchCmd,
		validateCmd,
		getConsensusCmd(),
		getPipelineCmd(),
		getStateCmd(),
		versionCmd,
	)

	return pluginCmd
}

// getConsensusCmd returns the consensus subcommand
func getConsensusCmd() *cobra.Command {
	consensusCmd := &cobra.Command{
		Use:   "consensus",
		Short: "Manage consensus parameters",
		Long:  "View and manage consensus parameters for all supported chains",
	}

	// List consensus params
	listConsensusCmd := &cobra.Command{
		Use:   "list",
		Short: "List consensus parameters for all chains",
		Run:   runListConsensus,
	}

	// Show specific chain consensus
	showConsensusCmd := &cobra.Command{
		Use:   "show [network]",
		Short: "Show consensus parameters for a specific network",
		Args:  cobra.ExactArgs(1),
		Run:   runShowConsensus,
	}

	// Update consensus params
	updateConsensusCmd := &cobra.Command{
		Use:   "update [network]",
		Short: "Update consensus parameters for a network",
		Args:  cobra.ExactArgs(1),
		Run:   runUpdateConsensus,
	}

	consensusCmd.AddCommand(listConsensusCmd, showConsensusCmd, updateConsensusCmd)
	return consensusCmd
}

// getPipelineCmd returns the pipeline subcommand
func getPipelineCmd() *cobra.Command {
	pipelineCmd := &cobra.Command{
		Use:   "pipeline",
		Short: "Run full genesis pipeline",
		Long:  "Execute the complete pipeline from cloning state to importing into node",
	}

	// Full pipeline
	fullCmd := &cobra.Command{
		Use:   "full [network]",
		Short: "Run full pipeline for a network",
		Args:  cobra.ExactArgs(1),
		Run:   runFullPipeline,
	}

	// Individual steps
	cloneCmd := &cobra.Command{
		Use:   "clone",
		Short: "Clone state repository",
		Run:   runCloneState,
	}

	processCmd := &cobra.Command{
		Use:   "process [network]",
		Short: "Process chaindata into ancient store format",
		Args:  cobra.ExactArgs(1),
		Run:   runProcessChaindata,
	}

	importCmd := &cobra.Command{
		Use:   "import [network]",
		Short: "Import processed data into node",
		Args:  cobra.ExactArgs(1),
		Run:   runImportToNode,
	}

	pipelineCmd.AddCommand(fullCmd, cloneCmd, processCmd, importCmd)
	return pipelineCmd
}

// getStateCmd returns the state management subcommand
func getStateCmd() *cobra.Command {
	stateCmd := &cobra.Command{
		Use:   "state",
		Short: "Manage state repository",
		Long:  "Clone, update, and manage historic chaindata from state repository",
	}

	// Clone state
	cloneStateCmd := &cobra.Command{
		Use:   "clone",
		Short: "Clone state repository",
		Run: func(cmd *cobra.Command, args []string) {
			runCloneState(cmd, args)
		},
	}

	// Update state
	updateStateCmd := &cobra.Command{
		Use:   "update",
		Short: "Update cloned state repository",
		Run: func(cmd *cobra.Command, args []string) {
			runUpdateState(cmd, args)
		},
	}

	// Clean state
	cleanStateCmd := &cobra.Command{
		Use:   "clean",
		Short: "Remove cloned state data",
		Run: func(cmd *cobra.Command, args []string) {
			runCleanState(cmd, args)
		},
	}

	stateCmd.AddCommand(cloneStateCmd, updateStateCmd, cleanStateCmd)
	return stateCmd
}

// Export for lux-cli plugin system
var (
	// PluginName is the name of this plugin
	PluginName = "genesis"
	
	// PluginVersion is the version of this plugin
	PluginVersion = "1.0.0"
	
	// PluginCommands exports the commands for lux-cli
	PluginCommands = GetRootCommand()
)