package main

import (
	"encoding/binary"
	"fmt"
	"log"
	
	"github.com/dgraph-io/badger/v4"
)

func main() {
	// Open the ethdb database
	ethdbPath := "/Users/z/.luxd/network-96369/chains/EWi9aPkGe6EfJ3SobCAmSUXRPLa4brF3cThwPwmHTrD1y13jy/ethdb"
	
	db, err := badger.Open(badger.DefaultOptions(ethdbPath))
	if err != nil {
		log.Fatal("Failed to open ethdb:", err)
	}
	defer db.Close()
	
	fmt.Println("Adding canonical hash mappings...")
	fmt.Println("================================")
	
	// First, scan for all headers to build mapping
	blockHashes := make(map[uint64][]byte)
	headerCount := 0
	
	err = db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchSize = 10
		opts.Prefix = []byte("h") // Headers prefix
		it := txn.NewIterator(opts)
		defer it.Close()
		
		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			key := item.Key()
			
			// 'h' prefix - headers (h + num(8) + hash(32) = 41 bytes)
			if len(key) == 41 && key[0] == 'h' {
				blockNum := binary.BigEndian.Uint64(key[1:9])
				hash := make([]byte, 32)
				copy(hash, key[9:41])
				
				blockHashes[blockNum] = hash
				headerCount++
				
				if headerCount <= 5 || headerCount%100000 == 0 {
					fmt.Printf("Found header at block %d with hash %x\n", blockNum, hash[:8])
				}
			}
		}
		
		return nil
	})
	
	if err != nil {
		log.Fatal("Error scanning headers:", err)
	}
	
	fmt.Printf("\nFound %d headers to map\n", headerCount)
	
	// Now write canonical mappings in batches
	fmt.Println("\nWriting canonical mappings in batches...")
	written := 0
	batchSize := 10000
	
	// Process in batches
	for batchStart := uint64(0); batchStart <= 1082780; batchStart += uint64(batchSize) {
		batchEnd := batchStart + uint64(batchSize)
		if batchEnd > 1082780 {
			batchEnd = 1082781
		}
		
		err = db.Update(func(txn *badger.Txn) error {
			for blockNum := batchStart; blockNum < batchEnd; blockNum++ {
				if hash, exists := blockHashes[blockNum]; exists {
					// Create canonical key: 'H' + blockNum(8 bytes)
					key := make([]byte, 9)
					key[0] = 'H'
					binary.BigEndian.PutUint64(key[1:], blockNum)
					
					// Write the hash as value
					err := txn.Set(key, hash)
					if err != nil {
						return err
					}
					
					written++
					if written <= 5 || written%100000 == 0 {
						fmt.Printf("Wrote canonical mapping for block %d -> %x\n", blockNum, hash[:8])
					}
				}
			}
			return nil
		})
		
		if err != nil {
			return
		}
		
		if batchStart%100000 == 0 {
			fmt.Printf("Processed batch %d-%d\n", batchStart, batchEnd)
		}
	}
	
	// Write head pointers in separate transaction
	err = db.Update(func(txn *badger.Txn) error {
		if lastHash, exists := blockHashes[1082780]; exists {
			fmt.Printf("\nSetting head block to 1082780 with hash %x\n", lastHash[:8])
			
			// LastBlock
			txn.Set([]byte("LastBlock"), lastHash)
			// LastHeader  
			txn.Set([]byte("LastHeader"), lastHash)
			// LastFast
			txn.Set([]byte("LastFast"), lastHash)
		}
		return nil
	})
	
	if err != nil {
		log.Fatal("Error writing mappings:", err)
	}
	
	fmt.Printf("\n================================\n")
	fmt.Printf("Successfully wrote %d canonical mappings\n", written)
	fmt.Println("Database is now ready for use!")
}