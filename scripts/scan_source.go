package main

import (
	"encoding/hex"
	"fmt"
	"log"
	
	"github.com/luxfi/geth/ethdb/pebble"
)

func main() {
	sourcePath := "/Users/z/work/lux/genesis/state/chaindata/lux-genesis-7777/db"
	
	fmt.Printf("Scanning source database: %s\n", sourcePath)
	
	// Open source PebbleDB
	sourceDB, err := pebble.New(sourcePath, 0, 0, "", true)
	if err != nil {
		log.Fatal("Failed to open source PebbleDB:", err)
	}
	defer sourceDB.Close()
	
	// Create categories
	categories := make(map[byte]int)
	samples := make(map[byte][]string)
	
	// Scan first 1000 keys
	it := sourceDB.NewIterator(nil, nil)
	defer it.Release()
	
	count := 0
	for it.Next() && count < 1000 {
		key := it.Key()
		if len(key) > 0 {
			firstByte := key[0]
			categories[firstByte]++
			
			// Save samples
			if len(samples[firstByte]) < 3 {
				keyHex := hex.EncodeToString(key)
				if len(keyHex) > 60 {
					keyHex = keyHex[:60] + "..."
				}
				samples[firstByte] = append(samples[firstByte], keyHex)
			}
		}
		count++
	}
	
	if it.Error() != nil {
		log.Fatal("Iterator error:", it.Error())
	}
	
	fmt.Printf("\nScanned %d keys\n", count)
	fmt.Println("\nKey categories found:")
	
	for b, cnt := range categories {
		fmt.Printf("  0x%02x ('%c'): %d keys\n", b, b, cnt)
		if samps, ok := samples[b]; ok {
			for _, s := range samps {
				fmt.Printf("    Sample: %s\n", s)
			}
		}
	}
	
	// Try specific keys
	fmt.Println("\nTrying specific keys:")
	
	// Try canonical hash key for block 0
	testKeys := [][]byte{
		[]byte("H"), // With just H
		append([]byte("H"), 0, 0, 0, 0, 0, 0, 0, 0), // H + 8 bytes
		[]byte("h"), // header prefix
		[]byte("LastBlock"),
		[]byte("lastBlock"),
		[]byte("LastHeader"),
	}
	
	for _, key := range testKeys {
		val, err := sourceDB.Get(key)
		if err == nil {
			fmt.Printf("  Found key %x: value length %d\n", key, len(val))
			if len(val) <= 32 {
				fmt.Printf("    Value: %x\n", val)
			}
		}
	}
}