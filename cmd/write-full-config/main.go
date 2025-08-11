package main

import (
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	
	"github.com/dgraph-io/badger/v4"
	"github.com/ethereum/go-ethereum/common"
)

// Minimal chain config structure with BlobSchedule
type ChainConfig struct {
	ChainID             *big.Int          `json:"chainId"`
	HomesteadBlock      *big.Int          `json:"homesteadBlock,omitempty"`
	EIP150Block         *big.Int          `json:"eip150Block,omitempty"`
	EIP155Block         *big.Int          `json:"eip155Block,omitempty"`
	EIP158Block         *big.Int          `json:"eip158Block,omitempty"`
	ByzantiumBlock      *big.Int          `json:"byzantiumBlock,omitempty"`
	ConstantinopleBlock *big.Int          `json:"constantinopleBlock,omitempty"`
	PetersburgBlock     *big.Int          `json:"petersburgBlock,omitempty"`
	IstanbulBlock       *big.Int          `json:"istanbulBlock,omitempty"`
	MuirGlacierBlock    *big.Int          `json:"muirGlacierBlock,omitempty"`
	BerlinBlock         *big.Int          `json:"berlinBlock,omitempty"`
	LondonBlock         *big.Int          `json:"londonBlock,omitempty"`
	ArrowGlacierBlock   *big.Int          `json:"arrowGlacierBlock,omitempty"`
	GrayGlacierBlock    *big.Int          `json:"grayGlacierBlock,omitempty"`
	MergeNetsplitBlock  *big.Int          `json:"mergeNetsplitBlock,omitempty"`
	ShanghaiTime        *uint64           `json:"shanghaiTime,omitempty"`
	CancunTime          *uint64           `json:"cancunTime,omitempty"`
	BlobSchedule        map[string]interface{} `json:"blobSchedule,omitempty"`
	TerminalTotalDifficulty *big.Int      `json:"terminalTotalDifficulty,omitempty"`
}

func main() {
	dbPath := "/home/z/.luxd/db/network-96369/chains/X6CU5qgMJfzsTB9UWxj2ZY5hd68x41HfZ4m4hCBWbHuj1Ebc3/ethdb"
	
	fmt.Printf("Writing full chain config to BadgerDB at: %s\n", dbPath)
	
	opts := badger.DefaultOptions(dbPath)
	opts.SyncWrites = false
	opts.Logger = nil
	
	db, err := badger.Open(opts)
	if err != nil {
		fmt.Printf("Error opening database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()
	
	// Create chain config with BlobSchedule
	zero := uint64(0)
	config := &ChainConfig{
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
		BlobSchedule: map[string]interface{}{
			"cancun": map[string]interface{}{
				"target":         3,
				"max":            6,
				"updateFraction": 3338477,
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
	
	fmt.Printf("\n✅ Successfully wrote full chain configuration with BlobSchedule!\n")
	fmt.Printf("Chain ID: 96369\n")
	fmt.Printf("All forks enabled from genesis\n")
	fmt.Printf("Cancun fork with blob schedule included\n")
}