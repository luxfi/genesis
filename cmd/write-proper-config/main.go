package main

import (
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	
	"github.com/dgraph-io/badger/v4"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/params"
)

func main() {
	dbPath := "/home/z/.luxd/db/network-96369/chains/X6CU5qgMJfzsTB9UWxj2ZY5hd68x41HfZ4m4hCBWbHuj1Ebc3/ethdb"
	
	fmt.Printf("Writing proper chain config to BadgerDB at: %s\n", dbPath)
	
	opts := badger.DefaultOptions(dbPath)
	opts.SyncWrites = false
	opts.Logger = nil
	
	db, err := badger.Open(opts)
	if err != nil {
		fmt.Printf("Error opening database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()
	
	// Create a proper chain config using the actual go-ethereum types
	zero := uint64(0)
	config := &params.ChainConfig{
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
		ShanghaiTime:        &zero,
		CancunTime:          &zero,
		BlobScheduleConfig: &params.BlobScheduleConfig{
			Cancun: &params.BlobConfig{
				Target:         3,
				Max:            6,
				UpdateFraction: 3338477,
			},
		},
		TerminalTotalDifficulty: big.NewInt(0),
	}
	
	// Encode the config
	configBytes, err := json.Marshal(config)
	if err != nil {
		fmt.Printf("Error encoding chain config: %v\n", err)
		os.Exit(1)
	}
	
	genesisHash := common.HexToHash("0x3f4fa2a0b0ce089f52bf0ae9199c75ffdd76ecafc987794050cb0d286f1ec61e")
	
	err = db.Update(func(txn *badger.Txn) error {
		// Write chain config under genesis hash
		configKey := append([]byte("ethereum-config-"), genesisHash[:]...)
		
		if err := txn.Set(configKey, configBytes); err != nil {
			return fmt.Errorf("failed to write chain config: %w", err)
		}
		fmt.Printf("✅ Wrote chain config for genesis %x\n", genesisHash)
		fmt.Printf("Config: %s\n", string(configBytes))
		
		return nil
	})
	
	if err != nil {
		fmt.Printf("Error updating database: %v\n", err)
		os.Exit(1)
	}
	
	fmt.Printf("\n✅ Successfully wrote proper chain configuration!\n")
	fmt.Printf("Chain ID: 96369\n")
	fmt.Printf("All forks enabled from genesis\n")
	fmt.Printf("Cancun fork with proper BlobConfig included\n")
}