package cmd

import (
	"github.com/luxfi/genesis/pkg/application"
	"github.com/luxfi/genesis/pkg/convert"
	"github.com/spf13/cobra"
)

// NewConvertCmd creates the convert command with subcommands
func NewConvertCmd(app *application.Genesis) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "convert",
		Short: "Convert blockchain formats",
		Long:  "Convert between different blockchain data and genesis formats",
	}

	cmd.AddCommand(newConvertDenamespaceCmd(app))
	cmd.AddCommand(newConvertGenesisCmd(app))
	cmd.AddCommand(newConvertAddressCmd(app))

	return cmd
}

func newConvertDenamespaceCmd(app *application.Genesis) *cobra.Command {
	var namespace string

	cmd := &cobra.Command{
		Use:   "denamespace [source-db] [dest-db]",
		Short: "Convert namespaced database to denamespaced format",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			conv := convert.New(app)
			return conv.DenamespaceDB(args[0], args[1], namespace)
		},
	}

	cmd.Flags().StringVar(&namespace, "namespace", "", "Namespace to remove")

	return cmd
}

func newConvertGenesisCmd(app *application.Genesis) *cobra.Command {
	var fromFormat, toFormat string

	cmd := &cobra.Command{
		Use:   "genesis [input] [output]",
		Short: "Convert genesis between different formats",
		Long: `Convert genesis files between different formats:
  - subnet: SubnetEVM format
  - cchain: C-Chain format
  - geth: Standard go-ethereum format
  - lux: Lux-specific format`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			conv := convert.New(app)
			return conv.ConvertGenesis(args[0], args[1], fromFormat, toFormat)
		},
	}

	cmd.Flags().StringVar(&fromFormat, "from", "", "Source format (subnet, cchain, geth)")
	cmd.Flags().StringVar(&toFormat, "to", "", "Target format (subnet, cchain, lux)")
	cmd.MarkFlagRequired("from")
	cmd.MarkFlagRequired("to")

	return cmd
}

func newConvertAddressCmd(app *application.Genesis) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "address [address]",
		Short: "Convert addresses between different formats",
		Long: `Convert addresses between:
  - EVM hex format (0x...)
  - Bech32 format (C-lux1...)
  - X/P chain format`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			conv := convert.New(app)
			results, err := conv.ConvertAddress(args[0])
			if err != nil {
				return err
			}

			// Display results
			for format, addr := range results {
				app.Log.Info("Address", format, addr)
			}

			return nil
		},
	}

	return cmd
}
