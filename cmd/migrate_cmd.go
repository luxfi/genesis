
package cmd

import (
	"fmt"

	"github.com/luxfi/genesis/pkg/application"
	"github.com/luxfi/genesis/pkg/migration"
	"github.com/spf13/cobra"
)

// NewMigrateCmd creates the new, subcommand-based `migrate` command.
func NewMigrateCmd(app *application.Genesis) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Provides tools to migrate and verify blockchain databases",
		Long: `The migrate command is a replacement for the various standalone migration scripts.

It uses a strategy-based approach to support different kinds of migrations (e.g., full copies, subnet-to-cchain) and includes tools to verify the integrity of a migration.`,
	}

	// Add subcommands
	cmd.AddCommand(newMigrateRunCmd(app))
	cmd.AddCommand(newMigrateVerifyCmd(app))

	return cmd
}

// newMigrateRunCmd creates the `migrate run` subcommand.
func newMigrateRunCmd(app *application.Genesis) *cobra.Command {
	var (
		sourcePath string
		destPath   string
		strategy   string
	)

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Runs a database migration from a source to a destination",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			// 1. Select the strategy
			var strat migration.Strategy
			switch strategy {
			case "full-copy":
				strat = &migration.FullCopyStrategy{}
			// In the future, we would add cases for "subnet-cchain", etc.
			default:
				return fmt.Errorf("unknown migration strategy '%s'. available: [full-copy]", strategy)
			}

			// 2. Create the migrator
			cfg := migration.Config{
				SourceDBPath: sourcePath,
				DestDBPath:   destPath,
				Strategy:     strat,
			}
			m, err := migration.NewMigrator(cfg)
			if err != nil {
				return fmt.Errorf("failed to set up migrator: %w", err)
			}
			defer m.Close()

			// 3. Run the migration
			if err := m.Run(); err != nil {
				return fmt.Errorf("migration failed: %w", err)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&sourcePath, "source", "", "Path to the source database (required)")
	cmd.Flags().StringVar(&destPath, "dest", "", "Path to the destination database (required)")
	cmd.Flags().StringVar(&strategy, "strategy", "full-copy", "The migration strategy to use (e.g., full-copy)")
	_ = cmd.MarkFlagRequired("source")
	_ = cmd.MarkFlagRequired("dest")

	return cmd
}

// newMigrateVerifyCmd creates the `migrate verify` subcommand.
func newMigrateVerifyCmd(app *application.Genesis) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "verify",
		Short: "Verifies the integrity of a migration by comparing two databases",
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.Println("Verification logic not yet implemented.")
			return nil
		},
	}

	cmd.Flags().String("db-a", "", "Path to the first database")
	cmd.Flags().String("db-b", "", "Path to the second database")

	return cmd
}
