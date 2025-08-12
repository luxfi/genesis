package main

import (
	"encoding/hex"
	"fmt"
	"log"
	"os"

	"github.com/cockroachdb/pebble"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Println("Usage: inspect_pebble <pebbledb_path>")
		os.Exit(1)
	}

	dbPath := os.Args[1]
	fmt.Printf("Inspecting PebbleDB: %s\n\n", dbPath)

	// Open PebbleDB
	db, err := pebble.Open(dbPath, &pebble.Options{ReadOnly: true})
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Create iterator
	iter, err := db.NewIter(&pebble.IterOptions{})
	if err != nil {
		log.Fatalf("Failed to create iterator: %v", err)
	}
	defer iter.Close()

	fmt.Println("First 30 keys in original database:")
	count := 0
	
	for iter.First(); iter.Valid() && count < 30; iter.Next() {
		key := iter.Key()
		val, err := iter.ValueAndErr()
		if err != nil {
			log.Printf("Error reading value: %v", err)
			continue
		}
		
		keyHex := hex.EncodeToString(key)
		fmt.Printf("  Key[%d]: %s (val: %d bytes)\n", count, keyHex, len(val))
		
		// If it looks like a prefixed key, show the breakdown
		if len(key) > 32 {
			fmt.Printf("    -> Prefix: %s\n", hex.EncodeToString(key[:32]))
			fmt.Printf("    -> Suffix: %s\n", hex.EncodeToString(key[32:]))
		}
		
		count++
	}
	
	fmt.Printf("\nTotal keys shown: %d\n", count)
}