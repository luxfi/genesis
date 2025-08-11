package main

import (
	"bytes"
	"fmt"
	"github.com/cockroachdb/pebble"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"
)

func main() {
	dbPath := "/home/z/work/lux/state/chaindata/lux-mainnet-96369/db/pebbledb"
	
	fmt.Printf("Opening database at: %s\n", dbPath)
	
	opts := &pebble.Options{
		ReadOnly: true,
	}
	
	db, err := pebble.Open(dbPath, opts)
	if err != nil {
		fmt.Printf("Failed to open database: %v\n", err)
		return
	}
	defer db.Close()
	
	namespace := []byte{
		0x33, 0x7f, 0xb7, 0x3f, 0x9b, 0xcd, 0xac, 0x8c,
		0x31, 0xa2, 0xd5, 0xf7, 0xb8, 0x77, 0xab, 0x1e,
		0x8a, 0x2b, 0x7f, 0x2a, 0x1e, 0x9b, 0xf0, 0x2a,
		0x0a, 0x0e, 0x6c, 0x6f, 0xd1, 0x64, 0xf1, 0xd1,
	}
	
	// Create iterator
	iter, _ := db.NewIter(nil)
	defer iter.Close()
	
	found := 0
	rlpHeaders := 0
	
	fmt.Println("Inspecting header values...")
	
	for iter.First(); iter.Valid() && found < 100; iter.Next() {
		key := iter.Key()
		
		// Look for header keys: namespace (32 bytes) + hash (32 bytes)
		if len(key) == 64 && bytes.HasPrefix(key, namespace) {
			found++
			value := iter.Value()
			
			hash := key[32:]
			blockNum := uint64(hash[0])<<16 | uint64(hash[1])<<8 | uint64(hash[2])
			
			fmt.Printf("\n=== Header %d ===\n", found)
			fmt.Printf("Hash: %x\n", hash[:8])
			fmt.Printf("Encoded block num: %d\n", blockNum)
			fmt.Printf("Value length: %d\n", len(value))
			fmt.Printf("First 20 bytes: %x\n", value[:min(20, len(value))])
			
			// Try to decode as RLP header
			var header types.Header
			if err := rlp.DecodeBytes(value, &header); err == nil {
				rlpHeaders++
				fmt.Printf("✅ Valid RLP header! Block number: %d\n", header.Number.Uint64())
			} else {
				fmt.Printf("❌ NOT RLP header: %v\n", err)
				
				// Check if it's just the hash (32 bytes)
				if len(value) == 32 {
					fmt.Printf("Value is 32 bytes - might be a hash: %x\n", value)
				}
			}
		}
	}
	
	fmt.Printf("\n=== SUMMARY ===\n")
	fmt.Printf("Inspected %d headers\n", found)
	fmt.Printf("Valid RLP headers: %d\n", rlpHeaders)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}