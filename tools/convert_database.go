package main

import (
	"encoding/binary"
	"fmt"
	"log"
	"os"
	
	"github.com/dgraph-io/badger/v4"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: convert_database <source_db> <dest_db>")
		fmt.Println("Converts migrated SubnetEVM format to standard Geth format")
		os.Exit(1)
	}
	
	sourceDB := os.Args[1]
	destDB := os.Args[2]
	
	fmt.Printf("Converting database\n")
	fmt.Printf("From: %s\n", sourceDB)
	fmt.Printf("To: %s\n", destDB)
	
	// Open source (read-only)
	srcOpts := badger.DefaultOptions(sourceDB)
	srcOpts.ReadOnly = true
	
	src, err := badger.Open(srcOpts)
	if err != nil {
		log.Fatalf("Failed to open source: %v", err)
	}
	defer src.Close()
	
	// Create destination
	os.MkdirAll(destDB, 0755)
	dstOpts := badger.DefaultOptions(destDB)
	
	dst, err := badger.Open(dstOpts)
	if err != nil {
		log.Fatalf("Failed to create destination: %v", err)
	}
	defer dst.Close()
	
	fmt.Println("Converting canonical mappings...")
	
	count := 0
	err = src.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()
		
		// Process 'h' prefix keys (migrated format)
		prefix := []byte("h")
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			key := item.Key()
			
			if len(key) == 41 {
				// Extract hash from key (blockNum at 1:9, hash at 9:41)
				hash := key[9:41]
				
				// Get block data
				val, err := item.ValueCopy(nil)
				if err == nil {
					// Write canonical: 'H' + blockNum -> hash
					canonicalKey := append([]byte("H"), key[1:9]...)
					
					err = dst.Update(func(txn *badger.Txn) error {
						return txn.Set(canonicalKey, hash)
					})
					if err != nil {
						return err
					}
					
					// Write header: 'h' + blockNum + hash + 'n'
					headerKey := make([]byte, 42)
					headerKey[0] = 'h'
					copy(headerKey[1:9], key[1:9])
					copy(headerKey[9:41], hash)
					headerKey[41] = 'n'
					
					err = dst.Update(func(txn *badger.Txn) error {
						return txn.Set(headerKey, val)
					})
					if err != nil {
						return err
					}
					
					count++
					if count%10000 == 0 {
						fmt.Printf("  Converted %d blocks\n", count)
					}
				}
			}
		}
		
		// Copy other keys
		fmt.Println("Copying state data...")
		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			key := item.Key()
			
			// Skip 'h' prefix (already processed)
			if len(key) > 0 && key[0] == 'h' {
				continue
			}
			
			val, err := item.ValueCopy(nil)
			if err == nil {
				dst.Update(func(txn *badger.Txn) error {
					return txn.Set(key, val)
				})
			}
		}
		
		return nil
	})
	
	if err != nil {
		log.Fatal(err)
	}
	
	// Set head pointers
	fmt.Println("Setting head pointers...")
	highestBlock := uint64(1082780)
	highestHash := make([]byte, 32)
	
	// Get hash of highest block
	canonicalKey := append([]byte("H"), encodeBlockNumber(highestBlock)...)
	dst.View(func(txn *badger.Txn) error {
		if item, err := txn.Get(canonicalKey); err == nil {
			highestHash, _ = item.ValueCopy(nil)
		}
		return nil
	})
	
	// Write head pointers
	dst.Update(func(txn *badger.Txn) error {
		txn.Set([]byte("LastBlock"), highestHash)
		txn.Set([]byte("LastHeader"), highestHash)
		txn.Set([]byte("LastFast"), highestHash)
		return nil
	})
	
	fmt.Printf("\n✓ Conversion complete\n")
	fmt.Printf("  Total blocks: %d\n", count)
}

func encodeBlockNumber(number uint64) []byte {
	enc := make([]byte, 8)
	binary.BigEndian.PutUint64(enc, number)
	return enc
}