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
		fmt.Println("Usage: find_genesis_with_namespace <pebbledb_path>")
		os.Exit(1)
	}

	dbPath := os.Args[1]
	fmt.Printf("Searching for genesis block with known namespace: %s\n\n", dbPath)

	// Known namespace from analysis
	namespace, _ := hex.DecodeString("337fb73f9bcdac8c31a2d5f7b877ab1e8a2b7f2a1e9bf02a0a0e6c6fd164f1d10000")
	fmt.Printf("Using namespace: %s (%d bytes)\n", hex.EncodeToString(namespace), len(namespace))

	// Expected genesis hash
	expectedGenesisHash := common.HexToHash("0x3f4fa2a0b0ce089f52bf0ae9199c75ffdd76ecafc987794050cb0d286f1ec61e")
	fmt.Printf("Looking for genesis hash: %s\n\n", expectedGenesisHash.Hex())

	// Open PebbleDB
	db, err := pebble.Open(dbPath, &pebble.Options{ReadOnly: true})
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Construct the expected keys for genesis block (block 0)
	blockNum := uint64(0)

	// Genesis header key: namespace + 'h' + 8-byte block number + 32-byte hash
	genesisHeaderKey := make([]byte, len(namespace)+1+8+32)
	copy(genesisHeaderKey, namespace)
	genesisHeaderKey[len(namespace)] = 'h'
	binary.BigEndian.PutUint64(genesisHeaderKey[len(namespace)+1:len(namespace)+9], blockNum)
	copy(genesisHeaderKey[len(namespace)+9:], expectedGenesisHash.Bytes())

	// Genesis body key: namespace + 'b' + 8-byte block number + 32-byte hash
	genesisBodyKey := make([]byte, len(namespace)+1+8+32)
	copy(genesisBodyKey, namespace)
	genesisBodyKey[len(namespace)] = 'b'
	binary.BigEndian.PutUint64(genesisBodyKey[len(namespace)+1:len(namespace)+9], blockNum)
	copy(genesisBodyKey[len(namespace)+9:], expectedGenesisHash.Bytes())

	// Canonical key: namespace + 'h' + 8-byte block number + 'n'
	canonicalKey := make([]byte, len(namespace)+1+8+1)
	copy(canonicalKey, namespace)
	canonicalKey[len(namespace)] = 'h'
	binary.BigEndian.PutUint64(canonicalKey[len(namespace)+1:len(namespace)+9], blockNum)
	canonicalKey[len(namespace)+9] = 'n'

	fmt.Println("Searching for specific genesis keys...")
	fmt.Printf("Header key: %s\n", hex.EncodeToString(genesisHeaderKey))
	fmt.Printf("Body key:   %s\n", hex.EncodeToString(genesisBodyKey))
	fmt.Printf("Canon key:  %s\n\n", hex.EncodeToString(canonicalKey))

	// Search for genesis header
	headerData, headerCloser, err := db.Get(genesisHeaderKey)
	if err != nil {
		fmt.Printf("Genesis header not found: %v\n", err)
	} else {
		fmt.Println("=== GENESIS HEADER FOUND ===")
		fmt.Printf("Key: %s\n", hex.EncodeToString(genesisHeaderKey))
		fmt.Printf("Size: %d bytes\n", len(headerData))
		fmt.Printf("Data: %s\n\n", hex.EncodeToString(headerData))
		headerCloser.Close()
	}

	// Search for genesis body
	bodyData, bodyCloser, err := db.Get(genesisBodyKey)
	if err != nil {
		fmt.Printf("Genesis body not found: %v\n", err)
	} else {
		fmt.Println("=== GENESIS BODY FOUND ===")
		fmt.Printf("Key: %s\n", hex.EncodeToString(genesisBodyKey))
		fmt.Printf("Size: %d bytes\n", len(bodyData))
		fmt.Printf("Data: %s\n\n", hex.EncodeToString(bodyData))
		bodyCloser.Close()
	}

	// Search for canonical mapping
	canonicalData, canonicalCloser, err := db.Get(canonicalKey)
	if err != nil {
		fmt.Printf("Canonical mapping not found: %v\n", err)
	} else {
		fmt.Println("=== CANONICAL MAPPING FOUND ===")
		fmt.Printf("Key: %s\n", hex.EncodeToString(canonicalKey))
		fmt.Printf("Hash: %s\n\n", hex.EncodeToString(canonicalData))
		canonicalCloser.Close()
	}

	// Also try to find ANY genesis block (block 0) headers in the database by iteration
	fmt.Println("Scanning for any block 0 headers...")
	iter, err := db.NewIter(&pebble.IterOptions{})
	if err != nil {
		log.Fatalf("Failed to create iterator: %v", err)
	}
	defer iter.Close()

	count := 0
	found := 0
	for iter.First(); iter.Valid() && found < 10; iter.Next() {
		key := iter.Key()
		count++

		// Check if key starts with namespace and then has Ethereum format
		if len(key) >= len(namespace)+41 {
			if string(key[:len(namespace)]) == string(namespace) {
				ethKey := key[len(namespace):]
				
				// Check for header key
				if len(ethKey) == 41 && ethKey[0] == 'h' && ethKey[9] != 'n' {
					blockNum := binary.BigEndian.Uint64(ethKey[1:9])
					if blockNum == 0 {
						hash := common.BytesToHash(ethKey[9:41])
						val, err := iter.ValueAndErr()
						if err == nil {
							found++
							fmt.Printf("\nFound Genesis Header #%d:\n", found)
							fmt.Printf("  Full Key: %s\n", hex.EncodeToString(key))
							fmt.Printf("  Block Hash: %s\n", hash.Hex())
							fmt.Printf("  Header Size: %d bytes\n", len(val))
							fmt.Printf("  Header Data: %s\n", hex.EncodeToString(val))
						}
					}
				}
				
				// Check for body key  
				if len(ethKey) == 41 && ethKey[0] == 'b' {
					blockNum := binary.BigEndian.Uint64(ethKey[1:9])
					if blockNum == 0 {
						hash := common.BytesToHash(ethKey[9:41])
						val, err := iter.ValueAndErr()
						if err == nil {
							found++
							fmt.Printf("\nFound Genesis Body #%d:\n", found)
							fmt.Printf("  Full Key: %s\n", hex.EncodeToString(key))
							fmt.Printf("  Block Hash: %s\n", hash.Hex())
							fmt.Printf("  Body Size: %d bytes\n", len(val))
							fmt.Printf("  Body Data: %s\n", hex.EncodeToString(val))
						}
					}
				}
			}
		}

		if count%500000 == 0 {
			fmt.Printf("Scanned %d keys...\n", count)
		}
		
		// Limit scan to reasonable number
		if count > 2000000 {
			break
		}
	}

	fmt.Printf("\nScanned %d keys, found %d genesis entries\n", count, found)
}