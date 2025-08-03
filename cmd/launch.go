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

	// Subcommands
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