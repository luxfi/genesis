package main

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"log"
	
	"github.com/dgraph-io/badger/v4"
)

func main() {
	// Open the ethdb database directly
	ethdbPath := "/Users/z/.luxd/network-96369/chains/EWi9aPkGe6EfJ3SobCAmSUXRPLa4brF3cThwPwmHTrD1y13jy/ethdb"
	
	db, err := badger.Open(badger.DefaultOptions(ethdbPath))
	if err != nil {
		log.Fatal("Failed to open ethdb:", err)
	}
	defer db.Close()
	
	fmt.Println("Checking migrated ethdb database...")
	fmt.Println("================================")
	
	// Count different key types
	headerCount := 0
	bodyCount := 0
	receiptCount := 0
	canonicalCount := 0
	lastBlockKey := ""
	lastBlockNum := uint64(0)
	
	err = db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchSize = 10
		it := txn.NewIterator(opts)
		defer it.Close()
		
		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			key := item.Key()
			
			// Check key prefix
			if len(key) > 0 {
				prefix := string(key[0])
				
				// 'h' prefix - headers (h + num(8) + hash(32))
				if prefix == "h" && len(key) == 41 {
					headerCount++
					blockNum := binary.BigEndian.Uint64(key[1:9])
					if blockNum > lastBlockNum {
						lastBlockNum = blockNum
						lastBlockKey = hex.EncodeToString(key)
					}
					if headerCount <= 5 || headerCount%100000 == 0 {
						fmt.Printf("Header at block %d\n", blockNum)
					}
				}
				
				// 'b' prefix - bodies  
				if prefix == "b" && len(key) == 41 {
					bodyCount++
				}
				
				// 'r' prefix - receipts
				if prefix == "r" && len(key) == 41 {
					receiptCount++
				}
				
				// 'H' prefix - canonical hash (H + num(8))
				if prefix == "H" && len(key) == 9 {
					canonicalCount++
					blockNum := binary.BigEndian.Uint64(key[1:9])
					if canonicalCount <= 5 || canonicalCount%100000 == 0 {
						val, _ := item.ValueCopy(nil)
						fmt.Printf("Canonical hash at block %d: %s\n", blockNum, hex.EncodeToString(val))
					}
				}
				
				// Special keys
				if string(key) == "LastBlock" {
					val, _ := item.ValueCopy(nil)
					fmt.Printf("LastBlock: %s\n", hex.EncodeToString(val))
				}
				if string(key) == "LastHeader" {
					val, _ := item.ValueCopy(nil)
					fmt.Printf("LastHeader: %s\n", hex.EncodeToString(val))
				}
				if string(key) == "LastFast" {
					val, _ := item.ValueCopy(nil)
					fmt.Printf("LastFast: %s\n", hex.EncodeToString(val))
				}
			}
		}
		
		return nil
	})
	
	if err != nil {
		log.Fatal("Error scanning database:", err)
	}
	
	fmt.Println("\n================================")
	fmt.Printf("Summary:\n")
	fmt.Printf("Headers:    %d\n", headerCount)
	fmt.Printf("Bodies:     %d\n", bodyCount)
	fmt.Printf("Receipts:   %d\n", receiptCount)
	fmt.Printf("Canonical:  %d\n", canonicalCount)
	fmt.Printf("Last block: %d\n", lastBlockNum)
	fmt.Printf("Last key:   %s\n", lastBlockKey)
}