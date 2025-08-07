package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/cockroachdb/pebble"
)

func main() {
	var dbPath string
	var limit int
	var prefix string

	flag.StringVar(&dbPath, "db", "", "Path to database")
	flag.IntVar(&limit, "limit", 100, "Number of keys to display")
	flag.StringVar(&prefix, "prefix", "", "Key prefix to filter (hex)")
	flag.Parse()

	if dbPath == "" {
		fmt.Println("Usage: dbinspect -db /path/to/db [-limit 100] [-prefix ff]")
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

	// Create iterator
	iter, err := db.NewIter(nil)
	if err != nil {
		fmt.Printf("Failed to create iterator: %v\n", err)
		os.Exit(1)
	}
	defer iter.Close()

	// Parse prefix if provided
	var prefixBytes []byte
	if prefix != "" {
		prefixBytes, err = hex.DecodeString(prefix)
		if err != nil {
			fmt.Printf("Invalid hex prefix: %v\n", err)
			os.Exit(1)
		}
	}

	// Analyze key patterns
	keyStats := make(map[byte]int)
	count := 0

	fmt.Println("=== Database Key Analysis ===")
	fmt.Printf("Database: %s\n", filepath.Base(dbPath))
	fmt.Println()

	for iter.First(); iter.Valid() && count < limit; iter.Next() {
		key := iter.Key()
		
		// Skip if doesn't match prefix
		if prefixBytes != nil && len(key) > 0 {
			if len(key) < len(prefixBytes) || string(key[:len(prefixBytes)]) != string(prefixBytes) {
				continue
			}
		}

		value := iter.Value()
		
		// Count key prefixes
		if len(key) > 0 {
			keyStats[key[0]]++
		}

		// Display key info
		keyHex := hex.EncodeToString(key)
		valueLen := len(value)
		
		// Decode key pattern
		pattern := ""
		if len(key) > 0 {
			switch key[0] {
			case 'H': // C-chain canonical hash
				pattern = "Canonical Hash"
			case 'h': // C-chain header
				pattern = "Header"
			case 'b': // C-chain body
				pattern = "Body"
			case 'r': // C-chain receipts
				pattern = "Receipts"
			case 't': // C-chain transactions
				pattern = "Transactions"
			case 0x33: // SubnetEVM prefix '3'
				if len(key) > 1 {
					pattern = fmt.Sprintf("SubnetEVM (0x33) subkey=0x%02x", key[1])
				}
			case 0x73: // 's' - state trie
				pattern = "State Trie"
			case 0x63: // 'c' - code
				pattern = "Code"
			default:
				pattern = fmt.Sprintf("Unknown (0x%02x)", key[0])
			}
		}

		fmt.Printf("Key: %s (len=%d) | Value: %d bytes | %s\n", 
			keyHex, len(key), valueLen, pattern)
		
		count++
	}

	if err := iter.Error(); err != nil {
		fmt.Printf("Iterator error: %v\n", err)
	}

	// Print statistics
	fmt.Println("\n=== Key Prefix Statistics ===")
	for prefix, count := range keyStats {
		fmt.Printf("Prefix 0x%02x ('%c'): %d keys\n", prefix, prefix, count)
	}
}