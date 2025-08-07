package main

import (
	"encoding/binary"
	"encoding/hex"
	"flag"
	"fmt"
	"os"

	"github.com/cockroachdb/pebble"
	"github.com/luxfi/geth/common"
)

func main() {
	var dbPath string
	var blockNum uint64

	flag.StringVar(&dbPath, "db", "", "Path to database")
	flag.Uint64Var(&blockNum, "block", 0, "Block number to debug")
	flag.Parse()

	if dbPath == "" {
		fmt.Println("Usage: debug-subnet -db /path/to/db -block 0")
		os.Exit(1)
	}

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

	// Find the canonical hash for this block
	fmt.Printf("=== Debugging block %d ===\n", blockNum)
	
	// Look for all keys that might contain this block number
	iter, err := db.NewIter(&pebble.IterOptions{
		LowerBound: []byte{0x33},
		UpperBound: []byte{0x34},
	})
	if err != nil {
		fmt.Printf("Failed to create iterator: %v\n", err)
		os.Exit(1)
	}
	defer iter.Close()

	numBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(numBytes, blockNum)
	
	foundCanonical := false
	var blockHash common.Hash

	for iter.First(); iter.Valid(); iter.Next() {
		key := iter.Key()
		value := iter.Value()

		// Check if key contains our block number
		for i := 0; i < len(key)-8; i++ {
			if string(key[i:i+8]) == string(numBytes) {
				fmt.Printf("\nFound key containing block number %d:\n", blockNum)
				fmt.Printf("  Key: %s\n", hex.EncodeToString(key))
				fmt.Printf("  Key length: %d\n", len(key))
				fmt.Printf("  Value length: %d\n", len(value))
				
				// Analyze key structure
				if len(key) >= 42 {
					fmt.Printf("  Key[32]: 0x%02x ('%c')\n", key[32], key[32])
					if len(key) >= 42 && key[32] == 0x68 && key[41] == 0x6e && len(value) == 32 {
						blockHash = common.BytesToHash(value)
						foundCanonical = true
						fmt.Printf("  -> This is a canonical mapping! Hash: %s\n", blockHash.Hex())
					}
				}
			}
		}
	}

	if !foundCanonical {
		fmt.Printf("\nNo canonical mapping found for block %d\n", blockNum)
		return
	}

	// Now look for the header using the found hash
	fmt.Printf("\n=== Looking for header with hash %s ===\n", blockHash.Hex())
	
	// Try different key patterns
	patterns := []struct{
		name string
		build func() []byte
	}{
		{
			name: "Pattern 1: 33..68<num><hash>",
			build: func() []byte {
				key := make([]byte, 0, 74)
				key = append(key, 0x33)
				key = append(key, make([]byte, 31)...)
				key = append(key, 0x68)
				key = append(key, numBytes...)
				key = append(key, blockHash.Bytes()...)
				return key
			},
		},
		{
			name: "Pattern 2: 33..68<hash><num>",  
			build: func() []byte {
				key := make([]byte, 0, 74)
				key = append(key, 0x33)
				key = append(key, make([]byte, 31)...)
				key = append(key, 0x68)
				key = append(key, blockHash.Bytes()...)
				key = append(key, numBytes...)
				return key
			},
		},
	}

	for _, pattern := range patterns {
		key := pattern.build()
		fmt.Printf("\nTrying %s:\n", pattern.name)
		fmt.Printf("  Key: %s\n", hex.EncodeToString(key))
		
		if value, closer, err := db.Get(key); err == nil {
			defer closer.Close()
			fmt.Printf("  SUCCESS! Found header, %d bytes\n", len(value))
		} else {
			fmt.Printf("  Not found\n")
		}
	}
}