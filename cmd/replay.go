package cmd

import (
	"github.com/luxfi/genesis/pkg/application"
	"github.com/luxfi/genesis/pkg/replay"
	"github.com/spf13/cobra"
)

// NewReplayCmd creates the replay command
func NewReplayCmd(app *application.Genesis) *cobra.Command {
	var opts replay.Options

	cmd := &cobra.Command{
		Use:   "replay [source-db]",
		Short: "Replay blockchain blocks",
		Long:  "Read finalized blocks from SubnetEVM database and replay them into C-Chain with proper state setup",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			replayer := replay.New(app)
			return replayer.ReplayBlocks(args[0], opts)
		},
	}

	cmd.Flags().StringVar(&opts.RPC, "rpc", "http://localhost:9630/ext/bc/C/rpc", "RPC endpoint")
	cmd.Flags().Uint64Var(&opts.Start, "start", 0, "Start block (0 = genesis)")
	cmd.Flags().Uint64Var(&opts.End, "end", 0, "End block (0 = all)")
	cmd.Flags().BoolVar(&opts.DirectDB, "direct-db", false, "Write directly to database instead of RPC")
	cmd.Flags().StringVar(&opts.Output, "output", "", "Output database path (for direct-db mode)")

	return cmd
}

// NewSubnetBlockReplayCmd creates the subnet-block-replay command (alias for replay with direct-db)
func NewSubnetBlockReplayCmd(app *application.Genesis) *cobra.Command {
	var output string

	cmd := &cobra.Command{
		Use:   "subnet-block-replay [source-db]",
		Short: "Replay subnet blocks directly to database",
		Long:  "Replay blocks from SubnetEVM database directly to output database (equivalent to replay --direct-db)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts := replay.Options{
				DirectDB: true,
				Output:   output,
			}
			replayer := replay.New(app)
			return replayer.ReplayBlocks(args[0], opts)
		},
	}

	cmd.Flags().StringVar(&output, "output", "", "Output database path (required)")
	_ = cmd.MarkFlagRequired("output")

	return cmd
}
