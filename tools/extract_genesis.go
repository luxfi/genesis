package main

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/cockroachdb/pebble"
	"github.com/ethereum/go-ethereum/common"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Println("Usage: extract_genesis <pebbledb_path>")
		os.Exit(1)
	}

	dbPath := os.Args[1]
	fmt.Printf("Extracting genesis block (block 0) from: %s\n\n", dbPath)

	// Open PebbleDB
	db, err := pebble.Open(dbPath, &pebble.Options{ReadOnly: true})
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Search for genesis block data
	var genesisHeader []byte
	var genesisBody []byte
	var genesisHash common.Hash
	var canonicalHash common.Hash

	// Create iterator
	iter, err := db.NewIter(&pebble.IterOptions{})
	if err != nil {
		log.Fatalf("Failed to create iterator: %v", err)
	}
	defer iter.Close()

	fmt.Println("Scanning database for genesis block data...")
	count := 0

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

		// Try to detect and handle namespaced keys
		originalKey := key
		
		// Check if this could be a namespaced key by looking for standard ethereum patterns at different offsets
		for offset := 0; offset <= len(key)-41; offset++ {
			if offset > 50 { // reasonable namespace size limit
				break
			}
			
			testKey := key[offset:]
			
			// Check for header key: 'h' + 8-byte block number + 32-byte hash
			if len(testKey) >= 41 && testKey[0] == 'h' {
				blockNum := binary.BigEndian.Uint64(testKey[1:9])
				if blockNum == 0 && len(testKey) == 41 {
					// Found potential genesis header
					hash := common.BytesToHash(testKey[9:41])
					fmt.Printf("\n=== GENESIS HEADER FOUND ===\n")
					fmt.Printf("Original Key (%d bytes): %s\n", len(originalKey), hex.EncodeToString(originalKey))
					if offset > 0 {
						fmt.Printf("Namespace (%d bytes): %s\n", offset, hex.EncodeToString(key[:offset]))
					}
					fmt.Printf("Ethereum Key: %s\n", hex.EncodeToString(testKey))
					fmt.Printf("Block Number: %d\n", blockNum)
					fmt.Printf("Block Hash: %s\n", hash.Hex())
					fmt.Printf("Header Data (%d bytes): %s\n", len(val), hex.EncodeToString(val))
					
					genesisHeader = make([]byte, len(val))
					copy(genesisHeader, val)
					genesisHash = hash
					fmt.Println()
				}
			}
			
			// Check for body key: 'b' + 8-byte block number + 32-byte hash
			if len(testKey) >= 41 && testKey[0] == 'b' {
				blockNum := binary.BigEndian.Uint64(testKey[1:9])
				if blockNum == 0 && len(testKey) == 41 {
					// Found potential genesis body
					hash := common.BytesToHash(testKey[9:41])
					fmt.Printf("\n=== GENESIS BODY FOUND ===\n")
					fmt.Printf("Original Key (%d bytes): %s\n", len(originalKey), hex.EncodeToString(originalKey))
					if offset > 0 {
						fmt.Printf("Namespace (%d bytes): %s\n", offset, hex.EncodeToString(key[:offset]))
					}
					fmt.Printf("Ethereum Key: %s\n", hex.EncodeToString(testKey))
					fmt.Printf("Block Number: %d\n", blockNum)
					fmt.Printf("Block Hash: %s\n", hash.Hex())
					fmt.Printf("Body Data (%d bytes): %s\n", len(val), hex.EncodeToString(val))
					
					genesisBody = make([]byte, len(val))
					copy(genesisBody, val)
					fmt.Println()
				}
			}
			
			// Check for canonical key: 'h' + 8-byte block number + 'n'
			if len(testKey) >= 10 && testKey[0] == 'h' && len(testKey) == 10 && testKey[9] == 'n' {
				blockNum := binary.BigEndian.Uint64(testKey[1:9])
				if blockNum == 0 {
					// Found canonical mapping for block 0
					hash := common.BytesToHash(val)
					fmt.Printf("\n=== CANONICAL MAPPING FOR BLOCK 0 ===\n")
					fmt.Printf("Original Key (%d bytes): %s\n", len(originalKey), hex.EncodeToString(originalKey))
					if offset > 0 {
						fmt.Printf("Namespace (%d bytes): %s\n", offset, hex.EncodeToString(key[:offset]))
					}
					fmt.Printf("Canonical Key: %s\n", hex.EncodeToString(testKey))
					fmt.Printf("Block Number: %d\n", blockNum)
					fmt.Printf("Canonical Hash: %s\n", hash.Hex())
					
					canonicalHash = hash
					fmt.Println()
				}
			}
		}

		// Stop after processing reasonable number of keys or if we found both header and body
		if (genesisHeader != nil && genesisBody != nil) || count > 2000000 {
			break
		}
	}

	fmt.Printf("\nProcessed %d keys total\n\n", count)

	// Print results summary
	fmt.Println(strings.Repeat("=", 60))
	fmt.Println("GENESIS BLOCK EXTRACTION SUMMARY")
	fmt.Println(strings.Repeat("=", 60))

	if genesisHeader != nil {
		fmt.Printf("✓ Genesis Header Found: %d bytes\n", len(genesisHeader))
		fmt.Printf("  Block Hash: %s\n", genesisHash.Hex())
	} else {
		fmt.Println("✗ Genesis Header Not Found")
	}

	if genesisBody != nil {
		fmt.Printf("✓ Genesis Body Found: %d bytes\n", len(genesisBody))
	} else {
		fmt.Println("✗ Genesis Body Not Found")
	}

	if canonicalHash != (common.Hash{}) {
		fmt.Printf("✓ Canonical Mapping Found: %s\n", canonicalHash.Hex())
	} else {
		fmt.Println("✗ Canonical Mapping Not Found")
	}

	// If we found data, provide the raw hex for analysis
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("RAW GENESIS DATA (HEX)")
	fmt.Println(strings.Repeat("=", 60))

	if genesisHeader != nil {
		fmt.Println("\nGenesis Header (Raw Hex):")
		fmt.Println(hex.EncodeToString(genesisHeader))
	}

	if genesisBody != nil {
		fmt.Println("\nGenesis Body (Raw Hex):")
		fmt.Println(hex.EncodeToString(genesisBody))
	}

	fmt.Println()
}