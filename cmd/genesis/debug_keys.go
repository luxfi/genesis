package main

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"github.com/cockroachdb/pebble"
	"github.com/spf13/cobra"
)

func getDebugKeysCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "debug-keys [db-path]",
		Short: "Debug database keys to understand structure",
		Args:  cobra.ExactArgs(1),
		RunE:  runDebugKeys,
	}
	
	cmd.Flags().String("prefix", "", "Filter by key prefix")
	cmd.Flags().Int("limit", 100, "Limit number of keys to show")
	
	return cmd
}

func runDebugKeys(cmd *cobra.Command, args []string) error {
	dbPath := args[0]
	prefix, _ := cmd.Flags().GetString("prefix")
	limit, _ := cmd.Flags().GetInt("limit")
	
	// Open database
	db, err := pebble.Open(dbPath, &pebble.Options{ReadOnly: true})
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()
	
	// Create iterator
	iterOpts := &pebble.IterOptions{}
	if prefix != "" {
		iterOpts.LowerBound = []byte(prefix)
		iterOpts.UpperBound = append([]byte(prefix), 0xff)
	}
	
	iter, err := db.NewIter(iterOpts)
	if err != nil {
		return fmt.Errorf("failed to create iterator: %w", err)
	}
	defer iter.Close()
	
	// Count different key types
	keyTypes := make(map[string]int)
	canonicalCount := 0
	headerCount := 0
	
	count := 0
	for iter.First(); iter.Valid() && count < limit; iter.Next() {
		key := iter.Key()
		value := iter.Value()
		
		if len(key) == 0 {
			continue
		}
		
		// Analyze key structure
		keyStr := string(key[0])
		keyTypes[keyStr]++
		
		// Show detailed info for interesting keys
		switch key[0] {
		case 'h': // Could be header or canonical hash
			if len(key) == 10 && key[9] == 'n' {
				// Canonical hash: h + num(8) + n
				blockNum := binary.BigEndian.Uint64(key[1:9])
				fmt.Printf("Canonical: block %d -> hash %s\n", blockNum, hex.EncodeToString(value))
				canonicalCount++
			} else if len(key) == 41 {
				// Header: h + num(8) + hash(32)
				blockNum := binary.BigEndian.Uint64(key[1:9])
				hash := hex.EncodeToString(key[9:41])
				fmt.Printf("Header: block %d, hash %s (value len=%d)\n", blockNum, hash, len(value))
				headerCount++
			} else {
				fmt.Printf("Unknown 'h' key: len=%d, hex=%s\n", len(key), hex.EncodeToString(key))
			}
			
		case 'H': // Hash to number mapping
			if len(key) == 33 {
				hash := hex.EncodeToString(key[1:33])
				if len(value) == 8 {
					blockNum := binary.BigEndian.Uint64(value)
					fmt.Printf("HashToNum: %s -> block %d\n", hash, blockNum)
				}
			}
			
		case 'b': // Block body
			if len(key) == 41 {
				blockNum := binary.BigEndian.Uint64(key[1:9])
				hash := hex.EncodeToString(key[9:41])
				fmt.Printf("Body: block %d, hash %s (value len=%d)\n", blockNum, hash, len(value))
			}
			
		case 'r': // Receipts
			if len(key) == 41 {
				blockNum := binary.BigEndian.Uint64(key[1:9])
				hash := hex.EncodeToString(key[9:41])
				fmt.Printf("Receipts: block %d, hash %s (value len=%d)\n", blockNum, hash, len(value))
			}
		}
		
		count++
	}
	
	fmt.Printf("\nKey type summary:\n")
	for k, v := range keyTypes {
		fmt.Printf("  '%s': %d keys\n", k, v)
	}
	
	fmt.Printf("\nFound %d canonical mappings\n", canonicalCount)
	fmt.Printf("Found %d headers\n", headerCount)
	
	return nil
}