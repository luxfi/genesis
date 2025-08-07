package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/luxfi/genesis/pkg/application"
	"github.com/spf13/cobra"
)

func newLaunchReplaySubCmd(app *application.Genesis) *cobra.Command {
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
		Use:   "replay",
		Short: "Launch luxd with genesis database replay",
		Long:  `Launch luxd with genesis database replay for mainnet.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// ... (logic from launch_replay.go) ...
			return nil
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
