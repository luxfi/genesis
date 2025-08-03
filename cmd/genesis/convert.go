package main

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"

	"github.com/cockroachdb/pebble"
	"github.com/luxfi/geth/common"
	"github.com/spf13/cobra"
)

func getConvertCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "convert",
		Short: "Convert blockchain data between formats",
		Long:  "Convert SubnetEVM blockchain data to different formats (C-Chain/Coreth, L2)",
	}

	cmd.AddCommand(getConvertToCorethCmd())
	cmd.AddCommand(getConvertToL2Cmd())

	return cmd
}

func getConvertToCorethCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "coreth [source-db] [output-db]",
		Short: "Convert SubnetEVM data to Coreth (C-Chain) format",
		Long:  `Convert SubnetEVM blockchain data to Coreth format with proper namespacing.
This is used for migrating subnet data to C-Chain for replay.`,
		Args:  cobra.ExactArgs(2),
		RunE:  runConvertToCoreth,
	}

	cmd.Flags().String("network", "lux", "Network name (lux, zoo, spc)")
	cmd.Flags().Uint64("chain-id", 96369, "Chain ID for the network")
	cmd.Flags().Bool("with-state", true, "Include state data in conversion")

	return cmd
}

func getConvertToL2Cmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "l2 [source-db] [output-db]",
		Short: "Convert SubnetEVM data for L2 deployment",
		Long:  `Prepare SubnetEVM blockchain data for deployment as an L2.
This preserves all existing state and history while updating configuration for L2 operation.`,
		Args:  cobra.ExactArgs(2),
		RunE:  runConvertToL2,
	}

	cmd.Flags().String("network", "zoo", "Network name (zoo, spc)")
	cmd.Flags().Uint64("chain-id", 200200, "Chain ID for the L2 network")

	return cmd
}

