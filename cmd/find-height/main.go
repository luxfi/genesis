package main

import (
	"encoding/binary"
	"fmt"
	"log"

	"github.com/cockroachdb/pebble"
)

func main() {
	// Open the database
	db, err := pebble.Open("/home/z/work/lux/genesis/state/chaindata/lux-mainnet-96369/db", &pebble.Options{})
	if err != nil {
		log.Fatal("Failed to open database:", err)
	}
	defer db.Close()

	// Find the highest canonical block
	maxHeight := uint64(0)
	
	// Iterate through all keys to find canonical hashes
	iter, err := db.NewIter(&pebble.IterOptions{})
	if err != nil {
		log.Fatal("Failed to create iterator:", err)
	}
	defer iter.Close()

	fmt.Println("Scanning for highest canonical block...")
	count := 0
	for iter.First(); iter.Valid(); iter.Next() {
		key := iter.Key()
		// Canonical hash keys are 9 bytes: 'H' + 8-byte block number
		if len(key) == 9 && key[0] == 'H' {
			blockNum := binary.BigEndian.Uint64(key[1:])
			if blockNum > maxHeight {
				maxHeight = blockNum
			}
			count++
			if count % 100000 == 0 {
				fmt.Printf("Checked %d canonical blocks, highest so far: %d\n", count, maxHeight)
			}
		}
	}

	fmt.Printf("\nTotal canonical blocks found: %d\n", count)
	fmt.Printf("Highest block number: %d\n", maxHeight)
	
	// Verify we can read the canonical hash at the highest block
	canonicalKey := make([]byte, 9)
	canonicalKey[0] = 'H'
	binary.BigEndian.PutUint64(canonicalKey[1:], maxHeight)
	
	if val, closer, err := db.Get(canonicalKey); err == nil && len(val) == 32 {
		fmt.Printf("Canonical hash at block %d: 0x%x\n", maxHeight, val)
		closer.Close()
	}
}