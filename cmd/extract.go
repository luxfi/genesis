package cmd

import (
	"github.com/luxfi/genesis/pkg/application"
	"github.com/luxfi/genesis/pkg/extract"
	"github.com/spf13/cobra"
)

// NewExtractCmd creates the extract command with subcommands
func NewExtractCmd(app *application.Genesis) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "extract",
		Short: "Extract blockchain data",
		Long:  "Extract genesis, state, or blockchain data from existing chain data",
	}

	cmd.AddCommand(newExtractBlockchainCmd(app))
	cmd.AddCommand(newExtractGenesisCmd(app))
	cmd.AddCommand(newExtractStateCmd(app))

	return cmd
}

func newExtractBlockchainCmd(app *application.Genesis) *cobra.Command {
	var opts extract.Options

	cmd := &cobra.Command{
		Use:   "blockchain [db-path] [output-path]",
		Short: "Extract blockchain data in various formats",
		Long: `Extract blockchain data from SubnetEVM format to different output formats.
Use --format=bytes for raw chaindata suitable for replay.
Use --format=json for human-readable blockchain data.
Use --format=coreth for C-Chain compatible namespaced format.`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			extractor := extract.New(app)
			return extractor.ExtractBlockchain(args[0], args[1], opts)
		},
	}

	cmd.Flags().StringVar(&opts.Format, "format", "bytes", "Output format: bytes, json, or coreth")
	cmd.Flags().Uint64Var(&opts.StartBlock, "start-block", 0, "Starting block number")
	cmd.Flags().Uint64Var(&opts.EndBlock, "end-block", 0, "Ending block number (0 = all blocks)")
	cmd.Flags().StringVar(&opts.Network, "network", "lux", "Network type for namespace conversion")

	return cmd
}

func newExtractGenesisCmd(app *application.Genesis) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "genesis [db-path] [output-path]",
		Short: "Extract genesis data from blockchain",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			extractor := extract.New(app)
			return extractor.ExtractGenesis(args[0], args[1])
		},
	}

	return cmd
}

func newExtractStateCmd(app *application.Genesis) *cobra.Command {
	var (
		blockNumber uint64
		includeCode bool
	)

	cmd := &cobra.Command{
		Use:   "state [db-path] [output-path]",
		Short: "Extract state data from blockchain",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			extractor := extract.New(app)
			return extractor.ExtractState(args[0], args[1], blockNumber, includeCode)
		},
	}

	cmd.Flags().Uint64Var(&blockNumber, "block", 0, "Block number to extract state from")
	cmd.Flags().BoolVar(&includeCode, "include-code", false, "Include contract code in state export")

	return cmd
}