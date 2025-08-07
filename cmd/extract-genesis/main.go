package main

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"os"

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

func main() {
	if len(os.Args) != 3 {
		log.Fatal("Usage: extract-genesis <subnet-db-path> <output-genesis.json>")
	}

	dbPath := os.Args[1]
	outputPath := os.Args[2]

	// Open the SubnetEVM database
	db, err := pebble.Open(dbPath, &pebble.Options{ReadOnly: true})
	if err != nil {
		log.Fatal("Failed to open database:", err)
	}
	defer db.Close()

	// Read block 0 header
	// SubnetEVM key format: 0x33 + 31-byte padding + "H" + number(8) + hash(32)
	padding := []byte{
		0x7f, 0xb7, 0x3f, 0x9b, 0xcd, 0xac, 0x8c, 0x31,
		0xa2, 0xd5, 0xf7, 0xb8, 0x77, 0xab, 0x1e, 0x8a,
		0x2b, 0x7f, 0x2a, 0x1e, 0x9b, 0xf0, 0x2a, 0x0a,
		0x0e, 0x6c, 0x6f, 0xd1, 0x64, 0xf1, 0xd1,
	}

	// First, get the genesis hash from canonical key
	canonicalKey := make([]byte, 41)
	canonicalKey[0] = 0x33
	copy(canonicalKey[1:32], padding)
	canonicalKey[32] = 'H'
	// Block number 0 as 8 bytes big endian

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

	// Read the header
	headerKey := make([]byte, 73)
	headerKey[0] = 0x33
	copy(headerKey[1:32], padding)
	headerKey[32] = 'h'
	binary.BigEndian.PutUint64(headerKey[33:41], 0)
	copy(headerKey[41:], genesisHash[:])

	headerData, closer, err := db.Get(headerKey)
	if err != nil {
		log.Fatal("Failed to read genesis header:", err)
	}
	closer.Close()

	// Decode the header
	var header SubnetEVMHeader
	if err := rlp.DecodeBytes(headerData, &header); err != nil {
		log.Fatal("Failed to decode header:", err)
	}

	fmt.Printf("Genesis block details:\n")
	fmt.Printf("  Timestamp: %d\n", header.Time)
	fmt.Printf("  Gas Limit: %d\n", header.GasLimit)
	fmt.Printf("  Difficulty: %s\n", header.Difficulty.String())
	fmt.Printf("  State Root: %s\n", header.Root.Hex())
	fmt.Printf("  Base Fee: %s\n", header.BaseFee.String())

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

	// Now we need to extract the state at block 0
	// This requires reading all accounts from the state trie
	fmt.Println("\nExtracting genesis state...")
	
	// For now, let's create a minimal genesis with just the important accounts
	// In production, we'd iterate through the state trie
	
	// Add some known accounts with balance
	genesis.Alloc[common.HexToAddress("0x8db97C7cEcE249c2b98bDC0226Cc4C2A57BF52FC")] = core.GenesisAccount{
		Balance: new(big.Int).Mul(big.NewInt(50000000), big.NewInt(1e18)), // 50M tokens
	}

	// Write genesis file
	genesisJSON, err := json.MarshalIndent(genesis, "", "  ")
	if err != nil {
		log.Fatal("Failed to marshal genesis:", err)
	}

	if err := os.WriteFile(outputPath, genesisJSON, 0644); err != nil {
		log.Fatal("Failed to write genesis file:", err)
	}

	fmt.Printf("\nGenesis file written to: %s\n", outputPath)
	fmt.Printf("Genesis hash: %s\n", genesisHash.Hex())
}