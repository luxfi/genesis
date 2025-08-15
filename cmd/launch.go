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
		binaryPath      string
		dataDir         string
		networkID       uint32
		httpPort        uint16
		stakingPort     uint16
		singleNode      bool
		genesisPath     string
		chainDataPath   string
		logLevel        string
		publicIP        string
		daemon          bool
		noBootstrap     bool
		bootstrapIPs    string
		bootstrapIDs    string
		stakingKey      string
		stakingCert     string
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
				NoBootstrap:   noBootstrap,
				BootstrapIPs:  bootstrapIPs,
				BootstrapIDs:  bootstrapIDs,
				StakingKey:    stakingKey,
				StakingCert:   stakingCert,
			}

			// Handle single node setup
			if singleNode {
				// Force no bootstrap for single node
				config.NoBootstrap = true
				
				// Generate genesis if not provided
				if genesisPath == "" {
					fmt.Println("Generating minimal genesis for single validator node...")
					genPath, err := launcher.GenerateSingleValidatorGenesis(networkID)
					if err != nil {
						return fmt.Errorf("failed to generate genesis: %w", err)
					}
					config.GenesisPath = genPath
					defer os.Remove(genPath) // Clean up temp file
					fmt.Printf("Genesis created: %s\n", genPath)
				}
				
				// Generate staking keys if not provided
				if stakingKey == "" || stakingCert == "" {
					fmt.Println("Generating staking keys for single validator...")
					keyPath, certPath, err := launcher.GenerateStakingKeys(dataDir)
					if err != nil {
						return fmt.Errorf("failed to generate staking keys: %w", err)
					}
					config.StakingKey = keyPath
					config.StakingCert = certPath
					fmt.Printf("Staking keys generated:\n  Key: %s\n  Cert: %s\n", keyPath, certPath)
				}
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
	cmd.Flags().BoolVar(&singleNode, "single-node", false, "Run as single validator (no bootstrap, k=1)")
	cmd.Flags().StringVar(&genesisPath, "genesis", "", "Path to genesis file")
	cmd.Flags().StringVar(&chainDataPath, "chain-data", "", "Path to C-chain data for replay")
	cmd.Flags().StringVar(&logLevel, "log-level", "info", "Log level")
	cmd.Flags().StringVar(&publicIP, "public-ip", "127.0.0.1", "Public IP address")
	cmd.Flags().BoolVar(&daemon, "daemon", false, "Run in daemon mode")
	cmd.Flags().BoolVar(&noBootstrap, "no-bootstrap", false, "Don't connect to bootstrap nodes")
	cmd.Flags().StringVar(&bootstrapIPs, "bootstrap-ips", "", "Comma-separated list of bootstrap node IPs")
	cmd.Flags().StringVar(&bootstrapIDs, "bootstrap-ids", "", "Comma-separated list of bootstrap node IDs")
	cmd.Flags().StringVar(&stakingKey, "staking-key", "", "Path to staking key file")
	cmd.Flags().StringVar(&stakingCert, "staking-cert", "", "Path to staking certificate file")

	// Add subcommands from other files
	// cmd.AddCommand(newLaunchBFTCmd(app)) // TODO: implement
	// cmd.AddCommand(newLaunchReplaySubCmd(app)) // Moved to separate command

	return cmd
}