package main

import (
	"fmt"
	"log"
	"math/big"
	"path/filepath"

	"github.com/luxfi/database/badgerdb"
	"github.com/luxfi/geth/common"
	"github.com/luxfi/geth/core/rawdb"
	"github.com/luxfi/geth/params"
	"github.com/prometheus/client_golang/prometheus"
)

func main() {
	// Point to the actual ethdb directory where we migrated the data
	ethdbPath := filepath.Clean(
		filepath.Join(
			"/home/z/.luxd",
			"chainData",
			"xBBY6aJcNichNCkCXgUcG5Gv2PW6FLS81LYDV8VwnPuadKGqm",
			"ethdb",
		),
	)

	fmt.Printf("Opening database at: %s\n", ethdbPath)
	badgerDB, err := badgerdb.New(ethdbPath, nil, "", prometheus.DefaultRegisterer)
	if err != nil {
		log.Fatal("Failed to open database:", err)
	}
	defer badgerDB.Close()
	
	// Wrap in a compatible interface
	db := rawdb.NewDatabase(badgerDB)

	// Get the genesis hash that's already in the DB
	ghash := rawdb.ReadCanonicalHash(db, 0)
	if ghash == (common.Hash{}) {
		log.Fatal("No canonical genesis hash in DB")
	}
	fmt.Printf("Genesis hash in DB: %s\n", ghash.Hex())

	// Read existing config (if any)
	oldCfg := rawdb.ReadChainConfig(db, ghash)
	if oldCfg != nil {
		fmt.Printf("Existing ChainID: %s\n", oldCfg.ChainID)
	} else {
		fmt.Println("No ChainConfig stored; will write one.")
	}

	// Build the CORRECT config for chain ID 96369
	newCfg := &params.ChainConfig{
		ChainID:             big.NewInt(96369),
		HomesteadBlock:      big.NewInt(0),
		EIP150Block:         big.NewInt(0),
		EIP155Block:         big.NewInt(0),
		EIP158Block:         big.NewInt(0),
		ByzantiumBlock:      big.NewInt(0),
		ConstantinopleBlock: big.NewInt(0),
		PetersburgBlock:     big.NewInt(0),
		IstanbulBlock:       big.NewInt(0),
		MuirGlacierBlock:    big.NewInt(0),
		BerlinBlock:         big.NewInt(0),
		LondonBlock:         big.NewInt(0),
		ArrowGlacierBlock:   big.NewInt(0),
		GrayGlacierBlock:    big.NewInt(0),
		MergeNetsplitBlock:  big.NewInt(0),
		ShanghaiTime:        new(uint64), // Set to 0
		CancunTime:          new(uint64), // Set to 0
	}

	// Write config under the genesis hash key
	rawdb.WriteChainConfig(db, ghash, newCfg)
	fmt.Println("ChainConfig written with ChainID 96369")

	// Re-read to confirm
	chk := rawdb.ReadChainConfig(db, ghash)
	if chk == nil || chk.ChainID.Cmp(newCfg.ChainID) != 0 {
		log.Fatal("Failed to persist ChainConfig or ChainID mismatch")
	}
	fmt.Printf("Confirmed ChainID in DB: %s\n", chk.ChainID)

	// Also check heads are set properly
	headHash := rawdb.ReadHeadBlockHash(db)
	if headHash != (common.Hash{}) {
		fmt.Printf("Head block hash: %s\n", headHash.Hex())
		
		// Get block number for this hash  
		numPtr := rawdb.ReadHeaderNumber(db, headHash)
		if numPtr != nil {
			fmt.Printf("Head block number: %d\n", *numPtr)
		}
	} else {
		fmt.Println("No head block hash set")
	}
}