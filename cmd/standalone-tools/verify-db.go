package main

import (
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
	
	fmt.Println("=== Database Verification ===")
	
	// Check canonical at key blocks
	for _, height := range []uint64{0, 1, 100, 1000, 10000, 100000, 1082780} {
		canonKey := make([]byte, 10)
		canonKey[0] = 'h'
		binary.BigEndian.PutUint64(canonKey[1:9], height)
		canonKey[9] = 'n'
		
		if hash, err := db.Get(canonKey); err == nil && len(hash) == 32 {
			// Check if header exists
			headerKey := make([]byte, 41)
			headerKey[0] = 'h'
			binary.BigEndian.PutUint64(headerKey[1:9], height)
			copy(headerKey[9:41], hash)
			
			hasHeader := false
			if _, err := db.Get(headerKey); err == nil {
				hasHeader = true
			}
			
			// Check TD
			tdKey := make([]byte, 42)
			tdKey[0] = 'h'
			binary.BigEndian.PutUint64(tdKey[1:9], height)
			copy(tdKey[9:41], hash)
			tdKey[41] = 't'
			
			hasTD := false
			if _, err := db.Get(tdKey); err == nil {
				hasTD = true
			}
			
			fmt.Printf("Block %d: hash=%x header=%v td=%v\n", 
				height, hash[:8], hasHeader, hasTD)
		} else {
			fmt.Printf("Block %d: NOT FOUND\n", height)
		}
	}
	
	// Check heads
	fmt.Println("\n=== Head Pointers ===")
	for _, key := range []string{"LastHeader", "LastBlock", "LastFast", "LastFinalized"} {
		if val, err := db.Get([]byte(key)); err == nil && len(val) == 32 {
			fmt.Printf("%s: %x\n", key, val[:8])
		} else {
			fmt.Printf("%s: NOT SET\n", key)
		}
	}
	
	// Check chain config
	fmt.Println("\n=== Chain Configs ===")
	genesisHashes := []string{
		"3f4fa2a0b0ce089f52bf0ae9199c75ffdd76ecafc987794050cb0d286f1ec61e", // correct
		"9a6005b9457b67f584794a91ca6987e00766f7861be7eee34f1bae12a5829e61", // wrong
	}
	
	for _, hashStr := range genesisHashes {
		hash, _ := hex.DecodeString(hashStr)
		configKey := append([]byte("ethereum-config-"), hash...)
		if config, err := db.Get(configKey); err == nil {
			fmt.Printf("Config for %s: %s\n", hashStr[:8], string(config))
		}
	}
	
	// Check VM metadata
	vmdbPath := "/home/z/.luxd/chainData/xBBY6aJcNichNCkCXgUcG5Gv2PW6FLS81LYDV8VwnPuadKGqm/vm"
	vmdb, err := badgerdb.New(vmdbPath, nil, "", prometheus.DefaultRegisterer)
	if err == nil {
		fmt.Println("\n=== VM Metadata ===")
		
		if lastAccepted, err := vmdb.Get([]byte("lastAccepted")); err == nil && len(lastAccepted) == 32 {
			fmt.Printf("lastAccepted: %x\n", lastAccepted[:8])
		}
		
		if heightBytes, err := vmdb.Get([]byte("lastAcceptedHeight")); err == nil && len(heightBytes) == 8 {
			height := binary.BigEndian.Uint64(heightBytes)
			fmt.Printf("lastAcceptedHeight: %d\n", height)
		}
		
		vmdb.Close()
	}
}