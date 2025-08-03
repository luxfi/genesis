package cmd

import (
	"fmt"

	"github.com/luxfi/genesis/pkg/application"
	"github.com/luxfi/genesis/pkg/genesis"
	"github.com/spf13/cobra"
)

var (
	// Generate command flags
	genNetwork   string
	genChainType string
	genOutputDir string
	genChainID   uint64
	genNetworkID uint64
	genValidators int
)

// NewGenerateCmd creates the generate command
func NewGenerateCmd(app *application.Genesis) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate genesis configurations",
		Long:  "Generate genesis configurations for different network types (mainnet, testnet, local, etc.)",
		Run: func(cmd *cobra.Command, args []string) {
			err := cmd.Help()
			if err != nil {
				fmt.Println(err)
			}
		},
	}

	// Add subcommands
	cmd.AddCommand(newGenerateMainnetCmd(app))
	cmd.AddCommand(newGenerateTestnetCmd(app))
	cmd.AddCommand(newGenerateLocalCmd(app))
	cmd.AddCommand(newGenerateCustomCmd(app))

	return cmd
}

func newGenerateMainnetCmd(app *application.Genesis) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mainnet",
		Short: "Generate mainnet genesis configuration",
		Long:  "Generate genesis configuration for Lux mainnet with proper validators and allocations",
		RunE: func(cmd *cobra.Command, args []string) error {
			if app == nil {
				return fmt.Errorf("application not initialized")
			}

			app.Log.Info("Generating mainnet genesis configuration")

			// Use the genesis package for business logic
			cfg := genesis.MainnetConfig()
			generator := genesis.NewGenerator(cfg)
			
			outputDir := genOutputDir
			if outputDir == "" {
				outputDir = app.GetOutputDir()
			}

			if err := generator.Generate(outputDir); err != nil {
				return fmt.Errorf("failed to generate mainnet genesis: %w", err)
			}

			app.Log.Info("Mainnet genesis generated successfully", 
				"output", outputDir)
			return nil
		},
	}

	cmd.Flags().StringVar(&genOutputDir, "output", "", "Output directory for genesis files")

	return cmd
}

func newGenerateTestnetCmd(app *application.Genesis) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "testnet",
		Short: "Generate testnet genesis configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			if app == nil {
				return fmt.Errorf("application not initialized")
			}

			app.Log.Info("Generating testnet genesis configuration")

			cfg := genesis.TestnetConfig()
			generator := genesis.NewGenerator(cfg)
			
			outputDir := genOutputDir
			if outputDir == "" {
				outputDir = app.GetOutputDir()
			}

			if err := generator.Generate(outputDir); err != nil {
				return fmt.Errorf("failed to generate testnet genesis: %w", err)
			}

			app.Log.Info("Testnet genesis generated successfully", 
				"output", outputDir)
			return nil
		},
	}

	cmd.Flags().StringVar(&genOutputDir, "output", "", "Output directory for genesis files")

	return cmd
}

func newGenerateLocalCmd(app *application.Genesis) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "local",
		Short: "Generate local network genesis configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			if app == nil {
				return fmt.Errorf("application not initialized")
			}

			app.Log.Info("Generating local network genesis configuration",
				"validators", genValidators)

			cfg := genesis.LocalConfig(genValidators)
			generator := genesis.NewGenerator(cfg)
			
			outputDir := genOutputDir
			if outputDir == "" {
				outputDir = app.GetOutputDir()
			}

			if err := generator.Generate(outputDir); err != nil {
				return fmt.Errorf("failed to generate local genesis: %w", err)
			}

			app.Log.Info("Local genesis generated successfully", 
				"output", outputDir,
				"validators", genValidators)
			return nil
		},
	}

	cmd.Flags().StringVar(&genOutputDir, "output", "", "Output directory for genesis files")
	cmd.Flags().IntVar(&genValidators, "validators", 5, "Number of validators")

	return cmd
}

func newGenerateCustomCmd(app *application.Genesis) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "custom",
		Short: "Generate custom genesis configuration",
		Long:  "Generate a custom genesis configuration with specified parameters",
		RunE: func(cmd *cobra.Command, args []string) error {
			if app == nil {
				return fmt.Errorf("application not initialized")
			}

			app.Log.Info("Generating custom genesis configuration",
				"network", genNetwork,
				"chainID", genChainID,
				"networkID", genNetworkID)

			cfg := &genesis.Config{
				Network:   genNetwork,
				ChainID:   genChainID,
				NetworkID: genNetworkID,
				ChainType: genChainType,
			}

			generator := genesis.NewGenerator(cfg)
			
			outputDir := genOutputDir
			if outputDir == "" {
				outputDir = app.GetOutputDir()
			}

			if err := generator.Generate(outputDir); err != nil {
				return fmt.Errorf("failed to generate custom genesis: %w", err)
			}

			app.Log.Info("Custom genesis generated successfully", 
				"output", outputDir)
			return nil
		},
	}

	cmd.Flags().StringVar(&genNetwork, "network", "custom", "Network name")
	cmd.Flags().StringVar(&genChainType, "type", "l1", "Chain type (l1, l2, l3)")
	cmd.Flags().StringVar(&genOutputDir, "output", "", "Output directory")
	cmd.Flags().Uint64Var(&genChainID, "chain-id", 99999, "Chain ID")
	cmd.Flags().Uint64Var(&genNetworkID, "network-id", 99999, "Network ID")

	return cmd
}