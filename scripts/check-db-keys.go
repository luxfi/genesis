package main

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"os"

	"github.com/syndtr/goleveldb/leveldb"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run check-db-keys.go <db-path>")
		return
	}

	db, err := leveldb.OpenFile(os.Args[1], nil)
	if err != nil {
		panic(err)
	}
	defer db.Close()

	// Check for Height key
	if heightBytes, err := db.Get([]byte("Height"), nil); err == nil {
		height := binary.BigEndian.Uint64(heightBytes)
		fmt.Printf("Found Height key: %d\n", height)
	} else {
		fmt.Printf("Height key not found: %v\n", err)
	}

	// Check for canonical hash at height 1082780
	canonicalKey := make([]byte, 10)
	canonicalKey[0] = 'h'
	binary.BigEndian.PutUint64(canonicalKey[1:9], 1082780)
	canonicalKey[9] = 'n'
	
	fmt.Printf("Looking for canonical key: %s\n", hex.EncodeToString(canonicalKey))
	if hashBytes, err := db.Get(canonicalKey, nil); err == nil {
		fmt.Printf("Found canonical hash at 1082780: %s\n", hex.EncodeToString(hashBytes))
	} else {
		fmt.Printf("Canonical hash at 1082780 not found: %v\n", err)
	}

	// Scan for any canonical keys
	iter := db.NewIterator(nil, nil)
	defer iter.Release()

	canonicalCount := 0
	maxHeight := uint64(0)
	for iter.Next() {
		key := iter.Key()
		if len(key) == 10 && key[0] == 'h' && key[9] == 'n' {
			blockNum := binary.BigEndian.Uint64(key[1:9])
			if canonicalCount < 5 {
				fmt.Printf("Found canonical key at height %d: %s\n", blockNum, hex.EncodeToString(iter.Value()))
			}
			canonicalCount++
			if blockNum > maxHeight {
				maxHeight = blockNum
			}
		}
	}

	fmt.Printf("\nTotal canonical blocks: %d\n", canonicalCount)
	fmt.Printf("Max height: %d\n", maxHeight)
}