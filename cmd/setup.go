package cmd

import (
	"github.com/luxfi/genesis/pkg/application"
	"github.com/luxfi/genesis/pkg/setup"
	"github.com/spf13/cobra"
)

// NewSetupCmd creates the setup command
func NewSetupCmd(app *application.Genesis) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Setup chain state and configuration",
		Long:  "Configure blockchain state for imported data",
	}

	cmd.AddCommand(newSetupChainStateCmd(app))

	return cmd
}

func newSetupChainStateCmd(app *application.Genesis) *cobra.Command {
	var targetHeight uint64

	cmd := &cobra.Command{
		Use:   "chain-state [db-path]",
		Short: "Setup C-Chain state with imported blockchain data",
		Long:  "Properly configure chain head and state references for imported blockchain",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			manager := setup.New(app)
			return manager.SetupChainState(args[0], targetHeight)
		},
	}

	cmd.Flags().Uint64Var(&targetHeight, "target-height", 0, "Target block height (0 = find highest)")

	return cmd
}