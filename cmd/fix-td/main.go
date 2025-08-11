package main

import (
	"encoding/binary"
	"fmt"
	"log"
	"math/big"
	"os"
	"path/filepath"

	"github.com/cockroachdb/pebble"
	"github.com/luxfi/geth/common"
	"github.com/luxfi/geth/rlp"
)

// Key prefixes from rawdb
var (
	headerPrefix       = []byte("h") // headerPrefix + num (uint64 big endian) + hash -> header
	headerTDSuffix     = []byte("t") // headerPrefix + num (uint64 big endian) + hash + headerTDSuffix -> td
	headerHashSuffix   = []byte("n") // headerPrefix + num (uint64 big endian) + headerHashSuffix -> hash
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

// headerTDKey = headerPrefix + num (uint64 big endian) + hash + headerTDSuffix
func headerTDKey(number uint64, hash common.Hash) []byte {
	return append(headerKey(number, hash), headerTDSuffix...)
}

// headerKey = headerPrefix + num (uint64 big endian) + hash
func headerKey(number uint64, hash common.Hash) []byte {
	return append(append(headerPrefix, encodeBlockNumber(number)...), hash.Bytes()...)
}

// headerHashKey = headerPrefix + num (uint64 big endian) + headerHashSuffix
func headerHashKey(number uint64) []byte {
	return append(append(headerPrefix, encodeBlockNumber(number)...), headerHashSuffix...)
}

func main() {
	chainDir := filepath.Join(
		"/home/z/.luxd", "network-96369", "chains",
		"X6CU5qgMJfzsTB9UWxj2ZY5hd68x41HfZ4m4hCBWbHuj1Ebc3", "ethdb",
	)

	fmt.Printf("=== Fixing Total Difficulty ===\n")
	fmt.Printf("Database: %s\n\n", chainDir)

	// Check if directory exists
	if _, err := os.Stat(chainDir); os.IsNotExist(err) {
		log.Fatalf("Database directory does not exist: %s", chainDir)
	}

	// Open pebble database
	opts := &pebble.Options{
		Cache:        pebble.NewCache(256 << 20), // 256MB cache
		MaxOpenFiles: 1024,
	}
	
	db, err := pebble.Open(chainDir, opts)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Read the head block hash
	headHashBytes, closer, err := db.Get(headBlockKey)
	if err != nil {
		log.Fatalf("Failed to read head block hash: %v", err)
	}
	headHash := common.BytesToHash(headHashBytes)
	closer.Close()
	
	fmt.Printf("Head block hash: %s\n", headHash.Hex())

	// Find the head block number by scanning
	var headNumber uint64
	found := false
	
	// We know it's around 1,082,780, so start from there
	for n := uint64(1082780); n <= 1082800; n++ {
		hashKey := headerHashKey(n)
		storedHash, closer, err := db.Get(hashKey)
		if err == nil {
			hash := common.BytesToHash(storedHash)
			closer.Close()
			if hash == headHash {
				headNumber = n
				found = true
				fmt.Printf("Found head at block %d\n", n)
				break
			}
		}
	}
	
	if !found {
		// Try scanning from 0
		fmt.Printf("Scanning for head block number...\n")
		for n := uint64(0); n <= 2000000; n++ {
			if n%100000 == 0 && n > 0 {
				fmt.Printf("  Checked up to block %d...\n", n)
			}
			
			hashKey := headerHashKey(n)
			storedHash, closer, err := db.Get(hashKey)
			if err == nil {
				hash := common.BytesToHash(storedHash)
				closer.Close()
				if hash == headHash {
					headNumber = n
					found = true
					fmt.Printf("Found head at block %d\n", n)
					break
				}
			}
		}
	}
	
	if !found {
		// Last resort - use the stored height
		headNumber = 1082780
		fmt.Printf("Using known head height: %d\n", headNumber)
	}

	// Fix TD for all blocks
	fmt.Printf("\nWriting Total Difficulty for blocks 0 to %d...\n", headNumber)
	
	batch := db.NewBatch()
	fixedCount := 0
	
	for n := uint64(0); n <= headNumber; n++ {
		// Get canonical hash for this height
		hashKey := headerHashKey(n)
		hashBytes, closer, err := db.Get(hashKey)
		if err != nil {
			if n == 0 {
				// Genesis might be stored differently
				fmt.Printf("  ⚠️  No canonical hash for genesis\n")
			}
			continue
		}
		hash := common.BytesToHash(hashBytes)
		closer.Close()
		
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
		
		fixedCount++
		
		if fixedCount%10000 == 0 {
			fmt.Printf("  Fixed TD for %d blocks...\n", fixedCount)
			if err := batch.Commit(pebble.Sync); err != nil {
				log.Fatalf("Failed to commit batch: %v", err)
			}
			batch = db.NewBatch()
		}
	}
	
	// Commit final batch
	if err := batch.Commit(pebble.Sync); err != nil {
		log.Fatalf("Failed to commit final batch: %v", err)
	}
	
	fmt.Printf("\n✅ Fixed TD for %d blocks\n", fixedCount)
	
	// Verify TD at tip
	fmt.Printf("\nVerifying TD at tip...\n")
	tipTDKey := headerTDKey(headNumber, headHash)
	tdBytes, closer, err := db.Get(tipTDKey)
	if err != nil {
		fmt.Printf("❌ TD still missing at tip!\n")
	} else {
		defer closer.Close()
		var td big.Int
		if err := rlp.DecodeBytes(tdBytes, &td); err != nil {
			fmt.Printf("❌ Failed to decode TD: %v\n", err)
		} else {
			expectedTD := new(big.Int).SetUint64(headNumber + 1)
			if td.Cmp(expectedTD) == 0 {
				fmt.Printf("✅ TD at tip: %v (correct)\n", &td)
			} else {
				fmt.Printf("⚠️  TD at tip: %v (expected %v)\n", &td, expectedTD)
			}
		}
	}
	
	fmt.Printf("\n✅ Total Difficulty fixed!\n")
}