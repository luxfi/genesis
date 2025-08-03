package cmd

import (
	"github.com/luxfi/genesis/pkg/application"
	"github.com/spf13/cobra"
)

// NewToolsCmd creates the tools command
func NewToolsCmd(app *application.Genesis) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tools",
		Short: "Various tools and utilities",
		Long:  "Collection of utility tools for blockchain operations",
	}

	// TODO: Add various utility commands
	// - tools verify-genesis
	// - tools calculate-hash
	// - tools encode/decode
	// - tools key-convert

	return cmd
}