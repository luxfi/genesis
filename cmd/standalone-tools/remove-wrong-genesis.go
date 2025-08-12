package main

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"log"

	"github.com/luxfi/database/badgerdb"
	"github.com/prometheus/client_golang/prometheus"
)

func main() {
	dbPath := "/home/z/.luxd/chainData/xBBY6aJcNichNCkCXgUcG5Gv2PW6FLS81LYDV8VwnPuadKGqm/ethdb"
	
	db, err := badgerdb.New(dbPath, nil, "", prometheus.DefaultRegisterer)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	
	// The wrong genesis that luxd created
	wrongGenesis, _ := hex.DecodeString("9a6005b9457b67f584794a91ca6987e00766f7861be7eee34f1bae12a5829e61")
	
	// The correct genesis from our migration
	correctGenesis, _ := hex.DecodeString("3f4fa2a0b0ce089f52bf0ae9199c75ffdd76ecafc987794050cb0d286f1ec61e")
	
	fmt.Printf("Wrong genesis:   %x\n", wrongGenesis)
	fmt.Printf("Correct genesis: %x\n", correctGenesis)
	
	// Check what's at block 0
	canonKey := make([]byte, 10)
	canonKey[0] = 'h'
	binary.BigEndian.PutUint64(canonKey[1:9], 0)
	canonKey[9] = 'n'
	
	currentGenesis, err := db.Get(canonKey)
	if err == nil && len(currentGenesis) == 32 {
		fmt.Printf("\nCurrent genesis at block 0: %x\n", currentGenesis)
		
		if bytes.Equal(currentGenesis, wrongGenesis) {
			fmt.Println("Found wrong genesis, replacing with correct one...")
			
			// Update canonical hash at block 0
			err = db.Put(canonKey, correctGenesis)
			if err != nil {
				log.Fatal("Failed to update canonical hash:", err)
			}
			
			// Also update the hash->number mapping for correct genesis
			hashNumKey := append([]byte("H"), correctGenesis...)
			numBytes := make([]byte, 8)
			binary.BigEndian.PutUint64(numBytes, 0)
			db.Put(hashNumKey, numBytes)
			
			// Remove wrong genesis header if it exists
			wrongHeaderKey := make([]byte, 41)
			wrongHeaderKey[0] = 'h'
			binary.BigEndian.PutUint64(wrongHeaderKey[1:9], 0)
			copy(wrongHeaderKey[9:41], wrongGenesis)
			db.Delete(wrongHeaderKey)
			
			// Remove wrong genesis body
			wrongBodyKey := make([]byte, 41)
			wrongBodyKey[0] = 'b'
			binary.BigEndian.PutUint64(wrongBodyKey[1:9], 0)
			copy(wrongBodyKey[9:41], wrongGenesis)
			db.Delete(wrongBodyKey)
			
			// Remove wrong genesis receipts
			wrongReceiptKey := make([]byte, 41)
			wrongReceiptKey[0] = 'r'
			binary.BigEndian.PutUint64(wrongReceiptKey[1:9], 0)
			copy(wrongReceiptKey[9:41], wrongGenesis)
			db.Delete(wrongReceiptKey)
			
			// Remove wrong genesis TD
			wrongTDKey := make([]byte, 42)
			wrongTDKey[0] = 'h'
			binary.BigEndian.PutUint64(wrongTDKey[1:9], 0)
			copy(wrongTDKey[9:41], wrongGenesis)
			wrongTDKey[41] = 't'
			db.Delete(wrongTDKey)
			
			// Remove wrong genesis config
			wrongConfigKey := append([]byte("ethereum-config-"), wrongGenesis...)
			db.Delete(wrongConfigKey)
			
			fmt.Println("Removed wrong genesis data")
			
			// Verify correct genesis has proper data
			correctHeaderKey := make([]byte, 41)
			correctHeaderKey[0] = 'h'
			binary.BigEndian.PutUint64(correctHeaderKey[1:9], 0)
			copy(correctHeaderKey[9:41], correctGenesis)
			
			if header, err := db.Get(correctHeaderKey); err == nil {
				fmt.Printf("Correct genesis header exists, size: %d\n", len(header))
			} else {
				fmt.Println("WARNING: Correct genesis header not found!")
			}
			
			// Verify the updated canonical
			updatedGenesis, _ := db.Get(canonKey)
			fmt.Printf("\nUpdated genesis at block 0: %x\n", updatedGenesis)
		} else if bytes.Equal(currentGenesis, correctGenesis) {
			fmt.Println("Genesis is already correct!")
		} else {
			fmt.Printf("Unknown genesis found: %x\n", currentGenesis)
		}
	} else {
		fmt.Println("No genesis found at block 0")
	}
	
	// Check tip block
	if tipHash, err := db.Get([]byte("LastBlock")); err == nil && len(tipHash) == 32 {
		fmt.Printf("\nLastBlock: %x\n", tipHash)
		
		// Get block number
		hashNumKey := append([]byte("H"), tipHash...)
		if numBytes, err := db.Get(hashNumKey); err == nil && len(numBytes) == 8 {
			num := binary.BigEndian.Uint64(numBytes)
			fmt.Printf("LastBlock height: %d\n", num)
		}
	}
}