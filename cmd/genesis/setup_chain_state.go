package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"github.com/cockroachdb/pebble"
	"github.com/ethereum/go-ethereum/common"
	"github.com/spf13/cobra"
)

// Key prefixes for chain state
var (
	// Chain index prefixes
	lastHeaderKey       = []byte("LastHeader")
	lastBlockKey        = []byte("LastBlock")
	lastFastBlockKey    = []byte("LastFast")
	
	// Head tracking
	headHeaderKey  = []byte("LastHeader")
	headBlockKey   = []byte("LastBlock")
	headFastKey    = []byte("LastFast")
	
	// Chain state keys
	acceptedKey    = []byte("snowman_lastAccepted")
	heightKey      = []byte("height")
)

func getSetupChainStateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "setup-chain-state [db-path]",
		Short: "Setup C-Chain state with imported blockchain data",
		Long:  "Properly configure chain head and state references for imported blockchain",
		Args:  cobra.ExactArgs(1),
		RunE:  runSetupChainState,
	}
	
	cmd.Flags().Uint64("target-height", 0, "Target block height (0 = find highest)")
	
	return cmd
}

func runSetupChainState(cmd *cobra.Command, args []string) error {
	dbPath := args[0]
	targetHeight, _ := cmd.Flags().GetUint64("target-height")
	
	fmt.Printf("Setting up chain state for database at %s\n", dbPath)
	
	// Open database
	db, err := pebble.Open(dbPath, &pebble.Options{})
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()
	
	// Find the highest block if target not specified
	if targetHeight == 0 {
		fmt.Println("Finding highest block...")
		highestBlock, highestHash, err := findHighestBlock(db)
		if err != nil {
			return fmt.Errorf("failed to find highest block: %w", err)
		}
		targetHeight = highestBlock
		fmt.Printf("Found highest block: %d (hash: %s)\n", highestBlock, highestHash.Hex())
	}
	
	// Find the block hash for target height
	blockHash, err := getBlockHash(db, targetHeight)
	if err != nil {
		return fmt.Errorf("failed to get block hash for height %d: %w", targetHeight, err)
	}
	
	fmt.Printf("Target block: %d (hash: %s)\n", targetHeight, blockHash.Hex())
	
	// Create a batch for atomic updates
	batch := db.NewBatch()
	
	// Set head block references
	fmt.Println("Setting head block references...")
	
	// Set LastHeader
	if err := batch.Set(lastHeaderKey, blockHash[:], nil); err != nil {
		return fmt.Errorf("failed to set LastHeader: %w", err)
	}
	
	// Set LastBlock
	if err := batch.Set(lastBlockKey, blockHash[:], nil); err != nil {
		return fmt.Errorf("failed to set LastBlock: %w", err)
	}
	
	// Set LastFast
	if err := batch.Set(lastFastBlockKey, blockHash[:], nil); err != nil {
		return fmt.Errorf("failed to set LastFast: %w", err)
	}
	
	// Set HeadHeaderHash
	if err := batch.Set([]byte("LastHeader"), blockHash[:], nil); err != nil {
		return fmt.Errorf("failed to set HeadHeaderHash: %w", err)
	}
	
	// Set HeadBlockHash
	if err := batch.Set([]byte("LastBlock"), blockHash[:], nil); err != nil {
		return fmt.Errorf("failed to set HeadBlockHash: %w", err)
	}
	
	// Set accepted block for consensus
	if err := batch.Set(acceptedKey, blockHash[:], nil); err != nil {
		return fmt.Errorf("failed to set lastAccepted: %w", err)
	}
	
	// Set height
	heightBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(heightBytes, targetHeight)
	if err := batch.Set(heightKey, heightBytes, nil); err != nil {
		return fmt.Errorf("failed to set height: %w", err)
	}
	
	// Also set some additional keys that might be needed
	// CurrentBlock
	if err := batch.Set([]byte("LastBlock"), blockHash[:], nil); err != nil {
		return fmt.Errorf("failed to set CurrentBlock: %w", err)
	}
	
	// CurrentHeader  
	if err := batch.Set([]byte("LastHeader"), blockHash[:], nil); err != nil {
		return fmt.Errorf("failed to set CurrentHeader: %w", err)
	}
	
	// Commit all changes
	fmt.Println("Committing chain state...")
	if err := batch.Commit(nil); err != nil {
		return fmt.Errorf("failed to commit batch: %w", err)
	}
	
	fmt.Printf("\n✅ Chain state setup complete!\n")
	fmt.Printf("   Height: %d\n", targetHeight)
	fmt.Printf("   Hash: %s\n", blockHash.Hex())
	fmt.Printf("\nThe C-Chain should now recognize block %d as the current head.\n", targetHeight)
	
	return nil
}

func findHighestBlock(db *pebble.DB) (uint64, common.Hash, error) {
	var highestNum uint64
	var highestHash common.Hash
	
	// Iterate through canonical hash mappings
	iter, err := db.NewIter(&pebble.IterOptions{
		LowerBound: []byte("H"),
		UpperBound: []byte("I"),
	})
	if err != nil {
		return 0, common.Hash{}, err
	}
	defer iter.Close()
	
	for iter.First(); iter.Valid(); iter.Next() {
		key := iter.Key()
		value := iter.Value()
		
		// Canonical hash keys are "H" + hash -> number
		if len(key) == 33 && key[0] == 'H' && len(value) == 8 {
			blockNum := binary.BigEndian.Uint64(value)
			if blockNum > highestNum {
				highestNum = blockNum
				copy(highestHash[:], key[1:33])
			}
		}
	}
	
	if err := iter.Error(); err != nil {
		return 0, common.Hash{}, err
	}
	
	return highestNum, highestHash, nil
}

func getBlockHash(db *pebble.DB, blockNum uint64) (common.Hash, error) {
	// Look for canonical hash at this height
	// The key format for canonical hash is: "h" + num (8 bytes) + "n"
	numBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(numBytes, blockNum)
	
	canonicalKey := append([]byte("h"), numBytes...)
	canonicalKey = append(canonicalKey, []byte("n")...)
	
	value, closer, err := db.Get(canonicalKey)
	if err != nil {
		// Try alternative format - iterate through headers at this height
		headerPrefix := append([]byte("h"), numBytes...)
		
		iter, err := db.NewIter(&pebble.IterOptions{
			LowerBound: headerPrefix,
			UpperBound: append(headerPrefix, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff),
		})
		if err != nil {
			return common.Hash{}, fmt.Errorf("failed to create iterator: %w", err)
		}
		defer iter.Close()
		
		for iter.First(); iter.Valid(); iter.Next() {
			key := iter.Key()
			if len(key) == 41 && bytes.HasPrefix(key, headerPrefix) {
				var hash common.Hash
				copy(hash[:], key[9:41])
				return hash, nil
			}
		}
		
		return common.Hash{}, fmt.Errorf("no canonical hash found for block %d", blockNum)
	}
	defer closer.Close()
	
	if len(value) != 32 {
		return common.Hash{}, fmt.Errorf("invalid hash length: %d", len(value))
	}
	
	var hash common.Hash
	copy(hash[:], value)
	return hash, nil
}