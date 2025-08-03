package cmd

import (
	"github.com/luxfi/genesis/pkg/application"
	"github.com/spf13/cobra"
)

// NewValidatorsCmd creates the validators command
func NewValidatorsCmd(app *application.Genesis) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "validators",
		Short: "Validator management",
		Long:  "Manage validators for P-Chain genesis",
	}

	// Subcommands
	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List validators",
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.Println("Validator list not yet implemented")
			return nil
		},
	})

	return cmd
}