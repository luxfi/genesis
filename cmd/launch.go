package cmd

import (
	"github.com/luxfi/genesis/pkg/application"
	"github.com/spf13/cobra"
)

// NewLaunchCmd creates the launch command
func NewLaunchCmd(app *application.Genesis) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "launch",
		Short: "Launch networks",
		Long:  "Launch various network configurations",
	}

	// TODO: Add subcommands for different launch scenarios
	// - launch mainnet
	// - launch testnet
	// - launch local
	// - launch l2
	// - launch migrated

	return cmd
}