package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"log"
	"os"

	"github.com/dgraph-io/badger/v4"
)

// Database key prefixes (matching go-ethereum rawdb)
var (
	headerPrefix       = []byte("h") // headerPrefix + num (uint64 big endian) + hash -> header
	headerHashSuffix   = []byte("n") // headerPrefix + num (uint64 big endian) + headerHashSuffix -> hash
	headerNumberPrefix = []byte("H") // headerNumberPrefix + hash -> num (uint64 big endian)
)

func encodeBlockNumber(number uint64) []byte {
	enc := make([]byte, 8)
	binary.BigEndian.PutUint64(enc, number)
	return enc
}

func main() {
	if len(os.Args) != 2 {
		fmt.Println("Usage: scan_canonical <badgerdb_path>")
		os.Exit(1)
	}

	dbPath := os.Args[1]
	fmt.Printf("Scanning canonical chain in: %s\n\n", dbPath)

	// Open BadgerDB
	opts := badger.DefaultOptions(dbPath)
	opts.Logger = nil
	db, err := badger.Open(opts)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Start scanning from block 0
	var lastGoodBlock uint64 = 0
	var firstBadBlock uint64 = 0
	foundIssue := false

	fmt.Println("Scanning canonical chain...")
	
	for blockNum := uint64(0); blockNum <= 1082781; blockNum++ {
		if blockNum%10000 == 0 {
			fmt.Printf("  Checking block %d...\n", blockNum)
		}

		// Check if canonical hash exists (h + num + 'n' -> hash)
		canonicalKey := append(headerPrefix, encodeBlockNumber(blockNum)...)
		canonicalKey = append(canonicalKey, headerHashSuffix...)
		
		var canonicalHash []byte
		err := db.View(func(txn *badger.Txn) error {
			item, err := txn.Get(canonicalKey)
			if err != nil {
				return err
			}
			canonicalHash, err = item.ValueCopy(nil)
			return err
		})

		if err != nil {
			if !foundIssue {
				firstBadBlock = blockNum
				foundIssue = true
				fmt.Printf("\n❌ First missing canonical mapping at block %d\n", blockNum)
			}
			continue
		}

		if len(canonicalHash) != 32 {
			fmt.Printf("❌ Invalid canonical hash length at block %d: %d bytes\n", blockNum, len(canonicalHash))
			if !foundIssue {
				firstBadBlock = blockNum
				foundIssue = true
			}
			continue
		}

		// Check if header exists (h + num + hash -> header)
		headerKey := append(headerPrefix, encodeBlockNumber(blockNum)...)
		headerKey = append(headerKey, canonicalHash...)
		
		var headerExists bool
		err = db.View(func(txn *badger.Txn) error {
			_, err := txn.Get(headerKey)
			headerExists = (err == nil)
			return nil
		})

		if !headerExists {
			fmt.Printf("❌ Missing header for canonical block %d (hash: %x)\n", blockNum, canonicalHash)
			if !foundIssue {
				firstBadBlock = blockNum
				foundIssue = true
			}
			continue
		}

		// Check hash->number mapping (H + hash -> num)
		hashNumKey := append(headerNumberPrefix, canonicalHash...)
		
		var numberExists bool
		err = db.View(func(txn *badger.Txn) error {
			item, err := txn.Get(hashNumKey)
			if err != nil {
				return err
			}
			val, err := item.ValueCopy(nil)
			if err != nil {
				return err
			}
			storedNum := binary.BigEndian.Uint64(val)
			numberExists = (storedNum == blockNum)
			if !numberExists {
				fmt.Printf("❌ Hash->number mismatch at block %d: stored=%d\n", blockNum, storedNum)
			}
			return nil
		})

		if err != nil {
			fmt.Printf("❌ Missing hash->number mapping for block %d\n", blockNum)
			if !foundIssue {
				firstBadBlock = blockNum
				foundIssue = true
			}
			continue
		}

		if foundIssue {
			// We found a good block after bad ones
			break
		}
		
		lastGoodBlock = blockNum
	}

	fmt.Println("\n========================================")
	if !foundIssue {
		fmt.Printf("✅ OK: Canonical chain verified 0..%d\n", lastGoodBlock)
		fmt.Println("Database is ready for use!")
	} else {
		fmt.Printf("❌ ISSUE: Canonical chain broken\n")
		fmt.Printf("  Last good block: %d\n", lastGoodBlock)
		fmt.Printf("  First bad block: %d\n", firstBadBlock)
		fmt.Printf("\nNeed to rebuild canonical mappings from block %d onwards.\n", firstBadBlock)
		
		// Count how many headers we actually have
		headerCount := 0
		db.View(func(txn *badger.Txn) error {
			opts := badger.DefaultIteratorOptions
			opts.PrefetchValues = false
			it := txn.NewIterator(opts)
			defer it.Close()
			
			for it.Seek(headerPrefix); it.Valid(); it.Next() {
				key := it.Item().Key()
				if !bytes.HasPrefix(key, headerPrefix) {
					break
				}
				// Check if it's a header (not canonical suffix)
				if len(key) > 41 && !bytes.HasSuffix(key[:10], headerHashSuffix) {
					headerCount++
				}
			}
			return nil
		})
		
		fmt.Printf("\nTotal headers in database: %d\n", headerCount)
		fmt.Println("\nRun the fix_canonical tool to repair the database.")
	}
	fmt.Println("========================================")
}