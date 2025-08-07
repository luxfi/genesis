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
		Short: "Launch luxd with historic chaindata replay and full mainnet consensus",
		Long: `Launches luxd with historic chaindata replay for mainnet:
- Uses existing chaindata from ~/work/lux/state/chaindata/lux-mainnet-96369
- Generates proper BLS staking keys and validator configuration
- Creates mainnet genesis with full 21-node consensus
- Replays historic blockchain data into C-chain genesis
- Launches luxd with proper mainnet settings`,
		RunE: func(cmd *cobra.Command, args []string) error {
			runner := mainnet.NewReplayRunner(app)
			return runner.Run(opts)
		},
	}

	// Key generation options
	cmd.Flags().StringVar(&opts.KeysDir, "keys-dir", "", "Directory for staking keys (generates new if not specified)")
	cmd.Flags().BoolVar(&opts.GenerateKeys, "generate-keys", true, "Generate new staking keys if keys-dir doesn't exist")

	// Genesis options - default to our existing chaindata
	cmd.Flags().StringVar(&opts.GenesisDB, "genesis-db", "/Users/z/work/lux/state/chaindata/lux-mainnet-96369/db/pebbledb", "Path to genesis database for replay")
	cmd.Flags().StringVar(&opts.GenesisDBType, "genesis-db-type", "pebbledb", "Type of genesis database (pebbledb, leveldb, badgerdb)")
	cmd.Flags().StringVar(&opts.NetworkID, "network-id", "mainnet", "Network ID (mainnet=96369, testnet=96368)")

	// Node configuration
	cmd.Flags().StringVar(&opts.DataDir, "data-dir", "", "Data directory for luxd (temp if not specified)")
	cmd.Flags().StringVar(&opts.DBType, "db-type", "badgerdb", "Default database type for new chains")
	cmd.Flags().StringVar(&opts.CChainDBType, "c-chain-db-type", "badgerdb", "C-Chain database type")
	cmd.Flags().IntVar(&opts.HTTPPort, "http-port", 9630, "HTTP API port")
	cmd.Flags().IntVar(&opts.StakingPort, "staking-port", 9631, "Staking port")

	// Consensus configuration - Mainnet defaults (21 nodes)
	cmd.Flags().IntVar(&opts.K, "k", 21, "Snow sample size (k) - mainnet=21, testnet=11")
	cmd.Flags().IntVar(&opts.SnowQuorumSize, "snow-quorum-size", 13, "Snow quorum size - mainnet=13, testnet=7")
	cmd.Flags().IntVar(&opts.AlphaPreference, "alpha-preference", 13, "Alpha preference for virtuous commit")
	cmd.Flags().IntVar(&opts.AlphaConfidence, "alpha-confidence", 18, "Alpha confidence threshold")
	cmd.Flags().IntVar(&opts.Beta, "beta", 8, "Beta rogue commit threshold")

	// Execution options
	cmd.Flags().BoolVar(&opts.SkipLaunch, "skip-launch", false, "Generate config but don't launch node")
	cmd.Flags().BoolVar(&opts.SingleNode, "single-node", false, "Configure for single node operation (testing only)")
	cmd.Flags().IntVar(&opts.NumNodes, "num-nodes", 1, "Number of nodes to run locally")
	cmd.Flags().IntVar(&opts.BasePort, "base-port", 9650, "Base port for multi-node setup")
	cmd.Flags().StringVar(&opts.LogLevel, "log-level", "info", "Log level for luxd")
	cmd.Flags().BoolVar(&opts.EnableStaking, "enable-staking", true, "Enable staking and validation")

	return cmd
}