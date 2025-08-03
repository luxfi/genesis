package cmd

import (
	"github.com/luxfi/genesis/pkg/application"
	"github.com/luxfi/genesis/pkg/inspect"
	"github.com/luxfi/geth/common"
	"github.com/spf13/cobra"
)

// NewInspectCmd creates the inspect command with subcommands
func NewInspectCmd(app *application.Genesis) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "inspect",
		Short: "Inspect blockchain data",
		Long:  "Inspect and analyze blockchain database contents",
	}

	cmd.AddCommand(newInspectTipCmd(app))
	cmd.AddCommand(newInspectBlocksCmd(app))
	cmd.AddCommand(newInspectKeysCmd(app))
	cmd.AddCommand(newInspectBalanceCmd(app))
	cmd.AddCommand(newInspectDebugKeysCmd(app))

	return cmd
}

func newInspectTipCmd(app *application.Genesis) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tip [db-path]",
		Short: "Find and display the chain tip",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			inspector := inspect.New(app)
			return inspector.InspectTip(args[0])
		},
	}
	return cmd
}

func newInspectBlocksCmd(app *application.Genesis) *cobra.Command {
	var start, count uint64

	cmd := &cobra.Command{
		Use:   "blocks [db-path]",
		Short: "Display information about blocks in the database",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			inspector := inspect.New(app)
			return inspector.InspectBlocks(args[0], start, count)
		},
	}

	cmd.Flags().Uint64Var(&start, "start", 0, "Starting block number")
	cmd.Flags().Uint64Var(&count, "count", 10, "Number of blocks to display")

	return cmd
}

func newInspectKeysCmd(app *application.Genesis) *cobra.Command {
	var limit int

	cmd := &cobra.Command{
		Use:   "keys [db-path]",
		Short: "Show the different key types in the database",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			inspector := inspect.New(app)
			return inspector.InspectKeys(args[0], limit)
		},
	}

	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum number of keys to examine (0 = all)")

	return cmd
}

func newInspectBalanceCmd(app *application.Genesis) *cobra.Command {
	var blockNum uint64

	cmd := &cobra.Command{
		Use:   "balance [db-path] [address]",
		Short: "Check the balance of an address at a specific block",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			inspector := inspect.New(app)
			addr := common.HexToAddress(args[1])
			return inspector.InspectBalance(args[0], addr, blockNum)
		},
	}

	cmd.Flags().Uint64Var(&blockNum, "block", 0, "Block number (0 = latest)")

	return cmd
}

func newInspectDebugKeysCmd(app *application.Genesis) *cobra.Command {
	var prefix string
	var limit int

	cmd := &cobra.Command{
		Use:   "debug-keys [db-path]",
		Short: "Debug database keys to understand structure",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			inspector := inspect.New(app)
			return inspector.DebugKeys(args[0], prefix, limit)
		},
	}

	cmd.Flags().StringVar(&prefix, "prefix", "", "Filter by key prefix")
	cmd.Flags().IntVar(&limit, "limit", 100, "Limit number of keys to show")

	return cmd
}
