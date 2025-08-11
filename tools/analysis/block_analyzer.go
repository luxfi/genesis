package main

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"path/filepath"
	
	"github.com/dgraph-io/badger/v4"
	"github.com/cockroachdb/pebble"
)

// Analyzes blocks in both BadgerDB and PebbleDB formats
// Useful for verifying migration completeness

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Block Analyzer - Analyze blockchain database")
		fmt.Println()
		fmt.Println("Usage: block_analyzer <db_path> [format]")
		fmt.Println("  format: 'badger' or 'pebble' (auto-detect if not specified)")
		os.Exit(1)
	}
	
	dbPath := os.Args[1]
	dbFormat := ""
	if len(os.Args) > 2 {
		dbFormat = os.Args[2]
	}
	
	// Auto-detect format
	if dbFormat == "" {
		if _, err := os.Stat(filepath.Join(dbPath, "MANIFEST")); err == nil {
			dbFormat = "badger"
		} else if _, err := os.Stat(filepath.Join(dbPath, "CURRENT")); err == nil {
			dbFormat = "pebble"
		} else {
			log.Fatal("Cannot determine database format")
		}
	}
	
	fmt.Printf("Database: %s\n", dbPath)
	fmt.Printf("Format: %s\n", dbFormat)
	fmt.Println()
	
	switch dbFormat {
	case "badger":
		analyzeBadgerDB(dbPath)
	case "pebble":
		analyzePebbleDB(dbPath)
	default:
		log.Fatal("Unknown format: ", dbFormat)
	}
}

func analyzeBadgerDB(dbPath string) {
	opts := badger.DefaultOptions(dbPath)
	opts.ReadOnly = true
	
	db, err := badger.Open(opts)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	
	fmt.Println("=== BadgerDB Analysis ===")
	
	blockCount := 0
	var genesisHash string
	var highestBlock uint64
	
	err = db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()
		
		// Check for 'h' prefix (migrated format)
		prefix := []byte("h")
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			key := item.Key()
			
			if len(key) == 41 {
				blockNum := binary.BigEndian.Uint64(key[1:9])
				hash := hex.EncodeToString(key[9:41])
				
				if blockNum == 0 {
					genesisHash = hash
				}
				if blockNum > highestBlock {
					highestBlock = blockNum
				}
				blockCount++
				
				if blockCount <= 5 || blockNum == highestBlock {
					fmt.Printf("  Block %d: 0x%s\n", blockNum, hash)
				}
			}
		}
		
		// Also check 'H' prefix (standard format)
		if blockCount == 0 {
			prefix = []byte("H")
			for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
				item := it.Item()
				key := item.Key()
				
				if len(key) == 9 {
					blockNum := binary.BigEndian.Uint64(key[1:9])
					val, _ := item.ValueCopy(nil)
					hash := hex.EncodeToString(val)
					
					if blockNum == 0 {
						genesisHash = hash
					}
					if blockNum > highestBlock {
						highestBlock = blockNum
					}
					blockCount++
					
					if blockCount <= 5 || blockNum == highestBlock {
						fmt.Printf("  Block %d: 0x%s\n", blockNum, hash)
					}
				}
			}
		}
		
		return nil
	})
	
	if err != nil {
		log.Fatal(err)
	}
	
	fmt.Println()
	fmt.Println("Summary:")
	fmt.Printf("  Total blocks: %d\n", blockCount)
	fmt.Printf("  Genesis: 0x%s\n", genesisHash)
	fmt.Printf("  Highest block: %d\n", highestBlock)
	
	// Check for gaps
	if blockCount > 0 && blockCount != int(highestBlock+1) {
		fmt.Printf("  WARNING: Gaps detected! Expected %d blocks\n", highestBlock+1)
	}
}

func analyzePebbleDB(dbPath string) {
	db, err := pebble.Open(dbPath, &pebble.Options{ReadOnly: true})
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	
	fmt.Println("=== PebbleDB Analysis ===")
	
	blockCount := 0
	var genesisHash string
	var highestBlock uint64
	
	// Iterate through keys
	iter := db.NewIter(&pebble.IterOptions{})
	defer iter.Close()
	
	for iter.First(); iter.Valid(); iter.Next() {
		key := iter.Key()
		
		// Check for canonical block mappings
		if len(key) > 9 && key[0] == 'H' {
			blockNum := binary.BigEndian.Uint64(key[1:9])
			val := iter.Value()
			hash := hex.EncodeToString(val)
			
			if blockNum == 0 {
				genesisHash = hash
			}
			if blockNum > highestBlock {
				highestBlock = blockNum
			}
			blockCount++
			
			if blockCount <= 5 || blockNum == highestBlock {
				fmt.Printf("  Block %d: 0x%s\n", blockNum, hash)
			}
		}
	}
	
	fmt.Println()
	fmt.Println("Summary:")
	fmt.Printf("  Total blocks: %d\n", blockCount)
	fmt.Printf("  Genesis: 0x%s\n", genesisHash)
	fmt.Printf("  Highest block: %d\n", highestBlock)
}