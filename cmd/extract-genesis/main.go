//go:build ignore

// extract-genesis extracts genesis configuration from an RLP file or PebbleDB
//
// Usage:
//
//	go run main.go --rlp <path-to-rlp-file> [--output <output-file>]
//	go run main.go --pebble <path-to-pebbledb> [--output <output-file>]
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/luxfi/geth/core/types"
	"github.com/luxfi/geth/rlp"
)

type GenesisExtract struct {
	Hash       string        `json:"hash"`
	StateRoot  string        `json:"stateRoot"`
	ParentHash string        `json:"parentHash"`
	Timestamp  uint64        `json:"timestamp"`
	GasLimit   uint64        `json:"gasLimit"`
	BaseFee    string        `json:"baseFee,omitempty"`
	Difficulty string        `json:"difficulty"`
	ExtraData  string        `json:"extraData"`
	Coinbase   string        `json:"coinbase"`
	Nonce      uint64        `json:"nonce"`
	Header     *types.Header `json:"header"`
}

func main() {
	rlpPath := flag.String("rlp", "", "Path to RLP file")
	pebblePath := flag.String("pebble", "", "Path to PebbleDB directory")
	output := flag.String("output", "", "Output file (default: stdout)")
	flag.Parse()

	if *rlpPath == "" && *pebblePath == "" {
		fmt.Fprintf(os.Stderr, "Usage: %s --rlp <file> | --pebble <dir> [--output <file>]\n", os.Args[0])
		os.Exit(1)
	}

	var genesis *GenesisExtract
	var err error

	if *rlpPath != "" {
		genesis, err = extractFromRLP(*rlpPath)
	} else {
		genesis, err = extractFromPebble(*pebblePath)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error extracting genesis: %v\n", err)
		os.Exit(1)
	}

	data, err := json.MarshalIndent(genesis, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error marshaling genesis: %v\n", err)
		os.Exit(1)
	}

	if *output != "" {
		if err := os.MkdirAll(filepath.Dir(*output), 0755); err != nil {
			fmt.Fprintf(os.Stderr, "Error creating output directory: %v\n", err)
			os.Exit(1)
		}
		if err := os.WriteFile(*output, data, 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing output file: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Genesis extracted to %s\n", *output)
	} else {
		fmt.Println(string(data))
	}
}

func extractFromRLP(path string) (*GenesisExtract, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open RLP file: %w", err)
	}
	defer file.Close()

	stream := rlp.NewStream(file, 0)

	// Read genesis block (block 0)
	block := new(types.Block)
	if err := stream.Decode(block); err != nil {
		if err == io.EOF {
			return nil, fmt.Errorf("RLP file is empty")
		}
		return nil, fmt.Errorf("failed to decode genesis block: %w", err)
	}

	// Verify it's block 0
	if block.NumberU64() != 0 {
		return nil, fmt.Errorf("first block in RLP is not genesis (block %d)", block.NumberU64())
	}

	header := block.Header()
	genesis := &GenesisExtract{
		Hash:       block.Hash().Hex(),
		StateRoot:  header.Root.Hex(),
		ParentHash: header.ParentHash.Hex(),
		Timestamp:  header.Time,
		GasLimit:   header.GasLimit,
		Difficulty: header.Difficulty.String(),
		ExtraData:  fmt.Sprintf("0x%x", header.Extra),
		Coinbase:   header.Coinbase.Hex(),
		Nonce:      header.Nonce.Uint64(),
		Header:     header,
	}

	if header.BaseFee != nil {
		genesis.BaseFee = header.BaseFee.String()
	}

	return genesis, nil
}

func extractFromPebble(path string) (*GenesisExtract, error) {
	// Check if genesis.json exists in the pebble directory
	genesisPath := filepath.Join(path, "genesis.json")
	if _, err := os.Stat(genesisPath); err == nil {
		// Read existing genesis.json
		data, err := os.ReadFile(genesisPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read genesis.json: %w", err)
		}

		var existing map[string]interface{}
		if err := json.Unmarshal(data, &existing); err != nil {
			return nil, fmt.Errorf("failed to parse genesis.json: %w", err)
		}

		// Extract relevant fields
		genesis := &GenesisExtract{
			ExtraData: "0x",
		}

		if ts, ok := existing["timestamp"].(string); ok {
			var timestamp uint64
			fmt.Sscanf(ts, "%x", &timestamp)
			genesis.Timestamp = timestamp
		}

		if gl, ok := existing["gasLimit"].(string); ok {
			var gasLimit uint64
			fmt.Sscanf(gl, "%x", &gasLimit)
			genesis.GasLimit = gasLimit
		}

		if bf, ok := existing["baseFeePerGas"].(string); ok {
			genesis.BaseFee = bf
		}

		return genesis, nil
	}

	return nil, fmt.Errorf("genesis.json not found in %s (PebbleDB direct reading not implemented)", path)
}
