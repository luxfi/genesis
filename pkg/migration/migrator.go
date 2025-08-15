package migration

import (
	"fmt"

	"github.com/luxfi/database/badgerdb"
	"github.com/luxfi/database/pebbledb"
	"github.com/luxfi/ethdb"
	"github.com/prometheus/client_golang/prometheus"
)

// Strategy defines the interface for a specific migration process.
type Strategy interface {
	// Name returns the name of the strategy.
	Name() string
	// Migrate performs the data migration from the source to the destination database.
	Migrate(source ethdb.Database, dest ethdb.Database) error
}

// Config holds the configuration for a migration.
type Config struct {
	SourceDBPath string
	DestDBPath   string
	Strategy     Strategy
}

// Migrator manages the overall migration process.
type Migrator struct {
	cfg    Config
	source ethdb.Database
	dest   ethdb.Database
}

// NewMigrator creates a new Migrator instance.
func NewMigrator(cfg Config) (*Migrator, error) {
	if cfg.Strategy == nil {
		return nil, fmt.Errorf("a migration strategy must be provided")
	}

	// Open source database (assume PebbleDB for source, as is common in scripts)
	sourceDB, err := pebbledb.New(cfg.SourceDBPath, nil, "", prometheus.DefaultRegisterer)
	if err != nil {
		return nil, fmt.Errorf("failed to open source pebble database at %s: %w", cfg.SourceDBPath, err)
	}

	// Open destination database (assume BadgerDB for destination)
	destDB, err := badgerdb.New(cfg.DestDBPath, nil, "", prometheus.DefaultRegisterer)
	if err != nil {
		sourceDB.Close()
		return nil, fmt.Errorf("failed to open destination badger database at %s: %w", cfg.DestDBPath, err)
	}

	return &Migrator{
		cfg:    cfg,
		source: sourceDB,
		dest:   destDB,
	},
}

// Run executes the migration using the configured strategy.
func (m *Migrator) Run() error {
	fmt.Printf("Starting migration with strategy: %s\n", m.cfg.Strategy.Name())
	fmt.Printf("Source: %s\n", m.cfg.SourceDBPath)
	fmt.Printf("Destination: %s\n", m.cfg.DestDBPath)

	err := m.cfg.Strategy.Migrate(m.source, m.dest)

	if err != nil {
		return fmt.Errorf("migration strategy '%s' failed: %w", m.cfg.Strategy.Name(), err)
	}

	fmt.Println("Migration completed successfully.")
	return nil
}

// Close closes the database connections.
func (m *Migrator) Close() {
	m.source.Close()
	m.dest.Close()
}

// --- Example Strategy: FullCopy ---

// FullCopyStrategy implements a simple, full copy of a database.
type FullCopyStrategy struct{}

func (s *FullCopyStrategy) Name() string { return "FullCopy" }

func (s *FullCopyStrategy) Migrate(source ethdb.Database, dest ethdb.Database) error {
	// This is a simplified implementation. A real implementation would use batching.
	iter := source.NewIterator(nil, nil)
	defer iter.Release()

	batch := dest.NewBatch()
	count := 0

	for iter.Next() {
		key := iter.Key()
		value := iter.Value()

		if err := batch.Put(key, value); err != nil {
			return err
		}
		if batch.ValueSize() > 10*1024*1024 { // Write every 10MB
			if err := batch.Write(); err != nil {
				return err
			}
			batch.Reset()
			fmt.Printf("Committed %d key-value pairs...\n", count)
		}
		count++
	}

	if err := batch.Write(); err != nil {
		return err
	}

	fmt.Printf("Total key-value pairs migrated: %d\n", count)
	return iter.Error()
}
