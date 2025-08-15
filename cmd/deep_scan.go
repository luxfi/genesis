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
	
	fmt.Println("Deep database scan...")
	
	// Count all keys
	totalKeys := 0
	namespacedKeys := 0
	headerKeys := 0
	bodyKeys := 0
	otherKeys := 0
	
	// Track block numbers found
	blockNumbers := make(map[uint64]bool)
	
	it, _ := db.NewIter(&pebble.IterOptions{})
	defer it.Close()
	
	for it.First(); it.Valid(); it.Next() {
		k := it.Key()
		v := it.Value()
		totalKeys++
		
		if len(k) == 64 && bytesEqual(k[:32], namespace) {
			namespacedKeys++
			
			// Check value type
			if len(v) > 200 && len(v) < 600 && v[0] == 0xf9 {
				headerKeys++
				
				// Extract block number
				if num := extractBlockNumber(v); num > 0 {
					blockNumbers[num] = true
				}
			} else if len(v) > 50 && (v[0] == 0xf8 || v[0] == 0xf9 || v[0] == 0xfa) {
				bodyKeys++
			} else {
				otherKeys++
			}
		}
		
		if totalKeys%100000 == 0 {
			fmt.Printf("Scanned %d keys...\n", totalKeys)
		}
	}
	
	fmt.Printf("\n=== SCAN RESULTS ===\n")
	fmt.Printf("Total keys: %d\n", totalKeys)
	fmt.Printf("Namespaced keys: %d\n", namespacedKeys)
	fmt.Printf("  Headers: %d\n", headerKeys)
	fmt.Printf("  Bodies/Receipts: %d\n", bodyKeys)
	fmt.Printf("  Other: %d\n", otherKeys)
	fmt.Printf("Unique block numbers: %d\n", len(blockNumbers))
	
	// Find gaps in block numbers
	if len(blockNumbers) > 0 {
		minBlock := uint64(^uint64(0))
		maxBlock := uint64(0)
		
		for num := range blockNumbers {
			if num < minBlock {
				minBlock = num
			}
			if num > maxBlock {
				maxBlock = num
			}
		}
		
		fmt.Printf("\nBlock range: %d to %d\n", minBlock, maxBlock)
		
		// Check for gaps
		gaps := []string{}
		for i := minBlock; i <= maxBlock && i <= minBlock+1000; i++ {
			if !blockNumbers[i] {
				gaps = append(gaps, fmt.Sprintf("%d", i))
				if len(gaps) >= 10 {
					gaps = append(gaps, "...")
					break
				}
			}
		}
		
		if len(gaps) > 0 {
			fmt.Printf("Missing blocks in first 1000: %v\n", gaps)
		}
	}
	
	// Let's also check what the keys actually look like
	fmt.Println("\n=== KEY STRUCTURE ANALYSIS ===")
	fmt.Println("First 10 namespaced keys:")
	
	it2, _ := db.NewIter(&pebble.IterOptions{})
	defer it2.Close()
	
	count := 0
	for it2.First(); it2.Valid() && count < 10; it2.Next() {
		k := it2.Key()
		if len(k) == 64 && bytesEqual(k[:32], namespace) {
			suffix := k[32:]
			fmt.Printf("%d. Suffix: %x\n", count+1, suffix)
			
			// Check if this looks like it could be a block number + hash
			// The suffix is 32 bytes
			count++
		}
	}
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

func extractBlockNumber(headerRLP []byte) uint64 {
	// Look for block number in RLP header
	// The block number is typically around offset 0x100-0x120
	for offset := 0x100; offset < len(headerRLP)-8 && offset < 0x130; offset++ {
		if headerRLP[offset] >= 0x83 && headerRLP[offset] <= 0x87 {
			numLen := int(headerRLP[offset] - 0x80)
			if offset+1+numLen <= len(headerRLP) && numLen <= 8 {
				num := uint64(0)
				for i := 0; i < numLen; i++ {
					num = (num << 8) | uint64(headerRLP[offset+1+i])
				}
				// Check if reasonable
				if num > 100000 && num < 10000000 {
					return num
				}
			}
		}
	}
	
	// Try alternate locations
	for offset := 0x80; offset < len(headerRLP)-8 && offset < 0x150; offset++ {
		if headerRLP[offset] == 0x83 {
			// 3-byte number
			if offset+4 <= len(headerRLP) {
				num := uint64(headerRLP[offset+1])<<16 | uint64(headerRLP[offset+2])<<8 | uint64(headerRLP[offset+3])
				if num > 100000 && num < 10000000 {
					return num
				}
			}
		}
	}
	
	return 0
}