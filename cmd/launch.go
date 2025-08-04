package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/luxfi/genesis/pkg/application"
	"github.com/luxfi/genesis/pkg/staking"
	"github.com/spf13/cobra"
)

// NewLaunchCmd creates the launch command
func NewLaunchCmd(app *application.Genesis) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "launch",
		Short: "Launch networks",
		Long:  "Launch various network configurations",
	}

	// Subcommands
	cmd.AddCommand(newLaunchSingleCmd(app))
	cmd.AddCommand(newLaunchReplayCmd(app))
	cmd.AddCommand(&cobra.Command{
		Use:   "local",
		Short: "Launch local development network",
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.Println("Local network launch not yet implemented")
			return nil
		},
	})

	return cmd
}

// newLaunchSingleCmd creates the single validator launch command
func newLaunchSingleCmd(app *application.Genesis) *cobra.Command {
	var (
		networkID uint32
		dataDir   string
		chaindata string
		mnemonic  string
		luxdPath  string
	)

	cmd := &cobra.Command{
		Use:   "single",
		Short: "Launch single validator network",
		Long:  "Launch a single validator network with k=1 consensus",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Get script directory
			scriptDir, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("failed to get working directory: %w", err)
			}

			// Resolve paths relative to script directory
			if !filepath.IsAbs(dataDir) {
				dataDir = filepath.Join(scriptDir, dataDir)
			}
			if !filepath.IsAbs(chaindata) {
				chaindata = filepath.Join(scriptDir, chaindata)
			}
			if !filepath.IsAbs(luxdPath) {
				luxdPath = filepath.Join(scriptDir, "..", luxdPath)
			}

			// Clean up old data
			os.RemoveAll(dataDir)
			os.MkdirAll(filepath.Join(dataDir, "db", "c-chain"), 0755)

			// Generate staking keys from mnemonic
			fmt.Println("Generating staking keys from mnemonic...")
			stakingDir := filepath.Join(dataDir, "staking")

			// Generate staking keys with a deterministic node ID
			nodeID := "NodeID-7Xhw2mDxuDS44j42TCB6U5579esbSt3Lg"
			if err := staking.GenerateStakingKeys(stakingDir, nodeID); err != nil {
				return fmt.Errorf("failed to generate staking keys: %w", err)
			}

			// Create genesis config
			genesisPath := filepath.Join(dataDir, "genesis.json")
			// TODO: Create proper genesis generation
			// For now, create a minimal genesis file
			genesisContent := fmt.Sprintf(`{
  "networkID": %d,
  "allocations": [
    {
      "ethAddr": "0x9011e888251ab053b7bd1cdb598db4f9ded94714",
      "luxAddr": "P-lux1qsrd262r5w9dswv2wwzj0un79ncpwvdgkpqzqu",
      "initialAmount": 1000000000000000000,
      "unlockSchedule": [
        {
          "amount": 1000000000000000000,
          "locktime": 0
        }
      ]
    }
  ],
  "startTime": 1649980800,
  "initialStakeDuration": 31536000,
  "initialStakeDurationOffset": 5400,
  "initialStakedFunds": [
    "P-lux1qsrd262r5w9dswv2wwzj0un79ncpwvdgkpqzqu"
  ],
  "initialStakers": [
    {
      "nodeID": "%s",
      "rewardAddress": "P-lux1qsrd262r5w9dswv2wwzj0un79ncpwvdgkpqzqu",
      "delegationFee": 20000,
      "signer": {
        "publicKey": "0x900c9b119b5c82d781d4b49be78c3fc7ae65f2b435b7ed9e3a8b9a03e475edff86d8a64827fec8db23a6f236afbf127d",
        "proofOfPossession": "0x9239f365a639849730078382d2f060c4d98cb02ad24fe8aad573ac10d317c6be004846ac11080569b12dbb2f34044dcf17c8d1c4bb3494fc62929bcb87e476a19bb51cdfe7882c899762100180e0122c64ca962816f6cbf67f852162295c19ed"
      }
    }
  ],
  "cChainGenesis": "{\"config\":{\"chainId\":%d,\"homesteadBlock\":0,\"eip150Block\":0,\"eip155Block\":0,\"eip158Block\":0,\"byzantiumBlock\":0,\"constantinopleBlock\":0,\"petersburgBlock\":0,\"istanbulBlock\":0,\"muirGlacierBlock\":0,\"apricotPhase1BlockTimestamp\":0,\"apricotPhase2BlockTimestamp\":0,\"apricotPhase3BlockTimestamp\":0,\"apricotPhase4BlockTimestamp\":0,\"apricotPhase5BlockTimestamp\":0},\"nonce\":\"0x0\",\"timestamp\":\"0x0\",\"extraData\":\"0x00\",\"gasLimit\":\"0x5f5e100\",\"difficulty\":\"0x0\",\"mixHash\":\"0x0000000000000000000000000000000000000000000000000000000000000000\",\"coinbase\":\"0x0000000000000000000000000000000000000000\",\"alloc\":{\"0x9011e888251ab053b7bd1cdb598db4f9ded94714\":{\"balance\":\"0x21e19e0c9bab2400000\"}},\"number\":\"0x0\",\"gasUsed\":\"0x0\",\"parentHash\":\"0x0000000000000000000000000000000000000000000000000000000000000000\"}",
  "message": "lux mainnet %d"
}`, networkID, nodeID, networkID, networkID)

			if err := os.WriteFile(genesisPath, []byte(genesisContent), 0644); err != nil {
				return fmt.Errorf("failed to write genesis: %w", err)
			}

			fmt.Println("\nConfiguration Summary:")
			fmt.Printf("Network ID: %d\n", networkID)
			fmt.Printf("Data Dir: %s\n", dataDir)
			fmt.Printf("Node ID: %s\n", nodeID)
			fmt.Printf("RPC: http://localhost:9630/ext/bc/C/rpc\n")
			fmt.Println("\nStarting luxd with single validator...")

			// Launch luxd
			luxdCmd := exec.Command(luxdPath,
				fmt.Sprintf("--network-id=%d", networkID),
				fmt.Sprintf("--data-dir=%s", dataDir),
				fmt.Sprintf("--genesis-file=%s", genesisPath),
				"--db-type=pebbledb",
				"--http-port=9630",
				"--http-host=0.0.0.0",
				"--http-allowed-origins=*",
				"--consensus-sample-size=1",
				"--consensus-quorum-size=1",
				"--skip-bootstrap",
				"--log-level=debug",
				"--api-admin-enabled=true",
				"--api-metrics-enabled=true",
				"--index-enabled=true",
				fmt.Sprintf("--staking-tls-cert-file=%s", filepath.Join(stakingDir, "staker.crt")),
				fmt.Sprintf("--staking-tls-key-file=%s", filepath.Join(stakingDir, "staker.key")),
				fmt.Sprintf("--staking-signer-key-file=%s", filepath.Join(stakingDir, "signer.key")),
			)

			luxdCmd.Stdout = os.Stdout
			luxdCmd.Stderr = os.Stderr

			if err := luxdCmd.Start(); err != nil {
				return fmt.Errorf("failed to start luxd: %w", err)
			}

			// Handle signals
			sigCh := make(chan os.Signal, 1)
			signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

			go func() {
				<-sigCh
				fmt.Println("\nShutting down...")
				luxdCmd.Process.Signal(syscall.SIGTERM)
			}()

			return luxdCmd.Wait()
		},
	}

	cmd.Flags().Uint32Var(&networkID, "network-id", 96369, "Network ID")
	cmd.Flags().StringVar(&dataDir, "data-dir", "runs/lux-mainnet-single", "Data directory")
	cmd.Flags().StringVar(&chaindata, "chaindata", "state/chaindata", "Path to chaindata for replay")
	cmd.Flags().StringVar(&mnemonic, "mnemonic", "light light light light light light light light light light light world", "Mnemonic for key generation")
	cmd.Flags().StringVar(&luxdPath, "luxd", "node/build/luxd", "Path to luxd binary")

	return cmd
}

