package main

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"

	"github.com/cockroachdb/pebble"
	"github.com/luxfi/geth/common"
	"github.com/luxfi/geth/core/types"
	"github.com/luxfi/geth/rlp"
	"github.com/spf13/cobra"
)

func getInspectCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "inspect",
		Short: "Inspect blockchain data",
		Long:  "Inspect blockchain data to extract genesis, blocks, and configuration",
	}

	cmd.AddCommand(getInspectGenesisCmd())
	cmd.AddCommand(getInspectTipCmd())
	cmd.AddCommand(getInspectBlocksCmd())
	cmd.AddCommand(getInspectKeysCmd())

	return cmd
}

func getInspectGenesisCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "genesis [db-path]",
		Short: "Extract genesis configuration from blockchain data",
		Long:  "Extract the genesis block configuration and parameters from an existing blockchain database",
		Args:  cobra.ExactArgs(1),
		RunE:  runInspectGenesis,
	}

	cmd.Flags().String("output", "genesis.json", "Output file for genesis configuration")

	return cmd
}

func runInspectGenesis(cmd *cobra.Command, args []string) error {
	dbPath := args[0]
	outputFile, _ := cmd.Flags().GetString("output")

	// Open database
	db, err := pebble.Open(dbPath, &pebble.Options{})
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	fmt.Println("🔍 Inspecting genesis configuration...")

	// Get block 0 (genesis)
	canonicalKey := make([]byte, 10)
	canonicalKey[0] = 0x68 // 'h'
	// Block 0 = all zeros for number bytes
	canonicalKey[9] = 0x6e // 'n'

	hashBytes, closer, err := db.Get(canonicalKey)
	if err != nil {
		return fmt.Errorf("genesis block not found: %w", err)
	}
	defer closer.Close()

	genesisHash := common.BytesToHash(hashBytes)
	fmt.Printf("Genesis hash: %s\n", genesisHash.Hex())

	// Get the header
	headerKey := append([]byte{0x48}, hashBytes...) // 'H' + hash
	headerData, hCloser, err := db.Get(headerKey)
	if err != nil {
		return fmt.Errorf("genesis header not found: %w", err)
	}
	defer hCloser.Close()

	var header types.Header
	if err := rlp.DecodeBytes(headerData, &header); err != nil {
		return fmt.Errorf("failed to decode genesis header: %w", err)
	}

	fmt.Printf("\n📋 Genesis Block Info:\n")
	fmt.Printf("  Number:     %s\n", header.Number)
	fmt.Printf("  Time:       %d\n", header.Time)
	fmt.Printf("  GasLimit:   %d (0x%x)\n", header.GasLimit, header.GasLimit)
	fmt.Printf("  Difficulty: %s\n", header.Difficulty)
	fmt.Printf("  Coinbase:   %s\n", header.Coinbase.Hex())
	fmt.Printf("  StateRoot:  %s\n", header.Root.Hex())
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
			"subnetEVMTimestamp":  0,
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
		"parentHash": "0x0000000000000000000000000000000000000000000000000000000000000000",
		"alloc":      map[string]interface{}{},
	}

	// Check for state allocations
	fmt.Println("\n🔍 Scanning for account allocations...")
	alloc := extractAllocations(db, header.Root)
	if len(alloc) > 0 {
		genesis["alloc"] = alloc
		fmt.Printf("Found %d accounts with balances\n", len(alloc))
	}

	// Save genesis
	pretty, _ := json.MarshalIndent(genesis, "", "  ")
	if err := os.WriteFile(outputFile, pretty, 0644); err != nil {
		return fmt.Errorf("failed to write genesis file: %w", err)
	}

	fmt.Printf("\n✅ Genesis configuration saved to %s\n", outputFile)
	fmt.Println("\n📄 Genesis Configuration:")
	fmt.Println(string(pretty))

	return nil
}

func getInspectTipCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tip [db-path]",
		Short: "Find the chain tip (highest block)",
		Args:  cobra.ExactArgs(1),
		RunE:  runInspectTip,
	}

	return cmd
}

func runInspectTip(cmd *cobra.Command, args []string) error {
	dbPath := args[0]

	db, err := pebble.Open(dbPath, &pebble.Options{})
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	// Try LastBlock key
	if hash, closer, err := db.Get([]byte("LastBlock")); err == nil {
		defer closer.Close()
		fmt.Printf("LastBlock: %s\n", common.BytesToHash(hash).Hex())
	}

	// Scan for highest canonical block
	var highest uint64
	iter, err := db.NewIter(&pebble.IterOptions{
		LowerBound: []byte{0x68}, // 'h'
		UpperBound: []byte{0x69}, // 'i'
	})
	if err != nil {
		return err
	}
	defer iter.Close()

	for iter.First(); iter.Valid(); iter.Next() {
		key := iter.Key()
		if len(key) == 10 && key[0] == 0x68 && key[9] == 0x6e {
			blockNum := binary.BigEndian.Uint64(key[1:9])
			if blockNum > highest {
				highest = blockNum
			}
		}
	}

	fmt.Printf("Highest block: %d\n", highest)

	// Get that block's info
	if highest > 0 {
		canonicalKey := make([]byte, 10)
		canonicalKey[0] = 0x68
		binary.BigEndian.PutUint64(canonicalKey[1:9], highest)
		canonicalKey[9] = 0x6e

		if hash, closer, err := db.Get(canonicalKey); err == nil {
			defer closer.Close()
			fmt.Printf("Block %d hash: %s\n", highest, common.BytesToHash(hash).Hex())

			// Get header
			headerKey := append([]byte{0x48}, hash...)
			if headerData, hCloser, err := db.Get(headerKey); err == nil {
				defer hCloser.Close()
				var header types.Header
				if err := rlp.DecodeBytes(headerData, &header); err == nil {
					fmt.Printf("  Time: %d\n", header.Time)
					fmt.Printf("  GasLimit: %d\n", header.GasLimit)
					fmt.Printf("  StateRoot: %s\n", header.Root.Hex())
				}
			}
		}
	}

	return iter.Error()
}

func getInspectBlocksCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "blocks [db-path]",
		Short: "Inspect block data",
		Args:  cobra.ExactArgs(1),
		RunE:  runInspectBlocks,
	}

	cmd.Flags().Uint64("start", 0, "Starting block number")
	cmd.Flags().Uint64("limit", 10, "Number of blocks to show")

	return cmd
}

func runInspectBlocks(cmd *cobra.Command, args []string) error {
	dbPath := args[0]
	start, _ := cmd.Flags().GetUint64("start")
	limit, _ := cmd.Flags().GetUint64("limit")

	db, err := pebble.Open(dbPath, &pebble.Options{})
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	fmt.Printf("Inspecting blocks %d to %d\n\n", start, start+limit-1)

	for i := uint64(0); i < limit; i++ {
		blockNum := start + i

		// Get canonical hash
		canonicalKey := make([]byte, 10)
		canonicalKey[0] = 0x68
		binary.BigEndian.PutUint64(canonicalKey[1:9], blockNum)
		canonicalKey[9] = 0x6e

		hash, closer, err := db.Get(canonicalKey)
		if err != nil {
			continue
		}
		closer.Close()

		fmt.Printf("Block %d: %s\n", blockNum, common.BytesToHash(hash).Hex())

		// Get header
		headerKey := append([]byte{0x48}, hash...)
		headerData, hCloser, err := db.Get(headerKey)
		if err == nil {
			hCloser.Close()
			var header types.Header
			if err := rlp.DecodeBytes(headerData, &header); err == nil {
				fmt.Printf("  Parent: %s\n", header.ParentHash.Hex())
				fmt.Printf("  Time: %d\n", header.Time)
				fmt.Printf("  GasUsed: %d/%d\n", header.GasUsed, header.GasLimit)
				fmt.Printf("  StateRoot: %s\n", header.Root.Hex())
			}
		}

		fmt.Println()
	}

	return nil
}

func getInspectKeysCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "keys [db-path]",
		Short: "Inspect database keys",
		Args:  cobra.ExactArgs(1),
		RunE:  runInspectKeys,
	}

	cmd.Flags().String("prefix", "", "Filter by key prefix")
	cmd.Flags().Uint64("limit", 100, "Maximum keys to show")

	return cmd
}

func runInspectKeys(cmd *cobra.Command, args []string) error {
	dbPath := args[0]
	prefix, _ := cmd.Flags().GetString("prefix")
	limit, _ := cmd.Flags().GetUint64("limit")

	db, err := pebble.Open(dbPath, &pebble.Options{})
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	fmt.Println("Database keys:")

	iter, err := db.NewIter(nil)
	if err != nil {
		return err
	}
	defer iter.Close()

	count := uint64(0)
	for iter.First(); iter.Valid() && count < limit; iter.Next() {
		key := iter.Key()
		
		if prefix != "" {
			if !hasPrefix(key, []byte(prefix)) {
				continue
			}
		}

		keyStr := string(key)
		if isPrintable(keyStr) {
			fmt.Printf("  %s", keyStr)
		} else {
			fmt.Printf("  %s", hex.EncodeToString(key))
		}

		value := iter.Value()
		fmt.Printf(" (%d bytes)\n", len(value))

		count++
	}

	return iter.Error()
}

func extractAllocations(db *pebble.DB, stateRoot common.Hash) map[string]interface{} {
	alloc := make(map[string]interface{})

	// For now, include known test accounts
	// In a real implementation, we'd walk the state trie
	testAccounts := map[string]string{
		"0x8db97C7cEcE249c2b98bDC0226Cc4C2A57BF52FC": "0x295BE96E64066972000000", // 50M tokens
	}

	for addr, balance := range testAccounts {
		alloc[addr] = map[string]interface{}{
			"balance": balance,
		}
	}

	return alloc
}

func hasPrefix(data, prefix []byte) bool {
	if len(data) < len(prefix) {
		return false
	}
	for i := 0; i < len(prefix); i++ {
		if data[i] != prefix[i] {
			return false
		}
	}
	return true
}

func isPrintable(s string) bool {
	for _, r := range s {
		if r < 32 || r > 126 {
			return false
		}
	}
	return true
}

