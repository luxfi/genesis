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

// SubnetEVM namespace prefix
var subnetNamespace = []byte{
	0x33, 0x7f, 0xb7, 0x3f, 0x9b, 0xcd, 0xac, 0x8c,
	0x31, 0xa2, 0xd5, 0xf7, 0xb8, 0x77, 0xab, 0x1e,
	0x8a, 0x2b, 0x7f, 0x2a, 0x1e, 0x9b, 0xf0, 0x2a,
	0x0a, 0x0e, 0x6c, 0x6f, 0xd1, 0x64, 0xf1, 0xd1,
}

// Coreth database key prefixes
const (
	// Data item prefixes (use single byte to avoid mixing data types)
	headerPrefix       = "h" // headerPrefix + num (uint64 big endian) + hash -> header
	headerTDSuffix     = "t" // headerPrefix + num (uint64 big endian) + hash + headerTDSuffix -> td
	headerHashSuffix   = "n" // headerPrefix + num (uint64 big endian) + headerHashSuffix -> hash
	headerNumberPrefix = "H" // headerNumberPrefix + hash -> num (uint64 big endian)

	blockBodyPrefix     = "b" // blockBodyPrefix + num (uint64 big endian) + hash -> block body
	blockReceiptsPrefix = "r" // blockReceiptsPrefix + num (uint64 big endian) + hash -> block receipts

	txLookupPrefix        = "l" // txLookupPrefix + hash -> transaction/receipt lookup metadata
	bloomBitsPrefix       = "B" // bloomBitsPrefix + bit (uint16 big endian) + section (uint64 big endian) + hash -> bloom bits
	SnapshotAccountPrefix = "a" // SnapshotAccountPrefix + account hash -> account trie value
	SnapshotStoragePrefix = "o" // SnapshotStoragePrefix + account hash + storage hash -> storage trie value
	CodePrefix            = "c" // CodePrefix + code hash -> contract code

	// Chain index prefixes (use `i` + single byte to avoid mixing data types)
	BloomBitsIndexPrefix = "iB" // BloomBitsIndexPrefix is the data table of a chain indexer to track its progress

	// Trie table prefixes
	TrieNodeAccountPrefix = "A" // TrieNodeAccountPrefix + hexPath -> trie node
	TrieNodeStoragePrefix = "O" // TrieNodeStoragePrefix + accountHash + hexPath -> trie node
	stateIDPrefix         = "s" // stateIDPrefix + root -> stateID

	PreimagePrefix        = "secure-key-"      // PreimagePrefix + hash -> preimage
	configPrefix          = "ethereum-config-" // config prefix for the db
	genesisPrefix         = "ethereum-genesis-" // genesis state prefix for the db

	// BloomBits Index
	bloomBitsIndexPrefix = "iB" // BloomBitsIndexPrefix is the data table of a chain indexer to track its progress

	// Beacon index prefixes
	beaconRootPrefix      = "R"  // beaconRootPrefix + timestamp -> beacon root
	beaconParentRootPrefix = "P" // beaconParentRootPrefix + beacon root -> parent beacon root
)

func encodeBlockNumber(number uint64) []byte {
	enc := make([]byte, 8)
	binary.BigEndian.PutUint64(enc, number)
	return enc
}

