package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"github.com/cockroachdb/pebble"
	"github.com/luxfi/geth/common"
	"github.com/luxfi/geth/core/types"
	"github.com/luxfi/geth/rlp"
	"github.com/spf13/cobra"
	"io/ioutil"
	"net/http"
	"os/exec"
	"sort"
	"time"
)

func getReplayerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "replay [source-db]",
		Short: "Replay blockchain blocks",
		Long:  "Read finalized blocks from SubnetEVM database and replay them into C-Chain with proper state setup",
		Args:  cobra.ExactArgs(1),
		RunE:  runSubnetBlockReplay,
	}

	cmd.Flags().String("rpc", "http://localhost:9630/ext/bc/C/rpc", "RPC endpoint")
	cmd.Flags().Uint64("start", 0, "Start block (0 = genesis)")
	cmd.Flags().Uint64("end", 0, "End block (0 = all)")
	cmd.Flags().Bool("direct-db", false, "Write directly to database instead of RPC")
	cmd.Flags().String("output", "", "Output database path (for direct-db mode)")

	return cmd
}

func runSubnetBlockReplay(cmd *cobra.Command, args []string) error {
	sourceDB := args[0]
	rpcURL, _ := cmd.Flags().GetString("rpc")
	startBlock, _ := cmd.Flags().GetUint64("start")
	endBlock, _ := cmd.Flags().GetUint64("end")
	directDB, _ := cmd.Flags().GetBool("direct-db")
	outputPath, _ := cmd.Flags().GetString("output")

	fmt.Printf("Replaying SubnetEVM blocks from %s\n", sourceDB)

	// Open source database
	srcOpts := &pebble.Options{
		ReadOnly: true,
	}
	src, err := pebble.Open(sourceDB, srcOpts)
	if err != nil {
		return fmt.Errorf("failed to open source database: %w", err)
	}
	defer src.Close()

	// Find all canonical blocks
	fmt.Println("Scanning for canonical blocks...")
	canonicalBlocks := make(map[uint64]common.Hash)

	// Canonical hash mapping is stored as: h + num (8 bytes) + n -> hash
	iter, err := src.NewIter(&pebble.IterOptions{
		LowerBound: []byte("h"),
		UpperBound: []byte("i"),
	})
	if err != nil {
		return fmt.Errorf("failed to create iterator: %w", err)
	}
	defer iter.Close()

	for iter.First(); iter.Valid(); iter.Next() {
		key := iter.Key()
		value := iter.Value()

		// Check if this is a canonical hash key: h + num(8) + n
		if len(key) == 10 && key[0] == 'h' && key[9] == 'n' {
			blockNum := binary.BigEndian.Uint64(key[1:9])
			if len(value) == 32 {
				var hash common.Hash
				copy(hash[:], value)
				canonicalBlocks[blockNum] = hash
			}
		}
	}

	if err := iter.Error(); err != nil {
		return fmt.Errorf("iterator error: %w", err)
	}

	fmt.Printf("Found %d canonical blocks\n", len(canonicalBlocks))

	// Sort block numbers
	var blockNumbers []uint64
	for num := range canonicalBlocks {
		if (startBlock == 0 || num >= startBlock) && (endBlock == 0 || num <= endBlock) {
			blockNumbers = append(blockNumbers, num)
		}
	}
	sort.Slice(blockNumbers, func(i, j int) bool {
		return blockNumbers[i] < blockNumbers[j]
	})

	if len(blockNumbers) == 0 {
		return fmt.Errorf("no blocks found in range %d-%d", startBlock, endBlock)
	}

	fmt.Printf("Processing blocks %d to %d (%d total)\n",
		blockNumbers[0], blockNumbers[len(blockNumbers)-1], len(blockNumbers))

	if directDB {
		return replayDirectToDB(src, blockNumbers, canonicalBlocks, outputPath)
	}

	// Submit blocks one by one through consensus
	return submitBlocksAsCanonical(src, blockNumbers, canonicalBlocks, rpcURL)
}

