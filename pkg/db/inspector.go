
package db

import (
	"fmt"

	"github.com/luxfi/genesis/pkg/application"
	"github.com/luxfi/geth/ethdb"
)

// Inspector provides tools for low-level database inspection.
type Inspector struct {
	app *application.Genesis
	db  ethdb.Database
}

// NewInspector creates a new database inspector.
func NewInspector(app *application.Genesis, db ethdb.Database) *Inspector {
	return &Inspector{app: app, db: db}
}

// ScanResult holds information about a single key-value pair.
type ScanResult struct {
	Key   []byte
	Value []byte
	// In the future, we can add decoded information here.
}

// ScanOptions provides options for a database scan.
type ScanOptions struct {
	Prefix []byte
	Limit  int
}

// Scan iterates over the database and returns key-value pairs.
func (i *Inspector) Scan(opts ScanOptions) ([]ScanResult, error) {
	var results []ScanResult
	iter := i.db.NewIterator(opts.Prefix, nil)
	defer iter.Release()

	for iter.Next() {
		key := make([]byte, len(iter.Key()))
		copy(key, iter.Key())

		value := make([]byte, len(iter.Value()))
		copy(value, iter.Value())

		results = append(results, ScanResult{
			Key:   key,
			Value: value,
		})

		if opts.Limit > 0 && len(results) >= opts.Limit {
			break
		}
	}

	if err := iter.Error(); err != nil {
		return nil, fmt.Errorf("iterator error: %w", err)
	}

	return results, nil
}

// GetDB returns the underlying database instance.
func (i *Inspector) GetDB() ethdb.Database {
	return i.db
}
