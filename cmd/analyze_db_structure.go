package main

import (
	"encoding/hex"
	"fmt"
	"log"
	
	"github.com/cockroachdb/pebble"
)

func main() {
	dbPath := "/Users/z/work/lux/state/chaindata/lux-mainnet-96369/db/pebbledb"
	
	db, err := pebble.Open(dbPath, &pebble.Options{ReadOnly: true})
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	
	// Known namespace
	namespace, _ := hex.DecodeString("337fb73f9bcdac8c31a2d5f7b877ab1e8a2b7f2a1e9bf02a0a0e6c6fd164f1d1")
	
	// Let's analyze the first header key we found
	// 337fb73f9bcdac8c31a2d5f7b877ab1e8a2b7f2a1e9bf02a0a0e6c6fd164f1d1 0000006c3a436500b20c0c80f5dae66e1233d84da4ddd5af2987cfdb1562eb9f
	// namespace (32 bytes) + suffix (32 bytes)
	
	fmt.Println("Database structure analysis:")
	fmt.Println("Namespace:", hex.EncodeToString(namespace))
	
	// Count different key patterns
	patterns := make(map[string]int)
	samples := make(map[string][]string)
	
	it, _ := db.NewIter(&pebble.IterOptions{})
	defer it.Close()
	
	count := 0
	for it.First(); it.Valid() && count < 10000; it.Next() {
		k := it.Key()
		v := it.Value()
		
		if len(k) == 64 && bytesEqual(k[:32], namespace) {
			suffix := k[32:]
			pattern := identifyPattern(suffix, v)
			patterns[pattern]++
			
			if len(samples[pattern]) < 3 {
				samples[pattern] = append(samples[pattern], fmt.Sprintf("%x", suffix))
			}
		}
		count++
	}
	
	fmt.Printf("\nAnalyzed %d keys\n", count)
	fmt.Println("\nKey patterns found:")
	for pattern, cnt := range patterns {
		fmt.Printf("  %s: %d occurrences\n", pattern, cnt)
		if samps, ok := samples[pattern]; ok {
			for _, s := range samps {
				fmt.Printf("    Sample: %s\n", s)
			}
		}
	}
	
	// Now let's find the actual latest block
	fmt.Println("\n=== Finding latest block ===")
	
	// The keys appear to be namespace + 32-byte hash
	// Let's look for patterns that might indicate block height
	maxNum := uint64(0)
	var maxHash []byte
	
	it2, _ := db.NewIter(&pebble.IterOptions{})
	defer it2.Close()
	
	for it2.First(); it2.Valid(); it2.Next() {
		k := it2.Key()
		v := it2.Value()
		
		if len(k) == 64 && bytesEqual(k[:32], namespace) && len(v) > 200 && v[0] == 0xf9 {
			// This looks like a header
			// Try to extract block number from RLP
			// Block number is usually around offset 0x100-0x120 in the RLP
			if num := extractBlockNumber(v); num > 0 {
				if num > maxNum {
					maxNum = num
					maxHash = k[32:]
					if num > 1082700 && num < 1082800 {
						fmt.Printf("Found block %d with hash %x\n", num, maxHash)
					}
				}
			}
		}
	}
	
	fmt.Printf("\nLatest block found: %d\n", maxNum)
	if maxHash != nil {
		fmt.Printf("Hash: %x\n", maxHash)
	}
}

func identifyPattern(suffix, value []byte) string {
	// Check value characteristics
	if len(value) > 200 && len(value) < 600 && value[0] == 0xf9 {
		return "header"
	}
	if len(value) > 50 && value[0] >= 0xf8 && value[0] <= 0xfa {
		return "body_or_receipts"
	}
	if len(value) == 32 {
		return "hash_mapping"
	}
	if len(value) == 8 {
		return "number_mapping"
	}
	if len(value) < 50 && value[0] >= 0xc0 {
		return "small_rlp"
	}
	return fmt.Sprintf("other_%d_bytes", len(value))
}

func extractBlockNumber(headerRLP []byte) uint64 {
	// Simple heuristic: look for the block number in common positions
	// Block number is typically around offset 0x100-0x120
	for offset := 0x100; offset < len(headerRLP)-8 && offset < 0x130; offset++ {
		// Look for patterns that might be block numbers
		if headerRLP[offset] == 0x83 || headerRLP[offset] == 0x84 {
			// This might be RLP encoding of a 3 or 4 byte integer
			numLen := int(headerRLP[offset] - 0x80)
			if offset+1+numLen <= len(headerRLP) {
				num := uint64(0)
				for i := 0; i < numLen && i < 8; i++ {
					num = (num << 8) | uint64(headerRLP[offset+1+i])
				}
				// Check if this looks like a reasonable block number
				if num > 1000000 && num < 2000000 {
					return num
				}
			}
		}
	}
	return 0
}

func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}