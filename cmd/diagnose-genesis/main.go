package main

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/cockroachdb/pebble"
	"github.com/luxfi/geth/common"
	"github.com/luxfi/geth/core"
)

func main() {
	dbPath := "/home/z/.luxd/network-96369/chains/X6CU5qgMJfzsTB9UWxj2ZY5hd68x41HfZ4m4hCBWbHuj1Ebc3/ethdb"
	
	fmt.Printf("=== Genesis Diagnosis ===\n")
	fmt.Printf("Database: %s\n\n", dbPath)
	
	// Open database
	opts := &pebble.Options{
		Cache:        pebble.NewCache(256 << 20),
		MaxOpenFiles: 1024,
		ReadOnly:     true,
	}
	
	db, err := pebble.Open(dbPath, opts)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()
	
	// Check for chain config under different keys
	fmt.Println("1. Checking for chain config...")
	
	// Check canonical genesis
	canonicalKey := append(append([]byte("h"), encodeBlockNumber(0)...), byte('n'))
	if val, closer, err := db.Get(canonicalKey); err == nil {
		genesisHash := common.BytesToHash(val)
		fmt.Printf("  Canonical genesis hash: %s\n", genesisHash.Hex())
		closer.Close()
		
		// Check for config under genesis hash
		configKey := append([]byte("ethereum-config-"), genesisHash.Bytes()...)
		if configVal, closer, err := db.Get(configKey); err == nil {
			fmt.Printf("  Found chain config under genesis hash\n")
			var config map[string]interface{}
			if err := json.Unmarshal(configVal, &config); err == nil {
				fmt.Printf("  ChainID: %v\n", config["chainId"])
			}
			closer.Close()
		} else {
			fmt.Printf("  No chain config found under key: ethereum-config-%s\n", genesisHash.Hex())
		}
	}
	
	// Check for genesis header
	fmt.Println("\n2. Checking genesis block...")
	genesisHeaderKey := append(append([]byte("h"), encodeBlockNumber(0)...), byte('n'))
	if hashBytes, closer, err := db.Get(genesisHeaderKey); err == nil {
		hash := common.BytesToHash(hashBytes)
		closer.Close()
		
		// Get the actual header
		headerKey := append(append([]byte("h"), encodeBlockNumber(0)...), hash.Bytes()...)
		if headerBytes, closer, err := db.Get(headerKey); err == nil {
			fmt.Printf("  Genesis header found (size: %d bytes)\n", len(headerBytes))
			closer.Close()
			
			// Just note we have the header
			fmt.Printf("  Genesis header stored under hash: %s\n", hash.Hex())
		}
	}
	
	// Check for genesis state
	fmt.Println("\n3. Checking for genesis state...")
	
	// Look for genesis JSON
	possibleKeys := []string{
		"genesis",
		"genesis-json", 
		"chain-genesis",
		"ethereum-genesis",
	}
	
	for _, key := range possibleKeys {
		if val, closer, err := db.Get([]byte(key)); err == nil {
			fmt.Printf("  Found data under key '%s' (size: %d bytes)\n", key, len(val))
			
			// Try to parse as genesis
			var genesis core.Genesis
			if err := json.Unmarshal(val, &genesis); err == nil {
				fmt.Printf("    Parsed as genesis: ChainID=%v, Nonce=%v\n", genesis.Config.ChainID, genesis.Nonce)
			}
			closer.Close()
		}
	}
	
	// Check what Coreth expects
	fmt.Println("\n4. Checking Coreth expectations...")
	fmt.Println("  Coreth expects:")
	fmt.Println("  - Canonical hash at key: h + num(8BE) + 'n'")
	fmt.Println("  - Header at key: h + num(8BE) + hash")
	fmt.Println("  - Chain config at key: ethereum-config- + genesisHash")
	fmt.Println("  - Optional: genesis JSON for initialization")
	
	// Check VM metadata
	fmt.Println("\n5. Checking VM metadata...")
	vmPath := "/home/z/.luxd/network-96369/chains/X6CU5qgMJfzsTB9UWxj2ZY5hd68x41HfZ4m4hCBWbHuj1Ebc3/vm/"
	checkFile := func(name string) {
		path := vmPath + name
		if data, err := os.ReadFile(path); err == nil {
			if name == "lastAccepted" {
				fmt.Printf("  %s: %s\n", name, hex.EncodeToString(data))
			} else if name == "lastAcceptedHeight" {
				height := binary.BigEndian.Uint64(data)
				fmt.Printf("  %s: %d\n", name, height)
			} else {
				fmt.Printf("  %s: %v\n", name, data)
			}
		} else {
			fmt.Printf("  %s: NOT FOUND\n", name)
		}
	}
	
	checkFile("lastAccepted")
	checkFile("lastAcceptedHeight")
	checkFile("initialized")
	
	fmt.Println("\n=== Diagnosis Complete ===")
}

func encodeBlockNumber(number uint64) []byte {
	enc := make([]byte, 8)
	binary.BigEndian.PutUint64(enc, number)
	return enc
}