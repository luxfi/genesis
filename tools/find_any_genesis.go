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
		fmt.Println("Usage: find_any_genesis <pebbledb_path>")
		os.Exit(1)
	}

	dbPath := os.Args[1]
	fmt.Printf("Searching for ANY genesis block (block 0): %s\n\n", dbPath)

	// Known namespace from analysis
	namespace, _ := hex.DecodeString("337fb73f9bcdac8c31a2d5f7b877ab1e8a2b7f2a1e9bf02a0a0e6c6fd164f1d10000")
	fmt.Printf("Using namespace: %s (%d bytes)\n\n", hex.EncodeToString(namespace), len(namespace))

	// Open PebbleDB
	db, err := pebble.Open(dbPath, &pebble.Options{ReadOnly: true})
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Scan for any genesis blocks
	fmt.Println("Scanning for any block 0 entries...")
	iter, err := db.NewIter(&pebble.IterOptions{})
	if err != nil {
		log.Fatalf("Failed to create iterator: %v", err)
	}
	defer iter.Close()

	count := 0
	foundHeaders := 0
	foundBodies := 0
	foundCanonical := 0

	for iter.First(); iter.Valid(); iter.Next() {
		key := iter.Key()
		count++

		// Check if key starts with namespace and then has Ethereum format
		if len(key) >= len(namespace)+10 { // minimum for canonical key
			if string(key[:len(namespace)]) == string(namespace) {
				ethKey := key[len(namespace):]
				
				// Check for header key: 'h' + 8-byte block number + 32-byte hash
				if len(ethKey) == 41 && ethKey[0] == 'h' && ethKey[9] != 'n' {
					blockNum := binary.BigEndian.Uint64(ethKey[1:9])
					if blockNum == 0 {
						hash := common.BytesToHash(ethKey[9:41])
						val, err := iter.ValueAndErr()
						if err == nil {
							foundHeaders++
							fmt.Printf("\n=== GENESIS HEADER #%d ===\n", foundHeaders)
							fmt.Printf("Full Key: %s\n", hex.EncodeToString(key))
							fmt.Printf("Ethereum Key: %s\n", hex.EncodeToString(ethKey))
							fmt.Printf("Block Hash: %s\n", hash.Hex())
							fmt.Printf("Header Size: %d bytes\n", len(val))
							fmt.Printf("Header Data: %s\n", hex.EncodeToString(val))
						}
					}
				}
				
				// Check for body key: 'b' + 8-byte block number + 32-byte hash  
				if len(ethKey) == 41 && ethKey[0] == 'b' {
					blockNum := binary.BigEndian.Uint64(ethKey[1:9])
					if blockNum == 0 {
						hash := common.BytesToHash(ethKey[9:41])
						val, err := iter.ValueAndErr()
						if err == nil {
							foundBodies++
							fmt.Printf("\n=== GENESIS BODY #%d ===\n", foundBodies)
							fmt.Printf("Full Key: %s\n", hex.EncodeToString(key))
							fmt.Printf("Ethereum Key: %s\n", hex.EncodeToString(ethKey))
							fmt.Printf("Block Hash: %s\n", hash.Hex())
							fmt.Printf("Body Size: %d bytes\n", len(val))
							fmt.Printf("Body Data: %s\n", hex.EncodeToString(val))
						}
					}
				}
				
				// Check for canonical key: 'h' + 8-byte block number + 'n'
				if len(ethKey) == 10 && ethKey[0] == 'h' && ethKey[9] == 'n' {
					blockNum := binary.BigEndian.Uint64(ethKey[1:9])
					if blockNum == 0 {
						val, err := iter.ValueAndErr()
						if err == nil {
							hash := common.BytesToHash(val)
							foundCanonical++
							fmt.Printf("\n=== CANONICAL MAPPING #%d ===\n", foundCanonical)
							fmt.Printf("Full Key: %s\n", hex.EncodeToString(key))
							fmt.Printf("Ethereum Key: %s\n", hex.EncodeToString(ethKey))
							fmt.Printf("Block Number: %d\n", blockNum)
							fmt.Printf("Canonical Hash: %s\n", hash.Hex())
						}
					}
				}
			}
		}

		if count%500000 == 0 {
			fmt.Printf("Scanned %d keys... (H:%d B:%d C:%d)\n", count, foundHeaders, foundBodies, foundCanonical)
		}
		
		// Continue scanning until we find something or finish
		if foundHeaders > 0 && foundBodies > 0 && foundCanonical > 0 {
			break
		}
		
		if count > 5000000 {
			break
		}
	}

	fmt.Printf("\n" + strings.Repeat("=", 80) + "\n")
	fmt.Printf("SCAN RESULTS\n")
	fmt.Printf(strings.Repeat("=", 80) + "\n")
	fmt.Printf("Total keys scanned: %d\n", count)
	fmt.Printf("Genesis headers found: %d\n", foundHeaders)
	fmt.Printf("Genesis bodies found: %d\n", foundBodies)
	fmt.Printf("Canonical mappings found: %d\n", foundCanonical)

	if foundHeaders == 0 && foundBodies == 0 && foundCanonical == 0 {
		fmt.Println("\n⚠️  NO GENESIS BLOCK DATA FOUND")
		fmt.Println("This database may not contain the genesis block or uses a different format.")
		
		// Let's check what the first few blocks are
		fmt.Println("\nChecking first few blocks in database...")
		iter.Close()
		iter, _ = db.NewIter(&pebble.IterOptions{})
		defer iter.Close()
		
		blockNums := make(map[uint64]bool)
		count = 0
		
		for iter.First(); iter.Valid() && count < 1000000; iter.Next() {
			key := iter.Key()
			count++
			
			if len(key) >= len(namespace)+41 {
				if string(key[:len(namespace)]) == string(namespace) {
					ethKey := key[len(namespace):]
					
					if len(ethKey) == 41 && ethKey[0] == 'h' && ethKey[9] != 'n' {
						blockNum := binary.BigEndian.Uint64(ethKey[1:9])
						if blockNum < 10 { // Only first 10 blocks
							if !blockNums[blockNum] {
								blockNums[blockNum] = true
								hash := common.BytesToHash(ethKey[9:41])
								fmt.Printf("Block %d: %s\n", blockNum, hash.Hex())
							}
						}
					}
				}
			}
		}
	}
}