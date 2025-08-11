package main

import (
	"encoding/binary"
	"fmt"
	"log"
	"math/big"

	"github.com/cockroachdb/pebble"
	"github.com/luxfi/geth/common"
	"github.com/luxfi/geth/rlp"
)

func main() {
	dbPath := "/home/z/.luxd/network-96369/chains/X6CU5qgMJfzsTB9UWxj2ZY5hd68x41HfZ4m4hCBWbHuj1Ebc3/ethdb"
	
	fmt.Printf("=== Checking Migration Status ===\n")
	fmt.Printf("Database: %s\n\n", dbPath)
	
	// Open database
	opts := &pebble.Options{
		Cache:        pebble.NewCache(256 << 20),
		MaxOpenFiles: 1024,
		ReadOnly:     true,
	}
	
	db, err := pebble.Open(dbPath, opts)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()
	
	// Check canonical blocks
	fmt.Println("1. Checking canonical blocks...")
	var lastCanonical uint64
	canonicalCount := 0
	
	for n := uint64(0); n <= 1082780; n++ {
		key := append(append([]byte("h"), encodeBlockNumber(n)...), byte('n'))
		if val, closer, err := db.Get(key); err == nil {
			canonicalCount++
			lastCanonical = n
			if n%100000 == 0 || n == 0 || n == 1082780 {
				hash := common.BytesToHash(val)
				fmt.Printf("  Block %d: %s\n", n, hash.Hex())
			}
			closer.Close()
		}
	}
	
	fmt.Printf("  Found %d canonical blocks (last: %d)\n", canonicalCount, lastCanonical)
	
	// Check heads
	fmt.Println("\n2. Checking head pointers...")
	checkHead := func(name string, key []byte) {
		if val, closer, err := db.Get(key); err == nil {
			hash := common.BytesToHash(val)
			fmt.Printf("  %s: %s\n", name, hash.Hex())
			closer.Close()
		} else {
			fmt.Printf("  %s: NOT SET\n", name)
		}
	}
	
	checkHead("LastHeader", []byte("LastHeader"))
	checkHead("LastBlock", []byte("LastBlock"))
	checkHead("LastFast", []byte("LastFast"))
	
	// Check TD at key heights
	fmt.Println("\n3. Checking Total Difficulty...")
	checkTD := func(n uint64) {
		// Get canonical hash
		canonKey := append(append([]byte("h"), encodeBlockNumber(n)...), byte('n'))
		if hashBytes, closer, err := db.Get(canonKey); err == nil {
			hash := common.BytesToHash(hashBytes)
			closer.Close()
			
			// Get TD
			tdKey := append(append(append([]byte("h"), encodeBlockNumber(n)...), hash.Bytes()...), byte('t'))
			if tdBytes, closer, err := db.Get(tdKey); err == nil {
				var td big.Int
				if err := rlp.DecodeBytes(tdBytes, &td); err == nil {
					expectedTD := new(big.Int).SetUint64(n + 1)
					if td.Cmp(expectedTD) == 0 {
						fmt.Printf("  Block %d TD: %v ✅\n", n, &td)
					} else {
						fmt.Printf("  Block %d TD: %v (expected %v) ⚠️\n", n, &td, expectedTD)
					}
				}
				closer.Close()
			} else {
				fmt.Printf("  Block %d TD: NOT FOUND\n", n)
			}
		} else {
			fmt.Printf("  Block %d: No canonical hash\n", n)
		}
	}
	
	checkTD(0)        // Genesis
	checkTD(500000)   // Mid-point
	checkTD(1082780)  // Tip
	
	// Count total keys
	fmt.Println("\n4. Counting keys by type...")
	keyTypes := make(map[string]int)
	iter, _ := db.NewIter(&pebble.IterOptions{})
	totalKeys := 0
	
	for iter.First(); iter.Valid() && totalKeys < 100000; iter.Next() {
		key := iter.Key()
		totalKeys++
		
		if len(key) > 0 {
			switch key[0] {
			case 'h':
				keyTypes["header-related"]++
			case 'H':
				keyTypes["hash-to-number"]++
			case 'b':
				keyTypes["bodies"]++
			case 'r':
				keyTypes["receipts"]++
			case 'c':
				keyTypes["code"]++
			case 'L':
				keyTypes["metadata"]++
			default:
				if len(key) == 32 {
					keyTypes["state-trie"]++
				} else {
					keyTypes["other"]++
				}
			}
		}
	}
	iter.Close()
	
	fmt.Printf("  Total keys sampled: %d\n", totalKeys)
	for typ, count := range keyTypes {
		fmt.Printf("  %s: %d\n", typ, count)
	}
	
	fmt.Println("\n✅ Migration check complete!")
}

func encodeBlockNumber(number uint64) []byte {
	enc := make([]byte, 8)
	binary.BigEndian.PutUint64(enc, number)
	return enc
}