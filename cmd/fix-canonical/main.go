package main

import (
	"encoding/binary"
	"fmt"
	"log"

	"github.com/cockroachdb/pebble"
	"github.com/ethereum/go-ethereum/common"
)

// canonicalKey returns the standard C-chain canonical key format:
//   "H" + blockNumber (8 bytes)
func canonicalKey(number uint64) []byte {
	key := make([]byte, 9)
	key[0] = 'H'
	binary.BigEndian.PutUint64(key[1:], number)
	return key
}

// headerHashKey returns the geth format key for canonical hash
func headerHashKey(number uint64) []byte {
	return append(append([]byte("h"), encodeBlockNumber(number)...), []byte("n")...)
}

func encodeBlockNumber(number uint64) []byte {
	enc := make([]byte, 8)
	binary.BigEndian.PutUint64(enc, number)
	return enc
}

func main() {
	// Open the database
	db, err := pebble.Open("/home/z/work/lux/genesis/state/chaindata/lux-mainnet-96369/db", &pebble.Options{})
	if err != nil {
		log.Fatal("Failed to open database:", err)
	}
	defer db.Close()

	// Read the canonical hash at block 0 from our format
	key := canonicalKey(0)
	val, closer, err := db.Get(key)
	if err != nil {
		log.Fatal("Failed to read canonical hash at block 0:", err)
	}
	defer closer.Close()

	if len(val) != 32 {
		log.Fatal("Invalid canonical hash length:", len(val))
	}

	var hash common.Hash
	copy(hash[:], val)
	fmt.Printf("Found canonical hash at block 0: %s\n", hash.Hex())

	// Write it in geth's expected format
	gethKey := headerHashKey(0)
	if err := db.Set(gethKey, val, nil); err != nil {
		log.Fatal("Failed to write geth canonical key:", err)
	}

	fmt.Printf("Wrote canonical hash in geth format at key: %x\n", gethKey)

	// Also check higher blocks
	for i := uint64(1); i <= 10; i++ {
		key := canonicalKey(i)
		if val, closer, err := db.Get(key); err == nil && len(val) == 32 {
			gethKey := headerHashKey(i)
			if err := db.Set(gethKey, val, nil); err != nil {
				fmt.Printf("Failed to write geth canonical key for block %d: %v\n", i, err)
			} else {
				fmt.Printf("Wrote canonical hash for block %d\n", i)
			}
			closer.Close()
		}
	}

	// Also write the head block hash pointers
	// Get the highest block
	highestBlock := uint64(1082780)
	key = canonicalKey(highestBlock)
	if val, closer, err := db.Get(key); err == nil && len(val) == 32 {
		// Write head pointers
		if err := db.Set([]byte("LastHeader"), val, nil); err == nil {
			fmt.Printf("Wrote LastHeader: %x\n", val)
		}
		if err := db.Set([]byte("LastBlock"), val, nil); err == nil {
			fmt.Printf("Wrote LastBlock: %x\n", val)
		}
		if err := db.Set([]byte("LastFast"), val, nil); err == nil {
			fmt.Printf("Wrote LastFast: %x\n", val)
		}
		closer.Close()
	}

	fmt.Println("Database updated successfully")
}