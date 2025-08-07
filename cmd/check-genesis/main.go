package main

import (
	"encoding/hex"
	"fmt"
	"log"

	"github.com/cockroachdb/pebble"
	"github.com/ethereum/go-ethereum/common"
)

func main() {
	// Open the migrated database
	db, err := pebble.Open("/home/z/work/lux/genesis/migrated-data/cchain-db", &pebble.Options{})
	if err != nil {
		log.Fatal("Failed to open database:", err)
	}
	defer db.Close()

	// Known key patterns for genesis information
	keys := []string{
		"LastHeader",
		"LastBlock",
		"LastFast",
		"SnapshotBlock",
		"SnapshotRoot",
		"chain-config-hash",
		"eth-config-hash",
		"LastAccepted",
		"H",        // Canonical hash at block 0
		"\x48",     // ASCII 'H' for canonical hash prefix
	}

	fmt.Println("Checking genesis-related keys in migrated database:")
	fmt.Println("==============================================================")

	// Check specific keys
	for _, key := range keys {
		val, closer, err := db.Get([]byte(key))
		if err == nil {
			fmt.Printf("Key: %s (0x%x)\n", key, key)
			fmt.Printf("Value: 0x%x\n", val)
			if len(val) == 32 {
				fmt.Printf("As hash: 0x%s\n", hex.EncodeToString(val))
			}
			fmt.Println()
			closer.Close()
		}
	}

	// Check canonical hash at block 0
	canonicalKey := make([]byte, 9)
	canonicalKey[0] = 'H'
	// Block number 0 as 8 bytes big endian

	val, closer, err := db.Get(canonicalKey)
	if err == nil {
		fmt.Println("Canonical hash at block 0:")
		fmt.Printf("Key: 0x%x\n", canonicalKey)
		fmt.Printf("Value: 0x%x\n", val)
		if len(val) == 32 {
			fmt.Printf("As hash: 0x%s\n", hex.EncodeToString(val))
		}
		closer.Close()
	} else {
		fmt.Printf("No canonical hash found at block 0: %v\n", err)
	}

	// Try to find any key starting with 'H' (canonical hash prefix)
	fmt.Println("\nScanning for canonical hash keys:")
	iter, err := db.NewIter(&pebble.IterOptions{})
	if err != nil {
		fmt.Printf("Failed to create iterator: %v\n", err)
		return
	}
	defer iter.Close()

	count := 0
	for iter.First(); iter.Valid() && count < 20; iter.Next() {
		key := iter.Key()
		if len(key) > 0 && key[0] == 'H' {
			fmt.Printf("Found canonical key: 0x%x\n", key)
			val := iter.Value()
			if len(val) == 32 {
				fmt.Printf("  Hash: 0x%s\n", hex.EncodeToString(val))
			}
			count++
		}
	}

	// Check for header at block 0
	headerKey := append([]byte("h"), make([]byte, 8)...)
	headerKey = append(headerKey, common.HexToHash("0x3f4fa2a0b0ce089f52bf0ae9199c75ffdd76ecafc987794050cb0d286f1ec61e").Bytes()...)
	
	val, closer, err = db.Get(headerKey)
	if err == nil {
		fmt.Println("\nFound header at block 0:")
		fmt.Printf("Value length: %d\n", len(val))
		closer.Close()
	}
}