package main

import (
	"encoding/hex"
	"fmt"
	"log"
	
	"github.com/dgraph-io/badger/v4"
)

func main() {
	// Open the ethdb database
	ethdbPath := "/Users/z/.luxd/network-96369/chains/EWi9aPkGe6EfJ3SobCAmSUXRPLa4brF3cThwPwmHTrD1y13jy/ethdb"
	
	db, err := badger.Open(badger.DefaultOptions(ethdbPath))
	if err != nil {
		log.Fatal("Failed to open ethdb:", err)
	}
	defer db.Close()
	
	fmt.Println("Analyzing database key types...")
	fmt.Println("================================")
	
	keyTypes := make(map[string]int)
	samples := make(map[string][]string)
	
	err = db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchSize = 10
		it := txn.NewIterator(opts)
		defer it.Close()
		
		count := 0
		for it.Rewind(); it.Valid() && count < 1000000; it.Next() {
			item := it.Item()
			key := item.Key()
			count++
			
			// Categorize by prefix
			var keyType string
			if len(key) > 0 {
				prefix := key[0]
				switch prefix {
				case 'H':
					keyType = "H - Canonical hash"
				case 'h':
					if len(key) == 41 {
						keyType = "h - Header (41 bytes)"
					} else {
						keyType = fmt.Sprintf("h - Other (%d bytes)", len(key))
					}
				case 'b':
					if len(key) == 41 {
						keyType = "b - Body (41 bytes)"
					} else {
						keyType = fmt.Sprintf("b - Other (%d bytes)", len(key))
					}
				case 'r':
					if len(key) == 41 {
						keyType = "r - Receipt (41 bytes)"
					} else {
						keyType = fmt.Sprintf("r - Other (%d bytes)", len(key))
					}
				case 't':
					keyType = "t - Transaction"
				case 'l':
					keyType = "l - Transaction lookup"
				case 'B':
					keyType = "B - Block height"
				case 0x26:
					keyType = "0x26 - Account leaf"
				case 0xa3:
					keyType = "0xa3 - Storage"
				default:
					if len(key) == 32 {
						keyType = "32-byte hash key"
					} else if len(key) < 10 {
						keyType = fmt.Sprintf("%s - Short key", hex.EncodeToString(key))
					} else {
						keyType = fmt.Sprintf("0x%02x - Unknown (%d bytes)", prefix, len(key))
					}
				}
			} else {
				keyType = "Empty key"
			}
			
			keyTypes[keyType]++
			
			// Store samples
			if len(samples[keyType]) < 3 {
				sample := hex.EncodeToString(key)
				if len(sample) > 100 {
					sample = sample[:100] + "..."
				}
				samples[keyType] = append(samples[keyType], sample)
			}
			
			if count%100000 == 0 {
				fmt.Printf("Processed %d keys...\n", count)
			}
		}
		
		return nil
	})
	
	if err != nil {
		log.Fatal("Error analyzing database:", err)
	}
	
	fmt.Println("\nKey Type Summary:")
	fmt.Println("-----------------")
	for keyType, count := range keyTypes {
		fmt.Printf("%s: %d\n", keyType, count)
		if sampleKeys, exists := samples[keyType]; exists {
			for _, sample := range sampleKeys {
				fmt.Printf("  Sample: %s\n", sample)
			}
		}
	}
}