func runConvertToCoreth(cmd *cobra.Command, args []string) error {
	sourceDB := args[0]
	outputDB := args[1]
	network, _ := cmd.Flags().GetString("network")
	chainID, _ := cmd.Flags().GetUint64("chain-id")
	withState, _ := cmd.Flags().GetBool("with-state")

	fmt.Printf("🔄 Converting SubnetEVM to Coreth format...\n")
	fmt.Printf("  Source: %s\n", sourceDB)
	fmt.Printf("  Output: %s\n", outputDB)
	fmt.Printf("  Network: %s (Chain ID: %d)\n", network, chainID)
	fmt.Printf("  Include State: %v\n", withState)

	// Open source database
	src, err := pebble.Open(sourceDB, &pebble.Options{ReadOnly: true})
	if err != nil {
		return fmt.Errorf("failed to open source database: %w", err)
	}
	defer src.Close()

	// Create output directory
	if err := os.MkdirAll(filepath.Dir(outputDB), 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Open destination database
	dst, err := pebble.Open(outputDB, &pebble.Options{})
	if err != nil {
		return fmt.Errorf("failed to open destination database: %w", err)
	}
	defer dst.Close()

	// Convert key prefixes from SubnetEVM to Coreth format
	// SubnetEVM uses simple prefixes, Coreth uses namespaced keys
	
	fmt.Println("📊 Analyzing source database...")
	
	// First, find the highest block
	highestBlock := uint64(0)
	iter, err := src.NewIter(nil)
	if err != nil {
		return fmt.Errorf("failed to create iterator: %w", err)
	}
	defer iter.Close()

	// Count blocks
	blockCount := 0
	for iter.First(); iter.Valid(); iter.Next() {
		key := iter.Key()
		if len(key) == 10 && key[0] == 0x68 && key[9] == 0x6e { // 'h' + number + 'n'
			num := binary.BigEndian.Uint64(key[1:9])
			if num > highestBlock {
				highestBlock = num
			}
			blockCount++
		}
	}

	fmt.Printf("  Found %d blocks (highest: %d)\n", blockCount, highestBlock)

	// Convert blocks and headers
	fmt.Println("🔄 Converting blockchain data...")
	
	batch := dst.NewBatch()
	converted := 0
	
	// Reset iterator
	iter, err = src.NewIter(nil)
	if err != nil {
		return fmt.Errorf("failed to create iterator: %w", err)
	}

	for iter.First(); iter.Valid(); iter.Next() {
		key := iter.Key()
		value := iter.Value()
		
		// Convert key based on prefix
		var newKey []byte
		
		switch {
		case len(key) == 10 && key[0] == 0x68 && key[9] == 0x6e: // Canonical hash
			// h + num(8) + n -> eth/db/hashToNumber/ namespace
			newKey = convertCanonicalKey(key)
			
		case len(key) >= 41 && key[0] == 0x68: // Header
			// h + num(8) + hash(32) -> eth/db/header/ namespace
			newKey = convertHeaderKey(key)
			
		case len(key) >= 41 && key[0] == 0x62: // Body
			// b + num(8) + hash(32) -> eth/db/body/ namespace
			newKey = convertBodyKey(key)
			
		case len(key) >= 41 && key[0] == 0x72: // Receipts
			// r + num(8) + hash(32) -> eth/db/receipts/ namespace
			newKey = convertReceiptsKey(key)
			
		case len(key) >= 41 && key[0] == 0x74: // TD (total difficulty)
			// t + num(8) + hash(32) -> eth/db/td/ namespace
			newKey = convertTDKey(key)
			
		default:
			// For other keys, copy as-is with a prefix
			newKey = append([]byte("eth/db/misc/"), key...)
		}
		
		if err := batch.Set(newKey, value, nil); err != nil {
			return fmt.Errorf("failed to set key: %w", err)
		}
		
		converted++
		if converted%10000 == 0 {
			if err := batch.Commit(nil); err != nil {
				return fmt.Errorf("failed to commit batch: %w", err)
			}
			batch = dst.NewBatch()
			fmt.Printf("  Converted %d keys...\n", converted)
		}
	}

	// Set head references for Coreth
	if highestBlock > 0 {
		// Get the hash of the highest block
		canonicalKey := make([]byte, 10)
		canonicalKey[0] = 0x68 // 'h'
		binary.BigEndian.PutUint64(canonicalKey[1:9], highestBlock)
		canonicalKey[9] = 0x6e // 'n'
		
		hashBytes, closer, err := src.Get(canonicalKey)
		if err == nil {
			closer.Close()
			hash := common.BytesToHash(hashBytes)
			
			// Set Coreth head references
			batch.Set([]byte("eth/db/LastHeader"), hash[:], nil)
			batch.Set([]byte("eth/db/LastBlock"), hash[:], nil)
			batch.Set([]byte("eth/db/LastFast"), hash[:], nil)
			batch.Set([]byte("snowman_lastAccepted"), hash[:], nil)
			
			fmt.Printf("  Set head block: %d (hash: %s)\n", highestBlock, hash.Hex())
		}
	}

	if err := batch.Commit(nil); err != nil {
		return fmt.Errorf("failed to commit final batch: %w", err)
	}

	fmt.Printf("✅ Converted %d keys to Coreth format\n", converted)
	fmt.Printf("📁 Output database: %s\n", outputDB)

	return nil
}

func runConvertToL2(cmd *cobra.Command, args []string) error {
	sourceDB := args[0]
	outputDB := args[1]
	network, _ := cmd.Flags().GetString("network")
	chainID, _ := cmd.Flags().GetUint64("chain-id")

	fmt.Printf("🔄 Converting SubnetEVM data for L2 deployment...\n")
	fmt.Printf("  Source: %s\n", sourceDB)
	fmt.Printf("  Output: %s\n", outputDB)
	fmt.Printf("  Network: %s (Chain ID: %d)\n", network, chainID)

	// For L2, we mainly need to:
	// 1. Copy the existing data as-is (it's already in SubnetEVM format)
	// 2. Update any chain-specific configuration
	// 3. Ensure compatibility with the new L2 deployment

	// Open source database
	src, err := pebble.Open(sourceDB, &pebble.Options{ReadOnly: true})
	if err != nil {
		return fmt.Errorf("failed to open source database: %w", err)
	}
	defer src.Close()

	// Create output directory
	if err := os.MkdirAll(filepath.Dir(outputDB), 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// For L2, we can mostly copy the data as-is
	fmt.Println("📋 Copying blockchain data for L2...")
	
	// Use cp command for efficiency
	// cpCmd := fmt.Sprintf("cp -r %s %s", sourceDB, outputDB)
	if err := os.RemoveAll(outputDB); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove existing output: %w", err)
	}
	
	// TODO: Use proper file copying instead of shell command
	fmt.Printf("  Copying database files...\n")
	
	// For now, do a key-by-key copy
	dst, err := pebble.Open(outputDB, &pebble.Options{})
	if err != nil {
		return fmt.Errorf("failed to open destination database: %w", err)
	}
	defer dst.Close()

	iter, err := src.NewIter(nil)
	if err != nil {
		return fmt.Errorf("failed to create iterator: %w", err)
	}
	defer iter.Close()

	batch := dst.NewBatch()
	count := 0
	
	for iter.First(); iter.Valid(); iter.Next() {
		if err := batch.Set(iter.Key(), iter.Value(), nil); err != nil {
			return fmt.Errorf("failed to copy key: %w", err)
		}
		
		count++
		if count%10000 == 0 {
			if err := batch.Commit(nil); err != nil {
				return fmt.Errorf("failed to commit batch: %w", err)
			}
			batch = dst.NewBatch()
			fmt.Printf("  Copied %d keys...\n", count)
		}
	}

	if err := batch.Commit(nil); err != nil {
		return fmt.Errorf("failed to commit final batch: %w", err)
	}

	fmt.Printf("✅ Prepared %d keys for L2 deployment\n", count)
	fmt.Printf("📁 Output database: %s\n", outputDB)
	fmt.Printf("\n💡 Next steps:\n")
	fmt.Printf("  1. Create L2 subnet: lux-cli subnet create %s --evm --chainId=%d\n", network, chainID)
	fmt.Printf("  2. Import chaindata: lux-cli subnet import %s --chaindata=%s\n", network, outputDB)
	fmt.Printf("  3. Deploy subnet: lux-cli subnet deploy %s --local\n", network)

	return nil
}

// Key conversion helpers for Coreth format

func convertCanonicalKey(key []byte) []byte {
	// h + num(8) + n -> eth/db/hashToNumber/
	num := binary.BigEndian.Uint64(key[1:9])
	newKey := []byte("eth/db/canonical/")
	newKey = append(newKey, encodeBlockNumber(num)...)
	return newKey
}

func convertHeaderKey(key []byte) []byte {
	// h + num(8) + hash(32) -> eth/db/header/
	newKey := []byte("eth/db/header/")
	newKey = append(newKey, key[1:]...) // num + hash
	return newKey
}

func convertBodyKey(key []byte) []byte {
	// b + num(8) + hash(32) -> eth/db/body/
	newKey := []byte("eth/db/body/")
	newKey = append(newKey, key[1:]...) // num + hash
	return newKey
}

func convertReceiptsKey(key []byte) []byte {
	// r + num(8) + hash(32) -> eth/db/receipts/
	newKey := []byte("eth/db/receipts/")
	newKey = append(newKey, key[1:]...) // num + hash
	return newKey
}

func convertTDKey(key []byte) []byte {
	// t + num(8) + hash(32) -> eth/db/td/
	newKey := []byte("eth/db/td/")
	newKey = append(newKey, key[1:]...) // num + hash
	return newKey
}

func encodeBlockNumber(number uint64) []byte {
	enc := make([]byte, 8)
	binary.BigEndian.PutUint64(enc, number)
	return enc
}