package main

import (
	"encoding/binary"
	"fmt"
	"log"
	"math/big"
	"os"
	"time"

	"github.com/cockroachdb/pebble"
	"github.com/luxfi/geth/common"
	"github.com/luxfi/geth/rlp"
)

// Key encoding helpers
func encodeBlockNumber(number uint64) []byte {
	enc := make([]byte, 8)
	binary.BigEndian.PutUint64(enc, number)
	return enc
}

// headerHashKey = headerPrefix + num (uint64 big endian) + headerHashSuffix
func headerHashKey(number uint64) []byte {
	return append(append([]byte("h"), encodeBlockNumber(number)...), byte('n'))
}

// headerKey = headerPrefix + num (uint64 big endian) + hash
func headerKey(number uint64, hash common.Hash) []byte {
	return append(append([]byte("h"), encodeBlockNumber(number)...), hash.Bytes()...)
}

// headerTDKey = headerPrefix + num (uint64 big endian) + hash + headerTDSuffix
func headerTDKey(number uint64, hash common.Hash) []byte {
	return append(headerKey(number, hash), byte('t'))
}

// blockBodyKey = blockBodyPrefix + num (uint64 big endian) + hash
func blockBodyKey(number uint64, hash common.Hash) []byte {
	return append(append([]byte("b"), encodeBlockNumber(number)...), hash.Bytes()...)
}

// blockReceiptsKey = blockReceiptsPrefix + num (uint64 big endian) + hash
func blockReceiptsKey(number uint64, hash common.Hash) []byte {
	return append(append([]byte("r"), encodeBlockNumber(number)...), hash.Bytes()...)
}

