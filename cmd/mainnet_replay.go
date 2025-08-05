package cmd

import (
	"github.com/luxfi/genesis/pkg/application"
	"github.com/luxfi/genesis/pkg/mainnet"
	"github.com/spf13/cobra"
)

// NewMainnetReplayCmd creates the mainnet-replay command
func NewMainnetReplayCmd(app *application.Genesis) *cobra.Command {
	var opts mainnet.ReplayOptions

	cmd := &cobra.Command{
		Use:   "mainnet-replay",
		Short: "Automated mainnet replay with proper BLS keys and genesis",
		Long: `Automates the entire mainnet replay process:
- Generates staking keys with proper BLS validation
- Creates genesis configuration matching the generated NodeID
- Configures database replay from existing blockchain data
- Launches luxd with all proper settings for mainnet validation`,
		RunE: func(cmd *cobra.Command, args []string) error {
			runner := mainnet.NewReplayRunner(app)
			return runner.Run(opts)
		},
	}

	// Key generation options
	cmd.Flags().StringVar(&opts.KeysDir, "keys-dir", "", "Directory for staking keys (generates new if not specified)")
	cmd.Flags().BoolVar(&opts.GenerateKeys, "generate-keys", true, "Generate new staking keys if keys-dir doesn't exist")
	
	// Genesis options
	cmd.Flags().StringVar(&opts.GenesisDB, "genesis-db", "", "Path to genesis database for replay")
	cmd.Flags().StringVar(&opts.GenesisDBType, "genesis-db-type", "pebbledb", "Type of genesis database (pebbledb, leveldb)")
	cmd.Flags().StringVar(&opts.NetworkID, "network-id", "mainnet", "Network ID (mainnet=96369, testnet=96368)")
	
	// Node configuration
	cmd.Flags().StringVar(&opts.DataDir, "data-dir", "", "Data directory for luxd (temp if not specified)")
	cmd.Flags().StringVar(&opts.DBType, "db-type", "badgerdb", "Default database type for new chains")
	cmd.Flags().StringVar(&opts.CChainDBType, "c-chain-db-type", "badgerdb", "C-Chain database type")
	cmd.Flags().IntVar(&opts.HTTPPort, "http-port", 9630, "HTTP API port")
	cmd.Flags().IntVar(&opts.StakingPort, "staking-port", 9631, "Staking port")
	
	// Consensus configuration
	cmd.Flags().IntVar(&opts.K, "k", 1, "Snow sample size (k)")
	cmd.Flags().IntVar(&opts.SnowQuorumSize, "snow-quorum-size", 1, "Snow quorum size")
	cmd.Flags().IntVar(&opts.AlphaPreference, "alpha-preference", 1, "Alpha preference for virtuous commit")
	cmd.Flags().IntVar(&opts.AlphaConfidence, "alpha-confidence", 1, "Alpha confidence threshold")
	cmd.Flags().IntVar(&opts.Beta, "beta", 1, "Beta rogue commit threshold")
	
	// Execution options
	cmd.Flags().BoolVar(&opts.SkipLaunch, "skip-launch", false, "Generate config but don't launch node")
	cmd.Flags().BoolVar(&opts.SingleNode, "single-node", true, "Configure for single node operation")
	cmd.Flags().IntVar(&opts.NumNodes, "num-nodes", 1, "Number of nodes to run (1 or 2)")
	cmd.Flags().IntVar(&opts.BasePort, "base-port", 9650, "Base port for multi-node setup")
	cmd.Flags().StringVar(&opts.LogLevel, "log-level", "info", "Log level for luxd")

	return cmd
}