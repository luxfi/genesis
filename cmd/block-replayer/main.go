package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/cockroachdb/pebble"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"
)

// RPCRequest represents a JSON-RPC request
type RPCRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params"`
	ID      int         `json:"id"`
}

// RPCResponse represents a JSON-RPC response
type RPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	Result  json.RawMessage `json:"result"`
	Error   *RPCError       `json:"error"`
	ID      int             `json:"id"`
}

// RPCError represents a JSON-RPC error
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: block-replayer <command> [args]")
		fmt.Println("Commands:")
		fmt.Println("  status                   - Check current blockchain height")
		fmt.Println("  replay <start> <count>   - Replay blocks from start height")
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "status":
		checkStatus()

	case "replay":
		if len(os.Args) != 4 {
			log.Fatal("Usage: block-replayer replay <start> <count>")
		}
		var start, count uint64
		fmt.Sscanf(os.Args[2], "%d", &start)
		fmt.Sscanf(os.Args[3], "%d", &count)
		replayBlocks(start, count)

	default:
		log.Fatal("Unknown command:", command)
	}
}

func checkStatus() {
	// Check current block height
	height, err := getCurrentHeight()
	if err != nil {
		log.Fatal("Failed to get current height:", err)
	}

	fmt.Printf("Current blockchain height: %d\n", height)
}

func getCurrentHeight() (uint64, error) {
	req := RPCRequest{
		JSONRPC: "2.0",
		Method:  "eth_blockNumber",
		Params:  []interface{}{},
		ID:      1,
	}

	data, err := json.Marshal(req)
	if err != nil {
		return 0, err
	}

	resp, err := http.Post("http://localhost:9630/ext/bc/C/rpc", "application/json", bytes.NewReader(data))
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	var rpcResp RPCResponse
	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		return 0, err
	}

	if rpcResp.Error != nil {
		return 0, fmt.Errorf("RPC error: %s", rpcResp.Error.Message)
	}

	var heightHex string
	if err := json.Unmarshal(rpcResp.Result, &heightHex); err != nil {
		return 0, err
	}

	var height uint64
	fmt.Sscanf(heightHex, "0x%x", &height)
	return height, nil
}

func replayBlocks(start, count uint64) {
	// Open the database
	dbPath := filepath.Join("state", "chaindata", "lux-mainnet-96369", "db")
	fmt.Printf("Opening database: %s\n", dbPath)
	
	db, err := pebble.Open(dbPath, &pebble.Options{ReadOnly: true})
	if err != nil {
		log.Fatal("Failed to open database:", err)
	}
	defer db.Close()

	fmt.Printf("Replaying %d blocks starting from block %d\n", count, start)

	successCount := 0
	errorCount := 0

	for blockNum := start; blockNum < start+count; blockNum++ {
		// Get canonical hash
		canonicalKey := make([]byte, 9)
		canonicalKey[0] = 'H'
		binary.BigEndian.PutUint64(canonicalKey[1:], blockNum)

		hashBytes, closer, err := db.Get(canonicalKey)
		if err != nil {
			fmt.Printf("Block %d not found, stopping replay\n", blockNum)
			break
		}
		closer.Close()

		var blockHash common.Hash
		copy(blockHash[:], hashBytes)

		// Get the block data
		block, err := getBlock(db, blockNum, blockHash)
		if err != nil {
			fmt.Printf("Failed to get block %d: %v\n", blockNum, err)
			errorCount++
			continue
		}

		// Submit the block through consensus
		if err := submitBlock(block); err != nil {
			fmt.Printf("Failed to submit block %d: %v\n", blockNum, err)
			errorCount++
			
			// Check if we should retry
			currentHeight, _ := getCurrentHeight()
			if currentHeight >= blockNum {
				fmt.Printf("Block %d already in chain, continuing...\n", blockNum)
				successCount++
				continue
			}
		} else {
			successCount++
			if blockNum%100 == 0 {
				fmt.Printf("Replayed block %d (hash: %s)\n", blockNum, blockHash.Hex())
			}
		}

		// Small delay to not overwhelm the node
		if blockNum%10 == 0 {
			time.Sleep(10 * time.Millisecond)
		}
	}

	fmt.Printf("\nReplay complete: %d successful, %d errors\n", successCount, errorCount)
	
	// Check final height
	finalHeight, _ := getCurrentHeight()
	fmt.Printf("Final blockchain height: %d\n", finalHeight)
}

