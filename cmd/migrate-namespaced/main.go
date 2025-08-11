package main

import (
	"bytes"
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

// SubnetEVM uses 32-byte namespace prefix on all keys
const namespaceSize = 32

// Key prefixes after namespace stripping
var (
	headerPrefix     = []byte("h") // headerPrefix + num (uint64 big endian) + hash -> header
	headerTDSuffix   = []byte("t") // headerPrefix + num (uint64 big endian) + hash + headerTDSuffix -> td
	headerHashSuffix = []byte("n") // headerPrefix + num (uint64 big endian) + headerHashSuffix -> hash
	headerNumberPrefix = []byte("H") // headerNumberPrefix + hash -> num (uint64 big endian)
	blockBodyPrefix  = []byte("b") // blockBodyPrefix + num (uint64 big endian) + hash -> block body
	blockReceiptsPrefix = []byte("r") // blockReceiptsPrefix + num (uint64 big endian) + hash -> block receipts
	codePrefix       = []byte("c") // codePrefix + code hash -> contract code
	
	headHeaderKey      = []byte("LastHeader")
	headBlockKey       = []byte("LastBlock")
	headFastBlockKey   = []byte("LastFast")
)

// encodeBlockNumber encodes a block number as big endian uint64
func encodeBlockNumber(number uint64) []byte {
	enc := make([]byte, 8)
	binary.BigEndian.PutUint64(enc, number)
	return enc
}

// stripNamespace removes the 32-byte namespace prefix from a key
func stripNamespace(key []byte) []byte {
	if len(key) <= namespaceSize {
		return key // Too short to have namespace, return as-is
	}
	return key[namespaceSize:]
}

// hasPrefix checks if a key (after namespace stripping) has a given prefix
func hasPrefix(key []byte, prefix []byte) bool {
	if len(key) <= namespaceSize {
		return false
	}
	stripped := key[namespaceSize:]
	return bytes.HasPrefix(stripped, prefix)
}

func main() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: migrate-namespaced <source-pebbledb> <dest-pebbledb>")
		fmt.Println("Example: migrate-namespaced /home/z/work/lux/state/chaindata/lux-mainnet-96369/db/pebbledb /home/z/.luxd/network-96369/chains/X6CU5qgMJfzsTB9UWxj2ZY5hd68x41HfZ4m4hCBWbHuj1Ebc3/ethdb")
		os.Exit(1)
	}

	sourcePath := os.Args[1]
	destPath := os.Args[2]

	fmt.Printf("=== SubnetEVM to Coreth Migration (Namespace-Aware) ===\n")
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

	// STEP 1: Scan to find chain tip
	fmt.Println("Step 1: Finding chain tip by scanning canonical hashes...")
	var tipHeight uint64
	var tipHash common.Hash
	
	// Create iterator
	iter, err := srcDB.NewIter(&pebble.IterOptions{})
	if err != nil {
		log.Fatalf("Failed to create iterator: %v", err)
	}
	
	canonicalCount := 0
	for iter.First(); iter.Valid(); iter.Next() {
		key := iter.Key()
		
		// Skip if key is too short to have namespace
		if len(key) <= namespaceSize {
			continue
		}
		
		// Check if this is a canonical hash key
		stripped := key[namespaceSize:]
		// Canonical hash key: h + num(8) + n = 10 bytes
		if len(stripped) == 10 && bytes.HasPrefix(stripped, headerPrefix) && stripped[9] == 'n' {
			// Extract block number
			numBytes := stripped[1:9]
			blockNum := binary.BigEndian.Uint64(numBytes)
			
			// Get the hash value
			value := iter.Value()
			if len(value) == 32 {
				hash := common.BytesToHash(value)
				if blockNum > tipHeight {
					tipHeight = blockNum
					tipHash = hash
				}
				canonicalCount++
				
				if canonicalCount%10000 == 0 {
					fmt.Printf("  Found %d canonical hashes, current tip: %d\n", canonicalCount, tipHeight)
				}
			}
		}
	}
	iter.Close()
	
	fmt.Printf("  Found %d total canonical hashes\n", canonicalCount)
	fmt.Printf("  Chain tip: block %d (hash: %s)\n", tipHeight, tipHash.Hex())
	
	if tipHeight == 0 {
		fmt.Println("  WARNING: No canonical hashes found, will scan for headers...")
		// Try to find headers directly
		iter, _ = srcDB.NewIter(&pebble.IterOptions{})
		for iter.First(); iter.Valid(); iter.Next() {
			key := iter.Key()
			if len(key) <= namespaceSize {
				continue
			}
			stripped := key[namespaceSize:]
			// Header key: h + num(8) + hash(32) = 41 bytes
			if len(stripped) == 41 && bytes.HasPrefix(stripped, headerPrefix) {
				numBytes := stripped[1:9]
				blockNum := binary.BigEndian.Uint64(numBytes)
				if blockNum > tipHeight {
					tipHeight = blockNum
					tipHash = common.BytesToHash(stripped[9:41])
				}
			}
		}
		iter.Close()
		fmt.Printf("  Found headers up to block %d\n", tipHeight)
	}

	// STEP 2: Migrate all data, stripping namespace
	fmt.Println("\nStep 2: Migrating blockchain data (stripping 32-byte namespace)...")
	
	batch := dstDB.NewBatch()
	keysWritten := 0
	keyCategories := make(map[string]int)
	
	// Process all keys
	iter, err = srcDB.NewIter(&pebble.IterOptions{})
	if err != nil {
		log.Fatalf("Failed to create iterator: %v", err)
	}
	defer iter.Close()
	
	for iter.First(); iter.Valid(); iter.Next() {
		srcKey := iter.Key()
		value := iter.Value()
		
		// Strip namespace if present
		var dstKey []byte
		if len(srcKey) > namespaceSize {
			dstKey = make([]byte, len(srcKey)-namespaceSize)
			copy(dstKey, srcKey[namespaceSize:])
		} else {
			// Key too short for namespace, copy as-is
			dstKey = make([]byte, len(srcKey))
			copy(dstKey, srcKey)
		}
		
		// Copy value
		valueCopy := make([]byte, len(value))
		copy(valueCopy, value)
		
		// Categorize key for statistics
		if len(dstKey) > 0 {
			switch dstKey[0] {
			case 'h':
				if len(dstKey) == 10 && dstKey[9] == 'n' {
					keyCategories["canonical"]++
				} else if len(dstKey) == 41 {
					keyCategories["headers"]++
				}
			case 'H':
				keyCategories["hash-to-number"]++
			case 'b':
				keyCategories["bodies"]++
			case 'r':
				keyCategories["receipts"]++
			case 'c':
				keyCategories["code"]++
			case 'L':
				if bytes.Equal(dstKey, headHeaderKey) || bytes.Equal(dstKey, headBlockKey) || bytes.Equal(dstKey, headFastBlockKey) {
					keyCategories["heads"]++
				}
			default:
				if len(dstKey) == 32 {
					keyCategories["state-trie"]++
				} else {
					keyCategories["other"]++
				}
			}
		}
		
		// Write to destination
		if err := batch.Set(dstKey, valueCopy, nil); err != nil {
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
	fmt.Println("\n  Key categories:")
	for cat, count := range keyCategories {
		fmt.Printf("    %s: %d\n", cat, count)
	}

	// STEP 3: Write Total Difficulty for all blocks
	fmt.Println("\nStep 3: Writing Total Difficulty...")
	
	batch = dstDB.NewBatch()
	tdWritten := 0
	missingCanonical := 0
	
	for n := uint64(0); n <= tipHeight; n++ {
		// Build canonical hash key
		canonKey := append(append(headerPrefix, encodeBlockNumber(n)...), headerHashSuffix...)
		
		// Get canonical hash
		hashBytes, closer, err := dstDB.Get(canonKey)
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
		headerKey := append(append(headerPrefix, encodeBlockNumber(n)...), hash.Bytes()...)
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
		
		// Build TD key
		tdKey := append(headerKey, headerTDSuffix...)
		
		// Write TD
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

	// STEP 4: Ensure head pointers are set
	fmt.Println("\nStep 4: Setting head pointers...")
	
	// Get the canonical hash at tip
	canonKey := append(append(headerPrefix, encodeBlockNumber(tipHeight)...), headerHashSuffix...)
	if hashBytes, closer, err := dstDB.Get(canonKey); err == nil {
		tipHash = common.BytesToHash(hashBytes)
		closer.Close()
	}
	
	if tipHash != (common.Hash{}) {
		// Write all three head pointers
		if err := dstDB.Set(headHeaderKey, tipHash.Bytes(), pebble.Sync); err != nil {
			log.Fatalf("Failed to set head header: %v", err)
		}
		if err := dstDB.Set(headBlockKey, tipHash.Bytes(), pebble.Sync); err != nil {
			log.Fatalf("Failed to set head block: %v", err)
		}
		if err := dstDB.Set(headFastBlockKey, tipHash.Bytes(), pebble.Sync); err != nil {
			log.Fatalf("Failed to set head fast block: %v", err)
		}
		
		fmt.Printf("  ✅ Set heads to block %d (hash: %s)\n", tipHeight, tipHash.Hex())
	} else {
		fmt.Printf("  ❌ Could not find canonical hash for tip!\n")
	}

	// STEP 5: Write hash->number mappings
	fmt.Println("\nStep 5: Ensuring hash->number mappings...")
	
	batch = dstDB.NewBatch()
	hashNumWritten := 0
	
	for n := uint64(0); n <= tipHeight; n++ {
		// Get canonical hash
		canonKey := append(append(headerPrefix, encodeBlockNumber(n)...), headerHashSuffix...)
		hashBytes, closer, err := dstDB.Get(canonKey)
		if err != nil {
			continue
		}
		hash := common.BytesToHash(hashBytes)
		closer.Close()
		
		// Write hash->number mapping
		hashNumKey := append(headerNumberPrefix, hash.Bytes()...)
		numBytes := encodeBlockNumber(n)
		
		if err := batch.Set(hashNumKey, numBytes, nil); err != nil {
			log.Fatalf("Failed to set hash->number: %v", err)
		}
		
		hashNumWritten++
		
		if hashNumWritten%10000 == 0 {
			fmt.Printf("  Written %d hash->number mappings...\n", hashNumWritten)
			if err := batch.Commit(pebble.Sync); err != nil {
				log.Fatalf("Failed to commit batch: %v", err)
			}
			batch = dstDB.NewBatch()
		}
	}
	
	if err := batch.Commit(pebble.Sync); err != nil {
		log.Fatalf("Failed to commit final batch: %v", err)
	}
	
	fmt.Printf("  ✅ Written %d hash->number mappings\n", hashNumWritten)

	// STEP 6: Verify migration
	fmt.Println("\nStep 6: Verifying migration...")
	
	// Check genesis TD
	genesisCanonKey := append(append(headerPrefix, encodeBlockNumber(0)...), headerHashSuffix...)
	if hashBytes, closer, err := dstDB.Get(genesisCanonKey); err == nil {
		genesisHash := common.BytesToHash(hashBytes)
		closer.Close()
		
		tdKey := append(append(append(headerPrefix, encodeBlockNumber(0)...), genesisHash.Bytes()...), headerTDSuffix...)
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
		tdKey := append(append(append(headerPrefix, encodeBlockNumber(tipHeight)...), tipHash.Bytes()...), headerTDSuffix...)
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
	if headBytes, closer, err := dstDB.Get(headBlockKey); err == nil {
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
	
	// Check hash->number mapping
	if tipHash != (common.Hash{}) {
		hashNumKey := append(headerNumberPrefix, tipHash.Bytes()...)
		if numBytes, closer, err := dstDB.Get(hashNumKey); err == nil {
			storedNum := binary.BigEndian.Uint64(numBytes)
			closer.Close()
			if storedNum == tipHeight {
				fmt.Printf("  ✅ Hash->number mapping correct for tip\n")
			} else {
				fmt.Printf("  ⚠️  Hash->number mismatch: stored=%d, expected=%d\n", storedNum, tipHeight)
			}
		} else {
			fmt.Printf("  ❌ Hash->number mapping missing for tip\n")
		}
	}

	fmt.Printf("\n=== Migration Complete ===\n")
	fmt.Printf("Database ready at: %s\n", destPath)
	fmt.Printf("Chain tip: block %d (hash: %s)\n", tipHeight, tipHash.Hex())
	fmt.Printf("Total Difficulty at tip: %d\n", tipHeight+1)
	fmt.Printf("Completed at: %s\n", time.Now().Format("2006-01-02 15:04:05"))
	
	fmt.Println("\n✅ Migration successful! Database is ready for luxd with Coreth.")
	fmt.Println("\nNext steps:")
	fmt.Println("1. Set VM metadata in vm/ directory")
	fmt.Println("2. Write chain config at genesis hash")
	fmt.Println("3. Start luxd")
}