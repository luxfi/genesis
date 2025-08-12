package cmd

import (
	"encoding/hex"
	"fmt"
	"strconv"

	"github.com/luxfi/genesis/pkg/application"
	"github.com/luxfi/genesis/pkg/database"
	"github.com/spf13/cobra"
)

// NewDatabaseCmd creates the database command with subcommands
func NewDatabaseCmd(app *application.Genesis) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "database",
		Short: "Database operations",
		Long:  "Commands for managing and inspecting blockchain databases",
	}

	cmd.AddCommand(newDatabaseWriteHeightCmd(app))
	cmd.AddCommand(newDatabaseGetCanonicalCmd(app))
	cmd.AddCommand(newDatabaseStatusCmd(app))
	cmd.AddCommand(newDatabasePrepareMigrationCmd(app))
	cmd.AddCommand(newDatabaseCompactCmd(app))
	cmd.AddCommand(newDatabaseConvertCmd(app))

	return cmd
}

func newDatabaseWriteHeightCmd(app *application.Genesis) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "write-height [db-path] [height]",
		Short: "Write a Height key to the database",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			height, err := strconv.ParseUint(args[1], 10, 64)
			if err != nil {
				return err
			}

			dbMgr := database.New(app)
			return dbMgr.WriteHeight(args[0], height)
		},
	}
	return cmd
}

func newDatabaseGetCanonicalCmd(app *application.Genesis) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get-canonical [db-path] [height]",
		Short: "Get the canonical block hash at a specific height",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			height, err := strconv.ParseUint(args[1], 10, 64)
			if err != nil {
				return err
			}

			dbMgr := database.New(app)
			hash, err := dbMgr.GetCanonicalHash(args[0], height)
			if err != nil {
				return err
			}

			app.Log.Info("Canonical hash", "height", height, "hash", hash)
			return nil
		},
	}
	return cmd
}

func newDatabaseStatusCmd(app *application.Genesis) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status [db-path]",
		Short: "Check database status and statistics",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dbMgr := database.New(app)
			return dbMgr.CheckStatus(args[0])
		},
	}
	return cmd
}

func newDatabasePrepareMigrationCmd(app *application.Genesis) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "prepare-migration [db-path] [height]",
		Short: "Prepare database for LUX_GENESIS migration",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			height, err := strconv.ParseUint(args[1], 10, 64)
			if err != nil {
				return err
			}

			dbMgr := database.New(app)
			return dbMgr.PrepareMigration(args[0], height)
		},
	}
	return cmd
}

func newDatabaseCompactCmd(app *application.Genesis) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "compact-ancient [db-path] [block-num]",
		Short: "Compact ancient data in the database",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			blockNum, err := strconv.ParseUint(args[1], 10, 64)
			if err != nil {
				return err
			}

			dbMgr := database.New(app)
			return dbMgr.CompactAncient(args[0], blockNum)
		},
	}
	return cmd
}

func newDatabaseConvertCmd(app *application.Genesis) *cobra.Command {
	var (
		sourceType      string
		destType        string
		conversionType  string
		namespace       string
		batchSize       int
		verbose         bool
		fixCanonical    bool
	)

	cmd := &cobra.Command{
		Use:   "convert [source-db] [dest-db]",
		Short: "Convert database between different formats",
		Long: `Convert database between different formats and structures.

Supported conversions:
- subnet-to-coreth: Convert SubnetEVM to Coreth format (removes namespace)
- coreth-to-subnet: Convert Coreth to SubnetEVM format (adds namespace)
- pebble-to-badger: Convert PebbleDB to BadgerDB
- badger-to-pebble: Convert BadgerDB to PebbleDB
- denamespace: Remove namespace prefix from all keys
- add-namespace: Add namespace prefix to all keys

Examples:
  # Convert SubnetEVM PebbleDB to Coreth BadgerDB
  genesis database convert /path/to/subnet.db /path/to/coreth.db \
    --source-type=pebbledb --dest-type=badgerdb \
    --conversion=subnet-to-coreth \
    --namespace=337fb73f9bcdac8c31a2d5f7b877ab1e8a2b7f2a1e9bf02a0a0e6c6fd164f1d1

  # Simple PebbleDB to BadgerDB conversion
  genesis database convert /path/to/pebble.db /path/to/badger.db \
    --conversion=pebble-to-badger`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			sourcePath := args[0]
			destPath := args[1]

			// Parse namespace if provided
			var namespaceBytes []byte
			if namespace != "" {
				var err error
				namespaceBytes, err = hex.DecodeString(namespace)
				if err != nil {
					return fmt.Errorf("invalid namespace hex: %w", err)
				}
			}

			// Create conversion config
			config := &database.ConversionConfig{
				SourcePath:      sourcePath,
				DestPath:        destPath,
				SourceType:      database.DatabaseType(sourceType),
				DestType:        database.DatabaseType(destType),
				ConversionType:  database.ConversionType(conversionType),
				Namespace:       namespaceBytes,
				BatchSize:       batchSize,
				Verbose:         verbose,
				FixCanonical:    fixCanonical,
			}

			// Run conversion
			converter := database.NewDatabaseConverter(config)
			if err := converter.Convert(); err != nil {
				return fmt.Errorf("conversion failed: %w", err)
			}

			fmt.Println("\n✅ Database conversion completed successfully!")
			return nil
		},
	}

	cmd.Flags().StringVar(&sourceType, "source-type", "pebbledb", "Source database type (pebbledb, badgerdb, leveldb)")
	cmd.Flags().StringVar(&destType, "dest-type", "badgerdb", "Destination database type (pebbledb, badgerdb, leveldb)")
	cmd.Flags().StringVar(&conversionType, "conversion", "pebble-to-badger", "Conversion type")
	cmd.Flags().StringVar(&namespace, "namespace", "", "Namespace to add/remove (hex encoded)")
	cmd.Flags().IntVar(&batchSize, "batch-size", 10000, "Batch size for conversion")
	cmd.Flags().BoolVar(&verbose, "verbose", false, "Verbose output")
	cmd.Flags().BoolVar(&fixCanonical, "fix-canonical", true, "Fix missing canonical mappings")

	return cmd
}