func getBlock(db *pebble.DB, blockNum uint64, blockHash common.Hash) (*types.Block, error) {
	// Get header
	headerKey := append([]byte("h"), make([]byte, 8)...)
	binary.BigEndian.PutUint64(headerKey[1:], blockNum)
	headerKey = append(headerKey, blockHash[:]...)

	headerData, closer, err := db.Get(headerKey)
	if err != nil {
		// Try without number prefix (some formats store just hash)
		headerKey = append([]byte("h"), blockHash[:]...)
		headerData, closer, err = db.Get(headerKey)
		if err != nil {
			return nil, fmt.Errorf("header not found")
		}
	}
	closer.Close()

	var header types.Header
	if err := rlp.DecodeBytes(headerData, &header); err != nil {
		return nil, fmt.Errorf("failed to decode header: %v", err)
	}

	// Get body
	bodyKey := append([]byte("b"), make([]byte, 8)...)
	binary.BigEndian.PutUint64(bodyKey[1:], blockNum)
	bodyKey = append(bodyKey, blockHash[:]...)

	bodyData, closer, err := db.Get(bodyKey)
	if err != nil {
		// Try without number prefix
		bodyKey = append([]byte("b"), blockHash[:]...)
		bodyData, closer, err = db.Get(bodyKey)
		if err != nil {
			// Empty body is valid
			return types.NewBlockWithHeader(&header), nil
		}
	}
	closer.Close()

	var body types.Body
	if err := rlp.DecodeBytes(bodyData, &body); err != nil {
		return nil, fmt.Errorf("failed to decode body: %v", err)
	}

	return types.NewBlockWithHeader(&header).WithBody(body.Transactions, body.Uncles), nil
}

func submitBlock(block *types.Block) error {
	// Submit block through the eth_sendRawBlock RPC method
	// This allows us to inject blocks into the consensus engine
	
	// First, encode the block to RLP
	blockRLP, err := rlp.EncodeToBytes(block)
	if err != nil {
		return fmt.Errorf("failed to encode block: %v", err)
	}
	
	// Create the RPC request
	req := RPCRequest{
		JSONRPC: "2.0",
		Method:  "debug_insertBlock",
		Params:  []interface{}{"0x" + common.Bytes2Hex(blockRLP)},
		ID:      1,
	}
	
	data, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %v", err)
	}
	
	// Send the request
	resp, err := http.Post("http://localhost:9630/ext/bc/C/rpc", "application/json", bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()
	
	var rpcResp RPCResponse
	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		return fmt.Errorf("failed to decode response: %v", err)
	}
	
	if rpcResp.Error != nil {
		// If method not found, try alternative approaches
		if rpcResp.Error.Code == -32601 {
			// Try admin_importChain with single block
			return submitBlockViaImport(block)
		}
		return fmt.Errorf("RPC error: %s", rpcResp.Error.Message)
	}
	
	return nil
}

func submitBlockViaImport(block *types.Block) error {
	// Alternative: Use admin_importChain with a temporary file
	tmpFile, err := os.CreateTemp("", "block-*.rlp")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()
	
	// Write block to file
	blockRLP, err := rlp.EncodeToBytes(block)
	if err != nil {
		return fmt.Errorf("failed to encode block: %v", err)
	}
	
	if _, err := tmpFile.Write(blockRLP); err != nil {
		return fmt.Errorf("failed to write block: %v", err)
	}
	tmpFile.Close()
	
	// Import via admin_importChain
	req := RPCRequest{
		JSONRPC: "2.0",
		Method:  "admin_importChain",
		Params:  []interface{}{tmpFile.Name()},
		ID:      1,
	}
	
	data, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %v", err)
	}
	
	resp, err := http.Post("http://localhost:9630/ext/bc/C/admin", "application/json", bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to send import request: %v", err)
	}
	defer resp.Body.Close()
	
	var rpcResp RPCResponse
	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		return fmt.Errorf("failed to decode import response: %v", err)
	}
	
	if rpcResp.Error != nil {
		return fmt.Errorf("import error: %s", rpcResp.Error.Message)
	}
	
	return nil
}