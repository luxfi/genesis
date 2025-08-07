package main

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/cockroachdb/pebble"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
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
		fmt.Println("Usage: consensus-replay <command> [args]")
		fmt.Println("Commands:")
		fmt.Println("  init                     - Initialize C-chain with genesis")
		fmt.Println("  replay <start> <end>     - Replay blocks from start to end")
		fmt.Println("  replay-all               - Replay all available blocks")
		fmt.Println("  status                   - Check current sync status")
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "init":
		initializeChain()

	case "replay":
		if len(os.Args) != 4 {
			log.Fatal("Usage: consensus-replay replay <start> <end>")
		}
		var start, end uint64
		fmt.Sscanf(os.Args[2], "%d", &start)
		fmt.Sscanf(os.Args[3], "%d", &end)
		replayBlocks(start, end)

	case "replay-all":
		replayAllBlocks()

	case "status":
		checkStatus()

	default:
		log.Fatal("Unknown command:", command)
	}
}

func initializeChain() {
	fmt.Println("Initializing C-chain with genesis state...")
	
	// Check if chain is already initialized
	height, err := getCurrentHeight()
	if err == nil && height > 0 {
		fmt.Printf("Chain already initialized at height %d\n", height)
		return
	}
	
	// Load genesis from replay.json
	replayData, err := os.ReadFile("replay.json")
	if err != nil {
		log.Fatal("Failed to read replay.json:", err)
	}
	
	var replay struct {
		Genesis *core.Genesis `json:"genesis"`
	}
	if err := json.Unmarshal(replayData, &replay); err != nil {
		log.Fatal("Failed to parse replay.json:", err)
	}
	
	fmt.Println("Genesis configuration loaded")
	fmt.Printf("Chain ID: %d\n", replay.Genesis.Config.ChainID)
	fmt.Printf("Timestamp: %d\n", replay.Genesis.Timestamp)
	fmt.Printf("Gas Limit: %d\n", replay.Genesis.GasLimit)
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

func replayBlocks(start, end uint64) {
	// Open the database - use absolute path
	dbPath := "/home/z/work/lux/genesis/state/chaindata/lux-mainnet-96369/db"
	if envPath := os.Getenv("REPLAY_DB_PATH"); envPath != "" {
		dbPath = envPath
	}
	
	fmt.Printf("[%s] Opening database: %s\n", time.Now().Format("15:04:05"), dbPath)
	
	db, err := pebble.Open(dbPath, &pebble.Options{ReadOnly: true})
	if err != nil {
		log.Fatal("Failed to open database:", err)
	}
	defer db.Close()

	fmt.Printf("\nReplaying blocks %d to %d\n", start, end)
	fmt.Println("=" + string(make([]byte, 50)))

	successCount := 0
	errorCount := 0
	startTime := time.Now()

	for blockNum := start; blockNum <= end; blockNum++ {
		// Get canonical hash
		canonicalKey := make([]byte, 9)
		canonicalKey[0] = 'H'
		binary.BigEndian.PutUint64(canonicalKey[1:], blockNum)

		hashBytes, closer, err := db.Get(canonicalKey)
		if err != nil {
			fmt.Printf("Block %d not found\n", blockNum)
			break
		}
		closer.Close()

		var blockHash common.Hash
		copy(blockHash[:], hashBytes)

		// Get and submit the block
		if err := replayBlock(db, blockNum, blockHash); err != nil {
			fmt.Printf("❌ Block %d failed: %v\n", blockNum, err)
			errorCount++
			
			// Check if block already exists
			currentHeight, _ := getCurrentHeight()
			if currentHeight >= blockNum {
				fmt.Printf("✓ Block %d already in chain\n", blockNum)
				successCount++
				continue
			}
		} else {
			successCount++
			if blockNum%100 == 0 || blockNum == end {
				elapsed := time.Since(startTime)
				blocksPerSec := float64(blockNum-start+1) / elapsed.Seconds()
				fmt.Printf("✓ Block %d | Speed: %.2f blocks/sec | Progress: %d/%d\n", 
					blockNum, blocksPerSec, blockNum-start+1, end-start+1)
			}
		}

		// Rate limiting
		if blockNum%10 == 0 {
			time.Sleep(10 * time.Millisecond)
		}
	}

	fmt.Println("=" + string(make([]byte, 50)))
	fmt.Printf("\nReplay Summary:\n")
	fmt.Printf("  Successful: %d\n", successCount)
	fmt.Printf("  Failed: %d\n", errorCount)
	fmt.Printf("  Total time: %v\n", time.Since(startTime))
	
	// Check final height
	if finalHeight, err := getCurrentHeight(); err == nil {
		fmt.Printf("  Final height: %d\n", finalHeight)
	}
}

func replayBlock(db *pebble.DB, blockNum uint64, blockHash common.Hash) error {
	// Get header
	headerKey := append([]byte("h"), make([]byte, 8)...)
	binary.BigEndian.PutUint64(headerKey[1:], blockNum)
	headerKey = append(headerKey, blockHash[:]...)

	headerData, closer, err := db.Get(headerKey)
	if err != nil {
		// Try without number prefix
		headerKey = append([]byte("h"), blockHash[:]...)
		headerData, closer, err = db.Get(headerKey)
		if err != nil {
			return fmt.Errorf("header not found")
		}
	}
	closer.Close()

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
			// Empty body is valid - create block with just header
			return submitBlockData(headerData, nil)
		}
	}
	closer.Close()

	return submitBlockData(headerData, bodyData)
}

