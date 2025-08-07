package main

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"os"
	"path/filepath"

	"github.com/cockroachdb/pebble"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/params"
)

// ReplayData contains information for replaying blocks
type ReplayData struct {
	Genesis     *core.Genesis         `json:"genesis"`
	TotalBlocks uint64                `json:"totalBlocks"`
	Blocks      map[uint64]BlockData  `json:"blocks,omitempty"` // For initial testing
}

// BlockData contains minimal block info for replay
type BlockData struct {
	Number uint64      `json:"number"`
	Hash   common.Hash `json:"hash"`
}

func main() {
	if len(os.Args) != 2 {
		log.Fatal("Usage: create-replay <output-file>")
	}

	outputFile := os.Args[1]

	// Open the migrated database
	dbPath := filepath.Join("state", "chaindata", "lux-mainnet-96369", "db")
	fmt.Printf("Opening database: %s\n", dbPath)
	
	db, err := pebble.Open(dbPath, &pebble.Options{ReadOnly: true})
	if err != nil {
		log.Fatal("Failed to open database:", err)
	}
	defer db.Close()

	// Get genesis hash
	canonicalKey := make([]byte, 9)
	canonicalKey[0] = 'H'
	// Block 0

	val, closer, err := db.Get(canonicalKey)
	if err != nil {
		log.Fatal("Failed to read canonical hash at block 0:", err)
	}
	closer.Close()

	var genesisHash common.Hash
	copy(genesisHash[:], val)
	fmt.Printf("Genesis hash: %s\n", genesisHash.Hex())

	// Create genesis configuration
	// This matches what the SubnetEVM originally had
	genesis := &core.Genesis{
		Config: &params.ChainConfig{
			ChainID:                 big.NewInt(96369),
			HomesteadBlock:          big.NewInt(0),
			EIP150Block:             big.NewInt(0),
			EIP155Block:             big.NewInt(0),
			EIP158Block:             big.NewInt(0),
			ByzantiumBlock:          big.NewInt(0),
			ConstantinopleBlock:     big.NewInt(0),
			PetersburgBlock:         big.NewInt(0),
			IstanbulBlock:           big.NewInt(0),
			MuirGlacierBlock:        big.NewInt(0),
			BerlinBlock:             big.NewInt(0),
			LondonBlock:             big.NewInt(0),
			TerminalTotalDifficulty: big.NewInt(0),
		},
		Nonce:      0,
		Timestamp:  1638316800, // Dec 1, 2021
		ExtraData:  []byte{},
		GasLimit:   15000000,
		Difficulty: big.NewInt(0),
		Mixhash:    common.Hash{},
		Coinbase:   common.Address{},
		Alloc:      make(core.GenesisAlloc),
		Number:     0,
		GasUsed:    0,
		ParentHash: common.Hash{},
		BaseFee:    big.NewInt(25000000000), // 25 gwei
	}

	// Add initial allocation
	genesis.Alloc[common.HexToAddress("0x8db97C7cEcE249c2b98bDC0226Cc4C2A57BF52FC")] = core.GenesisAccount{
		Balance: new(big.Int).Mul(big.NewInt(50000000), big.NewInt(1e18)), // 50M tokens
	}

	// Find the highest block
	blocks := make(map[uint64]BlockData)

	// Sample first 100 blocks for testing
	for blockNum := uint64(0); blockNum <= 100; blockNum++ {
		canonicalKey := make([]byte, 9)
		canonicalKey[0] = 'H'
		binary.BigEndian.PutUint64(canonicalKey[1:], blockNum)

		hashBytes, closer, err := db.Get(canonicalKey)
		if err != nil {
			if blockNum > 0 {
				break
			}
			continue
		}
		closer.Close()

		var blockHash common.Hash
		copy(blockHash[:], hashBytes)
		
		blocks[blockNum] = BlockData{
			Number: blockNum,
			Hash:   blockHash,
		}
	}

	// Count total blocks
	fmt.Println("Counting total blocks...")
	totalBlocks := uint64(0)
	
	// Binary search for the highest block
	low := uint64(0)
	high := uint64(2000000) // Start with 2M as upper bound
	
	for low <= high {
		mid := (low + high) / 2
		
		canonicalKey := make([]byte, 9)
		canonicalKey[0] = 'H'
		binary.BigEndian.PutUint64(canonicalKey[1:], mid)
		
		_, closer, err := db.Get(canonicalKey)
		if err == nil {
			closer.Close()
			totalBlocks = mid
			low = mid + 1
		} else {
			high = mid - 1
		}
	}

	fmt.Printf("Total blocks in database: %d\n", totalBlocks+1)

	// Create replay data
	replayData := ReplayData{
		Genesis:     genesis,
		TotalBlocks: totalBlocks + 1,
		Blocks:      blocks,
	}

	// Write to file
	data, err := json.MarshalIndent(replayData, "", "  ")
	if err != nil {
		log.Fatal("Failed to marshal replay data:", err)
	}

	if err := os.WriteFile(outputFile, data, 0644); err != nil {
		log.Fatal("Failed to write replay file:", err)
	}

	fmt.Printf("\nReplay data written to: %s\n", outputFile)
	fmt.Printf("Total blocks: %d\n", totalBlocks+1)
	fmt.Printf("Genesis hash: %s\n", genesisHash.Hex())
}