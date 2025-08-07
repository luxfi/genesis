package main

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"log"

	"github.com/cockroachdb/pebble"
)

func main() {
	db, err := pebble.Open("/home/z/work/lux/genesis/migrated-data/cchain-db", &pebble.Options{ReadOnly: true})
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// Check canonical hash at height 0
	canonicalKey := make([]byte, 9)
	canonicalKey[0] = 'H'
	binary.BigEndian.PutUint64(canonicalKey[1:], 0)

	value, closer, err := db.Get(canonicalKey)
	if err != nil {
		fmt.Printf("Error getting canonical hash at height 0: %v\n", err)
		fmt.Printf("Key hex: %s\n", hex.EncodeToString(canonicalKey))
		
		// Try to list keys starting with H
		iter, err := db.NewIter(&pebble.IterOptions{
			LowerBound: []byte("H"),
			UpperBound: []byte("I"),
		})
		if err != nil {
			log.Fatal(err)
		}
		defer iter.Close()
		
		fmt.Println("\nKeys starting with 'H':")
		count := 0
		for iter.First(); iter.Valid() && count < 5; iter.Next() {
			key := iter.Key()
			fmt.Printf("Key: %s (hex: %s)\n", key, hex.EncodeToString(key))
			count++
		}
	} else {
		defer closer.Close()
		fmt.Printf("Found canonical hash at height 0: %s\n", hex.EncodeToString(value))
	}
}