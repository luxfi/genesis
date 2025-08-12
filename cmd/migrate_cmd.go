package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/luxfi/genesis/pkg/application"
	"github.com/luxfi/genesis/pkg/migration"
	"github.com/spf13/cobra"
)

// NewMigrateCmd creates the migrate command
func NewMigrateCmd(app *application.Genesis) *cobra.Command {
	var (
		srcPath     string
		dstPath     string
		srcType     string
		dstType     string
		networkID   uint32
		skipBackup  bool
	)

	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Migrate database from one format to another",
		Long:  `Migrate database from PebbleDB to BadgerDB or between other formats for Lux mainnet`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Default paths if not specified
			if srcPath == "" {
				srcPath = filepath.Join(baseDir, "state", "chaindata")
			}
			if dstPath == "" {
				dstPath = filepath.Join(baseDir, "migrated")
			}

			fmt.Printf("Starting migration from %s to %s\n", srcPath, dstPath)
			fmt.Printf("Source type: %s, Destination type: %s\n", srcType, dstType)
			fmt.Printf("Network ID: %d\n", networkID)

			// Create migrator for subnet to C-Chain migration
			migrator, err := migration.NewSubnetToCChain(srcPath, dstPath)
			if err != nil {
				return fmt.Errorf("failed to create migrator: %w", err)
			}
			
			// Run migration
			if err := migrator.Migrate(); err != nil {
				return fmt.Errorf("migration failed: %w", err)
			}

			fmt.Println("Migration completed successfully!")
			return nil
		},
	}

	// Add flags
	cmd.Flags().StringVar(&srcPath, "src", "", "Source database path")
	cmd.Flags().StringVar(&dstPath, "dst", "", "Destination database path")
	cmd.Flags().StringVar(&srcType, "src-type", "pebbledb", "Source database type (pebbledb, badgerdb, leveldb)")
	cmd.Flags().StringVar(&dstType, "dst-type", "badgerdb", "Destination database type (pebbledb, badgerdb, leveldb)")
	cmd.Flags().Uint32Var(&networkID, "network-id", 96369, "Network ID for the chain")
	cmd.Flags().BoolVar(&skipBackup, "skip-backup", false, "Skip creating backup before migration")

	return cmd
}