package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"log"
	
	"github.com/cockroachdb/pebble"
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
	
	fmt.Println("Finding headers by looking up canonical hashes...")
	
	// Open source PebbleDB
	db, err := pebble.Open(sourcePath, &pebble.Options{
		ReadOnly: true,
	})
	if err != nil {
		log.Fatal("Failed to open database:", err)
	}
	defer db.Close()
	
	// Try to read headers for specific blocks by looking up their hashes first
	testBlocks := []uint64{0, 1, 100, 1000, 10000, 100000, 1000000, 1082780}
	
	for _, blockNum := range testBlocks {
		// First get the canonical hash for this block number
		// Key format: namespace + 'H' + hash(32) -> value is block number
		// But we need the reverse: block number -> hash
		// Actually, let's scan for H entries and build a map
		
		fmt.Printf("\nLooking for block %d...\n", blockNum)
		
		// Scan for the hash->number mapping
		iter, _ := db.NewIter(nil)
		defer iter.Close()
		
		var blockHash []byte
		found := false
		
		for iter.First(); iter.Valid(); iter.Next() {
			key := iter.Key()
			val := iter.Value()
			
			// Look for H entries (hash->number mapping)
			if len(key) == 65 && bytes.HasPrefix(key, subnetNamespace) && key[32] == 'H' {
				// Value should be 8-byte block number
				if len(val) == 8 {
					num := binary.BigEndian.Uint64(val)
					if num == blockNum {
						blockHash = key[33:65] // The hash
						found = true
						fmt.Printf("  Found hash for block %d: %x\n", blockNum, blockHash[:8])
						break
					}
				}
			}
		}
		
		if found {
			// Now try to read the header
			// Key: namespace + 'h' + num(8) + hash(32)
			headerKey := make([]byte, 73)
			copy(headerKey[:32], subnetNamespace)
			headerKey[32] = 'h'
			binary.BigEndian.PutUint64(headerKey[33:41], blockNum)
			copy(headerKey[41:], blockHash)
			
			if headerVal, closer, err := db.Get(headerKey); err == nil {
				defer closer.Close()
				
				var header types.Header
				if err := rlp.DecodeBytes(headerVal, &header); err == nil {
					fmt.Printf("  ✓ Header found: Number=%s, ParentHash=%s\n", 
						header.Number, header.ParentHash.Hex()[:16])
				} else {
					fmt.Printf("  Header data exists but failed to decode: %v\n", err)
				}
			} else {
				// Try without block number (just hash)
				headerKey2 := make([]byte, 65)
				copy(headerKey2[:32], subnetNamespace)
				headerKey2[32] = 'h'
				copy(headerKey2[33:], blockHash)
				
				if headerVal, closer, err := db.Get(headerKey2); err == nil {
					defer closer.Close()
					fmt.Printf("  Header found with alternate key format (size=%d)\n", len(headerVal))
				} else {
					fmt.Printf("  Header not found\n")
				}
			}
		}
	}
}