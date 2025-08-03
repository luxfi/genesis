// +build ignore

package main

import (
	"encoding/hex"
	"fmt"
	"log"
	"os"

	"github.com/cockroachdb/pebble"
	"github.com/luxfi/geth/rlp"
)

// Example: Inspect a blockchain database
func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run database_inspect.go <database-path>")
		os.Exit(1)
	}

	dbPath := os.Args[1]

	// Open database
	db, err := pebble.Open(dbPath, &pebble.Options{ReadOnly: true})
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	fmt.Printf("Inspecting database: %s\n\n", dbPath)

	// Count total keys
	totalKeys := 0
	keyTypes := make(map[string]int)

	// Iterate through all keys
	it := db.NewIter(nil)
	defer it.Close()

	for it.First(); it.Valid(); it.Next() {
		key := it.Key()
		totalKeys++

		// Categorize keys by prefix
		if len(key) > 0 {
			prefix := hex.EncodeToString(key[:1])
			keyTypes[prefix]++
		}

		// Show first 10 keys as examples
		if totalKeys <= 10 {
			fmt.Printf("Key %d: %x\n", totalKeys, key)
			
			// Try to decode value for known key types
			value := it.Value()
			if len(key) == 33 && key[0] == 'h' { // Block hash to number
				var blockNum uint64
				if err := rlp.DecodeBytes(value, &blockNum); err == nil {
					fmt.Printf("  Block Number: %d\n", blockNum)
				}
			} else if len(key) == 9 && key[0] == 'H' { // Block number to hash
				fmt.Printf("  Block Hash: %x\n", value)
			}
		}
	}

	if err := it.Error(); err != nil {
		log.Fatalf("Iterator error: %v", err)
	}

	// Print summary
	fmt.Printf("\nDatabase Summary:\n")
	fmt.Printf("Total Keys: %d\n", totalKeys)
	fmt.Printf("\nKey Types by Prefix:\n")
	
	// Common prefixes in blockchain databases
	prefixes := map[string]string{
		"48": "H - Block number to hash",
		"68": "h - Block hash to number", 
		"62": "b - Block body",
		"74": "t - Transaction",
		"72": "r - Receipt",
		"6c": "l - Transaction lookup",
		"73": "s - Account snapshot",
		"63": "c - Code",
		"41": "A - Account trie",
		"53": "S - Storage trie",
	}

	for prefix, count := range keyTypes {
		description := prefixes[prefix]
		if description == "" {
			description = "Unknown"
		}
		fmt.Printf("  %s: %d keys (%s)\n", prefix, count, description)
	}

	// Try to find the latest block
	fmt.Printf("\nSearching for latest block...\n")
	
	// LastBlock key
	lastBlockKey := []byte("LastBlock")
	value, closer, err := db.Get(lastBlockKey)
	if err == nil && closer != nil {
		defer closer.Close()
		var blockNum uint64
		if err := rlp.DecodeBytes(value, &blockNum); err == nil {
			fmt.Printf("Latest Block: %d\n", blockNum)
		}
	}

	// Alternative: scan for highest block number
	var highestBlock uint64
	it2 := db.NewIter(nil)
	defer it2.Close()

	for it2.First(); it2.Valid(); it2.Next() {
		key := it2.Key()
		// Block number to hash keys (prefix 'H')
		if len(key) == 9 && key[0] == 'H' {
			blockNum := uint64(0)
			for i := 1; i < 9; i++ {
				blockNum = (blockNum << 8) | uint64(key[i])
			}
			if blockNum > highestBlock {
				highestBlock = blockNum
			}
		}
	}

	if highestBlock > 0 {
		fmt.Printf("Highest Block Found: %d\n", highestBlock)
	}
}