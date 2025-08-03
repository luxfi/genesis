package cmd

import (
	"github.com/luxfi/genesis/pkg/application"
	"github.com/luxfi/genesis/pkg/importer"
	"github.com/spf13/cobra"
)

// NewImportCmd creates the import command with subcommands
func NewImportCmd(app *application.Genesis) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "import",
		Short: "Import blockchain data",
		Long:  "Import blockchain data from various sources",
	}

	cmd.AddCommand(newImportBlockchainCmd(app))

	return cmd
}

func newImportBlockchainCmd(app *application.Genesis) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "blockchain [source-db] [dest-db]",
		Short: "Import blockchain data from extracted database",
		Long:  "Import blockchain headers, bodies, and receipts from extracted SubnetEVM data into C-Chain format",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			imp := importer.New(app)
			return imp.ImportBlockchain(args[0], args[1])
		},
	}

	return cmd
}