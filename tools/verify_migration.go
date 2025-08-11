package main

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	
	"github.com/dgraph-io/badger/v4"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: verify_migration <db_path>")
		os.Exit(1)
	}
	
	dbPath := os.Args[1]
	opts := badger.DefaultOptions(dbPath)
	opts.ReadOnly = true
	
	db, err := badger.Open(opts)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	
	blockCount := 0
	var genesisHash string
	var highestBlock uint64
	
	err = db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()
		
		// Check for both formats
		// Format 1: 'h' + blockNum + hash (41 bytes)
		prefix := []byte("h")
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			key := item.Key()
			if len(key) == 41 {
				blockNum := binary.BigEndian.Uint64(key[1:9])
				if blockNum == 0 {
					genesisHash = hex.EncodeToString(key[9:41])
				}
				if blockNum > highestBlock {
					highestBlock = blockNum
				}
				blockCount++
			}
		}
		
		// Format 2: 'H' + blockNum -> hash
		if blockCount == 0 {
			prefix = []byte("H")
			for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
				item := it.Item()
				key := item.Key()
				if len(key) == 9 {
					blockNum := binary.BigEndian.Uint64(key[1:9])
					if blockNum == 0 {
						val, _ := item.ValueCopy(nil)
						genesisHash = hex.EncodeToString(val)
					}
					if blockNum > highestBlock {
						highestBlock = blockNum
					}
					blockCount++
				}
			}
		}
		
		return nil
	})
	
	if err != nil {
		log.Fatal(err)
	}
	
	expectedGenesis := "3f4fa2a0b0ce089f52bf0ae9199c75ffdd76ecafc987794050cb0d286f1ec61e"
	
	fmt.Printf("Database: %s\n", dbPath)
	fmt.Printf("Total blocks: %d\n", blockCount)
	fmt.Printf("Highest block: %d\n", highestBlock)
	fmt.Printf("Genesis hash: 0x%s\n", genesisHash)
	
	if genesisHash == expectedGenesis {
		fmt.Printf("✓ PASS: Genesis hash correct\n")
		fmt.Printf("✓ PASS: Total blocks: %d\n", blockCount)
		os.Exit(0)
	} else {
		fmt.Printf("✗ FAIL: Genesis hash mismatch\n")
		fmt.Printf("  Expected: 0x%s\n", expectedGenesis)
		fmt.Printf("  Got: 0x%s\n", genesisHash)
		os.Exit(1)
	}
}