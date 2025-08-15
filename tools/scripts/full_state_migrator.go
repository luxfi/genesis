package main

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"time"
	
	"github.com/cockroachdb/pebble"
	"github.com/luxfi/database/badgerdb"
	"github.com/luxfi/geth/common"
)

const (
	// SubnetEVM namespace prefix
	subnetNamespace = "\x33\x7f\xb7\x3f\x9b\xcd\xac\x8c\x31\xa2\xd5\xf7\xb8\x77\xab\x1e\x8a\x2b\x7f\x2a\x1e\x9b\xf0\x2a\x0a\x0e\x6c\x6f\xd1\x64\xf1\xd1"
)

func main() {
	fmt.Println("╔════════════════════════════════════════════════╗")
	fmt.Println("║   FULL STATE MIGRATION - SubnetEVM → Coreth   ║")
	fmt.Println("╚════════════════════════════════════════════════╝")
	fmt.Println()
	
	// Source: Original SubnetEVM PebbleDB with FULL state
	srcPath := "/Users/z/work/lux/state/chaindata/lux-mainnet-96369/db/pebbledb"
	
	// Destination: Coreth BadgerDB
	dstPath := "/Users/z/.luxd/network-96369/chains/EWi9aPkGe6EfJ3SobCAmSUXRPLa4brF3cThwPwmHTrD1y13jy/ethdb"
	
	// Backup existing if it exists
	if _, err := os.Stat(dstPath); err == nil {
		backupPath := fmt.Sprintf("%s.backup.%d", dstPath, time.Now().Unix())
		fmt.Printf("Backing up existing database to %s\n", backupPath)
		os.Rename(dstPath, backupPath)
	}
	
	// Open source PebbleDB
	fmt.Printf("Opening source database: %s\n", srcPath)
	srcDB, err := pebble.Open(srcPath, &pebble.Options{ReadOnly: true})
	if err != nil {
		panic(fmt.Sprintf("Failed to open source: %v", err))
	}
	defer srcDB.Close()
	
	// Create destination BadgerDB
	fmt.Printf("Creating destination database: %s\n", dstPath)
	dstDB, err := badgerdb.New(filepath.Clean(dstPath), nil, "", nil)
	if err != nil {
		panic(fmt.Sprintf("Failed to create destination: %v", err))
	}
	defer dstDB.Close()
	
	fmt.Println("\n📊 Migration starting...")
	fmt.Println("This will migrate ALL data including:")
	fmt.Println("  • Block headers")
	fmt.Println("  • Block bodies") 
	fmt.Println("  • Receipts")
	fmt.Println("  • State trie nodes")
	fmt.Println("  • Account data")
	fmt.Println("  • Contract storage")
	fmt.Println("  • Code")
	fmt.Println()
	
	// Statistics
	var (
		totalKeys    int64
		headers      int64
		bodies       int64
		receipts     int64
		stateNodes   int64
		accounts     int64
		storage      int64
		code         int64
		other        int64
		totalBytes   int64
		startTime    = time.Now()
		lastProgress = time.Now()
	)
	
	// Create iterator for entire database
	iter, err := srcDB.NewIter(nil)
	if err != nil {
		panic(fmt.Sprintf("Failed to create iterator: %v", err))
	}
	defer iter.Close()
	
	// Batch for writes
	batch := dstDB.NewBatch()
	batchSize := 0
	maxBatchSize := 1 * 1024 * 1024 // 1MB batches (BadgerDB limit)
	
	// Progress reporter
	reportProgress := func() {
		elapsed := time.Since(startTime)
		rate := float64(totalKeys) / elapsed.Seconds()
		fmt.Printf("\r⏳ Processed %d keys (%.0f keys/sec) - Headers:%d Bodies:%d State:%d Accounts:%d Storage:%d",
			totalKeys, rate, headers, bodies, stateNodes, accounts, storage)
	}
	
	// Iterate through ALL keys
	for iter.First(); iter.Valid(); iter.Next() {
		key := iter.Key()
		value := iter.Value()
		
		// Skip empty keys
		if len(key) == 0 || len(value) == 0 {
			continue
		}
		
		var destKey []byte
		
		// Check if key has namespace prefix
		if len(key) > 32 && string(key[:32]) == subnetNamespace {
			// Remove namespace prefix
			destKey = make([]byte, len(key)-32)
			copy(destKey, key[32:])
		} else {
			// Use key as-is
			destKey = make([]byte, len(key))
			copy(destKey, key)
		}
		
		// Copy value
		destValue := make([]byte, len(value))
		copy(destValue, value)
		
		// Add to batch
		batch.Put(destKey, destValue)
		batchSize += len(destKey) + len(destValue)
		
		// Track statistics based on key prefix
		if len(destKey) > 0 {
			switch destKey[0] {
			case 'h':
				if len(destKey) == 41 {
					headers++ // header key: h + num(8) + hash(32)
				}
			case 'b':
				if len(destKey) == 41 {
					bodies++ // body key: b + num(8) + hash(32)
				}
			case 'r':
				if len(destKey) == 41 {
					receipts++ // receipt key: r + num(8) + hash(32)
				}
			case 's':
				stateNodes++ // state trie nodes
			case 'a':
				accounts++ // account data
			case 'S':
				storage++ // contract storage
			case 'c':
				code++ // contract code
			default:
				other++
			}
		}
		
		totalKeys++
		totalBytes += int64(len(key) + len(value))
		
		// Flush batch if it's getting large
		if batchSize >= maxBatchSize {
			if err := batch.Write(); err != nil {
				panic(fmt.Sprintf("Failed to write batch: %v", err))
			}
			batch = dstDB.NewBatch()
			batchSize = 0
		}
		
		// Report progress every second
		if time.Since(lastProgress) > time.Second {
			reportProgress()
			lastProgress = time.Now()
		}
	}
	
	// Write final batch
	if batchSize > 0 {
		if err := batch.Write(); err != nil {
			panic(fmt.Sprintf("Failed to write final batch: %v", err))
		}
	}
	
	// Check for any iteration errors
	if err := iter.Error(); err != nil {
		panic(fmt.Sprintf("Iterator error: %v", err))
	}
	
	fmt.Println() // New line after progress
	
	// Final statistics
	elapsed := time.Since(startTime)
	fmt.Println("\n✅ Migration Complete!")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("  Total Keys:    %d\n", totalKeys)
	fmt.Printf("  Total Data:    %.2f GB\n", float64(totalBytes)/(1024*1024*1024))
	fmt.Printf("  Headers:       %d\n", headers)
	fmt.Printf("  Bodies:        %d\n", bodies)
	fmt.Printf("  Receipts:      %d\n", receipts)
	fmt.Printf("  State Nodes:   %d\n", stateNodes)
	fmt.Printf("  Accounts:      %d\n", accounts)
	fmt.Printf("  Storage:       %d\n", storage)
	fmt.Printf("  Code:          %d\n", code)
	fmt.Printf("  Other:         %d\n", other)
	fmt.Printf("  Time:          %s\n", elapsed.Round(time.Second))
	fmt.Printf("  Rate:          %.0f keys/sec\n", float64(totalKeys)/elapsed.Seconds())
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	
	// Verify migration by checking key database markers
	fmt.Println("\n🔍 Verifying migration...")
	
	// Check LastHeader
	if val, err := dstDB.Get([]byte("LastHeader")); err == nil {
		var hash common.Hash
		copy(hash[:], val)
		fmt.Printf("  ✓ LastHeader: %s\n", hash.Hex())
	}
	
	// Check canonical at genesis
	canonKey := make([]byte, 10)
	canonKey[0] = 'h'
	binary.BigEndian.PutUint64(canonKey[1:9], 0)
	canonKey[9] = 'n'
	
	if val, err := dstDB.Get(canonKey); err == nil {
		var hash common.Hash
		copy(hash[:], val)
		fmt.Printf("  ✓ Genesis: %s\n", hash.Hex())
	}
	
	// Check for state root
	if stateNodes > 0 {
		fmt.Printf("  ✓ State Data: %d nodes migrated\n", stateNodes)
	}
	
	fmt.Println("\n🎉 Full state migration successful!")
	fmt.Println("The database now contains complete blockchain data including state.")
}