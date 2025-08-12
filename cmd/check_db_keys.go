package main

import (
	"encoding/hex"
	"fmt"
	"log"
	
	"github.com/luxfi/database/badgerdb"
	"github.com/prometheus/client_golang/prometheus"
)

func main() {
	dbPath := "/Users/z/.luxd/network-96369/chains/X6CU5qgMJfzsTB9UWxj2ZY5hd68x41HfZ4m4hCBWbHuj1Ebc3/ethdb"
	
	// Open BadgerDB
	db, err := badgerdb.New(dbPath, nil, "", prometheus.DefaultRegisterer)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()
	
	// Check for specific keys
	keys := []string{
		"LastHeader",
		"LastBlock", 
		"LastFast",
		"lastAccepted",
		"lastAcceptedHeight",
		"vmID",
	}
	
	fmt.Println("Checking for specific keys:")
	for _, key := range keys {
		val, err := db.Get([]byte(key))
		if err == nil && val != nil {
			fmt.Printf("  %s: %s (hex: %s)\n", key, string(val), hex.EncodeToString(val))
		}
	}
	
	// List first 20 keys
	fmt.Println("\nFirst 20 keys in database:")
	iter := db.NewIterator()
	defer iter.Release()
	
	count := 0
	for iter.Next() && count < 20 {
		key := iter.Key()
		val := iter.Value()
		
		keyStr := string(key)
		if len(keyStr) > 50 {
			keyStr = keyStr[:50] + "..."
		}
		
		valStr := ""
		if len(val) < 100 {
			valStr = hex.EncodeToString(val)
			if len(valStr) > 60 {
				valStr = valStr[:60] + "..."
			}
		} else {
			valStr = fmt.Sprintf("(%d bytes)", len(val))
		}
		
		fmt.Printf("  Key: %s\n    Value: %s\n", keyStr, valStr)
		count++
	}
}