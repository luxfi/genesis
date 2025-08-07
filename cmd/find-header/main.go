package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"os"

	"github.com/cockroachdb/pebble"
	"github.com/luxfi/geth/common"
	"github.com/luxfi/geth/core/types"
	"github.com/luxfi/geth/rlp"
)

func main() {
	var dbPath string
	var hashStr string

	flag.StringVar(&dbPath, "db", "", "Path to database")
	flag.StringVar(&hashStr, "hash", "", "Block hash to find")
	flag.Parse()

	if dbPath == "" || hashStr == "" {
		fmt.Println("Usage: find-header -db /path/to/db -hash 0x3f4fa2...")
		os.Exit(1)
	}

	// Parse hash
	hash := common.HexToHash(hashStr)
	fmt.Printf("Looking for header with hash: %s\n", hash.Hex())

	// Open database
	opts := &pebble.Options{
		ReadOnly: true,
	}
	
	db, err := pebble.Open(dbPath, opts)
	if err != nil {
		fmt.Printf("Failed to open database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	// Search for keys containing the hash
	iter, err := db.NewIter(&pebble.IterOptions{
		LowerBound: []byte{0x33},
		UpperBound: []byte{0x34},
	})
	if err != nil {
		fmt.Printf("Failed to create iterator: %v\n", err)
		os.Exit(1)
	}
	defer iter.Close()

	hashBytes := hash.Bytes()
	found := 0

	for iter.First(); iter.Valid(); iter.Next() {
		key := iter.Key()
		value := iter.Value()

		// Check if key contains our hash
		for i := 0; i <= len(key)-32; i++ {
			if string(key[i:i+32]) == string(hashBytes) {
				fmt.Printf("\nFound key containing hash:\n")
				fmt.Printf("  Key: %s\n", hex.EncodeToString(key))
				fmt.Printf("  Key length: %d\n", len(key))
				fmt.Printf("  Value length: %d\n", len(value))
				
				// Try to decode as header if value is large enough
				if len(value) > 400 {
					var header types.Header
					if err := rlp.DecodeBytes(value, &header); err == nil {
						fmt.Printf("  -> This is a HEADER! Block #%d\n", header.Number.Uint64())
						found++
					}
				}
				
				// Show key structure
				if len(key) >= 33 {
					fmt.Printf("  Key[32]: 0x%02x ('%c')\n", key[32], key[32])
				}
			}
		}
	}

	fmt.Printf("\nFound %d headers\n", found)
}