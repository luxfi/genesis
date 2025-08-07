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
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rlp"
)

// SubnetEVMHeader represents the header structure with withdrawals
type SubnetEVMHeader struct {
	ParentHash       common.Hash    `json:"parentHash"`
	UncleHash        common.Hash    `json:"sha3Uncles"`
	Coinbase         common.Address `json:"miner"`
	Root             common.Hash    `json:"stateRoot"`
	TxHash           common.Hash    `json:"transactionsRoot"`
	ReceiptHash      common.Hash    `json:"receiptsRoot"`
	Bloom            types.Bloom    `json:"logsBloom"`
	Difficulty       *big.Int       `json:"difficulty"`
	Number           *big.Int       `json:"number"`
	GasLimit         uint64         `json:"gasLimit"`
	GasUsed          uint64         `json:"gasUsed"`
	Time             uint64         `json:"timestamp"`
	Extra            []byte         `json:"extraData"`
	MixDigest        common.Hash    `json:"mixHash"`
	Nonce            types.BlockNonce `json:"nonce"`
	BaseFee          *big.Int       `json:"baseFeePerGas"`
	WithdrawalsHash  *common.Hash   `json:"withdrawalsHash,omitempty"`
}

// BlockInfo stores block data for replay
type BlockInfo struct {
	Number uint64       `json:"number"`
	Hash   common.Hash  `json:"hash"`
	Header []byte       `json:"header"`
	Body   []byte       `json:"body"`
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: replay <command> [args]")
		fmt.Println("Commands:")
		fmt.Println("  extract-genesis <output-dir>  - Extract genesis from zoo-mainnet")
		fmt.Println("  prepare-blocks <output-dir>   - Prepare blocks for replay")
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "extract-genesis":
		if len(os.Args) != 3 {
			log.Fatal("Usage: replay extract-genesis <output-dir>")
		}
		extractGenesis(os.Args[2])

	case "prepare-blocks":
		if len(os.Args) != 3 {
			log.Fatal("Usage: replay prepare-blocks <output-dir>")
		}
		prepareBlocks(os.Args[2])

	default:
		log.Fatal("Unknown command:", command)
	}
}

func extractGenesis(outputDir string) {
	// Use the migrated C-chain data
	dbPath := filepath.Join("state", "chaindata", "lux-mainnet-96369", "db")
	
	fmt.Printf("Opening database: %s\n", dbPath)
	db, err := pebble.Open(dbPath, &pebble.Options{ReadOnly: true})
	if err != nil {
		log.Fatal("Failed to open database:", err)
	}
	defer db.Close()

	// Zoo mainnet uses standard key format without prefix
	// Canonical hash key: "H" + number(8)
	canonicalKey := make([]byte, 9)
	canonicalKey[0] = 'H'
	// Block 0

	val, closer, err := db.Get(canonicalKey)
	if err != nil {
		log.Fatal("Failed to read canonical hash at block 0:", err)
	}
	closer.Close()

	if len(val) != 32 {
		log.Fatal("Invalid canonical hash length:", len(val))
	}

	var genesisHash common.Hash
	copy(genesisHash[:], val)
	fmt.Printf("Genesis hash: %s\n", genesisHash.Hex())

	// Read the header - try both formats
	// Format 1: "h" + number(8) + hash(32)
	headerKey := append([]byte("h"), make([]byte, 8)...)
	headerKey = append(headerKey, genesisHash[:]...)
	
	// Also prepare format 2: just hash as key
	hashKey := genesisHash[:]

	headerData, closer, err := db.Get(headerKey)
	if err != nil {
		log.Fatal("Failed to read genesis header:", err)
	}
	closer.Close()

	// Decode the header
	var header types.Header
	if err := rlp.DecodeBytes(headerData, &header); err != nil {
		// Try SubnetEVM format
		var subnetHeader SubnetEVMHeader
		if err := rlp.DecodeBytes(headerData, &subnetHeader); err != nil {
			log.Fatal("Failed to decode header:", err)
		}
		// Convert to standard header
		header = types.Header{
			ParentHash:  subnetHeader.ParentHash,
			UncleHash:   subnetHeader.UncleHash,
			Coinbase:    subnetHeader.Coinbase,
			Root:        subnetHeader.Root,
			TxHash:      subnetHeader.TxHash,
			ReceiptHash: subnetHeader.ReceiptHash,
			Bloom:       subnetHeader.Bloom,
			Difficulty:  subnetHeader.Difficulty,
			Number:      subnetHeader.Number,
			GasLimit:    subnetHeader.GasLimit,
			GasUsed:     subnetHeader.GasUsed,
			Time:        subnetHeader.Time,
			Extra:       subnetHeader.Extra,
			MixDigest:   subnetHeader.MixDigest,
			Nonce:       subnetHeader.Nonce,
			BaseFee:     subnetHeader.BaseFee,
		}
	}

	fmt.Printf("Genesis block details:\n")
	fmt.Printf("  Timestamp: %d\n", header.Time)
	fmt.Printf("  Gas Limit: %d\n", header.GasLimit)
	fmt.Printf("  Difficulty: %s\n", header.Difficulty.String())
	fmt.Printf("  State Root: %s\n", header.Root.Hex())
	if header.BaseFee != nil {
		fmt.Printf("  Base Fee: %s\n", header.BaseFee.String())
	}

	// Create genesis structure
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
		Nonce:      uint64(header.Nonce.Uint64()),
		Timestamp:  header.Time,
		ExtraData:  header.Extra,
		GasLimit:   header.GasLimit,
		Difficulty: header.Difficulty,
		Mixhash:    header.MixDigest,
		Coinbase:   header.Coinbase,
		Alloc:      make(core.GenesisAlloc),
		Number:     0,
		GasUsed:    header.GasUsed,
		ParentHash: header.ParentHash,
		BaseFee:    header.BaseFee,
	}

	// For now, create minimal allocation
	// In production, we'd need to extract the full state
	genesis.Alloc[common.HexToAddress("0x8db97C7cEcE249c2b98bDC0226Cc4C2A57BF52FC")] = core.GenesisAccount{
		Balance: new(big.Int).Mul(big.NewInt(50000000), big.NewInt(1e18)), // 50M tokens
	}

	// Create output directory
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		log.Fatal("Failed to create output directory:", err)
	}

	// Write genesis file
	genesisPath := filepath.Join(outputDir, "genesis.json")
	genesisJSON, err := json.MarshalIndent(genesis, "", "  ")
	if err != nil {
		log.Fatal("Failed to marshal genesis:", err)
	}

	if err := os.WriteFile(genesisPath, genesisJSON, 0644); err != nil {
		log.Fatal("Failed to write genesis file:", err)
	}

	fmt.Printf("\nGenesis file written to: %s\n", genesisPath)
	fmt.Printf("Expected genesis hash: %s\n", genesisHash.Hex())
}

