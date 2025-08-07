package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/luxfi/genesis/pkg/application"
	"github.com/luxfi/genesis/pkg/migration"
	"github.com/spf13/cobra"
)

// NewMigrateCmd creates the migrate command
func NewMigrateCmd(app *application.Genesis) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Database migration tools",
		Long:  `Tools for migrating blockchain databases between different formats`,
	}

	cmd.AddCommand(newMigrateSubnetCmd(app))
	return cmd
}

func newMigrateSubnetCmd(app *application.Genesis) *cobra.Command {
	return &cobra.Command{
		Use:   "subnet <source-db> <target-db>",
		Short: "Migrate SubnetEVM database to C-chain format",
		Long: `Migrates a SubnetEVM PebbleDB database to C-chain format.
This preserves all blocks, receipts, and state data while converting
the key formats from SubnetEVM (prefix 0x33) to C-chain format.`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			sourcePath := args[0]
			targetPath := args[1]

			// Log operation
			if app != nil && app.Log != nil {
				app.Log.Info("Starting SubnetEVM migration",
					"source", sourcePath,
					"target", targetPath)
			}

			// Validate source exists
			if !fileExists(sourcePath) {
				return fmt.Errorf("source database does not exist: %s", sourcePath)
			}

			// Create target directory
			targetDir := filepath.Dir(targetPath)
			if err := ensureDir(targetDir); err != nil {
				return fmt.Errorf("failed to create target directory: %w", err)
			}

			// Create migrator
			migrator, err := migration.NewSubnetToCChain(sourcePath, targetPath)
			if err != nil {
				return fmt.Errorf("failed to create migrator: %w", err)
			}
			defer migrator.Close()

			// Run migration
			if err := migrator.Migrate(); err != nil {
				return fmt.Errorf("migration failed: %w", err)
			}

			fmt.Println("Migration completed successfully!")
			fmt.Printf("C-chain database created at: %s\n", targetPath)
			return nil
		},
	}
}

// fileExists checks if a file exists
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// ensureDir creates a directory if it doesn't exist
func ensureDir(dir string) error {
	return os.MkdirAll(dir, 0755)
}