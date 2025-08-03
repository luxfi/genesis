package cmd

import (
	"fmt"
	"strings"

	"github.com/luxfi/genesis/pkg/application"
	"github.com/luxfi/genesis/pkg/l2"
	"github.com/spf13/cobra"
)

// NewL2Cmd creates the L2 command with subcommands
func NewL2Cmd(app *application.Genesis) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "l2",
		Short: "L2 network management",
		Long:  "Create, configure, and manage Layer 2 networks on Lux",
	}

	cmd.AddCommand(newL2CreateCmd(app))
	cmd.AddCommand(newL2ListCmd(app))
	cmd.AddCommand(newL2InfoCmd(app))
	cmd.AddCommand(newL2DeleteCmd(app))

	return cmd
}

func newL2CreateCmd(app *application.Genesis) *cobra.Command {
	var config l2.L2Config

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new L2 network configuration",
		Long: `Create a new L2 network configuration that can be launched later.

Examples:
  # Create a new L2 for mainnet
  genesis l2 create --name zoo --chain-id 200200 --symbol ZOO --display "Zoo Network"

  # Create with existing chain data
  genesis l2 create --name zoo --chain-id 200200 --symbol ZOO --chain-data /path/to/chaindata`,
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr := l2.New(app)
			return mgr.Create(config)
		},
	}

	cmd.Flags().StringVar(&config.Name, "name", "", "L2 network name (required)")
	cmd.Flags().Uint64Var(&config.ChainID, "chain-id", 0, "Chain ID (required)")
	cmd.Flags().Uint64Var(&config.TestnetID, "testnet-id", 0, "Testnet chain ID")
	cmd.Flags().StringVar(&config.Symbol, "symbol", "", "Token symbol (required)")
	cmd.Flags().StringVar(&config.DisplayName, "display", "", "Display name")
	cmd.Flags().StringVar(&config.BaseNetwork, "base", "lux", "Base network (lux, zoo, spc)")
	cmd.Flags().StringVar(&config.ChainDataPath, "chain-data", "", "Path to existing chain data")

	cmd.MarkFlagRequired("name")
	cmd.MarkFlagRequired("chain-id")
	cmd.MarkFlagRequired("symbol")

	return cmd
}

func newL2ListCmd(app *application.Genesis) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all L2 networks",
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr := l2.New(app)
			configs, err := mgr.List()
			if err != nil {
				return err
			}

			if len(configs) == 0 {
				fmt.Println("No L2 networks configured")
				return nil
			}

			fmt.Printf("%-15s %-10s %-10s %-20s\n", "NAME", "CHAIN ID", "SYMBOL", "CREATED")
			fmt.Println(strings.Repeat("-", 60))

			for _, cfg := range configs {
				fmt.Printf("%-15s %-10d %-10s %-20s\n",
					cfg.Name,
					cfg.ChainID,
					cfg.Symbol,
					cfg.CreatedAt.Format("2006-01-02 15:04:05"))
			}

			return nil
		},
	}
	return cmd
}

func newL2InfoCmd(app *application.Genesis) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "info [name]",
		Short: "Show detailed information about an L2 network",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr := l2.New(app)
			config, err := mgr.Get(args[0])
			if err != nil {
				return err
			}

			fmt.Printf("L2 Network: %s\n", config.Name)
			fmt.Printf("===================\n")
			fmt.Printf("Chain ID:      %d\n", config.ChainID)
			if config.TestnetID > 0 {
				fmt.Printf("Testnet ID:    %d\n", config.TestnetID)
			}
			fmt.Printf("Symbol:        %s\n", config.Symbol)
			fmt.Printf("Display Name:  %s\n", config.DisplayName)
			fmt.Printf("Base Network:  %s\n", config.BaseNetwork)
			if config.ChainDataPath != "" {
				fmt.Printf("Chain Data:    %s\n", config.ChainDataPath)
			}
			fmt.Printf("Created:       %s\n", config.CreatedAt.Format("2006-01-02 15:04:05"))
			fmt.Printf("Updated:       %s\n", config.UpdatedAt.Format("2006-01-02 15:04:05"))

			return nil
		},
	}
	return cmd
}

func newL2DeleteCmd(app *application.Genesis) *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "delete [name]",
		Short: "Delete an L2 network configuration",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			if !force {
				fmt.Printf("Are you sure you want to delete L2 network '%s'? (y/N): ", name)
				var response string
				fmt.Scanln(&response)
				if response != "y" && response != "Y" {
					fmt.Println("Deletion cancelled")
					return nil
				}
			}

			mgr := l2.New(app)
			return mgr.Delete(name)
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "Force deletion without confirmation")

	return cmd
}
