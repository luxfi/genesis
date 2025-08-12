package main

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/dgraph-io/badger/v4"
	"github.com/ethereum/go-ethereum/common"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Println("Usage: inspect_badger_detailed <badgerdb_path>")
		os.Exit(1)
	}

	dbPath := os.Args[1]
	fmt.Printf("Inspecting BadgerDB in detail: %s\n\n", dbPath)

	// Open BadgerDB
	opts := badger.DefaultOptions(dbPath)
	opts.Logger = nil
	db, err := badger.Open(opts)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Search for genesis block and show detailed information
	var genesisHeader []byte
	var genesisBody []byte
	var genesisHash common.Hash
	
	fmt.Println("Searching for genesis block (block 0)...")
	
	err = db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = true
		it := txn.NewIterator(opts)
		defer it.Close()

		count := 0
		for it.Rewind(); it.Valid(); it.Next() {
			key := it.Item().Key()
			count++

			// Check for header key: 'h' + 8-byte block number + 32-byte hash
			if len(key) == 41 && key[0] == 'h' && key[9] != 'n' {
				blockNum := binary.BigEndian.Uint64(key[1:9])
				if blockNum == 0 {
					// Found genesis header
					hash := common.BytesToHash(key[9:41])
					val, err := it.Item().ValueCopy(nil)
					if err != nil {
						return err
					}
					
					fmt.Printf("\n=== GENESIS HEADER FOUND ===\n")
					fmt.Printf("Key: %s\n", hex.EncodeToString(key))
					fmt.Printf("Block Number: %d\n", blockNum)
					fmt.Printf("Block Hash: %s\n", hash.Hex())
					fmt.Printf("Header Size: %d bytes\n", len(val))
					fmt.Printf("Header Data: %s\n", hex.EncodeToString(val))
					
					genesisHeader = make([]byte, len(val))
					copy(genesisHeader, val)
					genesisHash = hash
					fmt.Println()
				}
			}
			
			// Check for body key: 'b' + 8-byte block number + 32-byte hash
			if len(key) == 41 && key[0] == 'b' {
				blockNum := binary.BigEndian.Uint64(key[1:9])
				if blockNum == 0 {
					// Found genesis body
					hash := common.BytesToHash(key[9:41])
					val, err := it.Item().ValueCopy(nil)
					if err != nil {
						return err
					}
					
					fmt.Printf("\n=== GENESIS BODY FOUND ===\n")
					fmt.Printf("Key: %s\n", hex.EncodeToString(key))
					fmt.Printf("Block Number: %d\n", blockNum)
					fmt.Printf("Block Hash: %s\n", hash.Hex())
					fmt.Printf("Body Size: %d bytes\n", len(val))
					fmt.Printf("Body Data: %s\n", hex.EncodeToString(val))
					
					genesisBody = make([]byte, len(val))
					copy(genesisBody, val)
					fmt.Println()
				}
			}
			
			// Check for canonical key: 'h' + 8-byte block number + 'n'
			if len(key) == 10 && key[0] == 'h' && key[9] == 'n' {
				blockNum := binary.BigEndian.Uint64(key[1:9])
				if blockNum == 0 {
					// Found canonical mapping for block 0
					val, err := it.Item().ValueCopy(nil)
					if err != nil {
						return err
					}
					hash := common.BytesToHash(val)
					
					fmt.Printf("\n=== CANONICAL MAPPING FOR BLOCK 0 ===\n")
					fmt.Printf("Key: %s\n", hex.EncodeToString(key))
					fmt.Printf("Block Number: %d\n", blockNum)
					fmt.Printf("Canonical Hash: %s\n", hash.Hex())
					fmt.Println()
				}
			}

			// Show first 20 keys for debugging
			if count <= 20 {
				val, err := it.Item().ValueCopy(nil)
				keyType := "unknown"
				if len(key) == 41 && key[0] == 'h' && key[9] != 'n' {
					keyType = "header"
					blockNum := binary.BigEndian.Uint64(key[1:9])
					fmt.Printf("[%s] Block %d: %s (val: %d bytes)\n", keyType, blockNum, hex.EncodeToString(key), len(val))
				} else if len(key) == 10 && key[0] == 'h' && key[9] == 'n' {
					keyType = "canonical"
					blockNum := binary.BigEndian.Uint64(key[1:9])
					fmt.Printf("[%s] Block %d: %s (val: %d bytes)\n", keyType, blockNum, hex.EncodeToString(key), len(val))
				} else if len(key) == 41 && key[0] == 'b' {
					keyType = "body"
					blockNum := binary.BigEndian.Uint64(key[1:9])
					fmt.Printf("[%s] Block %d: %s (val: %d bytes)\n", keyType, blockNum, hex.EncodeToString(key), len(val))
				} else {
					fmt.Printf("[%s] %s (val: %d bytes)\n", keyType, hex.EncodeToString(key), len(val))
				}
				if err != nil {
					fmt.Printf("  Error reading value: %v\n", err)
				}
			}

			if count%50000 == 0 {
				fmt.Printf("Processed %d keys...\n", count)
			}
		}
		
		fmt.Printf("\nTotal keys processed: %d\n", count)
		return nil
	})

	if err != nil {
		log.Fatalf("Failed to iterate: %v", err)
	}

	// Print final summary
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("GENESIS BLOCK SUMMARY")
	fmt.Println(strings.Repeat("=", 60))

	if genesisHeader != nil {
		fmt.Printf("✓ Genesis Header: %d bytes\n", len(genesisHeader))
		fmt.Printf("  Block Hash: %s\n", genesisHash.Hex())
		fmt.Printf("  Raw Header: %s\n", hex.EncodeToString(genesisHeader))
	} else {
		fmt.Println("✗ Genesis Header: Not found")
	}

	if genesisBody != nil {
		fmt.Printf("✓ Genesis Body: %d bytes\n", len(genesisBody))
		fmt.Printf("  Raw Body: %s\n", hex.EncodeToString(genesisBody))
	} else {
		fmt.Println("✗ Genesis Body: Not found")
	}
}