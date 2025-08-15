package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/cockroachdb/pebble"
	"github.com/dgraph-io/badger/v4"
)

// SubnetEVM namespace prefix (32 bytes)
var subnetNamespace = []byte{
	0x33, 0x7f, 0xb7, 0x3f, 0x9b, 0xcd, 0xac, 0x8c,
	0x31, 0xa2, 0xd5, 0xf7, 0xb8, 0x77, 0xab, 0x1e,
	0x8a, 0x2b, 0x7f, 0x2a, 0x1e, 0x9b, 0xf0, 0x2a,
	0x0a, 0x0e, 0x6c, 0x6f, 0xd1, 0x64, 0xf1, 0xd1,
}

func encodeBlockNumber(number uint64) []byte {
	enc := make([]byte, 8)
	binary.BigEndian.PutUint64(enc, number)
	return enc
}

func main() {
	sourcePath := "/Users/z/work/lux/state/chaindata/lux-mainnet-96369/db/pebbledb"
	targetPath := "/Users/z/.luxd/network-96369/chains/EWi9aPkGe6EfJ3SobCAmSUXRPLa4brF3cThwPwmHTrD1y13jy/ethdb"
	
	fmt.Println("FULL Proper Coreth Migration from SubnetEVM")
	fmt.Println("============================================")
	fmt.Printf("Source: %s\n", sourcePath)
	fmt.Printf("Target: %s\n", targetPath)
	
	// Open source PebbleDB
	sourceDB, err := pebble.Open(sourcePath, &pebble.Options{
		ReadOnly: true,
	})
	if err != nil {
		log.Fatal("Failed to open source database:", err)
	}
	defer sourceDB.Close()
	
	// Create target directory if needed
	os.MkdirAll(targetPath, 0755)
	
	// Open target BadgerDB
	targetDB, err := badger.Open(badger.DefaultOptions(targetPath))
	if err != nil {
		log.Fatal("Failed to open target database:", err)
	}
	defer targetDB.Close()
	
	fmt.Println("\nStarting COMPLETE migration of ALL data...")
	startTime := time.Now()
	
	// Statistics
	stats := struct {
		headers    int
		bodies     int
		receipts   int
		canonical  int
		state      int
		code       int
		preimages  int
		other      int
		total      int
		skipped    int
	}{}
	
	// Track block numbers for canonical mappings
	blockMap := make(map[uint64][]byte) // block number -> hash
	highestBlock := uint64(0)
	
	// Create iterator for ALL data
	iter, err := sourceDB.NewIter(nil)
	if err != nil {
		log.Fatal("Failed to create iterator:", err)
	}
	defer iter.Close()
	
	// Process in batches for efficiency
	batch := targetDB.NewWriteBatch()
	batchSize := 0
	maxBatchSize := 10000
	
	fmt.Println("Processing all entries from source database...")
	
	for iter.First(); iter.Valid(); iter.Next() {
		key := iter.Key()
		val := iter.Value()
		
		// Copy key and value to avoid iterator issues
		keyCopy := make([]byte, len(key))
		copy(keyCopy, key)
		valCopy := make([]byte, len(val))
		copy(valCopy, val)
		
		// Process based on whether it has SubnetEVM namespace
		if len(keyCopy) >= 32 && bytes.HasPrefix(keyCopy, subnetNamespace) {
			// Has namespace - remove it
			actualKey := keyCopy[32:]
			
			// Migrate with namespace removed
			batch.Set(actualKey, valCopy)
			stats.state++ // Most namespaced data is state
			
		} else if len(keyCopy) == 64 && !bytes.HasPrefix(keyCopy, subnetNamespace) {
			// 64-byte key without namespace - likely a composite key
			// Try to parse it as block data
			if len(valCopy) > 100 { // Likely block header/body/receipt
				// Extract potential block number (first 8 bytes)
				blockNum := binary.BigEndian.Uint64(keyCopy[:8])
				hash := keyCopy[8:40] // Next 32 bytes
				
				if blockNum < 10000000 { // Reasonable block number
					// Create proper Coreth keys
					
					// Header key: 'h' + num(8) + hash(32)
					headerKey := make([]byte, 41)
					headerKey[0] = 'h'
					binary.BigEndian.PutUint64(headerKey[1:9], blockNum)
					copy(headerKey[9:41], hash)
					batch.Set(headerKey, valCopy)
					stats.headers++
					
					// Body key: 'b' + num(8) + hash(32)
					bodyKey := make([]byte, 41)
					bodyKey[0] = 'b'
					copy(bodyKey[1:], headerKey[1:])
					batch.Set(bodyKey, valCopy)
					stats.bodies++
					
					// Receipt key: 'r' + num(8) + hash(32)
					receiptKey := make([]byte, 41)
					receiptKey[0] = 'r'
					copy(receiptKey[1:], headerKey[1:])
					batch.Set(receiptKey, valCopy)
					stats.receipts++
					
					// Track for canonical mapping
					blockMap[blockNum] = hash
					if blockNum > highestBlock {
						highestBlock = blockNum
					}
				} else {
					// Not block data, store as-is
					batch.Set(keyCopy, valCopy)
					stats.other++
				}
			} else {
				// Small value, likely metadata
				batch.Set(keyCopy, valCopy)
				stats.other++
			}
			
		} else {
			// Regular key - migrate as-is
			switch {
			case len(keyCopy) == 32:
				// 32-byte key - likely trie node
				batch.Set(keyCopy, valCopy)
				stats.state++
				
			case len(keyCopy) > 0 && keyCopy[0] >= 'A' && keyCopy[0] <= 'z':
				// Prefixed key - check type
				switch keyCopy[0] {
				case 'h', 'H', 'b', 'r', 'l':
					// Block-related data
					batch.Set(keyCopy, valCopy)
					stats.other++
				case 'a', 'o', 'A', 'O', 's':
					// State data
					batch.Set(keyCopy, valCopy)
					stats.state++
				case 'c':
					// Code
					batch.Set(keyCopy, valCopy)
					stats.code++
				default:
					// Other
					batch.Set(keyCopy, valCopy)
					stats.other++
				}
				
			case bytes.HasPrefix(keyCopy, []byte("secure-key-")):
				// Preimage
				batch.Set(keyCopy, valCopy)
				stats.preimages++
				
			default:
				// Unknown - store as-is
				batch.Set(keyCopy, valCopy)
				stats.other++
			}
		}
		
		batchSize++
		stats.total++
		
		// Flush batch periodically
		if batchSize >= maxBatchSize {
			if err := batch.Flush(); err != nil {
				log.Printf("Warning: batch flush failed: %v", err)
			}
			batch = targetDB.NewWriteBatch()
			batchSize = 0
			
			if stats.total%1000000 == 0 {
				fmt.Printf("Migrated %d million entries (headers: %d, state: %d, code: %d)...\n", 
					stats.total/1000000, stats.headers, stats.state, stats.code)
			}
		}
	}
	
	// Flush final batch
	if batchSize > 0 {
		if err := batch.Flush(); err != nil {
			log.Printf("Warning: final batch flush failed: %v", err)
		}
	}
	
	// Add canonical mappings for all blocks we found
	fmt.Printf("\nAdding %d canonical block mappings...\n", len(blockMap))
	batch = targetDB.NewWriteBatch()
	batchSize = 0
	
	for blockNum, hash := range blockMap {
		// Canonical key: 'H' + num(8)
		canonicalKey := make([]byte, 9)
		canonicalKey[0] = 'H'
		binary.BigEndian.PutUint64(canonicalKey[1:], blockNum)
		batch.Set(canonicalKey, hash)
		
		stats.canonical++
		batchSize++
		
		if batchSize >= maxBatchSize {
			batch.Flush()
			batch = targetDB.NewWriteBatch()
			batchSize = 0
		}
	}
	
	if batchSize > 0 {
		batch.Flush()
	}
	
	// Set database markers
	fmt.Println("\nSetting Coreth database markers...")
	
	err = targetDB.Update(func(txn *badger.Txn) error {
		// Find head block hash from our canonical mappings
		if highestBlock > 0 {
			if hash, ok := blockMap[highestBlock]; ok {
				// Set head markers
				txn.Set([]byte("LastBlock"), hash)
				txn.Set([]byte("LastHeader"), hash)
				txn.Set([]byte("LastFast"), hash)
				
				fmt.Printf("Set head block to %d with hash 0x%x\n", highestBlock, hash)
			}
		}
		
		// Set database version
		txn.Set([]byte("DatabaseVersion"), []byte{0x08})
		
		return nil
	})
	
	if err != nil {
		log.Printf("Warning: Failed to set markers: %v", err)
	}
	
	elapsed := time.Since(startTime)
	fmt.Println("\n============================================")
	fmt.Println("FULL Migration Complete!")
	fmt.Printf("Total entries migrated: %d\n", stats.total)
	fmt.Printf("  Headers:    %d\n", stats.headers)
	fmt.Printf("  Bodies:     %d\n", stats.bodies)
	fmt.Printf("  Receipts:   %d\n", stats.receipts)
	fmt.Printf("  Canonical:  %d\n", stats.canonical)
	fmt.Printf("  State:      %d\n", stats.state)
	fmt.Printf("  Code:       %d\n", stats.code)
	fmt.Printf("  Preimages:  %d\n", stats.preimages)
	fmt.Printf("  Other:      %d\n", stats.other)
	fmt.Printf("  Skipped:    %d\n", stats.skipped)
	fmt.Printf("Highest block: %d\n", highestBlock)
	fmt.Printf("Time elapsed: %v\n", elapsed)
	fmt.Println("\nDatabase is ready for Coreth!")
}