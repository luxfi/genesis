
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

// InspectResult holds information about an inspected key.
type InspectResult struct {
	Type    string
	Decoded string
	Details string
}

// InspectKey decodes and analyzes a specific database key.
func (i *Inspector) InspectKey(key []byte) (*InspectResult, error) {
	value, err := i.db.Get(key)
	if err != nil {
		return nil, fmt.Errorf("key not found: %w", err)
	}

	// Basic inspection - in the future, we can add more sophisticated decoding
	result := &InspectResult{
		Type:    "raw",
		Decoded: fmt.Sprintf("%d bytes", len(value)),
	}

	// Try to identify common key types
	if len(key) > 0 {
		switch key[0] {
		case 'h':
			result.Type = "header"
		case 'b':
			result.Type = "body"
		case 'r':
			result.Type = "receipt"
		case 'a':
			result.Type = "account"
		case 'H':
			result.Type = "canonical-hash"
		case 'n':
			result.Type = "number-to-hash"
		}
	}

	return result, nil
}

// TipInfo holds information about the chain tip.
type TipInfo struct {
	Height     uint64
	Hash       []byte
	ParentHash []byte
	Timestamp  uint64
}

// FindTip finds the highest block in the database.
func (i *Inspector) FindTip() (*TipInfo, error) {
	// This is a simplified implementation
	// In a real implementation, we would use rawdb functions to find the actual tip
	return &TipInfo{
		Height:     0,
		Hash:       []byte{},
		ParentHash: []byte{},
		Timestamp:  0,
	}, nil
}

// Stats holds database statistics.
type Stats struct {
	TotalKeys    uint64
	TotalSize    uint64
	KeyTypes     map[string]uint64
	LatestBlock  uint64
}

// GetStats gathers statistics about the database.
func (i *Inspector) GetStats() (*Stats, error) {
	stats := &Stats{
		KeyTypes: make(map[string]uint64),
	}

	iter := i.db.NewIterator(nil, nil)
	defer iter.Release()

	for iter.Next() {
		stats.TotalKeys++
		stats.TotalSize += uint64(len(iter.Key()) + len(iter.Value()))

		// Categorize keys by type
		if len(iter.Key()) > 0 {
			switch iter.Key()[0] {
			case 'h':
				stats.KeyTypes["header"]++
			case 'b':
				stats.KeyTypes["body"]++
			case 'r':
				stats.KeyTypes["receipt"]++
			case 'a':
				stats.KeyTypes["account"]++
			case 'H':
				stats.KeyTypes["canonical-hash"]++
			case 'n':
				stats.KeyTypes["number-to-hash"]++
			default:
				stats.KeyTypes["other"]++
			}
		}
	}

	if err := iter.Error(); err != nil {
		return nil, fmt.Errorf("iterator error: %w", err)
	}

	return stats, nil
}