// newLaunchReplayCmd creates the replay launch command
func newLaunchReplayCmd(app *application.Genesis) *cobra.Command {
	var (
		networkID uint32
		dataDir   string
		genesisDB string
		dbType    string
	)

	cmd := &cobra.Command{
		Use:   "replay",
		Short: "Launch with genesis database replay",
		Long:  "Launch a node that replays from an existing genesis database",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Auto-detect database type if not specified
			if dbType == "" {
				// Check for MANIFEST file (PebbleDB indicator)
				if _, err := os.Stat(filepath.Join(genesisDB, "MANIFEST")); err == nil {
					dbType = "pebbledb"
					fmt.Println("Auto-detected database type: PebbleDB")
				} else if _, err := os.Stat(filepath.Join(genesisDB, "CURRENT")); err == nil {
					dbType = "leveldb"
					fmt.Println("Auto-detected database type: LevelDB")
				} else {
					return fmt.Errorf("could not auto-detect database type")
				}
			}

			fmt.Printf("Using genesis database: %s (type: %s)\n", genesisDB, dbType)
			fmt.Println("Genesis database replay functionality would be implemented here")
			fmt.Println("Note: This requires the --genesis-db flag to be implemented in luxd")

			return nil
		},
	}

	cmd.Flags().Uint32Var(&networkID, "network-id", 96369, "Network ID")
	cmd.Flags().StringVar(&dataDir, "data-dir", "runs/lux-mainnet-replay", "Data directory")
	cmd.Flags().StringVar(&genesisDB, "genesis-db", "state/chaindata", "Path to genesis database")
	cmd.Flags().StringVar(&dbType, "db-type", "", "Database type (auto-detect if empty)")

	return cmd
}
