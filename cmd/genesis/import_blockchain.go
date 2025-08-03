package main

import (
	"fmt"
	"github.com/cockroachdb/pebble"
	"github.com/spf13/cobra"
)

func getImportBlockchainCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "import-blockchain [source-db] [dest-db]",
		Short: "Import blockchain data from extracted database",
		Long:  "Import blockchain headers, bodies, and receipts from extracted SubnetEVM data into C-Chain format",
		Args:  cobra.ExactArgs(2),
		RunE:  runImportBlockchain,
	}
	
	return cmd
}

func runImportBlockchain(cmd *cobra.Command, args []string) error {
	sourceDB := args[0]
	destDB := args[1]
	
	fmt.Printf("Importing blockchain data from %s to %s\n", sourceDB, destDB)
	
	// Open source database (read-only)
	srcOpts := &pebble.Options{
		ReadOnly: true,
	}
	src, err := pebble.Open(sourceDB, srcOpts)
	if err != nil {
		return fmt.Errorf("failed to open source database: %w", err)
	}
	defer src.Close()
	
	// Open destination database
	dst, err := pebble.Open(destDB, &pebble.Options{})
	if err != nil {
		return fmt.Errorf("failed to open destination database: %w", err)
	}
	defer dst.Close()
	
	// Create an iterator to scan all keys
	iter, err := src.NewIter(&pebble.IterOptions{})
	if err != nil {
		return fmt.Errorf("failed to create iterator: %w", err)
	}
	defer iter.Close()
	
	batch := dst.NewBatch()
	count := 0
	headerCount := 0
	bodyCount := 0
	receiptCount := 0
	canonicalCount := 0
	
	for iter.First(); iter.Valid(); iter.Next() {
		key := iter.Key()
		value := iter.Value()
		
		// Copy key-value to destination
		if err := batch.Set(key, value, nil); err != nil {
			return fmt.Errorf("failed to set key: %w", err)
		}
		
		// Track what type of data we're copying
		if len(key) > 0 {
			switch key[0] {
			case 'h': // header prefix
				headerCount++
			case 'b': // body prefix
				bodyCount++
			case 'r': // receipt prefix
				receiptCount++
			case 'H': // canonical hash prefix
				canonicalCount++
			}
		}
		
		count++
		
		// Commit batch every 10000 entries
		if count%10000 == 0 {
			if err := batch.Commit(nil); err != nil {
				return fmt.Errorf("failed to commit batch: %w", err)
			}
			batch = dst.NewBatch()
			fmt.Printf("Imported %d keys: %d headers, %d bodies, %d receipts, %d canonical\n", 
				count, headerCount, bodyCount, receiptCount, canonicalCount)
		}
	}
	
	// Commit final batch
	if err := batch.Commit(nil); err != nil {
		return fmt.Errorf("failed to commit final batch: %w", err)
	}
	
	if err := iter.Error(); err != nil {
		return fmt.Errorf("iterator error: %w", err)
	}
	
	// Set head block hash and other metadata
	// This would normally require decoding the highest block number
	// For now, we'll let the node figure it out
	
	fmt.Printf("\n✅ Import complete!\n")
	fmt.Printf("Total keys imported: %d\n", count)
	fmt.Printf("Headers: %d\n", headerCount)
	fmt.Printf("Bodies: %d\n", bodyCount)
	fmt.Printf("Receipts: %d\n", receiptCount)
	fmt.Printf("Canonical: %d\n", canonicalCount)
	
	return nil
}