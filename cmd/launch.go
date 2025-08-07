package cmd

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/luxfi/genesis/pkg/application"
	"github.com/luxfi/genesis/pkg/launcher"
	"github.com/spf13/cobra"
)

// NewLaunchCmd creates the launch command, now acting as the primary entry point for launching nodes.
func NewLaunchCmd(app *application.Genesis) *cobra.Command {
	var (
		binaryPath    string
		dataDir       string
		networkID     uint32
		httpPort      uint16
		stakingPort   uint16
		singleNode    bool
		genesisPath   string
		chainDataPath string
		logLevel      string
		publicIP      string
		daemon        bool
	)

	cmd := &cobra.Command{
		Use:   "launch",
		Short: "Launch a Lux node with various configurations",
		Long:  `The primary command for launching a Lux node. Use subcommands for specific scenarios like BFT or replay.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Default data directory
			if dataDir == "" {
				homeDir, _ := os.UserHomeDir()
				dataDir = filepath.Join(homeDir, ".luxd-launch")
			}

			// Create launcher config
			config := launcher.Config{
				BinaryPath:    binaryPath,
				DataDir:       dataDir,
				NetworkID:     networkID,
				HTTPPort:      httpPort,
				StakingPort:   stakingPort,
				SingleNode:    singleNode,
				GenesisPath:   genesisPath,
				ChainDataPath: chainDataPath,
				LogLevel:      logLevel,
				PublicIP:      publicIP,
			}

			// Create genesis if not provided and single node
			if singleNode && genesisPath == "" {
				fmt.Println("Generating minimal genesis for single node...")
				genPath, err := launcher.GenerateMinimalGenesis(networkID, "0x9011e888251ab053b7bd1cdb598db4f9ded94714")
				if err != nil {
					return fmt.Errorf("failed to generate genesis: %w", err)
				}
				config.GenesisPath = genPath
				defer os.Remove(genPath) // Clean up temp file
				fmt.Printf("Genesis created: %s\n", genPath)
			}

			// Create launcher
			nl := launcher.New(config)

			// Start the node
			fmt.Println("Starting Lux node...")
			if err := nl.Start(); err != nil {
				return fmt.Errorf("failed to start node: %w", err)
			}

			fmt.Println("\nNode started successfully!")
			fmt.Printf("RPC endpoint: http://localhost:%d/ext/bc/C/rpc\n", httpPort)

			if daemon {
				fmt.Println("\nNode is running in daemon mode.")
				return nil
			}

			// Handle signals for graceful shutdown
			sigCh := make(chan os.Signal, 1)
			signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

			fmt.Println("\nPress Ctrl+C to stop the node...")

			go func() {
				<-sigCh
				fmt.Println("\nShutting down node...")
				if err := nl.Stop(); err != nil {
					fmt.Printf("Error stopping node: %v\n", err)
				}
			}()

			return nl.Wait()
		},
	}

	// Flags
	cmd.Flags().StringVar(&binaryPath, "binary", "/home/z/work/lux/node/build/luxd", "Path to luxd binary")
	cmd.Flags().StringVar(&dataDir, "data-dir", "", "Data directory (default: ~/.luxd-launch)")
	cmd.Flags().Uint32Var(&networkID, "network-id", 96369, "Network ID")
	cmd.Flags().Uint16Var(&httpPort, "http-port", 9630, "HTTP API port")
	cmd.Flags().Uint16Var(&stakingPort, "staking-port", 9631, "Staking port")
	cmd.Flags().BoolVar(&singleNode, "single-node", false, "Run in single node mode (k=1)")
	cmd.Flags().StringVar(&genesisPath, "genesis", "", "Path to genesis file")
	cmd.Flags().StringVar(&chainDataPath, "chain-data", "", "Path to C-chain data for replay")
	cmd.Flags().StringVar(&logLevel, "log-level", "info", "Log level")
	cmd.Flags().StringVar(&publicIP, "public-ip", "127.0.0.1", "Public IP address")
	cmd.Flags().BoolVar(&daemon, "daemon", false, "Run in daemon mode")

	// Add subcommands from other files
	cmd.AddCommand(newLaunchBFTCmd(app))
	cmd.AddCommand(newLaunchReplaySubCmd(app)) // Renamed to avoid conflict

	return cmd
}