func prepareBlocks(outputDir string) {
	// Use the migrated C-chain data
	dbPath := filepath.Join("state", "chaindata", "lux-mainnet-96369", "db")
	
	fmt.Printf("Opening database: %s\n", dbPath)
	db, err := pebble.Open(dbPath, &pebble.Options{ReadOnly: true})
	if err != nil {
		log.Fatal("Failed to open database:", err)
	}
	defer db.Close()

	// Create output directory
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		log.Fatal("Failed to create output directory:", err)
	}

	// Create blocks subdirectory
	blocksDir := filepath.Join(outputDir, "blocks")
	if err := os.MkdirAll(blocksDir, 0755); err != nil {
		log.Fatal("Failed to create blocks directory:", err)
	}

	// Prepare blocks for replay (first 100 as a test)
	fmt.Println("Preparing blocks for replay...")
	
	for blockNum := uint64(1); blockNum <= 100; blockNum++ {
		// Get canonical hash
		canonicalKey := make([]byte, 9)
		canonicalKey[0] = 'H'
		binary.BigEndian.PutUint64(canonicalKey[1:], blockNum)

		hashBytes, closer, err := db.Get(canonicalKey)
		if err != nil {
			fmt.Printf("No block at height %d, stopping\n", blockNum)
			break
		}
		closer.Close()

		var blockHash common.Hash
		copy(blockHash[:], hashBytes)

		// Get header
		headerKey := append([]byte("h"), make([]byte, 8)...)
		binary.BigEndian.PutUint64(headerKey[1:], blockNum)
		headerKey = append(headerKey, blockHash[:]...)

		headerData, closer, err := db.Get(headerKey)
		if err != nil {
			fmt.Printf("Failed to get header for block %d: %v\n", blockNum, err)
			continue
		}
		closer.Close()

		// Get body
		bodyKey := append([]byte("b"), make([]byte, 8)...)
		binary.BigEndian.PutUint64(bodyKey[1:], blockNum)
		bodyKey = append(bodyKey, blockHash[:]...)

		bodyData, closer, err := db.Get(bodyKey)
		if err != nil {
			fmt.Printf("Failed to get body for block %d: %v\n", blockNum, err)
			continue
		}
		closer.Close()

		// Save block info
		blockInfo := BlockInfo{
			Number: blockNum,
			Hash:   blockHash,
			Header: headerData,
			Body:   bodyData,
		}

		blockPath := filepath.Join(blocksDir, fmt.Sprintf("block_%06d.json", blockNum))
		blockJSON, err := json.Marshal(blockInfo)
		if err != nil {
			fmt.Printf("Failed to marshal block %d: %v\n", blockNum, err)
			continue
		}

		if err := os.WriteFile(blockPath, blockJSON, 0644); err != nil {
			fmt.Printf("Failed to write block %d: %v\n", blockNum, err)
			continue
		}

		if blockNum%10 == 0 {
			fmt.Printf("Prepared block %d\n", blockNum)
		}
	}

	fmt.Printf("\nBlocks prepared in: %s\n", blocksDir)
}