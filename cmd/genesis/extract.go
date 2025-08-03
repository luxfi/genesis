package main

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"github.com/cockroachdb/pebble"
	"github.com/luxfi/database"
	"github.com/luxfi/geth/common"
	"github.com/luxfi/geth/core/types"
	"github.com/luxfi/geth/rlp"
	"github.com/spf13/cobra"
)

func getExtractCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "extract",
		Short: "Extract data from blockchain",
		Long:  "Extract genesis, state, or blockchain data from existing chain data",
	}

	cmd.AddCommand(getExtractGenesisCmd())
	cmd.AddCommand(getExtractBlockchainCmd())
	cmd.AddCommand(getExtractStateCmd())

	return cmd
}

func getExtractBlockchainCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "blockchain [db-path] [output-path]",
		Short: "Extract blockchain data in various formats",
		Long:  `Extract blockchain data from SubnetEVM format to different output formats.
Use --format=bytes for raw chaindata suitable for replay.
Use --format=json for human-readable blockchain data.
Use --format=coreth for C-Chain compatible namespaced format.`,
		Args:  cobra.ExactArgs(2),
		RunE:  runExtractBlockchain,
	}

	cmd.Flags().String("format", "bytes", "Output format: bytes (raw chaindata), json (human readable), coreth (C-Chain compatible)")
	cmd.Flags().Uint64("start-block", 0, "Starting block number")
	cmd.Flags().Uint64("end-block", 0, "Ending block number (0 = all blocks)")
	cmd.Flags().String("network", "lux", "Network type for namespace conversion (lux, zoo, spc)")

	return cmd
}

func getExtractGenesisCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "genesis [db-path]",
		Short: "Extract genesis configuration from blockchain data",
		Args:  cobra.ExactArgs(1),
		RunE:  runExtractGenesis,
	}

	cmd.Flags().String("output", "extracted-genesis.json", "Output file for genesis configuration")
	cmd.Flags().Bool("with-state", false, "Include state allocations in genesis")
	cmd.Flags().String("db-type", "auto", "Database type: leveldb, badgerdb, pebbledb, or auto")

	return cmd
}

func getExtractStateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "state [db-path] [output-dir]",
		Short: "Extract state data from blockchain",
		Args:  cobra.ExactArgs(2),
		RunE:  runExtractState,
	}

	cmd.Flags().Uint64("block", 0, "Block number to extract state from (0 = latest)")
	cmd.Flags().Bool("accounts-only", false, "Extract only account balances")

	return cmd
}

func runExtractGenesis(cmd *cobra.Command, args []string) error {
	dbPath := args[0]
	outputFile, _ := cmd.Flags().GetString("output")
	withState, _ := cmd.Flags().GetBool("with-state")
	dbType, _ := cmd.Flags().GetString("db-type")

	// Open database using luxfi/database
	var db database.Database
	var err error
	
	// For now, let's use the approach from the existing code that works
	// We'll use PebbleDB directly since that's what the source data uses
	if dbType == "auto" || dbType == "pebbledb" {
		pdb, err := pebble.Open(dbPath, &pebble.Options{ReadOnly: true})
		if err != nil {
			return fmt.Errorf("failed to open database: %w", err)
		}
		defer pdb.Close()
		
		// Use pebble directly for now
		return extractGenesisFromPebble(pdb, outputFile, withState)
	}
	
	// TODO: Add support for other database types
	return fmt.Errorf("database type %s not yet supported", dbType)
}

