package main

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"log"
	"os"

	"github.com/cockroachdb/pebble"
	"github.com/ethereum/go-ethereum/common"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Println("Usage: analyze_namespace <pebbledb_path>")
		os.Exit(1)
	}

	dbPath := os.Args[1]
	fmt.Printf("Analyzing namespace structure in: %s\n\n", dbPath)

	// Open PebbleDB
	db, err := pebble.Open(dbPath, &pebble.Options{ReadOnly: true})
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Look for the expected genesis hash
	expectedGenesisHash := common.HexToHash("0x3f4fa2a0b0ce089f52bf0ae9199c75ffdd76ecafc987794050cb0d286f1ec61e")
	fmt.Printf("Looking for expected genesis hash: %s\n\n", expectedGenesisHash.Hex())

	// Create iterator
	iter, err := db.NewIter(&pebble.IterOptions{})
	if err != nil {
		log.Fatalf("Failed to create iterator: %v", err)
	}
	defer iter.Close()

	fmt.Println("Analyzing key patterns to detect namespace...")

	count := 0
	var commonPrefix []byte
	var prefixLength = -1

	for iter.First(); iter.Valid() && count < 100; iter.Next() {
		key := iter.Key()
		val, err := iter.ValueAndErr()
		if err != nil {
			continue
		}

		keyHex := hex.EncodeToString(key)
		fmt.Printf("Key[%d] (%d bytes): %s\n", count, len(key), keyHex)

		// Analyze key structure to find potential namespace
		if count == 0 {
			commonPrefix = make([]byte, len(key))
			copy(commonPrefix, key)
			prefixLength = len(key)
		} else {
			// Find common prefix with all previous keys
			newPrefixLength := 0
			for i := 0; i < min(prefixLength, len(key)); i++ {
				if commonPrefix[i] == key[i] {
					newPrefixLength = i + 1
				} else {
					break
				}
			}
			prefixLength = newPrefixLength
		}

		// Look for patterns that might indicate Ethereum keys after namespace stripping
		for offset := 0; offset <= len(key)-41; offset++ {
			if offset > 50 { // reasonable limit
				break
			}
			testKey := key[offset:]
			
			// Check for header with the expected genesis hash
			if len(testKey) >= 41 && testKey[0] == 'h' {
				blockNum := binary.BigEndian.Uint64(testKey[1:9])
				hash := common.BytesToHash(testKey[9:41])
				
				if hash == expectedGenesisHash && blockNum == 0 {
					fmt.Printf("*** FOUND GENESIS HEADER ***\n")
					fmt.Printf("    Namespace: %s (%d bytes)\n", hex.EncodeToString(key[:offset]), offset)
					fmt.Printf("    Ethereum Key: %s\n", hex.EncodeToString(testKey))
					fmt.Printf("    Block Number: %d\n", blockNum)
					fmt.Printf("    Block Hash: %s\n", hash.Hex())
					fmt.Printf("    Header Data (%d bytes): %s\n", len(val), hex.EncodeToString(val))
					return
				}
				
				// Show any block 0 headers even if hash doesn't match
				if blockNum == 0 {
					fmt.Printf("    -> Block 0 header (offset %d): hash=%s\n", offset, hash.Hex())
				}
			}
			
			// Check for body with the expected genesis hash
			if len(testKey) >= 41 && testKey[0] == 'b' {
				blockNum := binary.BigEndian.Uint64(testKey[1:9])
				hash := common.BytesToHash(testKey[9:41])
				
				if hash == expectedGenesisHash && blockNum == 0 {
					fmt.Printf("*** FOUND GENESIS BODY ***\n")
					fmt.Printf("    Namespace: %s (%d bytes)\n", hex.EncodeToString(key[:offset]), offset)
					fmt.Printf("    Ethereum Key: %s\n", hex.EncodeToString(testKey))
					fmt.Printf("    Block Number: %d\n", blockNum)
					fmt.Printf("    Block Hash: %s\n", hash.Hex())
					fmt.Printf("    Body Data (%d bytes): %s\n", len(val), hex.EncodeToString(val))
					return
				}
				
				// Show any block 0 bodies
				if blockNum == 0 {
					fmt.Printf("    -> Block 0 body (offset %d): hash=%s\n", offset, hash.Hex())
				}
			}
		}

		count++
	}

	fmt.Printf("\nAnalyzed %d keys\n", count)
	if prefixLength > 0 {
		fmt.Printf("Common prefix found: %s (%d bytes)\n", hex.EncodeToString(commonPrefix[:prefixLength]), prefixLength)
	} else {
		fmt.Println("No common prefix found")
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}