func submitBlockData(headerData, bodyData []byte) error {
	// First decode to validate
	var header types.Header
	if err := rlp.DecodeBytes(headerData, &header); err != nil {
		return fmt.Errorf("invalid header: %v", err)
	}

	var block *types.Block
	if bodyData != nil {
		var body types.Body
		if err := rlp.DecodeBytes(bodyData, &body); err != nil {
			return fmt.Errorf("invalid body: %v", err)
		}
		block = types.NewBlockWithHeader(&header).WithBody(body.Transactions, body.Uncles)
	} else {
		block = types.NewBlockWithHeader(&header)
	}

	// Encode full block
	blockRLP, err := rlp.EncodeToBytes(block)
	if err != nil {
		return fmt.Errorf("failed to encode block: %v", err)
	}

	// Try eth_sendRawBlock first
	req := RPCRequest{
		JSONRPC: "2.0",
		Method:  "eth_sendRawBlock",
		Params:  []interface{}{"0x" + hex.EncodeToString(blockRLP)},
		ID:      1,
	}

	if err := sendRPCRequest(req); err == nil {
		return nil
	}

	// Try debug_insertBlock
	req.Method = "debug_insertBlock"
	if err := sendRPCRequest(req); err == nil {
		return nil
	}

	// Finally try admin_importChain
	return importViaFile(blockRLP)
}

func sendRPCRequest(req RPCRequest) error {
	data, err := json.Marshal(req)
	if err != nil {
		return err
	}

	resp, err := http.Post("http://localhost:9630/ext/bc/C/rpc", "application/json", bytes.NewReader(data))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var rpcResp RPCResponse
	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		return err
	}

	if rpcResp.Error != nil {
		return fmt.Errorf("RPC error: %s", rpcResp.Error.Message)
	}

	return nil
}