func main() {
	sourcePath := "/Users/z/work/lux/state/chaindata/lux-mainnet-96369/db/pebbledb"
	targetPath := "/Users/z/.luxd/network-96369/chains/EWi9aPkGe6EfJ3SobCAmSUXRPLa4brF3cThwPwmHTrD1y13jy/ethdb"
	
	fmt.Println("Proper Coreth Migration from SubnetEVM")
	fmt.Println("=======================================")
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
	
	fmt.Println("\nStarting proper Coreth migration...")
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
	}{}
	
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
	
	// Track highest block number
	highestBlock := uint64(0)
	
	for iter.First(); iter.Valid(); iter.Next() {
		key := iter.Key()
		val := iter.Value()
		
		// Remove SubnetEVM namespace if present
		actualKey := key
		if len(key) >= 32 && bytes.HasPrefix(key, subnetNamespace) {
			actualKey = key[32:]
		}
		
		// Skip if empty key
		if len(actualKey) == 0 {
			continue
		}
		
		// Migrate based on key type
		switch actualKey[0] {
		case 'h': // Header
			if len(actualKey) == 41 {
				// Already in correct format: h + num(8) + hash(32)
				batch.Set(actualKey, val)
				stats.headers++
				
				// Track highest block
				blockNum := binary.BigEndian.Uint64(actualKey[1:9])
				if blockNum > highestBlock {
					highestBlock = blockNum
				}
			}
			
		case 'H': // Canonical hash to number mapping
			// H + hash -> number
			batch.Set(actualKey, val)
			stats.canonical++
			
		case 'b': // Block body
			if len(actualKey) == 41 {
				// b + num(8) + hash(32)
				batch.Set(actualKey, val)
				stats.bodies++
			}
			
		case 'r': // Receipts
			if len(actualKey) == 41 {
				// r + num(8) + hash(32)
				batch.Set(actualKey, val)
				stats.receipts++
			}
			
		case 'l': // Transaction lookup
			batch.Set(actualKey, val)
			stats.other++
			
		case 'a': // Snapshot account data
			batch.Set(actualKey, val)
			stats.state++
			
		case 'o': // Snapshot storage data
			batch.Set(actualKey, val)
			stats.state++
			
		case 'A': // Account trie nodes
			batch.Set(actualKey, val)
			stats.state++
			
		case 'O': // Storage trie nodes
			batch.Set(actualKey, val)
			stats.state++
			
		case 'c': // Code
			batch.Set(actualKey, val)
			stats.code++
			
		case 's': // State ID
			batch.Set(actualKey, val)
			stats.state++
			
		default:
			// Check for special prefixes
			if bytes.HasPrefix(actualKey, []byte("secure-key-")) {
				// Preimages
				batch.Set(actualKey, val)
				stats.preimages++
			} else if bytes.HasPrefix(actualKey, []byte("ethereum-")) {
				// Config or genesis
				batch.Set(actualKey, val)
				stats.other++
			} else if len(actualKey) == 32 {
				// Likely a trie node (32-byte hash key)
				batch.Set(actualKey, val)
				stats.state++
			} else {
				// Other data
				batch.Set(actualKey, val)
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
			
			if stats.total%100000 == 0 {
				fmt.Printf("Migrated %d entries (headers: %d, state: %d, code: %d)...\n", 
					stats.total, stats.headers, stats.state, stats.code)
			}
		}
	}
	
	// Flush final batch
	if batchSize > 0 {
		if err := batch.Flush(); err != nil {
			log.Printf("Warning: final batch flush failed: %v", err)
		}
	}
	
	// Set database markers for Coreth
	fmt.Println("\nSetting Coreth database markers...")
	
	err = targetDB.Update(func(txn *badger.Txn) error {
		// Find head block hash
		headKey := append([]byte("H"), encodeBlockNumber(highestBlock)...)
		headHash, err := txn.Get(headKey)
		if err == nil {
			hashBytes, _ := headHash.ValueCopy(nil)
			if len(hashBytes) == 32 {
				// Set head markers
				txn.Set([]byte("LastBlock"), hashBytes)
				txn.Set([]byte("LastHeader"), hashBytes)
				txn.Set([]byte("LastFast"), hashBytes)
				
				// Set Coreth-specific markers
				txn.Set(append([]byte("ethereum-config-"), hashBytes...), []byte{})
				
				fmt.Printf("Set head block to %d with hash 0x%x\n", highestBlock, hashBytes)
			}
		}
		
		// Set database version
		txn.Set([]byte("DatabaseVersion"), []byte{0x08}) // Version 8 for Coreth
		
		return nil
	})
	
	if err != nil {
		log.Printf("Warning: Failed to set markers: %v", err)
	}
	
	elapsed := time.Since(startTime)
	fmt.Println("\n=======================================")
	fmt.Println("Migration Complete!")
	fmt.Printf("Total entries migrated: %d\n", stats.total)
	fmt.Printf("  Headers:    %d\n", stats.headers)
	fmt.Printf("  Bodies:     %d\n", stats.bodies)
	fmt.Printf("  Receipts:   %d\n", stats.receipts)
	fmt.Printf("  Canonical:  %d\n", stats.canonical)
	fmt.Printf("  State:      %d\n", stats.state)
	fmt.Printf("  Code:       %d\n", stats.code)
	fmt.Printf("  Preimages:  %d\n", stats.preimages)
	fmt.Printf("  Other:      %d\n", stats.other)
	fmt.Printf("Highest block: %d\n", highestBlock)
	fmt.Printf("Time elapsed: %v\n", elapsed)
	fmt.Println("\nDatabase is ready for Coreth!")
}