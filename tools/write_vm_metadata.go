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
	vmPath := "/Users/z/.luxd/network-96369/chains/X6CU5qgMJfzsTB9UWxj2ZY5hd68x41HfZ4m4hCBWbHuj1Ebc3/vm"
	
	// Open VM database
	vmdb, err := badgerdb.New(vmPath, nil, "", prometheus.DefaultRegisterer)
	if err != nil {
		log.Fatalf("Failed to open VM database: %v", err)
	}
	defer vmdb.Close()
	
	// Last accepted block hash
	lastBlockHashHex := "32dede1fc8e0f11ecde12fb42aef7933fc6c5fcf863bc277b5eac08ae4d461f0"
	lastBlockHash, _ := hex.DecodeString(lastBlockHashHex)
	
	// Last accepted height
	lastBlockHeight := uint64(1082780)
	
	// Write lastAccepted
	if err := vmdb.Put([]byte("lastAccepted"), lastBlockHash); err != nil {
		log.Fatalf("Failed to write lastAccepted: %v", err)
	}
	
	// Write lastAcceptedHeight
	heightBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(heightBytes, lastBlockHeight)
	if err := vmdb.Put([]byte("lastAcceptedHeight"), heightBytes); err != nil {
		log.Fatalf("Failed to write lastAcceptedHeight: %v", err)
	}
	
	// Write initialized flag
	if err := vmdb.Put([]byte("initialized"), []byte{1}); err != nil {
		log.Fatalf("Failed to write initialized: %v", err)
	}
	
	fmt.Printf("✅ VM metadata written successfully\n")
	fmt.Printf("   lastAccepted: 0x%s\n", lastBlockHashHex)
	fmt.Printf("   lastAcceptedHeight: %d\n", lastBlockHeight)
}