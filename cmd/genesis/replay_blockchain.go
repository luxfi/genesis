package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"github.com/cockroachdb/pebble"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/spf13/cobra"
	"sort"
)

// SubnetEVM key prefixes
var (
	headerPrefix       = []byte("h") // headerPrefix + num (uint64 big endian) + hash -> header
	headerHashSuffix   = []byte("n") // headerPrefix + num (uint64 big endian) + headerHashSuffix -> hash
	headerNumberPrefix = []byte("H") // headerNumberPrefix + hash -> num (uint64 big endian)
	
	blockBodyPrefix     = []byte("b") // blockBodyPrefix + num (uint64 big endian) + hash -> block body
	blockReceiptsPrefix = []byte("r") // blockReceiptsPrefix + num (uint64 big endian) + hash -> block receipts
	
	headerTDSuffix = []byte("t") // headerPrefix + num (uint64 big endian) + hash + headerTDSuffix -> td
	hashScheme     = []byte("h") // hashScheme -> hash scheme
)

type BlockData struct {
	Number   uint64
	Hash     common.Hash
	Header   *types.Header
	Body     *types.Body
	Receipts types.Receipts
	TD       []byte
}

func getReplayBlockchainCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "replay-blockchain [source-db] [dest-node-url]",
		Short: "Replay blockchain data through consensus",
		Long:  "Read blocks from SubnetEVM PebbleDB and replay them through C-Chain consensus",
		Args:  cobra.ExactArgs(2),
		RunE:  runReplayBlockchain,
	}
	
	return cmd
}

func runReplayBlockchain(cmd *cobra.Command, args []string) error {
	sourceDB := args[0]
	nodeURL := args[1]
	
	fmt.Printf("Replaying blockchain from %s to node at %s\n", sourceDB, nodeURL)
	
	// Open source database (read-only)
	srcOpts := &pebble.Options{
		ReadOnly: true,
	}
	src, err := pebble.Open(sourceDB, srcOpts)
	if err != nil {
		return fmt.Errorf("failed to open source database: %w", err)
	}
	defer src.Close()
	
	// First, find all block numbers
	blockNumbers := make(map[uint64]bool)
	iter, err := src.NewIter(&pebble.IterOptions{})
	if err != nil {
		return fmt.Errorf("failed to create iterator: %w", err)
	}
	defer iter.Close()
	
	fmt.Println("Scanning for block numbers...")
	for iter.First(); iter.Valid(); iter.Next() {
		key := iter.Key()
		
		// Look for header number keys (H prefix)
		if len(key) > 0 && key[0] == 'H' && len(key) == 33 {
			value := iter.Value()
			if len(value) == 8 {
				blockNum := binary.BigEndian.Uint64(value)
				blockNumbers[blockNum] = true
			}
		}
	}
	
	if err := iter.Error(); err != nil {
		return fmt.Errorf("iterator error: %w", err)
	}
	
	fmt.Printf("Found %d unique block numbers\n", len(blockNumbers))
	
	// Sort block numbers
	var sortedNumbers []uint64
	for num := range blockNumbers {
		sortedNumbers = append(sortedNumbers, num)
	}
	sort.Slice(sortedNumbers, func(i, j int) bool {
		return sortedNumbers[i] < sortedNumbers[j]
	})
	
	if len(sortedNumbers) > 0 {
		fmt.Printf("Block range: %d to %d\n", sortedNumbers[0], sortedNumbers[len(sortedNumbers)-1])
	}
	
	// Process blocks in order
	processedCount := 0
	for _, blockNum := range sortedNumbers {
		block, err := readBlock(src, blockNum)
		if err != nil {
			fmt.Printf("Warning: Failed to read block %d: %v\n", blockNum, err)
			continue
		}
		
		// Here we would replay the block through consensus
		// For now, just print progress
		if processedCount%1000 == 0 {
			fmt.Printf("Processing block %d (hash: %s)\n", block.Number, block.Hash.Hex())
		}
		
		// TODO: Replay block through C-Chain consensus API
		// This would involve:
		// 1. Creating quantum consensus proof
		// 2. Submitting block through RPC
		// 3. Waiting for confirmation
		
		processedCount++
	}
	
	fmt.Printf("\n✅ Processed %d blocks\n", processedCount)
	
	return nil
}

func readBlock(db *pebble.DB, blockNum uint64) (*BlockData, error) {
	block := &BlockData{
		Number: blockNum,
	}
	
	// Encode block number
	numBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(numBytes, blockNum)
	
	// Find block hash for this number
	// First try to find a header with this number
	headerKey := append(headerPrefix, numBytes...)
	
	// We need to iterate to find the right hash
	iter, err := db.NewIter(&pebble.IterOptions{
		LowerBound: headerKey,
		UpperBound: append(headerKey, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create header iterator: %w", err)
	}
	defer iter.Close()
	
	// Find header
	for iter.First(); iter.Valid(); iter.Next() {
		key := iter.Key()
		if len(key) == 41 && bytes.HasPrefix(key, headerKey) {
			// Extract hash from key
			copy(block.Hash[:], key[9:41])
			
			// Decode header
			var header types.Header
			if err := rlp.DecodeBytes(iter.Value(), &header); err != nil {
				return nil, fmt.Errorf("failed to decode header: %w", err)
			}
			block.Header = &header
			break
		}
	}
	
	if block.Header == nil {
		return nil, fmt.Errorf("header not found for block %d", blockNum)
	}
	
	// Read body
	bodyKey := append(blockBodyPrefix, numBytes...)
	bodyKey = append(bodyKey, block.Hash[:]...)
	bodyData, closer, err := db.Get(bodyKey)
	if err != nil {
		return nil, fmt.Errorf("failed to read body: %w", err)
	}
	defer closer.Close()
	
	var body types.Body
	if err := rlp.DecodeBytes(bodyData, &body); err != nil {
		return nil, fmt.Errorf("failed to decode body: %w", err)
	}
	block.Body = &body
	
	// Read receipts (optional)
	receiptsKey := append(blockReceiptsPrefix, numBytes...)
	receiptsKey = append(receiptsKey, block.Hash[:]...)
	receiptsData, closer2, err := db.Get(receiptsKey)
	if err == nil {
		defer closer2.Close()
		var receipts types.Receipts
		if err := rlp.DecodeBytes(receiptsData, &receipts); err == nil {
			block.Receipts = receipts
		}
	}
	
	return block, nil
}