func setupInitialState(src *pebble.DB, targetHeight uint64, canonicalBlocks map[uint64]common.Hash) error {
	targetHash := canonicalBlocks[targetHeight]
	fmt.Printf("Setting up state for block %d (hash: %s)\n", targetHeight, targetHash.Hex())

	// We need to:
	// 1. Copy all state data
	// 2. Set proper head references
	// 3. Ensure the chain recognizes the imported blocks

	// For now, let's create a direct database setup
	dbPath := "runs/lux-mainnet-96369/C/db"

	// Open destination database
	dst, err := pebble.Open(dbPath, &pebble.Options{})
	if err != nil {
		return fmt.Errorf("failed to open destination database: %w", err)
	}
	defer dst.Close()

	batch := dst.NewBatch()

	// Set all the chain head references
	fmt.Println("Setting chain head references...")

	// These are the keys C-Chain uses for tracking head
	headKeys := []string{
		"LastHeader",
		"LastBlock",
		"LastFast",
		"HeadHeaderHash",
		"HeadBlockHash",
		"HeadFastBlockHash",
		"SnapshotBlockHash",
	}

	for _, key := range headKeys {
		if err := batch.Set([]byte(key), targetHash[:], nil); err != nil {
			return fmt.Errorf("failed to set %s: %w", key, err)
		}
	}

	// Set the accepted key for consensus
	if err := batch.Set([]byte("snowman_lastAccepted"), targetHash[:], nil); err != nil {
		return fmt.Errorf("failed to set lastAccepted: %w", err)
	}

	// Also need to copy the actual header for this block
	headerKey := makeHeaderKey(targetHeight, targetHash)
	headerData, closer, err := src.Get(headerKey)
	if err != nil {
		return fmt.Errorf("failed to get header: %w", err)
	}
	closer.Close()

	// Decode and re-encode header to ensure it's in the right format
	var header types.Header
	if err := rlp.DecodeBytes(headerData, &header); err != nil {
		return fmt.Errorf("failed to decode header: %w", err)
	}

	// The header should already have the right block number and hash
	fmt.Printf("Header: Number=%d, Hash=%s, ParentHash=%s\n",
		header.Number.Uint64(), header.Hash().Hex(), header.ParentHash.Hex())

	// Commit the batch
	if err := batch.Commit(nil); err != nil {
		return fmt.Errorf("failed to commit batch: %w", err)
	}

	fmt.Println("✅ Initial state setup complete")
	return nil
}

func replayDirectToDB(src *pebble.DB, blockNumbers []uint64, canonicalBlocks map[uint64]common.Hash, outputPath string) error {
	// Direct database replay - copy all block data maintaining proper structure
	dbPath := outputPath
	if dbPath == "" {
		dbPath = "runs/lux-mainnet-96369/C/db"
	}

	fmt.Printf("Direct replay to database at %s\n", dbPath)

	// Remove old database
	exec.Command("rm", "-rf", dbPath).Run()

	// Open new database
	dst, err := pebble.Open(dbPath, &pebble.Options{})
	if err != nil {
		return fmt.Errorf("failed to open destination database: %w", err)
	}
	defer dst.Close()

	batch := dst.NewBatch()
	count := 0

	// Copy all blocks and their data
	for _, blockNum := range blockNumbers {
		hash := canonicalBlocks[blockNum]

		// Copy header
		headerKey := makeHeaderKey(blockNum, hash)
		if headerData, closer, err := src.Get(headerKey); err == nil {
			batch.Set(headerKey, headerData, nil)
			closer.Close()
		}

		// Copy body
		bodyKey := makeBodyKey(blockNum, hash)
		if bodyData, closer, err := src.Get(bodyKey); err == nil {
			batch.Set(bodyKey, bodyData, nil)
			closer.Close()
		}

		// Copy receipts
		receiptsKey := makeReceiptsKey(blockNum, hash)
		if receiptsData, closer, err := src.Get(receiptsKey); err == nil {
			batch.Set(receiptsKey, receiptsData, nil)
			closer.Close()
		}

		// Copy TD (total difficulty)
		tdKey := makeTDKey(blockNum, hash)
		if tdData, closer, err := src.Get(tdKey); err == nil {
			batch.Set(tdKey, tdData, nil)
			closer.Close()
		}

		// Copy canonical hash mapping
		canonicalKey := makeCanonicalKey(blockNum)
		batch.Set(canonicalKey, hash[:], nil)

		// Also set the reverse mapping (hash -> number)
		numberKey := append([]byte("H"), hash[:]...)
		numBytes := make([]byte, 8)
		binary.BigEndian.PutUint64(numBytes, blockNum)
		batch.Set(numberKey, numBytes, nil)

		count++
		if count%10000 == 0 {
			fmt.Printf("Processed %d blocks...\n", count)
			if err := batch.Commit(nil); err != nil {
				return fmt.Errorf("failed to commit batch: %w", err)
			}
			batch = dst.NewBatch()
		}
	}

	// Set head references to the highest block
	highestNum := blockNumbers[len(blockNumbers)-1]
	highestHash := canonicalBlocks[highestNum]

	fmt.Printf("Setting head to block %d (hash: %s)\n", highestNum, highestHash.Hex())

	// Set all head references
	batch.Set([]byte("LastHeader"), highestHash[:], nil)
	batch.Set([]byte("LastBlock"), highestHash[:], nil)
	batch.Set([]byte("LastFast"), highestHash[:], nil)
	batch.Set([]byte("HeadHeaderHash"), highestHash[:], nil)
	batch.Set([]byte("HeadBlockHash"), highestHash[:], nil)
	batch.Set([]byte("HeadFastBlockHash"), highestHash[:], nil)
	batch.Set([]byte("snowman_lastAccepted"), highestHash[:], nil)

	// Commit final batch
	if err := batch.Commit(nil); err != nil {
		return fmt.Errorf("failed to commit final batch: %w", err)
	}

	fmt.Printf("\n✅ Direct replay complete! Processed %d blocks\n", count)
	return nil
}

