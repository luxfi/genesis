package main

import (
	"encoding/binary"
	"fmt"
	"os"

	"github.com/luxfi/geth/common"
	"github.com/luxfi/geth/core/rawdb"
)

func main() {
	// Open the actual ethdb directory that the VM uses
	dbPath := "/home/z/.luxd/network-96369/chains/X6CU5qgMJfzsTB9UWxj2ZY5hd68x41HfZ4m4hCBWbHuj1Ebc3/ethdb"
	
	if len(os.Args) > 1 {
		dbPath = os.Args[1]
	}
	
	fmt.Printf("=== Probing Database: %s ===\n\n", dbPath)
	
	// Open the database
	db, err := rawdb.Open(rawdb.OpenOptions{
		Type:              "pebble",
		Directory:         dbPath,
		AncientsDirectory: "",
		Namespace:         "",
		Cache:             0,
		Handles:           0,
		ReadOnly:          true,
	})
	if err != nil {
		fmt.Printf("Error opening database: %v\n", err)
		return
	}
	defer db.Close()
	
	// 1. Check genesis hash (H0db)
	H0db := rawdb.ReadCanonicalHash(db, 0)
	fmt.Printf("1. Genesis hash (H0db): %s\n", H0db.Hex())
	if H0db == (common.Hash{}) {
		fmt.Printf("   ❌ No genesis hash found!\n")
		return
	}
	fmt.Printf("   ✅ Genesis hash present\n\n")
	
	// 2. Check genesis header
	hdr0 := rawdb.ReadHeader(db, H0db, 0)
	if hdr0 == nil {
		fmt.Printf("2. Genesis header: ❌ NOT FOUND\n")
		fmt.Printf("   Migration didn't write h+num(8)+hash(32) headers properly\n")
		return
	}
	fmt.Printf("2. Genesis header: ✅ Found\n")
	fmt.Printf("   - Number: %d\n", hdr0.Number.Uint64())
	fmt.Printf("   - Time: %d\n", hdr0.Time)
	fmt.Printf("   - GasLimit: %d\n", hdr0.GasLimit)
	fmt.Printf("   - Difficulty: %v\n", hdr0.Difficulty)
	fmt.Printf("   - Root: %s\n\n", hdr0.Root.Hex())
	
	// 3. Check chain config
	cfgDB := rawdb.ReadChainConfig(db, H0db)
	if cfgDB == nil {
		fmt.Printf("3. Chain config: ❌ MISSING\n")
		fmt.Printf("   Need to write chain config for genesis %s\n\n", H0db.Hex())
	} else {
		fmt.Printf("3. Chain config: ✅ Present\n")
		fmt.Printf("   - ChainID: %v\n", cfgDB.ChainID)
		if cfgDB.BlobScheduleConfig != nil {
			fmt.Printf("   - BlobScheduleConfig: ✅ Present\n\n")
		} else {
			fmt.Printf("   - BlobScheduleConfig: ❌ Missing\n\n")
		}
	}
	
	// 4. Check current heads
	headHash := rawdb.ReadHeadBlockHash(db)
	headHeaderHash := rawdb.ReadHeadHeaderHash(db)
	fastHash := rawdb.ReadHeadFastBlockHash(db)
	
	fmt.Printf("4. Current heads:\n")
	fmt.Printf("   - HeadBlockHash: %s\n", headHash.Hex())
	fmt.Printf("   - HeadHeaderHash: %s\n", headHeaderHash.Hex())
	fmt.Printf("   - HeadFastBlockHash: %s\n", fastHash.Hex())
	
	if headHash != (common.Hash{}) {
		number, found := rawdb.ReadHeaderNumber(db, headHash)
		if found {
			fmt.Printf("   - Head block number: %d\n", number)
			header := rawdb.ReadHeader(db, headHash, number)
			if header != nil {
				fmt.Printf("   - ✅ Head header found at height %d\n\n", number)
			} else {
				fmt.Printf("   - ❌ Head header NOT found at height %d\n\n", number)
			}
		}
	}
	
	// 5. Check VM metadata (lastAccepted)
	var lastAcceptedKey = []byte("lastAccepted")
	var lastAcceptedHeightKey = []byte("lastAcceptedHeight")
	
	lastAcceptedBytes, err := db.Get(lastAcceptedKey)
	if err == nil && len(lastAcceptedBytes) == 32 {
		lastAccepted := common.BytesToHash(lastAcceptedBytes)
		fmt.Printf("5. VM metadata:\n")
		fmt.Printf("   - lastAccepted: %s\n", lastAccepted.Hex())
		
		heightBytes, err := db.Get(lastAcceptedHeightKey)
		if err == nil && len(heightBytes) == 8 {
			height := binary.BigEndian.Uint64(heightBytes)
			fmt.Printf("   - lastAcceptedHeight: %d\n", height)
			fmt.Printf("   - ✅ VM metadata present\n\n")
		} else {
			fmt.Printf("   - ❌ lastAcceptedHeight missing or invalid\n\n")
		}
	} else {
		fmt.Printf("5. VM metadata: ❌ Missing or invalid\n\n")
	}
	
	// 6. Summary
	fmt.Printf("=== Summary ===\n")
	if H0db != (common.Hash{}) && hdr0 != nil {
		fmt.Printf("✅ Genesis present and valid\n")
		if cfgDB != nil {
			fmt.Printf("✅ Chain config present\n")
		} else {
			fmt.Printf("❌ Chain config missing - needs to be written\n")
		}
		if headHash != (common.Hash{}) {
			fmt.Printf("✅ Tip invariants OK\n")
		} else {
			fmt.Printf("⚠️  No head block set\n")
		}
	} else {
		fmt.Printf("❌ Database not properly initialized for migration\n")
	}
}