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
		fmt.Println("Usage: go run test-canonical.go <db-path>")
		return
	}

	// Open database
	db, err := pebbledb.New(os.Args[1], 0, 0, "", false)
	if err != nil {
		panic(err)
	}
	defer db.Close()

	// Check for canonical hash at height 1082780
	height := uint64(1082780)
	canonicalKey := make([]byte, 10)
	canonicalKey[0] = 'h'
	binary.BigEndian.PutUint64(canonicalKey[1:9], height)
	canonicalKey[9] = 'n'
	
	fmt.Printf("Looking for canonical key at height %d\n", height)
	fmt.Printf("Key: %s\n", hex.EncodeToString(canonicalKey))
	
	if hashBytes, err := db.Get(canonicalKey); err == nil {
		hashHex := hex.EncodeToString(hashBytes)
		fmt.Printf("\nFound canonical hash: %s\n", hashHex)
		fmt.Printf("\nExport these for launch script:\n")
		fmt.Printf("export LUX_IMPORTED_HEIGHT=%d\n", height)
		fmt.Printf("export LUX_IMPORTED_BLOCK_ID=%s\n", hashHex)
	} else {
		fmt.Printf("Canonical hash not found: %v\n", err)
	}
}