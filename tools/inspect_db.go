package main

import (
	"encoding/hex"
	"fmt"
	"log"
	"os"

	"github.com/dgraph-io/badger/v4"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Println("Usage: inspect_db <badgerdb_path>")
		os.Exit(1)
	}

	dbPath := os.Args[1]
	fmt.Printf("Inspecting database: %s\n\n", dbPath)

	// Open BadgerDB
	opts := badger.DefaultOptions(dbPath)
	opts.Logger = nil
	db, err := badger.Open(opts)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Count different key types
	stats := map[string]int{
		"header":          0, // h + num + hash
		"canonical":       0, // h + num + n
		"hash_to_number":  0, // H + hash
		"body":            0, // b + num + hash
		"receipt":         0, // r + num + hash
		"td":              0, // h + num + hash + t
		"unknown":         0,
		"total":           0,
	}

	// Sample some keys
	fmt.Println("First 20 keys in database:")
	count := 0
	
	err = db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = false
		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			key := it.Item().Key()
			stats["total"]++
			
			// Classify key
			if len(key) > 0 {
				switch key[0] {
				case 'h': // header prefix
					if len(key) == 41 {
						stats["header"]++
					} else if len(key) == 10 && key[9] == 'n' {
						stats["canonical"]++
					} else if len(key) == 42 && key[41] == 't' {
						stats["td"]++
					}
				case 'H': // hash to number
					if len(key) == 33 {
						stats["hash_to_number"]++
					}
				case 'b': // body
					if len(key) == 41 {
						stats["body"]++
					}
				case 'r': // receipt
					if len(key) == 41 {
						stats["receipt"]++
					}
				default:
					stats["unknown"]++
				}
			}
			
			// Show first few keys
			if count < 20 {
				keyHex := hex.EncodeToString(key)
				valueSize := it.Item().ValueSize()
				
				// Decode key type
				keyType := "?"
				if len(key) > 0 {
					switch key[0] {
					case 'h':
						if len(key) == 41 {
							keyType = "header"
						} else if len(key) == 10 && key[9] == 'n' {
							keyType = "canonical"
						} else if len(key) == 42 && key[41] == 't' {
							keyType = "td"
						}
					case 'H':
						keyType = "hash->num"
					case 'b':
						keyType = "body"
					case 'r':
						keyType = "receipt"
					}
				}
				
				fmt.Printf("  [%s] %s (val: %d bytes)\n", keyType, keyHex, valueSize)
				count++
			}
		}
		return nil
	})

	if err != nil {
		log.Fatalf("Failed to iterate: %v", err)
	}

	fmt.Println("\nDatabase Statistics:")
	fmt.Println("====================")
	for k, v := range stats {
		if k != "total" && k != "unknown" {
			fmt.Printf("  %-15s: %d\n", k, v)
		}
	}
	fmt.Printf("  %-15s: %d\n", "unknown", stats["unknown"])
	fmt.Printf("  %-15s: %d\n", "TOTAL", stats["total"])
	
	if stats["header"] > 0 && stats["canonical"] == 0 {
		fmt.Println("\n⚠️  WARNING: Database has headers but no canonical mappings!")
		fmt.Println("This will prevent the chain from loading properly.")
		fmt.Println("Need to rebuild canonical chain mappings.")
	}
}