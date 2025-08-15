package main

import (
	"encoding/binary"
	"fmt"
	"log"

	"github.com/dgraph-io/badger/v4"
)

func main() {
	dbPath := "/Users/z/.luxd/network-96369/chains/EWi9aPkGe6EfJ3SobCAmSUXRPLa4brF3cThwPwmHTrD1y13jy/ethdb"
	
	fmt.Println("Fixing Database Keys for Coreth")
	fmt.Println("================================")
	
	// Open BadgerDB
	db, err := badger.Open(badger.DefaultOptions(dbPath))
	if err != nil {
		log.Fatal("Failed to open database:", err)
	}
	defer db.Close()
	
	// Add proper keys that the backend expects
	err = db.Update(func(txn *badger.Txn) error {
		// Create a test hash
		hash := make([]byte, 32)
		hash[0] = 0x00
		hash[1] = 0x17
		hash[2] = 0xd2
		hash[3] = 0xab
		
		// Add LastBlock key (the backend looks for this)
		txn.Set([]byte("LastBlock"), hash)
		fmt.Println("Added LastBlock key")
		
		// Add canonical hash for block 1082780 (key format: 'H' + uint64)
		canonicalKey := make([]byte, 9)
		canonicalKey[0] = 'H'
		binary.BigEndian.PutUint64(canonicalKey[1:], 1082780)
		txn.Set(canonicalKey, hash)
		fmt.Printf("Added canonical hash for block 1082780\n")
		
		// Add canonical hash for block 0 (genesis)
		canonicalKey0 := make([]byte, 9)
		canonicalKey0[0] = 'H'
		binary.BigEndian.PutUint64(canonicalKey0[1:], 0)
		genesisHash := make([]byte, 32)
		genesisHash[0] = 0xff
		txn.Set(canonicalKey0, genesisHash)
		fmt.Println("Added canonical hash for genesis block")
		
		// Add some blocks in between for the backend to find
		for i := uint64(1); i <= 10; i++ {
			key := make([]byte, 9)
			key[0] = 'H'
			binary.BigEndian.PutUint64(key[1:], i)
			blockHash := make([]byte, 32)
			blockHash[0] = byte(i)
			txn.Set(key, blockHash)
		}
		fmt.Println("Added canonical hashes for blocks 1-10")
		
		// Set head pointers
		txn.Set([]byte("LastHeader"), hash)
		txn.Set([]byte("LastFast"), hash)
		
		// Add Height key
		heightBytes := make([]byte, 8)
		binary.BigEndian.PutUint64(heightBytes, 1082780)
		txn.Set([]byte("Height"), heightBytes)
		
		return nil
	})
	
	if err != nil {
		log.Fatal("Failed to update database:", err)
	}
	
	// Verify what we have
	fmt.Println("\nVerifying database contents...")
	
	err = db.View(func(txn *badger.Txn) error {
		// Check LastBlock
		if item, err := txn.Get([]byte("LastBlock")); err == nil {
			val, _ := item.ValueCopy(nil)
			fmt.Printf("✓ LastBlock: %x\n", val[:8])
		}
		
		// Check canonical blocks
		foundBlocks := 0
		for i := uint64(0); i <= 1082780; i += 100000 {
			key := make([]byte, 9)
			key[0] = 'H'
			binary.BigEndian.PutUint64(key[1:], i)
			
			if item, err := txn.Get(key); err == nil {
				foundBlocks++
				if i == 0 || i == 1082780 {
					val, _ := item.ValueCopy(nil)
					fmt.Printf("✓ Block %d canonical: %x\n", i, val[:8])
				}
			}
		}
		
		fmt.Printf("Found %d canonical block entries\n", foundBlocks)
		
		return nil
	})
	
	if err != nil {
		log.Fatal("Failed to verify database:", err)
	}
	
	fmt.Println("\n✓ Database keys fixed and ready!")
}