package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"log"
	
	"github.com/cockroachdb/pebble"
	"github.com/luxfi/geth/common"
	"github.com/luxfi/geth/core/types"
	"github.com/luxfi/geth/rlp"
)

// SubnetEVM namespace prefix (32 bytes)
var subnetNamespace = []byte{
	0x33, 0x7f, 0xb7, 0x3f, 0x9b, 0xcd, 0xac, 0x8c,
	0x31, 0xa2, 0xd5, 0xf7, 0xb8, 0x77, 0xab, 0x1e,
	0x8a, 0x2b, 0x7f, 0x2a, 0x1e, 0x9b, 0xf0, 0x2a,
	0x0a, 0x0e, 0x6c, 0x6f, 0xd1, 0x64, 0xf1, 0xd1,
}

func main() {
	sourcePath := "/Users/z/work/lux/state/chaindata/lux-mainnet-96369/db/pebbledb"
	
	fmt.Println("Scanning for headers...")
	
	// Open source PebbleDB
	db, err := pebble.Open(sourcePath, &pebble.Options{
		ReadOnly: true,
	})
	if err != nil {
		log.Fatal("Failed to open database:", err)
	}
	defer db.Close()
	
	// Create iterator
	iter, err := db.NewIter(nil)
	if err != nil {
		log.Fatal("Failed to create iterator:", err)
	}
	defer iter.Close()
	
	headerCount := 0
	samples := 0
	maxBlock := uint64(0)
	
	for iter.First(); iter.Valid(); iter.Next() {
		key := iter.Key()
		val := iter.Value()
		
		// Look for headers - they should be 73 bytes (32 ns + 'h' + 8 num + 32 hash)
		if len(key) == 73 && bytes.HasPrefix(key, subnetNamespace) && key[32] == 'h' {
			num := binary.BigEndian.Uint64(key[33:41])
			hash := common.BytesToHash(key[41:73])
			
			// Try to decode as header
			var header types.Header
			if err := rlp.DecodeBytes(val, &header); err == nil {
				headerCount++
				
				if num > maxBlock {
					maxBlock = num
				}
				
				if samples < 10 || num == 1082780 {
					fmt.Printf("Header %d: hash=%s, size=%d bytes\n", num, hash.Hex()[:16], len(val))
					samples++
				}
			}
		}
		
		// Also check for different key formats
		if len(key) == 41 && key[0] == 'h' {
			// Non-namespaced header
			num := binary.BigEndian.Uint64(key[1:9])
			hash := common.BytesToHash(key[9:41])
			
			var header types.Header
			if err := rlp.DecodeBytes(val, &header); err == nil {
				fmt.Printf("Found non-namespaced header %d: hash=%s\n", num, hash.Hex()[:16])
			}
		}
	}
	
	fmt.Printf("\nTotal headers found: %d\n", headerCount)
	fmt.Printf("Max block: %d\n", maxBlock)
}