package main

import (
	"encoding/binary"
	"fmt"
	"log"
	"os"
	
	"github.com/dgraph-io/badger/v4"
)

func main() {
	dbPath := "/home/z/.luxd/chainData/converted_mainnet"
	
	fmt.Printf("Verifying converted database at: %s\n\n", dbPath)
	
	opts := badger.DefaultOptions(dbPath)
	opts.ReadOnly = true
	
	db, err := badger.Open(opts)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	
	// Check canonical mapping for block 0
	err = db.View(func(txn *badger.Txn) error {
		// Standard geth format: 'H' + blockNum(8 bytes) -> hash(32 bytes)
		key := append([]byte("H"), make([]byte, 8)...)
		
		fmt.Printf("Checking canonical mapping with key: %x\n", key)
		
		item, err := txn.Get(key)
		if err != nil {
			fmt.Printf("Error: No canonical mapping found for block 0: %v\n", err)
			return err
		}
		
		hash, err := item.ValueCopy(nil)
		if err != nil {
			return err
		}
		
		fmt.Printf("✓ Block 0 canonical hash: %x\n", hash)
		
		expectedHash := "3f4fa2a0b0ce089f52bf0ae9199c75ffdd76ecafc987794050cb0d286f1ec61e"
		actualHash := fmt.Sprintf("%x", hash)
		
		if actualHash == expectedHash {
			fmt.Printf("✓ CORRECT GENESIS HASH!\n")
		} else {
			fmt.Printf("✗ WRONG GENESIS: got %s, expected %s\n", actualHash, expectedHash)
		}
		
		// Check highest block
		highestKey := append([]byte("H"), encodeBlockNumber(1082780)...)
		item, err = txn.Get(highestKey)
		if err == nil {
			hash, _ := item.ValueCopy(nil)
			fmt.Printf("✓ Block 1082780 canonical hash: %x\n", hash)
		}
		
		// Check head pointers
		headKeys := []string{"LastBlock", "LastHeader", "LastFast"}
		for _, k := range headKeys {
			if item, err := txn.Get([]byte(k)); err == nil {
				val, _ := item.ValueCopy(nil)
				fmt.Printf("✓ %s: %x\n", k, val)
			}
		}
		
		return nil
	})
	
	if err != nil {
		fmt.Printf("\nDatabase verification failed: %v\n", err)
		os.Exit(1)
	}
	
	fmt.Printf("\n✓ Converted database is ready for use!\n")
}

func encodeBlockNumber(number uint64) []byte {
	enc := make([]byte, 8)
	binary.BigEndian.PutUint64(enc, number)
	return enc
}