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

// Key encoding helpers
func encodeBlockNumber(number uint64) []byte {
	enc := make([]byte, 8)
	binary.BigEndian.PutUint64(enc, number)
	return enc
}

func main() {
	// The actual C-Chain database path
	chainDir := filepath.Join(
		"/home/z/.luxd", "network-96369", "chains",
		"X6CU5qgMJfzsTB9UWxj2ZY5hd68x41HfZ4m4hCBWbHuj1Ebc3", "ethdb",
	)

	fmt.Printf("=== Writing Total Difficulty to Database ===\n")
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

	// We know the tip is at 1,082,780
	tipHeight := uint64(1082780)
	
	fmt.Printf("Writing TD for blocks 0 to %d...\n", tipHeight)
	
	batch := db.NewBatch()
	written := 0
	missing := 0
	
	for n := uint64(0); n <= tipHeight; n++ {
		// Get canonical hash for this height
		// Canonical hash key: 'h' + num (8 bytes BE) + 'n'
		canonicalKey := append([]byte("h"), encodeBlockNumber(n)...)
		canonicalKey = append(canonicalKey, byte('n'))
		
		hashBytes, closer, err := db.Get(canonicalKey)
		if err != nil {
			missing++
			if n == 0 {
				// Try to find genesis with a different key pattern
				// Genesis might be stored as canonical at 0
				fmt.Printf("  Genesis canonical hash not found at expected key\n")
			}
			continue
		}
		hash := common.BytesToHash(hashBytes)
		closer.Close()
		
		// TD = height + 1 for this chain
		td := new(big.Int).SetUint64(n + 1)
		
		// Encode TD using RLP
		tdBytes, err := rlp.EncodeToBytes(td)
		if err != nil {
			log.Fatalf("Failed to encode TD: %v", err)
		}
		
		// TD key: 'h' + num (8 bytes BE) + hash (32 bytes) + 't'
		tdKey := append([]byte("h"), encodeBlockNumber(n)...)
		tdKey = append(tdKey, hash.Bytes()...)
		tdKey = append(tdKey, byte('t'))
		
		// Write TD
		if err := batch.Set(tdKey, tdBytes, nil); err != nil {
			log.Fatalf("Failed to set TD: %v", err)
		}
		
		written++
		
		// Commit batch periodically
		if written%10000 == 0 {
			fmt.Printf("  Written TD for %d blocks...\n", written)
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
	
	fmt.Printf("\n✅ Written TD for %d blocks\n", written)
	if missing > 0 {
		fmt.Printf("⚠️  %d blocks had no canonical hash\n", missing)
	}
	
	// Verify TD at specific heights
	fmt.Printf("\nVerifying TD...\n")
	
	// Check genesis (block 0)
	checkTD(db, 0)
	
	// Check midpoint
	checkTD(db, tipHeight/2)
	
	// Check tip
	checkTD(db, tipHeight)
	
	fmt.Printf("\n✅ Total Difficulty written successfully!\n")
}

func checkTD(db *pebble.DB, height uint64) {
	// Get canonical hash
	canonicalKey := append([]byte("h"), encodeBlockNumber(height)...)
	canonicalKey = append(canonicalKey, byte('n'))
	
	hashBytes, closer, err := db.Get(canonicalKey)
	if err != nil {
		fmt.Printf("  Block %d: No canonical hash\n", height)
		return
	}
	hash := common.BytesToHash(hashBytes)
	closer.Close()
	
	// Get TD
	tdKey := append([]byte("h"), encodeBlockNumber(height)...)
	tdKey = append(tdKey, hash.Bytes()...)
	tdKey = append(tdKey, byte('t'))
	
	tdBytes, closer, err := db.Get(tdKey)
	if err != nil {
		fmt.Printf("  Block %d: TD not found\n", height)
		return
	}
	defer closer.Close()
	
	var td big.Int
	if err := rlp.DecodeBytes(tdBytes, &td); err != nil {
		fmt.Printf("  Block %d: Failed to decode TD: %v\n", height, err)
		return
	}
	
	expectedTD := new(big.Int).SetUint64(height + 1)
	if td.Cmp(expectedTD) == 0 {
		fmt.Printf("  Block %d: TD = %v ✅\n", height, &td)
	} else {
		fmt.Printf("  Block %d: TD = %v (expected %v) ⚠️\n", height, &td, expectedTD)
	}
}