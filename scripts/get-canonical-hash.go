package main

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"os"

	"github.com/luxfi/database/pebbledb"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run get-canonical-hash.go <db-path>")
		return
	}

	// Open database
	db, err := pebbledb.New(os.Args[1], 0, 0, "", false)
	if err != nil {
		panic(err)
	}
	defer db.Close()

	// Check for canonical hash at height 1082780
	canonicalKey := make([]byte, 10)
	canonicalKey[0] = 'h'
	binary.BigEndian.PutUint64(canonicalKey[1:9], 1082780)
	canonicalKey[9] = 'n'
	
	fmt.Printf("Looking for canonical key: %s\n", hex.EncodeToString(canonicalKey))
	if hashBytes, err := db.Get(canonicalKey); err == nil {
		fmt.Printf("Found canonical hash at 1082780: %s\n", hex.EncodeToString(hashBytes))
		
		// Write this hash to environment
		fmt.Printf("\nTo export: export LUX_IMPORTED_BLOCK_ID=%s\n", hex.EncodeToString(hashBytes))
	} else {
		fmt.Printf("Canonical hash at 1082780 not found: %v\n", err)
	}
}