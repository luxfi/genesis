package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"os"

	"github.com/cockroachdb/pebble"
	"github.com/luxfi/geth/common"
	"github.com/luxfi/geth/core/types"
	"github.com/luxfi/geth/rlp"
)

// SubnetEVM key prefixes - found by analyzing actual DB
const (
	// Main prefix for SubnetEVM
	subnetPrefix = 0x33
	
	// Known sub-prefixes
	subStatePrefix = 0x7f // State trie nodes
)

func main() {
	var dbPath string
	var findBlocks bool

	flag.StringVar(&dbPath, "db", "", "Path to SubnetEVM database")
	flag.BoolVar(&findBlocks, "blocks", false, "Find and decode blocks")
	flag.Parse()

	if dbPath == "" {
		fmt.Println("Usage: subnet-decode -db /path/to/db [-blocks]")
		os.Exit(1)
	}

	// Open database
	opts := &pebble.Options{
		ReadOnly: true,
	}
	
	db, err := pebble.Open(dbPath, opts)
	if err != nil {
		fmt.Printf("Failed to open database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	if findBlocks {
		findBlockData(db)
	} else {
		analyzeKeyPatterns(db)
	}
}

func analyzeKeyPatterns(db *pebble.DB) {
	fmt.Println("=== SubnetEVM Key Pattern Analysis ===")
	
	// Create iterator
	iter, err := db.NewIter(nil)
	if err != nil {
		fmt.Printf("Failed to create iterator: %v\n", err)
		return
	}
	defer iter.Close()

	patterns := make(map[string]int)
	samples := make(map[string][]string)
	
	count := 0
	for iter.First(); iter.Valid() && count < 1000; iter.Next() {
		key := iter.Key()
		value := iter.Value()
		
		if len(key) < 2 {
			continue
		}
		
		// Analyze key structure
		pattern := analyzeKey(key, value)
		patterns[pattern]++
		
		// Keep samples
		if len(samples[pattern]) < 3 {
			samples[pattern] = append(samples[pattern], hex.EncodeToString(key))
		}
		
		count++
	}

	// Print analysis
	fmt.Println("\nKey Patterns Found:")
	for pattern, count := range patterns {
		fmt.Printf("\n%s: %d occurrences\n", pattern, count)
		fmt.Println("Sample keys:")
		for _, sample := range samples[pattern] {
			fmt.Printf("  %s\n", sample)
		}
	}
}

func analyzeKey(key, value []byte) string {
	if len(key) == 0 {
		return "empty"
	}

	// Check for SubnetEVM prefix
	if key[0] != subnetPrefix {
		return fmt.Sprintf("non-subnet (0x%02x)", key[0])
	}

	if len(key) < 2 {
		return "subnet-short"
	}

	// Analyze second byte
	switch key[1] {
	case 0x7f:
		// State trie - check if it's account data
		if len(value) > 0 {
			var acc types.StateAccount
			if err := rlp.DecodeBytes(value, &acc); err == nil {
				return "subnet-state-account"
			}
		}
		return "subnet-state-trie"
	case 0x48: // 'H'
		return "subnet-canonical-hash"
	case 0x68: // 'h'
		return "subnet-header"
	case 0x62: // 'b'
		return "subnet-body"
	case 0x72: // 'r'
		return "subnet-receipts"
	case 0x74: // 't'
		return "subnet-transaction"
	case 0x6c: // 'l'
		return "subnet-transaction-lookup"
	default:
		return fmt.Sprintf("subnet-unknown (0x%02x)", key[1])
	}
}

func findBlockData(db *pebble.DB) {
	fmt.Println("=== Searching for Block Data ===")
	
	// Look for different potential block storage patterns
	iter, err := db.NewIter(nil)
	if err != nil {
		fmt.Printf("Failed to create iterator: %v\n", err)
		return
	}
	defer iter.Close()

	blockHashes := []common.Hash{}
	blockNumbers := []uint64{}
	
	// First pass: find potential block markers
	for iter.First(); iter.Valid(); iter.Next() {
		key := iter.Key()
		value := iter.Value()
		
		// Try to decode as header
		if couldBeHeader(key, value) {
			fmt.Printf("Potential header at key: %s\n", hex.EncodeToString(key))
			
			var header types.Header
			if err := rlp.DecodeBytes(value, &header); err == nil {
				fmt.Printf("  Block #%d, Hash: %s\n", header.Number.Uint64(), header.Hash().Hex())
				blockHashes = append(blockHashes, header.Hash())
				blockNumbers = append(blockNumbers, header.Number.Uint64())
			}
		}
		
		// Look for canonical number mappings
		if couldBeCanonicalMapping(key, value) {
			if len(value) == 32 {
				hash := common.BytesToHash(value)
				fmt.Printf("Potential canonical mapping: key=%s -> hash=%s\n", 
					hex.EncodeToString(key), hash.Hex())
			}
		}
	}

	fmt.Printf("\nFound %d potential blocks\n", len(blockHashes))
}

func couldBeHeader(key, value []byte) bool {
	// Headers are typically 500+ bytes when RLP encoded
	if len(value) < 400 || len(value) > 2000 {
		return false
	}
	
	// Try to decode as header
	var header types.Header
	return rlp.DecodeBytes(value, &header) == nil
}

func couldBeCanonicalMapping(key, value []byte) bool {
	// Canonical mappings are number->hash, so value should be 32 bytes
	return len(value) == 32 && len(key) >= 8
}