package main

import (
	"encoding/binary"
	"fmt"
	"os"

	"github.com/luxfi/database/pebbledb"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run write-height-key.go <db-path>")
		return
	}

	// Open database using Lux's pebbledb wrapper
	// New(file string, cacheSize int, handles int, namespace string, readonly bool)
	db, err := pebbledb.New(os.Args[1], 0, 0, "", false)
	if err != nil {
		panic(err)
	}
	defer db.Close()

	// Write Height key
	heightBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(heightBytes, 1082780)
	
	if err := db.Put([]byte("Height"), heightBytes); err != nil {
		panic(err)
	}
	
	fmt.Println("Successfully wrote Height=1082780 to database")
	
	// Verify it was written
	if val, err := db.Get([]byte("Height")); err == nil {
		height := binary.BigEndian.Uint64(val)
		fmt.Printf("Verified: Height=%d\n", height)
	}
}