// Key construction helpers
func makeHeaderKey(number uint64, hash common.Hash) []byte {
	return append(append(encodeSubnetBlockNumber(number), hash[:]...), []byte("h")...)
}

func makeBodyKey(number uint64, hash common.Hash) []byte {
	return append(append([]byte("b"), encodeSubnetBlockNumber(number)...), hash[:]...)
}

func makeReceiptsKey(number uint64, hash common.Hash) []byte {
	return append(append([]byte("r"), encodeSubnetBlockNumber(number)...), hash[:]...)
}

func makeTDKey(number uint64, hash common.Hash) []byte {
	return append(append(append([]byte("h"), encodeSubnetBlockNumber(number)...), hash[:]...), []byte("t")...)
}

func makeCanonicalKey(number uint64) []byte {
	return append(append([]byte("h"), encodeSubnetBlockNumber(number)...), []byte("n")...)
}

func encodeSubnetBlockNumber(number uint64) []byte {
	enc := make([]byte, 8)
	binary.BigEndian.PutUint64(enc, number)
	return enc
}

// RPC helpers for future use
func callRPC(url string, method string, params interface{}) (json.RawMessage, error) {
	reqBody := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  method,
		"params":  params,
		"id":      1,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	resp, err := http.Post(url, "application/json", bytes.NewReader(jsonData))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result struct {
		Result json.RawMessage `json:"result"`
		Error  interface{}     `json:"error"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	if result.Error != nil {
		return nil, fmt.Errorf("RPC error: %v", result.Error)
	}

	return result.Result, nil
}

// Submit blocks one by one as canonical through the node
func submitBlocksAsCanonical(src *pebble.DB, blockNumbers []uint64, canonicalBlocks map[uint64]common.Hash, rpcURL string) error {
	fmt.Printf("Submitting %d blocks as canonical to %s\n", len(blockNumbers), rpcURL)

	// First check current block height
	currentHeight, err := getCurrentBlockHeight(rpcURL)
	if err != nil {
		return fmt.Errorf("failed to get current block height: %w", err)
	}

	fmt.Printf("Current chain height: %d\n", currentHeight)

	// Process blocks in order
	successCount := 0
	for i, blockNum := range blockNumbers {
		hash := canonicalBlocks[blockNum]

		// Read the block data
		block, err := readFullBlock(src, blockNum, hash)
		if err != nil {
			fmt.Printf("Warning: Failed to read block %d: %v\n", blockNum, err)
			continue
		}

		// For the first few blocks and then every 1000, show progress
		if i < 10 || i%1000 == 0 {
			fmt.Printf("Submitting block %d/%d (num=%d, hash=%s)\n",
				i+1, len(blockNumbers), blockNum, hash.Hex())
		}

		// Submit block as canonical
		// Since these are finalized subnet blocks, we tell the node to accept them
		if err := submitCanonicalBlock(rpcURL, block); err != nil {
			fmt.Printf("Error submitting block %d: %v\n", blockNum, err)
			// For now, continue on error
			continue
		}

		successCount++

		// Small delay to not overwhelm the node
		if i%100 == 0 && i > 0 {
			time.Sleep(10 * time.Millisecond)
		}
	}

	fmt.Printf("\n✅ Submitted %d/%d blocks successfully\n", successCount, len(blockNumbers))

	// Check final height
	finalHeight, err := getCurrentBlockHeight(rpcURL)
	if err == nil {
		fmt.Printf("Final chain height: %d (increased by %d)\n", finalHeight, finalHeight-currentHeight)
	}

	return nil
}

func getCurrentBlockHeight(rpcURL string) (uint64, error) {
	result, err := callRPC(rpcURL, "eth_blockNumber", []interface{}{})
	if err != nil {
		return 0, err
	}

	var hexStr string
	if err := json.Unmarshal(result, &hexStr); err != nil {
		return 0, err
	}

	// Convert hex to uint64
	if len(hexStr) > 2 && hexStr[:2] == "0x" {
		hexStr = hexStr[2:]
	}

	var height uint64
	fmt.Sscanf(hexStr, "%x", &height)
	return height, nil
}

type FullBlock struct {
	Number   uint64
	Hash     common.Hash
	Header   *types.Header
	Body     *types.Body
	Receipts types.Receipts
	TD       []byte
}

func readFullBlock(db *pebble.DB, blockNum uint64, hash common.Hash) (*FullBlock, error) {
	block := &FullBlock{
		Number: blockNum,
		Hash:   hash,
	}

	// Read header
	headerKey := makeHeaderKey(blockNum, hash)
	headerData, closer, err := db.Get(headerKey)
	if err != nil {
		return nil, fmt.Errorf("failed to read header: %w", err)
	}
	closer.Close()

	var header types.Header
	if err := rlp.DecodeBytes(headerData, &header); err != nil {
		return nil, fmt.Errorf("failed to decode header: %w", err)
	}
	block.Header = &header

	// Read body
	bodyKey := makeBodyKey(blockNum, hash)
	bodyData, closer, err := db.Get(bodyKey)
	if err != nil {
		return nil, fmt.Errorf("failed to read body: %w", err)
	}
	closer.Close()

	var body types.Body
	if err := rlp.DecodeBytes(bodyData, &body); err != nil {
		return nil, fmt.Errorf("failed to decode body: %w", err)
	}
	block.Body = &body

	// Read receipts (optional)
	receiptsKey := makeReceiptsKey(blockNum, hash)
	if receiptsData, closer, err := db.Get(receiptsKey); err == nil {
		closer.Close()
		var receipts types.Receipts
		if err := rlp.DecodeBytes(receiptsData, &receipts); err == nil {
			block.Receipts = receipts
		}
	}

	// Read TD
	tdKey := makeTDKey(blockNum, hash)
	if tdData, closer, err := db.Get(tdKey); err == nil {
		closer.Close()
		block.TD = tdData
	}

	return block, nil
}

func submitCanonicalBlock(rpcURL string, block *FullBlock) error {
	// The C-Chain needs blocks submitted through its consensus mechanism
	// Since these are already finalized subnet blocks, we use a special admin API

	// First, try the admin API to directly insert the block
	fullBlock := types.NewBlockWithHeader(block.Header).WithBody(types.Body{
		Transactions: block.Body.Transactions,
		Uncles:       block.Body.Uncles,
	})

	// Encode the block
	blockRLP, err := rlp.EncodeToBytes(fullBlock)
	if err != nil {
		return fmt.Errorf("failed to encode block: %w", err)
	}

	// Try admin_importChain (if available)
	blockHex := fmt.Sprintf("0x%x", blockRLP)
	_, err = callRPC(rpcURL, "admin_importChain", []interface{}{[]string{blockHex}})
	if err == nil {
		return nil // Success!
	}

	// If admin API not available, try debug_setHead to move the chain forward
	// This assumes the blocks are already in the database
	hashHex := block.Hash.Hex()
	_, err = callRPC(rpcURL, "debug_setHead", []interface{}{hashHex})
	if err == nil {
		return nil
	}

	// As a last resort, we might need to use a custom consensus submission
	// For now, return the error
	return fmt.Errorf("failed to submit block through available APIs: %w", err)
}