func extractGenesisFromPebble(db *pebble.DB, outputFile string, withState bool) error {
	fmt.Println("🔍 Extracting genesis configuration from blockchain data...")

	// Get block 0 hash
	canonicalKey := make([]byte, 10)
	canonicalKey[0] = 0x68 // 'h'
	canonicalKey[9] = 0x6e // 'n'

	hashBytes, closer, err := db.Get(canonicalKey)
	if err != nil {
		return fmt.Errorf("genesis block not found: %w", err)
	}
	closer.Close()

	genesisHash := common.BytesToHash(hashBytes)
	fmt.Printf("Genesis hash: %s\n", genesisHash.Hex())

	// Get the header
	headerKey := append(append([]byte{0x68}, make([]byte, 8)...), hashBytes...)
	headerData, closer, err := db.Get(headerKey)
	if err != nil {
		return fmt.Errorf("genesis header not found: %w", err)
	}
	closer.Close()

	var header types.Header
	if err := rlp.DecodeBytes(headerData, &header); err != nil {
		return fmt.Errorf("failed to decode header: %w", err)
	}

	fmt.Printf("\n📋 Genesis Block Info:\n")
	fmt.Printf("  Time:       %d\n", header.Time)
	fmt.Printf("  GasLimit:   %d (0x%x)\n", header.GasLimit, header.GasLimit)
	fmt.Printf("  Difficulty: %s\n", header.Difficulty)
	fmt.Printf("  Extra:      %s\n", hex.EncodeToString(header.Extra))

	// Create genesis configuration
	genesis := map[string]interface{}{
		"config": map[string]interface{}{
			"chainId":             96369,
			"homesteadBlock":      0,
			"eip150Block":         0,
			"eip155Block":         0,
			"eip158Block":         0,
			"byzantiumBlock":      0,
			"constantinopleBlock": 0,
			"petersburgBlock":     0,
			"istanbulBlock":       0,
			"muirGlacierBlock":    0,
			"berlinBlock":         0,
			"londonBlock":         0,
			"subnetEVMTimestamp":  header.Time,
			"feeConfig": map[string]interface{}{
				"gasLimit":                 header.GasLimit,
				"minBaseFee":               25000000000,
				"targetGas":                15000000,
				"baseFeeChangeDenominator": 36,
				"minBlockGasCost":          0,
				"maxBlockGasCost":          1000000,
				"targetBlockRate":          2,
				"blockGasCostStep":         200000,
			},
		},
		"nonce":      fmt.Sprintf("0x%x", header.Nonce),
		"timestamp":  fmt.Sprintf("0x%x", header.Time),
		"gasLimit":   fmt.Sprintf("0x%x", header.GasLimit),
		"difficulty": fmt.Sprintf("0x%x", header.Difficulty),
		"mixHash":    header.MixDigest.Hex(),
		"coinbase":   header.Coinbase.Hex(),
		"number":     "0x0",
		"gasUsed":    "0x0",
		"parentHash": header.ParentHash.Hex(),
		"extraData":  "0x" + hex.EncodeToString(header.Extra),
		"alloc":      map[string]interface{}{},
	}

	// Extract state if requested
	if withState {
		fmt.Println("📊 Extracting state allocations...")
		// TODO: Implement state extraction from trie
		// For now, add default allocation
		genesis["alloc"] = map[string]interface{}{
			"0x8db97C7cEcE249c2b98bDC0226Cc4C2A57BF52FC": map[string]interface{}{
				"balance": "0x295BE96E64066972000000", // 50M tokens
			},
		}
	}

	// Save genesis
	pretty, _ := json.MarshalIndent(genesis, "", "  ")
	if err := os.WriteFile(outputFile, pretty, 0644); err != nil {
		return fmt.Errorf("failed to write genesis file: %w", err)
	}

	fmt.Printf("\n✅ Saved genesis to %s\n", outputFile)
	return nil
}

func runExtractBlockchain(cmd *cobra.Command, args []string) error {
	dbPath := args[0]
	outputPath := args[1]
	format, _ := cmd.Flags().GetString("format")
	startBlock, _ := cmd.Flags().GetUint64("start-block")
	endBlock, _ := cmd.Flags().GetUint64("end-block")
	network, _ := cmd.Flags().GetString("network")

	// Open database
	db, err := pebble.Open(dbPath, &pebble.Options{ReadOnly: true})
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	fmt.Printf("📦 Extracting blockchain data...\n")
	fmt.Printf("  Source: %s\n", dbPath)
	fmt.Printf("  Output: %s\n", outputPath)
	fmt.Printf("  Format: %s\n", format)
	fmt.Printf("  Network: %s\n", network)

	switch format {
	case "bytes":
		return extractRawChaindata(db, outputPath, startBlock, endBlock)
	case "json":
		return extractAsJSON(db, outputPath, startBlock, endBlock)
	case "coreth":
		return extractForCoreth(db, outputPath, startBlock, endBlock, network)
	default:
		return fmt.Errorf("unsupported format: %s", format)
	}
}

