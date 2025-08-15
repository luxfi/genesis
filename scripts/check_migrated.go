package main

import (
	"encoding/binary"
	"fmt"
	"log"
	
	"github.com/dgraph-io/badger/v4"
	"github.com/luxfi/geth/common"
)

func main() {
	dbPath := "/Users/z/work/lux/genesis/state/chaindata/lux-genesis-7777/db"
	
	fmt.Printf("Checking migrated database: %s\n", dbPath)
	
	opts := badger.DefaultOptions(dbPath).WithReadOnly(true)
	db, err := badger.Open(opts)
	if err != nil {
		log.Fatal("Failed to open database:", err)
	}
	defer db.Close()
	
	fmt.Println("\nChecking for canonical blocks...")
	
	err = db.View(func(txn *badger.Txn) error {
		// Check for block 0
		key0 := make([]byte, 9)
		key0[0] = 'H'
		binary.BigEndian.PutUint64(key0[1:], 0)
		
		if item, err := txn.Get(key0); err == nil {
			val, _ := item.ValueCopy(nil)
			fmt.Printf("✓ Block 0 canonical hash: %x\n", val)
			
			// Check for header
			headerKey := make([]byte, 41)
			headerKey[0] = 'h'
			binary.BigEndian.PutUint64(headerKey[1:9], 0)
			copy(headerKey[9:], val)
			
			if item, err := txn.Get(headerKey); err == nil {
				hval, _ := item.ValueCopy(nil)
				fmt.Printf("✓ Block 0 header found: %d bytes\n", len(hval))
			} else {
				fmt.Printf("✗ Block 0 header not found\n")
			}
		} else {
			fmt.Printf("✗ Block 0 not found\n")
		}
		
		// Check for block 1082780
		targetBlock := uint64(1082780)
		keyTarget := make([]byte, 9)
		keyTarget[0] = 'H'
		binary.BigEndian.PutUint64(keyTarget[1:], targetBlock)
		
		if item, err := txn.Get(keyTarget); err == nil {
			val, _ := item.ValueCopy(nil)
			fmt.Printf("✓ Block %d canonical hash: %x\n", targetBlock, val[:8])
			
			// Check for header
			headerKey := make([]byte, 41)
			headerKey[0] = 'h'
			binary.BigEndian.PutUint64(headerKey[1:9], targetBlock)
			copy(headerKey[9:], val)
			
			if item, err := txn.Get(headerKey); err == nil {
				hval, _ := item.ValueCopy(nil)
				fmt.Printf("✓ Block %d header found: %d bytes\n", targetBlock, len(hval))
			} else {
				fmt.Printf("✗ Block %d header not found\n", targetBlock)
			}
		} else {
			fmt.Printf("✗ Block %d not found\n", targetBlock)
		}
		
		// Check LastBlock
		if item, err := txn.Get([]byte("LastBlock")); err == nil {
			val, _ := item.ValueCopy(nil)
			var hash common.Hash
			copy(hash[:], val)
			fmt.Printf("✓ LastBlock: %x\n", hash)
		} else {
			fmt.Printf("✗ LastBlock not found\n")
		}
		
		// Count total canonical blocks in first 10
		count := 0
		for i := uint64(0); i <= 10; i++ {
			key := make([]byte, 9)
			key[0] = 'H'
			binary.BigEndian.PutUint64(key[1:], i)
			
			if _, err := txn.Get(key); err == nil {
				count++
			}
		}
		fmt.Printf("\nFound %d canonical blocks in range 0-10\n", count)
		
		return nil
	})
	
	if err != nil {
		log.Fatal("Error:", err)
	}
}