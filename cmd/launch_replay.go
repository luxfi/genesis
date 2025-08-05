package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/luxfi/genesis/pkg/application"
	"github.com/spf13/cobra"
)

// NewLaunchReplayCmd creates the launch-replay command
func NewLaunchReplayCmd(app *application.Genesis) *cobra.Command {
	var (
		networkID      string
		dataDir        string
		genesisDB      string
		dbType         string
		cChainDBType   string
		httpPort       int
		stakingPort    int
		logLevel       string
		singleNode     bool
		skipValidation bool
	)

	cmd := &cobra.Command{
		Use:   "launch-replay",
		Short: "Launch luxd with genesis database replay",
		Long: `Launch luxd with genesis database replay for mainnet.
This command:
1. Uses existing genesis database from state/chaindata
2. Configures proper consensus parameters
3. Replays historical blocks
4. Builds new chain state with BadgerDB`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// All paths relative to genesis directory
			baseDir := "."
			if app != nil && app.BaseDir != "" {
				baseDir = app.BaseDir
			}
			
			// Determine network paths
			var genesisDBPath string
			switch networkID {
			case "96369", "mainnet", "lux-mainnet":
				genesisDBPath = filepath.Join("state", "chaindata", "lux-mainnet-96369")
			case "200200", "zoo-mainnet":
				genesisDBPath = filepath.Join("state", "chaindata", "zoo-mainnet-200200")
			case "36911", "spc-mainnet":
				genesisDBPath = filepath.Join("state", "chaindata", "spc-mainnet-36911")
			default:
				if genesisDB != "" {
					genesisDBPath = genesisDB
				} else {
					return fmt.Errorf("unknown network ID: %s", networkID)
				}
			}

			// Check if genesis database exists
			fullGenesisPath := filepath.Join(baseDir, genesisDBPath)
			if _, err := os.Stat(fullGenesisPath); err != nil {
				return fmt.Errorf("genesis database not found at %s", fullGenesisPath)
			}

			// Create data directory
			runDir := filepath.Join("runs", dataDir)
			if err := os.MkdirAll(runDir, 0755); err != nil {
				return fmt.Errorf("failed to create run directory: %w", err)
			}

			// Create chain configurations
			chainConfigDir := filepath.Join(runDir, "configs", "chains", "C")
			if err := os.MkdirAll(chainConfigDir, 0755); err != nil {
				return fmt.Errorf("failed to create chain config dir: %w", err)
			}

			// Write C-Chain config
			cChainConfig := fmt.Sprintf(`{
  "db-type": "%s",
  "log-level": "%s",
  "state-sync-enabled": false,
  "offline-pruning-enabled": false,
  "allow-unprotected-txs": true
}`, cChainDBType, logLevel)

			configPath := filepath.Join(chainConfigDir, "config.json")
			if err := os.WriteFile(configPath, []byte(cChainConfig), 0644); err != nil {
				return fmt.Errorf("failed to write C-chain config: %w", err)
			}

			// Build luxd command
			luxdPath := filepath.Join("..", "node", "build", "luxd")
			
			// Check if luxd exists
			if _, err := os.Stat(luxdPath); err != nil {
				// Try to build it
				fmt.Println("Building luxd...")
				buildCmd := exec.Command("make", "build")
				buildCmd.Dir = filepath.Join("..", "node")
				buildCmd.Stdout = os.Stdout
				buildCmd.Stderr = os.Stderr
				if err := buildCmd.Run(); err != nil {
					return fmt.Errorf("failed to build luxd: %w", err)
				}
			}

			// Prepare luxd arguments
			luxdArgs := []string{
				fmt.Sprintf("--network-id=%s", networkID),
				fmt.Sprintf("--data-dir=%s", filepath.Join(runDir, "data")),
				fmt.Sprintf("--genesis-db=%s", genesisDBPath),
				"--genesis-db-type=pebbledb",
				fmt.Sprintf("--db-type=%s", dbType),
				fmt.Sprintf("--c-chain-db-type=%s", cChainDBType),
				fmt.Sprintf("--chain-config-dir=%s", filepath.Join(runDir, "configs", "chains")),
				"--http-host=0.0.0.0",
				fmt.Sprintf("--http-port=%d", httpPort),
				fmt.Sprintf("--staking-port=%d", stakingPort),
				fmt.Sprintf("--log-level=%s", logLevel),
				"--api-admin-enabled=true",
			}

			// Add consensus parameters
			if singleNode {
				luxdArgs = append(luxdArgs,
					"--sybil-protection-enabled=false",
					"--consensus-sample-size=1",
					"--consensus-quorum-size=1",
					"--consensus-commit-threshold=1",
					"--consensus-concurrent-repolls=1",
					"--consensus-optimal-processing=1",
					"--consensus-max-processing=1",
					"--consensus-max-time-processing=2s",
				)
			}

			// Print configuration
			fmt.Println("=== Lux Genesis Database Replay ===")
			fmt.Printf("Network ID: %s\n", networkID)
			fmt.Printf("Genesis DB: %s\n", genesisDBPath)
			fmt.Printf("Data Dir: %s\n", runDir)
			fmt.Printf("DB Type: %s (runtime), pebbledb (replay)\n", dbType)
			fmt.Printf("C-Chain DB: %s\n", cChainDBType)
			fmt.Printf("HTTP Port: %d\n", httpPort)
			fmt.Printf("Staking Port: %d\n", stakingPort)
			if singleNode {
				fmt.Println("Consensus: Single-node mode")
			}
			fmt.Println("")
			fmt.Println("Starting luxd...")

			// Run luxd
			luxdCmd := exec.Command(luxdPath, luxdArgs...)
			luxdCmd.Stdout = os.Stdout
			luxdCmd.Stderr = os.Stderr
			luxdCmd.Stdin = os.Stdin

			return luxdCmd.Run()
		},
	}

	// Flags
	cmd.Flags().StringVar(&networkID, "network-id", "96369", "Network ID (96369=mainnet, 200200=zoo, 36911=spc)")
	cmd.Flags().StringVar(&dataDir, "data-dir", "replay", "Data directory name under runs/")
	cmd.Flags().StringVar(&genesisDB, "genesis-db", "", "Override genesis database path (relative to genesis dir)")
	cmd.Flags().StringVar(&dbType, "db-type", "badgerdb", "Database type for runtime")
	cmd.Flags().StringVar(&cChainDBType, "c-chain-db-type", "badgerdb", "C-Chain database type")
	cmd.Flags().IntVar(&httpPort, "http-port", 9630, "HTTP API port")
	cmd.Flags().IntVar(&stakingPort, "staking-port", 9631, "Staking port")
	cmd.Flags().StringVar(&logLevel, "log-level", "info", "Log level")
	cmd.Flags().BoolVar(&singleNode, "single-node", false, "Run in single-node consensus mode")
	cmd.Flags().BoolVar(&skipValidation, "skip-validation", false, "Skip BLS validation")

	return cmd
}