func importViaFile(blockRLP []byte) error {
	tmpFile, err := os.CreateTemp("", "block-*.rlp")
	if err != nil {
		return err
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	if _, err := tmpFile.Write(blockRLP); err != nil {
		return err
	}
	tmpFile.Close()

	req := RPCRequest{
		JSONRPC: "2.0",
		Method:  "admin_importChain",
		Params:  []interface{}{tmpFile.Name()},
		ID:      1,
	}

	data, err := json.Marshal(req)
	if err != nil {
		return err
	}

	// Try admin endpoint
	resp, err := http.Post("http://localhost:9630/ext/bc/C/admin", "application/json", bytes.NewReader(data))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var rpcResp RPCResponse
	if err := json.Unmarshal(body, &rpcResp); err != nil {
		return fmt.Errorf("failed to parse response: %s", string(body))
	}

	if rpcResp.Error != nil {
		return fmt.Errorf("import error: %s", rpcResp.Error.Message)
	}

	return nil
}

func replayAllBlocks() {
	// Open database to find total blocks - use absolute path
	dbPath := "/home/z/work/lux/genesis/state/chaindata/lux-mainnet-96369/db"
	if envPath := os.Getenv("REPLAY_DB_PATH"); envPath != "" {
		dbPath = envPath
	}
	
	fmt.Printf("[%s] Opening database for full replay: %s\n", time.Now().Format("15:04:05"), dbPath)
	
	db, err := pebble.Open(dbPath, &pebble.Options{ReadOnly: true})
	if err != nil {
		log.Fatal("Failed to open database:", err)
	}
	defer db.Close()

	// Binary search for highest block
	fmt.Println("Finding highest block number...")
	low := uint64(0)
	high := uint64(2000000)
	highest := uint64(0)
	
	for low <= high {
		mid := (low + high) / 2
		
		canonicalKey := make([]byte, 9)
		canonicalKey[0] = 'H'
		binary.BigEndian.PutUint64(canonicalKey[1:], mid)
		
		_, closer, err := db.Get(canonicalKey)
		if err == nil {
			closer.Close()
			highest = mid
			low = mid + 1
		} else {
			high = mid - 1
		}
	}

	fmt.Printf("Found %d blocks in database\n", highest+1)
	
	// Start replay from block 1
	replayBlocks(1, highest)
}

func checkStatus() {
	// Get current height
	height, err := getCurrentHeight()
	if err != nil {
		log.Fatal("Failed to get current height:", err)
	}

	fmt.Printf("Current blockchain height: %d\n", height)

	// Get block details
	if height > 0 {
		req := RPCRequest{
			JSONRPC: "2.0",
			Method:  "eth_getBlockByNumber",
			Params:  []interface{}{fmt.Sprintf("0x%x", height), false},
			ID:      1,
		}

		data, _ := json.Marshal(req)
		resp, err := http.Post("http://localhost:9630/ext/bc/C/rpc", "application/json", bytes.NewReader(data))
		if err == nil {
			defer resp.Body.Close()
			var rpcResp RPCResponse
			if json.NewDecoder(resp.Body).Decode(&rpcResp) == nil && rpcResp.Error == nil {
				var block map[string]interface{}
				if json.Unmarshal(rpcResp.Result, &block) == nil {
					fmt.Printf("Latest block hash: %v\n", block["hash"])
					fmt.Printf("Latest block timestamp: %v\n", block["timestamp"])
				}
			}
		}
	}

	// Check database for total available blocks
	dbPath := "/home/z/work/lux/genesis/state/chaindata/lux-mainnet-96369/db"
	if envPath := os.Getenv("REPLAY_DB_PATH"); envPath != "" {
		dbPath = envPath
	}
	if db, err := pebble.Open(dbPath, &pebble.Options{ReadOnly: true}); err == nil {
		defer db.Close()
		
		// Count blocks
		count := uint64(0)
		iter := db.NewIter(&pebble.IterOptions{})
		defer iter.Close()
		
		for iter.First(); iter.Valid(); iter.Next() {
			key := iter.Key()
			if len(key) == 9 && key[0] == 'H' {
				count++
			}
		}
		
		fmt.Printf("\nDatabase contains %d blocks ready for replay\n", count)
		if count > height {
			fmt.Printf("Blocks remaining to replay: %d\n", count-height)
		}
	}
}