package main

import (
	"encoding/binary"
	"fmt"
	"path/filepath"
	
	"github.com/luxfi/database/badgerdb"
)

func main() {
	fmt.Println("=== Simple Database Check ===")
	
	// Open the migrated database
	ethdbPath := "/Users/z/.luxd/network-96369/chains/EWi9aPkGe6EfJ3SobCAmSUXRPLa4brF3cThwPwmHTrD1y13jy/ethdb"
	db, err := badgerdb.New(filepath.Clean(ethdbPath), nil, "", nil)
	if err != nil {
		panic(fmt.Sprintf("Failed to open ethdb: %v", err))
	}
	defer db.Close()
	
	// Count entries by prefix
	prefixes := make(map[byte]int)
	total := 0
	
	it := db.NewIterator(nil, nil)
	defer it.Release()
	
	for it.Next() {
		key := it.Key()
		if len(key) > 0 {
			prefixes[key[0]]++
			total++
			
			// Show some example keys
			if total <= 10 {
				fmt.Printf("Example key: %x (len=%d)\n", key[:min(20, len(key))], len(key))
			}
		}
		
		if total >= 100000 {
			break // Sample enough
		}
	}
	
	fmt.Printf("\nTotal entries sampled: %d\n", total)
	fmt.Printf("\nKey prefixes found:\n")
	for prefix, count := range prefixes {
		fmt.Printf("  '%c' (0x%02x): %d entries\n", prefix, prefix, count)
	}
	
	// Check specific important keys
	fmt.Println("\n--- Checking Critical Keys ---")
	
	// Check LastHeader
	if val, err := db.Get([]byte("LastHeader")); err == nil {
		fmt.Printf("✓ LastHeader: %x\n", val[:8])
	} else {
		fmt.Printf("✗ LastHeader not found\n")
	}
	
	// Check LastBlock
	if val, err := db.Get([]byte("LastBlock")); err == nil {
		fmt.Printf("✓ LastBlock: %x\n", val[:8])
	} else {
		fmt.Printf("✗ LastBlock not found\n")
	}
	
	// Check canonical at different heights
	heights := []uint64{0, 1000000, 1082780}
	for _, height := range heights {
		canonKey := make([]byte, 10)
		canonKey[0] = 'h'
		binary.BigEndian.PutUint64(canonKey[1:9], height)
		canonKey[9] = 'n'
		
		if val, err := db.Get(canonKey); err == nil {
			fmt.Printf("✓ Canonical hash at %d: %x...\n", height, val[:8])
		} else {
			fmt.Printf("✗ No canonical hash at %d\n", height)
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}