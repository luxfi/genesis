package main

import (
	"encoding/binary"
	"encoding/hex"
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

	// Check canonical hash at block 0
	canonicalKey := make([]byte, 9)
	canonicalKey[0] = 'H'
	// Block number 0 as 8 bytes big endian (all zeros)

	fmt.Printf("Looking for canonical hash at block 0 with key: 0x%x\n", canonicalKey)
	
	val, closer, err := db.Get(canonicalKey)
	if err == nil {
		fmt.Printf("Found canonical hash: 0x%s\n", hex.EncodeToString(val))
		closer.Close()
	} else {
		fmt.Printf("No canonical hash found: %v\n", err)
	}

	// Try iterating to see what keys exist
	fmt.Println("\nFirst 10 keys in database:")
	iter, err := db.NewIter(&pebble.IterOptions{})
	if err != nil {
		log.Fatal("Failed to create iterator:", err)
	}
	defer iter.Close()

	count := 0
	for iter.First(); iter.Valid() && count < 10; iter.Next() {
		key := iter.Key()
		fmt.Printf("Key: 0x%x", key)
		if len(key) > 0 && key[0] == 'H' {
			// Decode block number for canonical keys
			if len(key) == 9 {
				blockNum := binary.BigEndian.Uint64(key[1:])
				fmt.Printf(" (canonical block %d)", blockNum)
			}
		}
		fmt.Printf(" val_len=%d\n", len(iter.Value()))
		count++
	}
}