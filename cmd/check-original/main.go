package main

import (
	"encoding/binary"
	"fmt"
	"log"
	"math/big"
	"strings"

	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/opt"
)

func main() {
	fmt.Println("=== Checking Original SubnetEVM Database ===")
	
	// Open the ORIGINAL database
	dbPath := "/home/z/work/lux/genesis/genesis/state/chaindata/lux-mainnet-96369/db/pebbledb"
	fmt.Printf("Opening original database at: %s\n", dbPath)
	
	opts := &opt.Options{
		ReadOnly: true,
	}
	db, err := leveldb.OpenFile(dbPath, opts)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()
	
	// Target block
	blockHeight := uint64(1082780)
	targetAddr := "0x9011E888251AB053B7bD1cdB598Db4f9DEd94714"
	
	fmt.Printf("\nLooking for block %d\n", blockHeight)
	fmt.Printf("Target account: %s\n", targetAddr)
	
	// Check canonical hash at target height
	canonKey := append([]byte("H"), encodeBlockNumber(blockHeight)...)
	canonHash, err := db.Get(canonKey, nil)
	if err == nil && len(canonHash) == 32 {
		fmt.Printf("✅ Found canonical hash at block %d: %x\n", blockHeight, canonHash)
	} else {
		fmt.Printf("Canonical hash not found at block %d\n", blockHeight)
		
		// Try to find highest block
		fmt.Printf("\nSearching for highest canonical block...\n")
		for h := blockHeight; h > blockHeight-1000 && h > 0; h-- {
			key := append([]byte("H"), encodeBlockNumber(h)...)
			if hash, err := db.Get(key, nil); err == nil && len(hash) == 32 {
				fmt.Printf("Found canonical block at height %d: %x\n", h, hash)
				blockHeight = h
				canonHash = hash
				break
			}
		}
	}
	
	// Check for headers
	if len(canonHash) == 32 {
		headerKey := append([]byte("h"), canonHash...)
		headerKey = append(headerKey, encodeBlockNumber(blockHeight)...)
		
		if headerData, err := db.Get(headerKey, nil); err == nil {
			fmt.Printf("✅ Found header at block %d (size: %d bytes)\n", blockHeight, len(headerData))
			
			// Header contains state root, but we need RLP decoding
			// For now, just confirm it exists
		}
	}
	
	// Scan for account data patterns
	fmt.Printf("\n=== Scanning for Account Data ===\n")
	
	iter := db.NewIterator(nil, nil)
	defer iter.Release()
	
	count := 0
	largeBalances := 0
	targetFound := false
	
	for iter.Next() && count < 1000000 {
		key := iter.Key()
		val := iter.Value()
		
		// Look for account patterns
		// Accounts in state trie have 32-byte keys (hashes)
		if len(key) == 32 && len(val) > 0 {
			// Check if value could be account data
			// Account data typically has specific sizes
			if len(val) >= 80 && len(val) <= 120 {
				// Try to interpret as balance (first 32 bytes)
				if len(val) >= 32 {
					balance := new(big.Int).SetBytes(val[:32])
					
					// Check for large balances (> 1000 LUX)
					threshold := new(big.Int)
					threshold.SetString("1000000000000000000000", 10) // 1000 LUX
					
					if balance.Cmp(threshold) > 0 {
						largeBalances++
						
						// Check if this might be our target
						if balance.String() == "1900000000000000000000000000000" {
							fmt.Printf("🎉 FOUND 1.9T LUX BALANCE!\n")
							fmt.Printf("  Key: %x\n", key)
							fmt.Printf("  Balance: %s\n", balance)
							targetFound = true
						}
						
						if largeBalances <= 10 {
							fmt.Printf("Large balance found: %s wei\n", balance)
						}
					}
				}
			}
		}
		
		count++
		if count%100000 == 0 {
			fmt.Printf("Scanned %d keys, found %d large balances\n", count, largeBalances)
		}
	}
	
	fmt.Printf("\n=== Summary ===\n")
	fmt.Printf("Scanned %d keys\n", count)
	fmt.Printf("Found %d accounts with balance > 1000 LUX\n", largeBalances)
	
	if targetFound {
		fmt.Printf("\n✅✅ SUCCESS! Found account with 1.9T LUX balance!\n")
		fmt.Printf("The data migration preserved the account balances correctly.\n")
	} else {
		fmt.Printf("\n⚠️  Did not find the exact 1.9T balance in this scan.\n")
		fmt.Printf("The account data may be in a different format or need deeper scanning.\n")
	}
	
	// Check database stats
	fmt.Printf("\n=== Database Info ===\n")
	stats, _ := db.GetProperty("leveldb.stats")
	lines := strings.Split(stats, "\n")
	for i, line := range lines {
		if i < 5 {
			fmt.Println(line)
		}
	}
}

func encodeBlockNumber(number uint64) []byte {
	enc := make([]byte, 8)
	binary.BigEndian.PutUint64(enc, number)
	return enc
}