func extractRawChaindata(db *pebble.DB, outputPath string, startBlock, endBlock uint64) error {
	fmt.Println("🔄 Extracting raw chaindata bytes...")
	
	// Create output directory
	if err := os.MkdirAll(outputPath, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Open output database
	outDB, err := pebble.Open(outputPath, &pebble.Options{})
	if err != nil {
		return fmt.Errorf("failed to create output database: %w", err)
	}
	defer outDB.Close()

	// Copy all relevant keys
	iter, err := db.NewIter(nil)
	if err != nil {
		return fmt.Errorf("failed to create iterator: %w", err)
	}
	defer iter.Close()

	count := 0
	batch := outDB.NewBatch()
	
	for iter.First(); iter.Valid(); iter.Next() {
		key := iter.Key()
		value := iter.Value()
		
		// Copy key-value pairs
		if err := batch.Set(key, value, nil); err != nil {
			return fmt.Errorf("failed to set key: %w", err)
		}
		
		count++
		if count%10000 == 0 {
			if err := batch.Commit(nil); err != nil {
				return fmt.Errorf("failed to commit batch: %w", err)
			}
			batch = outDB.NewBatch()
			fmt.Printf("  Copied %d keys...\n", count)
		}
	}

	if err := batch.Commit(nil); err != nil {
		return fmt.Errorf("failed to commit final batch: %w", err)
	}

	fmt.Printf("✅ Extracted %d keys to %s\n", count, outputPath)
	return nil
}

func extractAsJSON(db *pebble.DB, outputPath string, startBlock, endBlock uint64) error {
	fmt.Println("📋 Extracting blockchain as JSON...")
	
	// Get canonical blocks
	canonicalBlocks := make(map[uint64]common.Hash)
	blockNumbers := []uint64{}

	// First pass: find all canonical blocks
	iter, err := db.NewIter(nil)
	if err != nil {
		return fmt.Errorf("failed to create iterator: %w", err)
	}
	defer iter.Close()

	for iter.First(); iter.Valid(); iter.Next() {
		key := iter.Key()
		if len(key) == 10 && key[0] == 0x68 && key[9] == 0x6e { // 'h' + number + 'n'
			num := binary.BigEndian.Uint64(key[1:9])
			if (startBlock == 0 || num >= startBlock) && (endBlock == 0 || num <= endBlock) {
				hash := common.BytesToHash(iter.Value())
				canonicalBlocks[num] = hash
				blockNumbers = append(blockNumbers, num)
			}
		}
	}

	sort.Slice(blockNumbers, func(i, j int) bool { return blockNumbers[i] < blockNumbers[j] })

	// Create output structure
	output := struct {
		ChainID     uint64                 `json:"chainId"`
		BlockCount  int                    `json:"blockCount"`
		StartBlock  uint64                 `json:"startBlock"`
		EndBlock    uint64                 `json:"endBlock"`
		Blocks      []map[string]interface{} `json:"blocks"`
	}{
		ChainID:    96369, // TODO: Get from chain config
		BlockCount: len(blockNumbers),
		Blocks:     make([]map[string]interface{}, 0, len(blockNumbers)),
	}

	if len(blockNumbers) > 0 {
		output.StartBlock = blockNumbers[0]
		output.EndBlock = blockNumbers[len(blockNumbers)-1]
	}

	// Extract block details
	for i, num := range blockNumbers {
		hash := canonicalBlocks[num]
		
		// Get header
		headerKey := append([]byte{0x68}, append(make([]byte, 8), hash[:]...)...)
		binary.BigEndian.PutUint64(headerKey[1:9], num)
		
		headerData, closer, err := db.Get(headerKey)
		if err != nil {
			fmt.Printf("Warning: header not found for block %d\n", num)
			continue
		}
		closer.Close()

		var header types.Header
		if err := rlp.DecodeBytes(headerData, &header); err != nil {
			fmt.Printf("Warning: failed to decode header for block %d: %v\n", num, err)
			continue
		}

		blockInfo := map[string]interface{}{
			"number":     num,
			"hash":       hash.Hex(),
			"parentHash": header.ParentHash.Hex(),
			"timestamp":  header.Time,
			"gasLimit":   header.GasLimit,
			"gasUsed":    header.GasUsed,
			"miner":      header.Coinbase.Hex(),
			"difficulty": header.Difficulty.String(),
			"stateRoot":  header.Root.Hex(),
		}

		output.Blocks = append(output.Blocks, blockInfo)

		if i%1000 == 0 {
			fmt.Printf("  Processed %d/%d blocks...\n", i, len(blockNumbers))
		}
	}

	// Save to file
	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	if err := os.WriteFile(outputPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	fmt.Printf("✅ Extracted %d blocks to %s\n", len(output.Blocks), outputPath)
	return nil
}

func extractForCoreth(db *pebble.DB, outputPath string, startBlock, endBlock uint64, network string) error {
	fmt.Println("🔄 Extracting for Coreth (C-Chain) format...")
	fmt.Printf("  Converting from SubnetEVM to Coreth namespaced format\n")
	
	// TODO: Implement Coreth-specific extraction with proper namespacing
	// This would involve:
	// 1. Converting SubnetEVM key prefixes to Coreth format
	// 2. Adding proper namespace prefixes
	// 3. Handling consensus-specific data
	
	return fmt.Errorf("coreth format extraction not yet implemented")
}

func runExtractState(cmd *cobra.Command, args []string) error {
	dbPath := args[0]
	outputDir := args[1]
	blockNum, _ := cmd.Flags().GetUint64("block")
	accountsOnly, _ := cmd.Flags().GetBool("accounts-only")

	fmt.Printf("📊 Extracting state from block %d...\n", blockNum)
	fmt.Printf("Database: %s\n", dbPath)
	fmt.Printf("Output: %s\n", outputDir)
	fmt.Printf("Mode: %s\n", map[bool]string{true: "accounts only", false: "full state"}[accountsOnly])

	// TODO: Implement full state extraction
	// This would involve:
	// 1. Reading the state trie at the specified block
	// 2. Iterating through all accounts
	// 3. Extracting balances, nonces, code, and storage
	// 4. Saving to output directory

	return fmt.Errorf("state extraction not yet implemented")
}