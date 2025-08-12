package main

import (
	"encoding/hex"
	"fmt"
	"log"
	"os"

	"github.com/cockroachdb/pebble"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Println("Usage: find_genesis <pebbledb_path>")
		os.Exit(1)
	}

	dbPath := os.Args[1]
	fmt.Printf("Searching for genesis block (block 0) in: %s\n\n", dbPath)

	// Open PebbleDB
	db, err := pebble.Open(dbPath, &pebble.Options{ReadOnly: true})
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Create iterator
	iter, err := db.NewIter(&pebble.IterOptions{})
	if err != nil {
		log.Fatalf("Failed to create iterator: %v", err)
	}
	defer iter.Close()

	fmt.Println("Searching for potential genesis block data...")
	count := 0
	genesisCount := 0

	for iter.First(); iter.Valid(); iter.Next() {
		key := iter.Key()
		val, err := iter.ValueAndErr()
		if err != nil {
			continue
		}

		count++
		if count%100000 == 0 {
			fmt.Printf("Processed %d keys...\n", count)
		}

		// Look for keys that might contain block 0 data
		// Check for 8-byte zero block number (0x0000000000000000)
		keyHex := hex.EncodeToString(key)
		
		// Look for patterns that might indicate block 0
		if len(key) >= 8 {
			// Check for 8 consecutive zero bytes somewhere in the key (block number 0)
			for i := 0; i <= len(key)-8; i++ {
				allZero := true
				for j := 0; j < 8; j++ {
					if key[i+j] != 0 {
						allZero = false
						break
					}
				}
				
				if allZero {
					genesisCount++
					fmt.Printf("\n=== POTENTIAL GENESIS BLOCK DATA #%d ===\n", genesisCount)
					fmt.Printf("Key (%d bytes): %s\n", len(key), keyHex)
					fmt.Printf("Value (%d bytes): %s\n", len(val), hex.EncodeToString(val[:min(100, len(val))]))
					if len(val) > 100 {
						fmt.Println("... (truncated)")
					}
					
					// Try to identify what type of data this might be
					if len(key) == 41 && (key[0] == 'h' || key[0] == 'b') {
						fmt.Printf("Looks like: %s key format\n", string(key[0]))
					} else if len(key) == 10 && key[9] == 'n' {
						fmt.Printf("Looks like: canonical mapping\n")
					} else if len(val) > 500 {
						fmt.Printf("Looks like: large data (possibly block body)\n")
					} else if len(val) < 100 && len(val) > 30 {
						fmt.Printf("Looks like: small data (possibly header)\n")
					}
					
					// Show first 200 chars of value for analysis
					if len(val) > 0 {
						fmt.Printf("Value preview: %s\n", hex.EncodeToString(val[:min(200, len(val))]))
						if len(val) > 200 {
							fmt.Println("...")
						}
					}
					fmt.Println()
					break // Only report one zero sequence per key
				}
			}
		}
		
		// Also check for standard Ethereum database key formats that might be block 0
		if len(key) >= 9 {
			// Header key: 'h' + 8-byte block number + 32-byte hash
			if key[0] == 'h' && len(key) == 41 {
				blockNumBytes := key[1:9]
				allZero := true
				for _, b := range blockNumBytes {
					if b != 0 {
						allZero = false
						break
					}
				}
				if allZero {
					fmt.Printf("\n=== ETHEREUM HEADER KEY FOR BLOCK 0 ===\n")
					fmt.Printf("Key: %s\n", keyHex)
					fmt.Printf("Block Hash: %s\n", hex.EncodeToString(key[9:]))
					fmt.Printf("Value (%d bytes): %s\n", len(val), hex.EncodeToString(val))
					fmt.Println()
				}
			}
			
			// Body key: 'b' + 8-byte block number + 32-byte hash
			if key[0] == 'b' && len(key) == 41 {
				blockNumBytes := key[1:9]
				allZero := true
				for _, b := range blockNumBytes {
					if b != 0 {
						allZero = false
						break
					}
				}
				if allZero {
					fmt.Printf("\n=== ETHEREUM BODY KEY FOR BLOCK 0 ===\n")
					fmt.Printf("Key: %s\n", keyHex)
					fmt.Printf("Block Hash: %s\n", hex.EncodeToString(key[9:]))
					fmt.Printf("Value (%d bytes): %s\n", len(val), hex.EncodeToString(val[:min(200, len(val))]))
					if len(val) > 200 {
						fmt.Println("... (truncated)")
					}
					fmt.Println()
				}
			}
			
			// Canonical key: 'h' + 8-byte block number + 'n'
			if key[0] == 'h' && len(key) == 10 && key[9] == 'n' {
				blockNumBytes := key[1:9]
				allZero := true
				for _, b := range blockNumBytes {
					if b != 0 {
						allZero = false
						break
					}
				}
				if allZero {
					fmt.Printf("\n=== CANONICAL MAPPING FOR BLOCK 0 ===\n")
					fmt.Printf("Key: %s\n", keyHex)
					fmt.Printf("Hash: %s\n", hex.EncodeToString(val))
					fmt.Println()
				}
			}
		}
		
		// Stop after finding a few candidates or processing a lot of keys
		if genesisCount >= 10 || count > 1000000 {
			break
		}
	}

	fmt.Printf("\nProcessed %d keys total\n", count)
	fmt.Printf("Found %d potential genesis block entries\n", genesisCount)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}