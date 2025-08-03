package main

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"os"

	"github.com/luxfi/database/pebbledb"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run check-canonical-hash.go <db-path>")
		return
	}

	// Open database using Lux's pebbledb wrapper
	db, err := pebbledb.New(os.Args[1], 0, 0, "", false)
	if err != nil {
		panic(err)
	}
	defer db.Close()

	// Check for canonical hash at height 1082780
	canonicalKey := make([]byte, 10)
	canonicalKey[0] = 'h'
	binary.BigEndian.PutUint64(canonicalKey[1:9], 1082780)
	canonicalKey[9] = 'n'
	
	fmt.Printf("Looking for canonical key: %s\n", hex.EncodeToString(canonicalKey))
	if hashBytes, err := db.Get(canonicalKey); err == nil {
		fmt.Printf("Found canonical hash at 1082780: %s\n", hex.EncodeToString(hashBytes))
	} else {
		fmt.Printf("Canonical hash at 1082780 not found: %v\n", err)
		
		// Check if we have blocks with 9-byte format instead
		canonicalKey9 := make([]byte, 9)
		canonicalKey9[0] = 'h'
		binary.BigEndian.PutUint64(canonicalKey9[1:], 1082780)
		
		fmt.Printf("\nTrying 9-byte format: %s\n", hex.EncodeToString(canonicalKey9))
		if hashBytes, err := db.Get(canonicalKey9); err == nil {
			fmt.Printf("Found canonical hash with 9-byte key: %s\n", hex.EncodeToString(hashBytes))
		}
	}

	// Scan for any canonical keys near the end
	iter := db.NewIterator(nil)
	defer iter.Close()

	fmt.Println("\nScanning for high block numbers...")
	highBlocks := map[uint64]string{}
	for iter.First(); iter.Valid(); iter.Next() {
		key := iter.Key()
		if len(key) == 10 && key[0] == 'h' && key[9] == 'n' {
			blockNum := binary.BigEndian.Uint64(key[1:9])
			if blockNum > 1082700 {
				highBlocks[blockNum] = hex.EncodeToString(iter.Value())
			}
		} else if len(key) == 9 && key[0] == 'h' {
			blockNum := binary.BigEndian.Uint64(key[1:])
			if blockNum > 1082700 {
				fmt.Printf("Found 9-byte canonical at %d: %s\n", blockNum, hex.EncodeToString(iter.Value()))
			}
		}
	}

	for num, hash := range highBlocks {
		fmt.Printf("Block %d: %s\n", num, hash)
	}
}