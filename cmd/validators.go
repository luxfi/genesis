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

	// TODO: Add subcommands
	// - validators list
	// - validators add
	// - validators remove
	// - validators generate

	return cmd
}