package importer

import (
	"fmt"

	"github.com/cockroachdb/pebble"
	"github.com/luxfi/genesis/pkg/application"
)

// Importer handles blockchain data import operations
type Importer struct {
	app *application.Genesis
}

// New creates a new Importer instance
func New(app *application.Genesis) *Importer {
	return &Importer{app: app}
}

// ImportBlockchain imports blockchain data from extracted database
func (i *Importer) ImportBlockchain(sourceDB, destDB string) error {
	i.app.Log.Info("Importing blockchain data", "source", sourceDB, "dest", destDB)

	// Open source database (read-only)
	srcOpts := &pebble.Options{
		ReadOnly: true,
	}
	src, err := pebble.Open(sourceDB, srcOpts)
	if err != nil {
		return fmt.Errorf("failed to open source database: %w", err)
	}
	defer src.Close()

	// Open destination database
	dst, err := pebble.Open(destDB, &pebble.Options{})
	if err != nil {
		return fmt.Errorf("failed to open destination database: %w", err)
	}
	defer dst.Close()

	// Create an iterator to scan all keys
	iter, err := src.NewIter(&pebble.IterOptions{})
	if err != nil {
		return fmt.Errorf("failed to create iterator: %w", err)
	}
	defer iter.Close()

	batch := dst.NewBatch()
	count := 0
	headerCount := 0
	bodyCount := 0
	receiptCount := 0
	canonicalCount := 0

	for iter.First(); iter.Valid(); iter.Next() {
		key := iter.Key()
		value := iter.Value()

		// Copy key-value to destination
		if err := batch.Set(key, value, nil); err != nil {
			return fmt.Errorf("failed to set key: %w", err)
		}

		// Track what type of data we're copying
		if len(key) > 0 {
			switch key[0] {
			case 'h': // header prefix
				headerCount++
			case 'b': // body prefix
				bodyCount++
			case 'r': // receipt prefix
				receiptCount++
			case 'H': // canonical hash prefix
				canonicalCount++
			}
		}

		count++

		// Commit batch every 10000 entries
		if count%10000 == 0 {
			if err := batch.Commit(nil); err != nil {
				return fmt.Errorf("failed to commit batch: %w", err)
			}
			batch = dst.NewBatch()
			i.app.Log.Info("Import progress",
				"total", count,
				"headers", headerCount,
				"bodies", bodyCount,
				"receipts", receiptCount,
				"canonical", canonicalCount)
		}
	}

	// Commit final batch
	if err := batch.Commit(nil); err != nil {
		return fmt.Errorf("failed to commit final batch: %w", err)
	}

	if err := iter.Error(); err != nil {
		return fmt.Errorf("iterator error: %w", err)
	}

	i.app.Log.Info("Import complete",
		"total", count,
		"headers", headerCount,
		"bodies", bodyCount,
		"receipts", receiptCount,
		"canonical", canonicalCount)

	return nil
}