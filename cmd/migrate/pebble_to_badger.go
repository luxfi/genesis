package migrate

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/luxfi/database"
	"github.com/luxfi/database/badgerdb"
	"github.com/luxfi/database/pebbledb"
)

var pebbleToBadgerCmd = &cobra.Command{
	Use:   "pebble-to-badger <source-path> <dest-path>",
	Short: "Migrate PebbleDB database to BadgerDB",
	Long: `Migrate all data from a PebbleDB database to a BadgerDB database.
This is useful when switching database backends while preserving all blockchain data.`,
	Args: cobra.ExactArgs(2),
	RunE: runPebbleToBadgerMigration,
}

func init() {
	Cmd.AddCommand(pebbleToBadgerCmd)
}

func runPebbleToBadgerMigration(cmd *cobra.Command, args []string) error {
	sourcePath := args[0]
	destPath := args[1]

	fmt.Printf("Migrating database from PebbleDB to BadgerDB...\n")
	fmt.Printf("Source: %s\n", sourcePath)
	fmt.Printf("Destination: %s\n", destPath)

	// Open source PebbleDB
	sourceDB, err := pebbledb.New(sourcePath, 512, 512, "migration", false)
	if err != nil {
		return fmt.Errorf("failed to open source PebbleDB: %w", err)
	}
	defer sourceDB.Close()

	// Open destination BadgerDB
	destDB, err := badgerdb.New(destPath, nil, "migration", nil)
	if err != nil {
		return fmt.Errorf("failed to open destination BadgerDB: %w", err)
	}
	defer destDB.Close()

	// Perform migration
	count, err := migrateData(sourceDB, destDB)
	if err != nil {
		return fmt.Errorf("migration failed: %w", err)
	}

	fmt.Printf("Successfully migrated %d keys from PebbleDB to BadgerDB\n", count)
	return nil
}

func migrateData(source, dest database.Database) (int64, error) {
	it := source.NewIterator()
	defer it.Release()

	batch := dest.NewBatch()
	count := int64(0)
	batchSize := int64(0)
	const maxBatchSize = 10000

	for it.Next() {
		key := it.Key()
		value := it.Value()

		// Copy the data to avoid issues with iterator
		keyCopy := make([]byte, len(key))
		copy(keyCopy, key)
		valueCopy := make([]byte, len(value))
		copy(valueCopy, value)

		if err := batch.Put(keyCopy, valueCopy); err != nil {
			return count, fmt.Errorf("failed to put key %x: %w", keyCopy, err)
		}

		count++
		batchSize++

		// Write batch periodically
		if batchSize >= maxBatchSize {
			if err := batch.Write(); err != nil {
				return count, fmt.Errorf("failed to write batch: %w", err)
			}
			batch.Reset()
			batchSize = 0
			
			if count%100000 == 0 {
				fmt.Printf("Migrated %d keys...\n", count)
			}
		}
	}

	// Write final batch
	if batchSize > 0 {
		if err := batch.Write(); err != nil {
			return count, fmt.Errorf("failed to write final batch: %w", err)
		}
	}

	if err := it.Error(); err != nil {
		return count, fmt.Errorf("iterator error: %w", err)
	}

	return count, nil
}