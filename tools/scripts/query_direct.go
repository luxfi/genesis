package main

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"log"
	"math/big"

	"github.com/dgraph-io/badger/v4"
)

func main() {
	dbPath := "/Users/z/.luxd/network-96369/chains/EWi9aPkGe6EfJ3SobCAmSUXRPLa4brF3cThwPwmHTrD1y13jy/ethdb"
	
	fmt.Println("Direct Database Query")
	fmt.Println("=====================")
	fmt.Printf("Database: %s\n\n", dbPath)
	
	// Open BadgerDB
	db, err := badger.Open(badger.DefaultOptions(dbPath).WithReadOnly(true))
	if err != nil {
		log.Fatal("Failed to open database:", err)
	}
	defer db.Close()
	
	// Count entries by prefix
	prefixCounts := make(map[string]int)
	totalCount := 0
	
	err = db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = false
		it := txn.NewIterator(opts)
		defer it.Close()
		
		for it.Rewind(); it.Valid(); it.Next() {
			key := it.Item().Key()
			totalCount++
			
			if len(key) > 0 {
				prefix := string(key[0])
				prefixCounts[prefix]++
			}
			
			// Sample some important addresses
			if totalCount <= 10 {
				fmt.Printf("Sample key %d: %x (len=%d)\n", totalCount, key[:min(32, len(key))], len(key))
			}
		}
		return nil
	})
	
	if err != nil {
		log.Fatal("Failed to iterate database:", err)
	}
	
	fmt.Printf("\nTotal entries: %d\n", totalCount)
	fmt.Println("\nEntries by prefix:")
	for prefix, count := range prefixCounts {
		fmt.Printf("  '%s' (0x%x): %d entries\n", prefix, []byte(prefix)[0], count)
	}
	
	// Check for specific account addresses
	fmt.Println("\n=== Checking Account Data ===")
	
	addresses := []string{
		"9011E888251AB053B7bD1cdB598Db4f9DEd94714", // luxdefi.eth
		"EAbCC110fAcBfebabC66Ad6f9E7B67288e720B59",
	}
	
	for _, addrHex := range addresses {
		fmt.Printf("\nAddress: 0x%s\n", addrHex)
		
		addr, _ := hex.DecodeString(addrHex)
		
		// Try different key formats
		keys := [][]byte{
			append([]byte("a"), addr...), // Snapshot account
			append([]byte("A"), addr...), // Account trie
			addr,                          // Direct
		}
		
		found := false
		err = db.View(func(txn *badger.Txn) error {
			for i, key := range keys {
				item, err := txn.Get(key)
				if err == nil {
					val, _ := item.ValueCopy(nil)
					fmt.Printf("  Found with format %d: %d bytes of data\n", i, len(val))
					if len(val) > 0 {
						fmt.Printf("  First 32 bytes: %x\n", val[:min(32, len(val))])
					}
					found = true
				}
			}
			return nil
		})
		
		if !found {
			fmt.Printf("  Not found in database\n")
		}
	}
	
	// Check for block at height 1082780
	fmt.Println("\n=== Checking Block 1082780 ===")
	
	targetBlock := uint64(1082780)
	canonicalKey := make([]byte, 9)
	canonicalKey[0] = 'H'
	binary.BigEndian.PutUint64(canonicalKey[1:], targetBlock)
	
	err = db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(canonicalKey)
		if err != nil {
			fmt.Printf("Canonical hash not found for block %d\n", targetBlock)
			
			// Try to find ANY canonical block
			fmt.Println("\nSearching for any canonical blocks...")
			opts := badger.DefaultIteratorOptions
			opts.PrefetchValues = true
			opts.Prefix = []byte{'H'}
			it := txn.NewIterator(opts)
			defer it.Close()
			
			count := 0
			for it.Rewind(); it.Valid() && count < 10; it.Next() {
				key := it.Item().Key()
				if len(key) == 9 {
					blockNum := binary.BigEndian.Uint64(key[1:])
					val, _ := it.Item().ValueCopy(nil)
					fmt.Printf("  Found canonical block %d: hash=%x\n", blockNum, val[:min(8, len(val))])
					count++
				}
			}
			
			if count == 0 {
				fmt.Println("  No canonical blocks found!")
			}
			
			return nil
		}
		
		hash, _ := item.ValueCopy(nil)
		fmt.Printf("Found canonical hash for block %d: %x\n", targetBlock, hash)
		
		// Check for header
		headerKey := make([]byte, 41)
		headerKey[0] = 'h'
		binary.BigEndian.PutUint64(headerKey[1:9], targetBlock)
		copy(headerKey[9:41], hash)
		
		if item2, err := txn.Get(headerKey); err == nil {
			headerData, _ := item2.ValueCopy(nil)
			fmt.Printf("Header found: %d bytes\n", len(headerData))
		} else {
			fmt.Printf("Header not found\n")
		}
		
		return nil
	})
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func formatBalance(balance *big.Int) string {
	if balance == nil {
		return "0"
	}
	
	// Convert from wei to LUX (18 decimals)
	ether := new(big.Float).SetInt(balance)
	divisor := new(big.Float).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil))
	ether.Quo(ether, divisor)
	
	return fmt.Sprintf("%s", ether.Text('f', 6))
}