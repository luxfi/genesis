package main

import (
	"fmt"
	"os"
	
	"github.com/dgraph-io/badger/v4"
	"github.com/ethereum/go-ethereum/common"
)

func main() {
	dbPath := "/home/z/.luxd/db/network-96369/chains/X6CU5qgMJfzsTB9UWxj2ZY5hd68x41HfZ4m4hCBWbHuj1Ebc3/ethdb"
	
	fmt.Printf("Verifying chain config in BadgerDB at: %s\n", dbPath)
	
	opts := badger.DefaultOptions(dbPath)
	opts.SyncWrites = false
	opts.Logger = nil
	opts.ReadOnly = true
	
	db, err := badger.Open(opts)
	if err != nil {
		fmt.Printf("Error opening database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()
	
	genesisHash := common.HexToHash("0x3f4fa2a0b0ce089f52bf0ae9199c75ffdd76ecafc987794050cb0d286f1ec61e")
	
	err = db.View(func(txn *badger.Txn) error {
		// Check for chain config under genesis hash
		configKey := append([]byte("ethereum-config-"), genesisHash[:]...)
		
		item, err := txn.Get(configKey)
		if err != nil {
			fmt.Printf("❌ No chain config found for key: %x\n", configKey)
			fmt.Printf("    Error: %v\n", err)
			
			// Try to find any ethereum-config keys
			fmt.Printf("\nSearching for any ethereum-config keys...\n")
			it := txn.NewIterator(badger.DefaultIteratorOptions)
			defer it.Close()
			
			prefix := []byte("ethereum-config")
			count := 0
			for it.Seek(prefix); it.Valid(); it.Next() {
				key := it.Item().Key()
				if len(key) >= len(prefix) && string(key[:len(prefix)]) == string(prefix) {
					fmt.Printf("  Found config key: %x\n", key)
					val, _ := it.Item().ValueCopy(nil)
					fmt.Printf("  Value (first 100 chars): %.100s\n", string(val))
					count++
				}
				if count >= 5 {
					break
				}
			}
			
			if count == 0 {
				fmt.Printf("  No ethereum-config keys found in database\n")
			}
		} else {
			val, _ := item.ValueCopy(nil)
			fmt.Printf("✅ Found chain config (size: %d bytes)\n", len(val))
			fmt.Printf("Config JSON: %s\n", string(val))
		}
		
		return nil
	})
	
	if err != nil {
		fmt.Printf("Error reading database: %v\n", err)
	}
}