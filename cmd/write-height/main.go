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

	// Write the highest block number
	highestBlock := uint64(1082780)
	heightBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(heightBytes, highestBlock)
	
	// Write to LastBlock key (what the VM expects)
	if err := db.Set([]byte("LastBlock"), heightBytes, nil); err != nil {
		log.Fatal("Failed to write LastBlock:", err)
	}
	
	fmt.Printf("Wrote LastBlock height: %d\n", highestBlock)
	
	// Also get the hash at this height and write it
	canonicalKey := make([]byte, 9)
	canonicalKey[0] = 'H'
	binary.BigEndian.PutUint64(canonicalKey[1:], highestBlock)
	
	if val, closer, err := db.Get(canonicalKey); err == nil && len(val) == 32 {
		// Write the hash as LastHash
		if err := db.Set([]byte("LastHash"), val, nil); err != nil {
			log.Fatal("Failed to write LastHash:", err)
		}
		fmt.Printf("Wrote LastHash: 0x%x\n", val)
		closer.Close()
	}
	
	// Sync the database
	if err := db.Close(); err != nil {
		log.Fatal("Failed to close database:", err)
	}
	
	fmt.Println("Database updated successfully")
}