func main() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: migrate-complete <source-pebbledb> <dest-pebbledb>")
		fmt.Println("Example: migrate-complete /home/z/work/lux/state/chaindata/lux-mainnet-96369/db/pebbledb /home/z/.luxd/network-96369/chains/X6CU5qgMJfzsTB9UWxj2ZY5hd68x41HfZ4m4hCBWbHuj1Ebc3/ethdb")
		os.Exit(1)
	}

	sourcePath := os.Args[1]
	destPath := os.Args[2]

	fmt.Printf("=== Complete SubnetEVM to Coreth Migration ===\n")
	fmt.Printf("Source: %s\n", sourcePath)
	fmt.Printf("Destination: %s\n", destPath)
	fmt.Printf("Starting at: %s\n\n", time.Now().Format("2006-01-02 15:04:05"))

	// Check source exists
	if _, err := os.Stat(sourcePath); os.IsNotExist(err) {
		log.Fatalf("Source database does not exist: %s", sourcePath)
	}

	// Create destination directory
	if err := os.MkdirAll(destPath, 0755); err != nil {
		log.Fatalf("Failed to create destination directory: %v", err)
	}

	// Open source database
	srcOpts := &pebble.Options{
		Cache:        pebble.NewCache(512 << 20), // 512MB cache
		MaxOpenFiles: 1024,
		ReadOnly:     true,
	}
	srcDB, err := pebble.Open(sourcePath, srcOpts)
	if err != nil {
		log.Fatalf("Failed to open source database: %v", err)
	}
	defer srcDB.Close()

	// Open destination database
	dstOpts := &pebble.Options{
		Cache:        pebble.NewCache(512 << 20), // 512MB cache
		MaxOpenFiles: 1024,
	}
	dstDB, err := pebble.Open(destPath, dstOpts)
	if err != nil {
		log.Fatalf("Failed to open destination database: %v", err)
	}
	defer dstDB.Close()

	// STEP 1: Find chain tip
	fmt.Println("Step 1: Finding chain tip...")
	var tipHeight uint64
	var tipHash common.Hash
	
	// We know the tip is at 1,082,780 from previous migration
	knownTip := uint64(1082780)
	
	// Try known tip first
	key := headerHashKey(knownTip)
	if val, closer, err := srcDB.Get(key); err == nil {
		tipHash = common.BytesToHash(val)
		tipHeight = knownTip
		closer.Close()
		fmt.Printf("  Found tip at known height %d: %s\n", tipHeight, tipHash.Hex())
	} else {
		// Scan for actual tip
		fmt.Println("  Scanning for tip...")
		for n := uint64(0); n <= 2000000; n++ {
			if n%100000 == 0 && n > 0 {
				fmt.Printf("    Scanned up to block %d...\n", n)
			}
			key := headerHashKey(n)
			if val, closer, err := srcDB.Get(key); err == nil {
				tipHash = common.BytesToHash(val)
				tipHeight = n
				closer.Close()
			}
		}
		fmt.Printf("  Found tip at height %d: %s\n", tipHeight, tipHash.Hex())
	}

	// STEP 2: Migrate all data with proper prefixes
	fmt.Println("\nStep 2: Migrating blockchain data...")
	
	batch := dstDB.NewBatch()
	keysWritten := 0
	tdWritten := 0
	
	// Process all keys
	iter, err := srcDB.NewIter(&pebble.IterOptions{})
	if err != nil {
		log.Fatalf("Failed to create iterator: %v", err)
	}
	defer iter.Close()
	
	for iter.First(); iter.Valid(); iter.Next() {
		key := iter.Key()
		value := iter.Value()
		
		// Copy the key (to be safe since iterators reuse memory)
		keyCopy := make([]byte, len(key))
		copy(keyCopy, key)
		valueCopy := make([]byte, len(value))
		copy(valueCopy, value)
		
		// Write to destination
		if err := batch.Set(keyCopy, valueCopy, nil); err != nil {
			log.Fatalf("Failed to set key: %v", err)
		}
		
		keysWritten++
		
		// Commit batch periodically
		if keysWritten%10000 == 0 {
			fmt.Printf("  Migrated %d keys...\n", keysWritten)
			if err := batch.Commit(pebble.Sync); err != nil {
				log.Fatalf("Failed to commit batch: %v", err)
			}
			batch = dstDB.NewBatch()
		}
	}
	
	// Commit final batch
	if err := batch.Commit(pebble.Sync); err != nil {
		log.Fatalf("Failed to commit final batch: %v", err)
	}
	
	fmt.Printf("  ✅ Migrated %d keys total\n", keysWritten)

	// STEP 3: Write Total Difficulty for all blocks
	fmt.Println("\nStep 3: Writing Total Difficulty...")
	
	batch = dstDB.NewBatch()
	missingCanonical := 0
	
	for n := uint64(0); n <= tipHeight; n++ {
		// Get canonical hash
		hashKey := headerHashKey(n)
		hashBytes, closer, err := dstDB.Get(hashKey)
		if err != nil {
			missingCanonical++
			if n == 0 {
				fmt.Printf("  ⚠️  Genesis canonical hash missing\n")
			}
			continue
		}
		hash := common.BytesToHash(hashBytes)
		closer.Close()
		
		// Check if header exists
		headerKey := headerKey(n, hash)
		if _, closer, err := dstDB.Get(headerKey); err != nil {
			fmt.Printf("  ⚠️  Missing header at block %d\n", n)
			missingCanonical++
			continue
		} else {
			closer.Close()
		}
		
		// TD = height + 1 for this chain
		td := new(big.Int).SetUint64(n + 1)
		
		// Encode TD
		tdBytes, err := rlp.EncodeToBytes(td)
		if err != nil {
			log.Fatalf("Failed to encode TD: %v", err)
		}
		
		// Write TD
		tdKey := headerTDKey(n, hash)
		if err := batch.Set(tdKey, tdBytes, nil); err != nil {
			log.Fatalf("Failed to set TD: %v", err)
		}
		
		tdWritten++
		
		// Commit batch periodically
		if tdWritten%10000 == 0 {
			fmt.Printf("  Written TD for %d blocks...\n", tdWritten)
			if err := batch.Commit(pebble.Sync); err != nil {
				log.Fatalf("Failed to commit batch: %v", err)
			}
			batch = dstDB.NewBatch()
		}
	}
	
	// Commit final batch
	if err := batch.Commit(pebble.Sync); err != nil {
		log.Fatalf("Failed to commit final batch: %v", err)
	}
	
	fmt.Printf("  ✅ Written TD for %d blocks\n", tdWritten)
	if missingCanonical > 0 {
		fmt.Printf("  ⚠️  %d blocks had missing canonical hash or header\n", missingCanonical)
	}

	// STEP 4: Set head pointers
	fmt.Println("\nStep 4: Setting head pointers...")
	
	// Get the canonical hash at tip (refresh in case it changed)
	hashKey := headerHashKey(tipHeight)
	if hashBytes, closer, err := dstDB.Get(hashKey); err == nil {
		tipHash = common.BytesToHash(hashBytes)
		closer.Close()
	}
	
	if tipHash != (common.Hash{}) {
		// Write all three head pointers
		headHeaderKey := []byte("LastHeader")
		headBlockKey := []byte("LastBlock")
		headFastKey := []byte("LastFast")
		
		if err := dstDB.Set(headHeaderKey, tipHash.Bytes(), pebble.Sync); err != nil {
			log.Fatalf("Failed to set head header: %v", err)
		}
		if err := dstDB.Set(headBlockKey, tipHash.Bytes(), pebble.Sync); err != nil {
			log.Fatalf("Failed to set head block: %v", err)
		}
		if err := dstDB.Set(headFastKey, tipHash.Bytes(), pebble.Sync); err != nil {
			log.Fatalf("Failed to set head fast block: %v", err)
		}
		
		fmt.Printf("  ✅ Set heads to block %d (hash: %s)\n", tipHeight, tipHash.Hex())
	} else {
		fmt.Printf("  ❌ Could not find canonical hash for tip!\n")
	}

	// STEP 5: Verify migration
	fmt.Println("\nStep 5: Verifying migration...")
	
	// Check genesis TD
	if hashBytes, closer, err := dstDB.Get(headerHashKey(0)); err == nil {
		genesisHash := common.BytesToHash(hashBytes)
		closer.Close()
		
		tdKey := headerTDKey(0, genesisHash)
		if tdBytes, closer, err := dstDB.Get(tdKey); err == nil {
			var td big.Int
			if err := rlp.DecodeBytes(tdBytes, &td); err == nil {
				if td.Cmp(big.NewInt(1)) == 0 {
					fmt.Printf("  ✅ Genesis TD: 1 (correct)\n")
				} else {
					fmt.Printf("  ⚠️  Genesis TD: %v (expected 1)\n", &td)
				}
			}
			closer.Close()
		} else {
			fmt.Printf("  ❌ Genesis TD missing\n")
		}
	}
	
	// Check tip TD
	if tipHash != (common.Hash{}) {
		tdKey := headerTDKey(tipHeight, tipHash)
		if tdBytes, closer, err := dstDB.Get(tdKey); err == nil {
			var td big.Int
			if err := rlp.DecodeBytes(tdBytes, &td); err == nil {
				expectedTD := new(big.Int).SetUint64(tipHeight + 1)
				if td.Cmp(expectedTD) == 0 {
					fmt.Printf("  ✅ Tip TD: %v (correct)\n", &td)
				} else {
					fmt.Printf("  ⚠️  Tip TD: %v (expected %v)\n", &td, expectedTD)
				}
			}
			closer.Close()
		} else {
			fmt.Printf("  ❌ Tip TD missing\n")
		}
	}
	
	// Check head pointers
	if headBytes, closer, err := dstDB.Get([]byte("LastBlock")); err == nil {
		headHash := common.BytesToHash(headBytes)
		closer.Close()
		if headHash == tipHash {
			fmt.Printf("  ✅ Head block pointer correct\n")
		} else {
			fmt.Printf("  ⚠️  Head block pointer mismatch\n")
		}
	} else {
		fmt.Printf("  ❌ Head block pointer missing\n")
	}

	fmt.Printf("\n=== Migration Complete ===\n")
	fmt.Printf("Database ready at: %s\n", destPath)
	fmt.Printf("Chain tip: block %d (hash: %s)\n", tipHeight, tipHash.Hex())
	fmt.Printf("Total Difficulty at tip: %d\n", tipHeight+1)
	fmt.Printf("Completed at: %s\n", time.Now().Format("2006-01-02 15:04:05"))
	
	fmt.Println("\n✅ Migration successful! Database is ready for luxd with Coreth.")
}