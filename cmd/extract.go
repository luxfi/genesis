package cmd

import (
	"fmt"

	"github.com/luxfi/database/badgerdb"
	"github.com/luxfi/genesis/pkg/application"
	"github.com/luxfi/genesis/pkg/extract"
	"github.com/luxfi/geth/ethdb"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/spf13/cobra"
)

// NewExtractCmd creates the extract command and its subcommands.
func NewExtractCmd(app *application.Genesis) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:   "extract",
		Short: "Extract blockchain data from a database",
		Long:  `Extract genesis, state, or blockchain data from an existing chain database.
This command requires a path to the database via the --db-path flag.`, 
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if dbPath == "" {
				return fmt.Errorf("the --db-path flag is required")
			}
			return nil
		},
	}

	cmd.PersistentFlags().StringVar(&dbPath, "db-path", "", "Path to the blockchain database")

	// Add subcommands
	cmd.AddCommand(newExtractGenesisCmd(app, &dbPath))

	return cmd
}

// newExtractGenesisCmd creates the `extract genesis` subcommand.
func newExtractGenesisCmd(app *application.Genesis, dbPath *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "genesis [output-path]",
		Short: "Extracts the genesis data to a JSON file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := openDatabase(*dbPath)
			if err != nil {
				return err
			}
			defer db.Close()

			extractor := extract.New(app, db)
			outputPath := args[0]

			cmd.Printf("Extracting genesis from %s to %s...\n", *dbPath, outputPath)

			if err := extractor.ExtractGenesis(outputPath); err != nil {
				return fmt.Errorf("genesis extraction failed: %w", err)
			}

			cmd.Printf("✅ Genesis extracted successfully.\n")
			return nil
		},
	}
	return cmd
}

// openDatabase is a helper to open a database and return it as a generic interface.
// For now, it defaults to BadgerDB, which is the Lux standard.
func openDatabase(path string) (ethdb.Database, error) {
	db, err := badgerdb.New(path, nil, "", prometheus.DefaultRegisterer)
	if err != nil {
		return nil, fmt.Errorf("failed to open badger database at %s: %w", path, err)
	}
	// In the future, we could add logic here to detect DB type (e.g., PebbleDB)
	// and return the appropriate wrapped instance.
	return &badgerWrapper{db}, nil // Using the same wrapper from the balance checker for now
}

// This is a temporary solution until we have a shared DB package.
// Ideally, this wrapper would be defined in a common `pkg/database`.
type badgerWrapper struct {
	*badgerdb.Database
}

func (b *badgerWrapper) Get(key []byte) ([]byte, error) { return b.Database.Get(key) }
func (b *badgerWrapper) Has(key []byte) (bool, error) {
	_, err := b.Database.Get(key)
	if err != nil {
		return false, err
	}
	return true, nil
}
func (b *badgerWrapper) Put(key, val []byte) error      { return b.Database.Put(key, val) }
func (b *badgerWrapper) Delete(key []byte) error         { return b.Database.Delete(key) }
func (b *badgerWrapper) DeleteRange(start, end []byte) error { return nil }
func (b *badgerWrapper) NewBatch() ethdb.Batch           { panic("not implemented") }
func (b *badgerWrapper) NewBatchWithSize(int) ethdb.Batch { panic("not implemented") }
func (b *badgerWrapper) NewIterator(prefix []byte, start []byte) ethdb.Iterator {
	panic("not implemented")
}
func (b *badgerWrapper) Stat() (string, error)                                  { return "", nil }
func (b *badgerWrapper) Compact([]byte, []byte) error                           { return nil }
func (b *badgerWrapper) HasAncient(string, uint64) (bool, error)                { return false, nil }
func (b *badgerWrapper) Ancient(string, uint64) ([]byte, error)                 { return nil, nil }
func (b *badgerWrapper) AncientRange(string, uint64, uint64, uint64) ([][]byte, error) { return nil, nil }
func (b *badgerWrapper) Ancients() (uint64, error)                              { return 0, nil }
func (b *badgerWrapper) Tail() (uint64, error)                                  { return 0, nil }
func (b *badgerWrapper) AncientSize(string) (uint64, error)                     { return 0, nil }
func (b *badgerWrapper) ReadAncients(func(ethdb.AncientReaderOp) error) error   { return nil }
func (b *badgerWrapper) ModifyAncients(func(ethdb.AncientWriteOp) error) (int64, error) { return 0, nil }
func (b *badgerWrapper) TruncateHead(n uint64) (uint64, error)                  { return 0, nil }
func (b *badgerWrapper) TruncateTail(n uint64) (uint64, error)                  { return 0, nil }
func (b *badgerWrapper) Sync() error                                            { return nil }
func (b *badgerWrapper) MigrateTable(string, func([]byte) ([]byte, error)) error { return nil }
func (b *badgerWrapper) AncientDatadir() (string, error)                        { return "", nil }
func (b *badgerWrapper) SyncAncient() error                                     { return nil }
func (b *badgerWrapper) SyncKeyValue() error                                    { return nil }
func (b *badgerWrapper) Close() error                                           { return b.Database.Close() }
