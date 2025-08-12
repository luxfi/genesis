package main

import (
	"fmt"
	"log"
	"os"

	"github.com/cockroachdb/pebble"
	"github.com/dgraph-io/badger/v4"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Println("Usage: pebble_to_badger <source_pebbledb> <dest_badgerdb>")
		os.Exit(1)
	}

	sourceDB := os.Args[1]
	destDB := os.Args[2]

	fmt.Printf("Migrating from PebbleDB to BadgerDB...\n")
	fmt.Printf("  Source: %s\n", sourceDB)
	fmt.Printf("  Destination: %s\n", destDB)

	// Open source PebbleDB
	src, err := pebble.Open(sourceDB, &pebble.Options{ReadOnly: true})
	if err != nil {
		log.Fatalf("Failed to open source database: %v", err)
	}
	defer src.Close()

	// Open destination BadgerDB
	opts := badger.DefaultOptions(destDB)
	opts.Logger = nil // Quiet mode
	dst, err := badger.Open(opts)
	if err != nil {
		log.Fatalf("Failed to open destination database: %v", err)
	}
	defer dst.Close()

	// Create iterator
	iter, err := src.NewIter(&pebble.IterOptions{})
	if err != nil {
		log.Fatalf("Failed to create iterator: %v", err)
	}
	defer iter.Close()

	// Migrate data in batches
	batch := dst.NewWriteBatch()
	count := 0
	totalBytes := int64(0)

	for iter.First(); iter.Valid(); iter.Next() {
		key := append([]byte(nil), iter.Key()...)
		val, err := iter.ValueAndErr()
		if err != nil {
			log.Fatalf("Failed to read value: %v", err)
		}
		value := append([]byte(nil), val...)

		if err := batch.Set(key, value); err != nil {
			log.Fatalf("Failed to set key: %v", err)
		}

		count++
		totalBytes += int64(len(key) + len(value))

		// Flush batch periodically
		if count%10000 == 0 {
			if err := batch.Flush(); err != nil {
				log.Fatalf("Failed to flush batch: %v", err)
			}
			batch = dst.NewWriteBatch()
			fmt.Printf("  Migrated %d keys (%d MB)...\n", count, totalBytes/1024/1024)
		}
	}

	// Flush final batch
	if err := batch.Flush(); err != nil {
		log.Fatalf("Failed to flush final batch: %v", err)
	}

	fmt.Printf("\n✓ Migration complete!\n")
	fmt.Printf("  Total keys: %d\n", count)
	fmt.Printf("  Total size: %d MB\n", totalBytes/1024/1024)
}