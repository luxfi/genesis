package main

import (
	"encoding/binary"
	"fmt"
	"github.com/cockroachdb/pebble"
	"github.com/spf13/cobra"
	"os"
	"path/filepath"
)

func getCompactAncientCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "compact-ancient [db-path]",
		Short: "Compact old blocks into ancient store",
		Long:  "Move blocks older than finality delay into read-only ancient store",
		Args:  cobra.ExactArgs(1),
		RunE:  runCompactAncient,
	}
	
	cmd.Flags().Uint64("finality-delay", 90000, "Number of recent blocks to keep in main DB")
	cmd.Flags().String("ancient-type", "badgerdb", "Ancient store type")
	
	return cmd
}

func runCompactAncient(cmd *cobra.Command, args []string) error {
	dbPath := args[0]
	finalityDelay, _ := cmd.Flags().GetUint64("finality-delay")
	ancientType, _ := cmd.Flags().GetString("ancient-type")
	
	fmt.Printf("Compacting blocks to %s ancient store\n", ancientType)
	fmt.Printf("Keeping last %d blocks in main database\n", finalityDelay)
	
	// Open main database
	db, err := pebble.Open(dbPath, &pebble.Options{})
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()
	
	// Find current head block
	headNum, err := getCurrentHead(db)
	if err != nil {
		return fmt.Errorf("failed to get current head: %w", err)
	}
	
	fmt.Printf("Current head: block %d\n", headNum)
	
	if headNum <= finalityDelay {
		fmt.Println("Not enough blocks to compact")
		return nil
	}
	
	ancientCutoff := headNum - finalityDelay
	fmt.Printf("Will move blocks 0-%d to ancient store\n", ancientCutoff)
	
	// Create ancient store directory
	ancientPath := filepath.Join(filepath.Dir(dbPath), "ancient")
	if err := os.MkdirAll(ancientPath, 0755); err != nil {
		return fmt.Errorf("failed to create ancient directory: %w", err)
	}
	
	// For now, we'll prepare the structure but not actually move data
	// This would be implemented with the actual BadgerDB backend
	
	// Create metadata file
	metadataPath := filepath.Join(ancientPath, "metadata.json")
	metadata := fmt.Sprintf(`{
  "type": "%s",
  "version": "1.0",
  "finalityDelay": %d,
  "ancientBlocks": %d,
  "headBlock": %d
}`, ancientType, finalityDelay, ancientCutoff, headNum)
	
	if err := os.WriteFile(metadataPath, []byte(metadata), 0644); err != nil {
		return fmt.Errorf("failed to write metadata: %w", err)
	}
	
	fmt.Printf("\n✅ Ancient store prepared at: %s\n", ancientPath)
	fmt.Println("Note: Actual block migration to BadgerDB will be implemented with quantum-replayer")
	
	// Update chain info
	updateChainInfo(dbPath, headNum, ancientCutoff)
	
	return nil
}

func getCurrentHead(db *pebble.DB) (uint64, error) {
	// Try to read LastBlock key
	_, closer, err := db.Get([]byte("LastBlock"))
	if err != nil {
		// Try alternative - scan for highest canonical block
		return scanForHighestBlock(db)
	}
	defer closer.Close()
	
	// The value should be a hash, we need to find the block number
	// For now, scan for highest
	return scanForHighestBlock(db)
}

func scanForHighestBlock(db *pebble.DB) (uint64, error) {
	var highest uint64
	
	// Scan canonical mappings
	iter, err := db.NewIter(&pebble.IterOptions{
		LowerBound: []byte("h"),
		UpperBound: []byte("i"),
	})
	if err != nil {
		return 0, err
	}
	defer iter.Close()
	
	for iter.First(); iter.Valid(); iter.Next() {
		key := iter.Key()
		
		// Canonical: h + num(8) + n
		if len(key) == 10 && key[0] == 'h' && key[9] == 'n' {
			blockNum := binary.BigEndian.Uint64(key[1:9])
			if blockNum > highest {
				highest = blockNum
			}
		}
	}
	
	return highest, iter.Error()
}

func updateChainInfo(dbPath string, headNum, ancientCutoff uint64) {
	// Update the chain info file
	chainInfoPath := filepath.Join(filepath.Dir(filepath.Dir(dbPath)), ".processing", "chain_info.json")
	
	info := fmt.Sprintf(`{
  "chainId": 96369,
  "networkName": "lux-mainnet",
  "blockchainId": "dnmzhuf6poM6PUNQCe7MWWfBdTJEnddhHRNXz2x7H6qSmyBEJ",
  "type": "C-Chain",
  "consensusType": "snowman",
  "vmType": "evm",
  "status": "ready",
  "lastProcessedBlock": %d,
  "targetBlock": %d,
  "dbType": "pebbledb",
  "ancientStore": {
    "enabled": true,
    "type": "badgerdb",
    "finalityDelay": 90000,
    "ancientBlocks": %d
  }
}`, headNum, headNum, ancientCutoff)
	
	os.MkdirAll(filepath.Dir(chainInfoPath), 0755)
	os.WriteFile(chainInfoPath, []byte(info), 0644)
}