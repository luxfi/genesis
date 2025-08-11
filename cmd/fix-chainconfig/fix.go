package main

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log"
	"math/big"

	"github.com/luxfi/database/badgerdb"
	"github.com/prometheus/client_golang/prometheus"
)

type ChainConfig struct {
	ChainID *big.Int `json:"chainId"`
}

func main() {
	// Open the migrated database
	ethdbPath := "/home/z/.luxd/chainData/xBBY6aJcNichNCkCXgUcG5Gv2PW6FLS81LYDV8VwnPuadKGqm/ethdb"
	
	fmt.Printf("Opening database at: %s\n", ethdbPath)
	db, err := badgerdb.New(ethdbPath, nil, "", prometheus.DefaultRegisterer)
	if err != nil {
		log.Fatal("Failed to open database:", err)
	}
	defer db.Close()

	// Get genesis hash (canonical hash at block 0)
	canonKey := make([]byte, 10)
	canonKey[0] = 'h'
	binary.BigEndian.PutUint64(canonKey[1:9], 0)
	canonKey[9] = 'n'
	
	genesisHash, err := db.Get(canonKey)
	if err != nil || len(genesisHash) != 32 {
		log.Fatal("No genesis hash found")
	}
	fmt.Printf("Genesis hash: %x\n", genesisHash)

	// Create chain config for chain ID 96369
	config := &ChainConfig{
		ChainID: big.NewInt(96369),
	}
	
	configBytes, err := json.Marshal(config)
	if err != nil {
		log.Fatal("Failed to marshal config:", err)
	}
	
	// Write chain config under ethereum-config-{genesis_hash} key
	configKey := append([]byte("ethereum-config-"), genesisHash...)
	err = db.Put(configKey, configBytes)
	if err != nil {
		log.Fatal("Failed to write chain config:", err)
	}
	
	fmt.Printf("Wrote chain config with ChainID 96369\n")
	
	// Verify it was written
	readConfig, err := db.Get(configKey)
	if err != nil {
		log.Fatal("Failed to read back config:", err)
	}
	
	var checkConfig ChainConfig
	json.Unmarshal(readConfig, &checkConfig)
	fmt.Printf("Verified ChainID in DB: %s\n", checkConfig.ChainID)
	
	// Check heads
	if headHash, err := db.Get([]byte("LastBlock")); err == nil && len(headHash) == 32 {
		fmt.Printf("LastBlock hash: %x\n", headHash)
		
		// Get block number
		hashNumKey := append([]byte("H"), headHash...)
		if numBytes, err := db.Get(hashNumKey); err == nil && len(numBytes) == 8 {
			num := binary.BigEndian.Uint64(numBytes)
			fmt.Printf("LastBlock is at height: %d\n", num)
		}
	}
}