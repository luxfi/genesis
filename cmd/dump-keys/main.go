package main

import (
	"encoding/hex"
	"fmt"
	"log"

	"github.com/cockroachdb/pebble"
)

func main() {
	// Open the database
	db, err := pebble.Open("/home/z/work/lux/genesis/state/chaindata/lux-mainnet-96369/db", &pebble.Options{})
	if err != nil {
		log.Fatal("Failed to open database:", err)
	}
	defer db.Close()

	// Dump first 50 keys
	iter, err := db.NewIter(&pebble.IterOptions{})
	if err != nil {
		log.Fatal("Failed to create iterator:", err)
	}
	defer iter.Close()

	fmt.Println("First 50 keys in database:")
	count := 0
	for iter.First(); iter.Valid() && count < 50; iter.Next() {
		key := iter.Key()
		val := iter.Value()
		
		keyStr := hex.EncodeToString(key)
		if len(keyStr) > 40 {
			keyStr = keyStr[:40] + "..."
		}
		
		fmt.Printf("%4d: key=%s (len=%d) val_len=%d", count, keyStr, len(key), len(val))
		
		// Decode key if it looks like our format
		if len(key) == 9 && key[0] == 'H' {
			// Canonical hash key
			fmt.Printf(" [canonical block]")
		} else if len(key) > 8 && key[0] == 'h' && key[len(key)-1] == 'n' {
			// Geth canonical hash key
			fmt.Printf(" [geth canonical]")
		} else if len(key) > 0 {
			// Try to decode as string
			str := string(key)
			if isPrintable(str) {
				fmt.Printf(" [string: %s]", str)
			}
		}
		
		fmt.Println()
		count++
	}
}

func isPrintable(s string) bool {
	for _, r := range s {
		if r < 32 || r > 126 {
			return false
		}
	}
	return len(s) > 0 && len(s) < 20
}