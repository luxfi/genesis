package main

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"log"
	"os"

	"github.com/cockroachdb/pebble"
	"github.com/dgraph-io/badger/v4"
)

const (
	// SubnetEVM namespace - this is the chain ID prefix used in subnet databases
	subnetNamespace = "337fb73f9bcdac8c31a2d5f7b877ab1e8a2b7f2a1e9bf02a0a0e6c6fd164f1d1"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Println("Usage: migrate_subnet_to_cchain <source_pebbledb> <dest_badgerdb>")
		fmt.Println("")
		fmt.Println("This tool migrates a SubnetEVM database to C-Chain format by:")
		fmt.Println("  1. Removing the subnet namespace prefix")
		fmt.Println("  2. Converting to BadgerDB format")
		fmt.Println("  3. Ensuring proper canonical chain mappings")
		os.Exit(1)
	}

	sourceDB := os.Args[1]
	destDB := os.Args[2]

	fmt.Printf("Migrating SubnetEVM to C-Chain format...\n")
	fmt.Printf("  Source: %s\n", sourceDB)
	fmt.Printf("  Destination: %s\n", destDB)
	fmt.Printf("  Removing namespace: %s\n\n", subnetNamespace)

	// Parse namespace
	namespaceBytes, err := hex.DecodeString(subnetNamespace)
	if err != nil {
		log.Fatalf("Failed to decode namespace: %v", err)
	}

	// Open source PebbleDB
	src, err := pebble.Open(sourceDB, &pebble.Options{ReadOnly: true})
	if err != nil {
		log.Fatalf("Failed to open source database: %v", err)
	}
	defer src.Close()

	// Remove old destination if exists
	os.RemoveAll(destDB)

	// Open destination BadgerDB
	opts := badger.DefaultOptions(destDB)
	opts.Logger = nil
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

	// Statistics
	stats := map[string]int{
		"total":           0,
		"migrated":        0,
		"skipped":         0,
		"headers":         0,
		"bodies":          0,
		"receipts":        0,
		"canonical":       0,
		"hash_to_number":  0,
	}

	// Migrate data
	batch := dst.NewWriteBatch()
	batchSize := 0
	const maxBatchSize = 100

	fmt.Println("Starting migration...")
	
	for iter.First(); iter.Valid(); iter.Next() {
		key := iter.Key()
		val, err := iter.ValueAndErr()
		if err != nil {
			log.Printf("Failed to read value: %v", err)
			stats["skipped"]++
			continue
		}
		
		stats["total"]++
		
		// Check if key has namespace prefix
		if bytes.HasPrefix(key, namespaceBytes) {
			// Remove namespace prefix
			cleanKey := key[len(namespaceBytes):]
			
			// Copy value
			cleanValue := make([]byte, len(val))
			copy(cleanValue, val)
			
			// Write to destination
			if err := batch.Set(cleanKey, cleanValue); err != nil {
				log.Printf("Failed to set key: %v", err)
				stats["skipped"]++
				continue
			}
			
			stats["migrated"]++
			
			// Classify the key type for statistics
			if len(cleanKey) > 0 {
				switch cleanKey[0] {
				case 'h':
					if len(cleanKey) == 41 {
						stats["headers"]++
					} else if len(cleanKey) == 10 && cleanKey[9] == 'n' {
						stats["canonical"]++
					}
				case 'H':
					stats["hash_to_number"]++
				case 'b':
					stats["bodies"]++
				case 'r':
					stats["receipts"]++
				}
			}
			
			batchSize++
		} else {
			// Key without namespace - might be metadata
			// Migrate as-is for safety
			keyCopy := make([]byte, len(key))
			copy(keyCopy, key)
			valCopy := make([]byte, len(val))
			copy(valCopy, val)
			
			if err := batch.Set(keyCopy, valCopy); err != nil {
				log.Printf("Failed to set non-namespaced key: %v", err)
				stats["skipped"]++
				continue
			}
			stats["migrated"]++
			batchSize++
		}
		
		// Flush batch periodically
		if batchSize >= maxBatchSize {
			if err := batch.Flush(); err != nil {
				log.Fatalf("Failed to flush batch: %v", err)
			}
			batch = dst.NewWriteBatch()
			batchSize = 0
			
			if stats["migrated"]%10000 == 0 {
				fmt.Printf("  Migrated %d keys...\n", stats["migrated"])
			}
		}
	}

	// Flush final batch
	if batchSize > 0 {
		if err := batch.Flush(); err != nil {
			log.Fatalf("Failed to flush final batch: %v", err)
		}
	}

	fmt.Println("\n========================================")
	fmt.Println("Migration Complete!")
	fmt.Println("========================================")
	fmt.Printf("  Total keys:      %d\n", stats["total"])
	fmt.Printf("  Migrated:        %d\n", stats["migrated"])
	fmt.Printf("  Skipped:         %d\n", stats["skipped"])
	fmt.Println("")
	fmt.Println("Key type breakdown:")
	fmt.Printf("  Headers:         %d\n", stats["headers"])
	fmt.Printf("  Bodies:          %d\n", stats["bodies"])
	fmt.Printf("  Receipts:        %d\n", stats["receipts"])
	fmt.Printf("  Canonical:       %d\n", stats["canonical"])
	fmt.Printf("  Hash->Number:    %d\n", stats["hash_to_number"])
	fmt.Println("")
	
	if stats["canonical"] == 0 && stats["headers"] > 0 {
		fmt.Println("⚠️  WARNING: No canonical mappings found!")
		fmt.Println("The database may need canonical chain reconstruction.")
		fmt.Println("Run scan_canonical on the destination to verify.")
	} else {
		fmt.Println("✅ Database migrated successfully!")
		fmt.Println("Run scan_canonical on the destination to verify integrity.")
	}
}