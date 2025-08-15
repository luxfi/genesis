package main

import (
	"encoding/binary"
	"fmt"
	"path/filepath"

	"github.com/luxfi/database/badgerdb"
	"github.com/luxfi/geth/common"
)

func main() {
	fmt.Println("╔════════════════════════════════════════════════════════╗")
	fmt.Println("║      VERIFYING FULL-HISTORY CHAIN MIGRATION           ║")
	fmt.Println("╚════════════════════════════════════════════════════════╝")
	fmt.Println()

	// Open the migrated database
	dbPath := "/Users/z/.luxd/network-96369/chains/EWi9aPkGe6EfJ3SobCAmSUXRPLa4brF3cThwPwmHTrD1y13jy/ethdb"
	fmt.Printf("Opening database: %s\n", dbPath)
	
	db, err := badgerdb.New(filepath.Clean(dbPath), nil, "", nil)
	if err != nil {
		panic(fmt.Sprintf("Failed to open database: %v", err))
	}
	defer db.Close()

	fmt.Println("\n🔍 Checking key database markers...")
	
	// Check LastHeader
	if val, err := db.Get([]byte("LastHeader")); err == nil {
		var hash common.Hash
		copy(hash[:], val)
		fmt.Printf("  ✓ LastHeader: %s\n", hash.Hex())
	} else {
		fmt.Printf("  ✗ LastHeader: not found\n")
	}

	// Check genesis (block 0 canonical)
	canonKey := make([]byte, 10)
	canonKey[0] = 'h'
	binary.BigEndian.PutUint64(canonKey[1:9], 0)
	canonKey[9] = 'n'
	
	if val, err := db.Get(canonKey); err == nil {
		var hash common.Hash
		copy(hash[:], val)
		fmt.Printf("  ✓ Genesis canonical: %s\n", hash.Hex())
		
		// Try to read genesis header
		headerKey := append([]byte{'h'}, append(make([]byte, 8), hash[:]...)...)
		binary.BigEndian.PutUint64(headerKey[1:9], 0)
		
		if hdr, err := db.Get(headerKey); err == nil {
			fmt.Printf("  ✓ Genesis header: %d bytes\n", len(hdr))
		} else {
			fmt.Printf("  ✗ Genesis header: not found\n")
		}
	} else {
		fmt.Printf("  ✗ Genesis canonical: not found\n")
	}

	// Check tip (block 1082780)
	fmt.Println("\n🔍 Checking tip block (1082780)...")
	tipCanonKey := make([]byte, 10)
	tipCanonKey[0] = 'h'
	binary.BigEndian.PutUint64(tipCanonKey[1:9], 1082780)
	tipCanonKey[9] = 'n'
	
	if val, err := db.Get(tipCanonKey); err == nil {
		var hash common.Hash
		copy(hash[:], val)
		fmt.Printf("  ✓ Block 1082780 canonical: %s\n", hash.Hex())
		
		// Try to read header
		headerKey := append([]byte{'h'}, append(make([]byte, 8), hash[:]...)...)
		binary.BigEndian.PutUint64(headerKey[1:9], 1082780)
		
		if hdr, err := db.Get(headerKey); err == nil {
			fmt.Printf("  ✓ Block 1082780 header: %d bytes\n", len(hdr))
		}
		
		// Try to read body
		bodyKey := append([]byte{'b'}, append(make([]byte, 8), hash[:]...)...)
		binary.BigEndian.PutUint64(bodyKey[1:9], 1082780)
		
		if body, err := db.Get(bodyKey); err == nil {
			fmt.Printf("  ✓ Block 1082780 body: %d bytes\n", len(body))
		}
	} else {
		fmt.Printf("  ✗ Block 1082780: not found\n")
	}

	// Check some intermediate blocks
	fmt.Println("\n🔍 Checking sample blocks...")
	samples := []uint64{1, 1000, 10000, 100000, 500000, 1000000}
	
	for _, num := range samples {
		canonKey := make([]byte, 10)
		canonKey[0] = 'h'
		binary.BigEndian.PutUint64(canonKey[1:9], num)
		canonKey[9] = 'n'
		
		if val, err := db.Get(canonKey); err == nil {
			var hash common.Hash
			copy(hash[:], val)
			fmt.Printf("  ✓ Block %d: %s\n", num, hash.Hex())
		} else {
			fmt.Printf("  ✗ Block %d: not found\n", num)
		}
	}

	// Check chain config
	fmt.Println("\n🔍 Checking chain configuration...")
	if config, err := db.Get([]byte("ethereum-chain-config")); err == nil {
		fmt.Printf("  ✓ Chain config: %d bytes\n", len(config))
		// Show first 100 chars
		if len(config) > 100 {
			fmt.Printf("    Preview: %s...\n", string(config[:100]))
		}
	} else {
		fmt.Printf("  ✗ Chain config: not found\n")
	}

	// Count total keys by prefix
	fmt.Println("\n📊 Counting keys by type...")
	counts := map[string]int{
		"headers":   0,
		"bodies":    0,
		"receipts":  0,
		"canonical": 0,
		"hash2num":  0,
		"state":     0,
		"other":     0,
	}

	// Create iterator
	iter := db.NewIterator()
	defer iter.Release()
	
	total := 0
	for iter.Next() {
		key := iter.Key()
		if len(key) > 0 {
			switch key[0] {
			case 'h':
				if len(key) == 41 {
					counts["headers"]++
				} else if len(key) == 10 && key[9] == 'n' {
					counts["canonical"]++
				} else {
					counts["other"]++
				}
			case 'H':
				counts["hash2num"]++
			case 'b':
				counts["bodies"]++
			case 'r':
				counts["receipts"]++
			case 's', 'S', 'a', 'c':
				counts["state"]++
			default:
				counts["other"]++
			}
			total++
		}
		
		if total%100000 == 0 && total > 0 {
			fmt.Printf("\r  Counted %d keys...", total)
		}
	}
	
	fmt.Printf("\r  Total keys: %d\n", total)
	fmt.Printf("    Headers:   %d\n", counts["headers"])
	fmt.Printf("    Bodies:    %d\n", counts["bodies"])
	fmt.Printf("    Receipts:  %d\n", counts["receipts"])
	fmt.Printf("    Canonical: %d\n", counts["canonical"])
	fmt.Printf("    Hash→Num:  %d\n", counts["hash2num"])
	fmt.Printf("    State:     %d\n", counts["state"])
	fmt.Printf("    Other:     %d\n", counts["other"])

	fmt.Println("\n✅ Verification complete!")
	fmt.Println("\nNext steps to boot the node:")
	fmt.Println("  1. Start luxd with: luxd --network-id=96369")
	fmt.Println("  2. Test RPC at: http://localhost:9630")
}