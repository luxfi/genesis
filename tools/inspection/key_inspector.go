package main

import (
	"encoding/binary"
	"fmt"
	"github.com/cockroachdb/pebble"
)

func main() {
	dbPath := "/home/z/work/lux/genesis/extracted-blockchain/pebbledb"
	
	opts := &pebble.Options{
		ReadOnly: true,
	}
	
	db, err := pebble.Open(dbPath, opts)
	if err != nil {
		panic(err)
	}
	defer db.Close()
	
	iter, _ := db.NewIter(nil)
	defer iter.Close()
	
	found := 0
	for iter.First(); iter.Valid() && found < 20; iter.Next() {
		key := iter.Key()
		
		if len(key) > 0 && key[0] == 'H' {
			found++
			value := iter.Value()
			
			fmt.Printf("Key: %x (len=%d)\n", key, len(key))
			fmt.Printf("  Value: %x (len=%d)\n", value, len(value))
			
			// Try to decode the key
			if len(key) == 9 {
				blockNum := binary.BigEndian.Uint64(key[1:9])
				fmt.Printf("  Block number: %d\n", blockNum)
			}
			
			fmt.Println()
		}
	}
	
	fmt.Printf("Found %d 'H' keys\n", found)
}