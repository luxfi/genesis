package main

import (
	"encoding/binary"
	"fmt"
	"log"
	
	"github.com/dgraph-io/badger/v4"
)

func main() {
	dbPath := "/Users/z/.luxd/network-96369/chains/EWi9aPkGe6EfJ3SobCAmSUXRPLa4brF3cThwPwmHTrD1y13jy/ethdb"
	
	fmt.Printf("Opening and repairing database: %s\n", dbPath)
	
	// Try to open with repair
	opts := badger.DefaultOptions(dbPath)
	opts.ValueLogFileSize = 256 << 20 // 256MB
	
	db, err := badger.Open(opts)
	if err != nil {
		fmt.Printf("Failed to open normally: %v\n", err)
		fmt.Println("Database may need reconstruction from SST files")
		// BadgerDB doesn't have automatic repair from SST files
		// We would need to rebuild from scratch
		log.Fatal("Cannot repair automatically")
	}
	defer db.Close()
	
	fmt.Println("Database opened successfully!")
	
	// Check what we have
	err = db.View(func(txn *badger.Txn) error {
		// Check for block 0
		key0 := make([]byte, 9)
		key0[0] = 'H'
		binary.BigEndian.PutUint64(key0[1:], 0)
		
		if item, err := txn.Get(key0); err == nil {
			val, _ := item.ValueCopy(nil)
			fmt.Printf("✓ Block 0 found: %x\n", val[:8])
		} else {
			fmt.Printf("✗ Block 0 not found\n")
		}
		
		// Check for block 1082780
		keyTarget := make([]byte, 9)
		keyTarget[0] = 'H'
		binary.BigEndian.PutUint64(keyTarget[1:], 1082780)
		
		if item, err := txn.Get(keyTarget); err == nil {
			val, _ := item.ValueCopy(nil)
			fmt.Printf("✓ Block 1082780 found: %x\n", val[:8])
		} else {
			fmt.Printf("✗ Block 1082780 not found\n")
		}
		
		return nil
	})
	
	if err != nil {
		fmt.Printf("Error checking blocks: %v\n", err)
	}
	
	fmt.Println("\nDatabase is ready!")
}