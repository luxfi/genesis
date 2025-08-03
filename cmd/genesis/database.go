package main

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"log"

	"github.com/luxfi/database/pebbledb"
	"github.com/spf13/cobra"
)

func getDatabaseCmd() *cobra.Command {
	dbCmd := &cobra.Command{
		Use:   "database",
		Short: "Database management commands",
		Long:  "Commands for managing and inspecting blockchain databases",
	}

	// Write height command
	writeHeightCmd := &cobra.Command{
		Use:   "write-height [db-path] [height]",
		Short: "Write a Height key to the database",
		Args:  cobra.ExactArgs(2),
		Run:   runWriteHeight,
	}

	// Get canonical hash command
	getCanonicalCmd := &cobra.Command{
		Use:   "get-canonical [db-path] [height]",
		Short: "Get the canonical block hash at a specific height",
		Args:  cobra.ExactArgs(2),
		Run:   runGetCanonical,
	}

	// Check database status command
	checkStatusCmd := &cobra.Command{
		Use:   "status [db-path]",
		Short: "Check database status and statistics",
		Args:  cobra.ExactArgs(1),
		Run:   runCheckStatus,
	}

	// Prepare for migration command
	prepareMigrationCmd := &cobra.Command{
		Use:   "prepare-migration [db-path] [height]",
		Short: "Prepare database for LUX_GENESIS migration",
		Args:  cobra.ExactArgs(2),
		Run:   runPrepareMigration,
	}

	dbCmd.AddCommand(writeHeightCmd, getCanonicalCmd, checkStatusCmd, prepareMigrationCmd)
	return dbCmd
}

func runWriteHeight(cmd *cobra.Command, args []string) {
	dbPath := args[0]
	var height uint64
	if _, err := fmt.Sscanf(args[1], "%d", &height); err != nil {
		log.Fatalf("Invalid height: %s", args[1])
	}

	// Open database using pebbledb
	db, err := pebbledb.New(dbPath, 0, 0, "", false)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Write Height key
	heightBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(heightBytes, height)
	
	if err := db.Put([]byte("Height"), heightBytes); err != nil {
		log.Fatalf("Failed to write Height key: %v", err)
	}
	
	fmt.Printf("Successfully wrote Height=%d to database\n", height)
	
	// Verify it was written
	if val, err := db.Get([]byte("Height")); err == nil {
		verifiedHeight := binary.BigEndian.Uint64(val)
		fmt.Printf("Verified: Height=%d\n", verifiedHeight)
	}
}

func runGetCanonical(cmd *cobra.Command, args []string) {
	dbPath := args[0]
	var height uint64
	if _, err := fmt.Sscanf(args[1], "%d", &height); err != nil {
		log.Fatalf("Invalid height: %s", args[1])
	}

	// Open database
	db, err := pebbledb.New(dbPath, 0, 0, "", false)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Check for canonical hash at specified height
	canonicalKey := make([]byte, 10)
	canonicalKey[0] = 'h'
	binary.BigEndian.PutUint64(canonicalKey[1:9], height)
	canonicalKey[9] = 'n'
	
	fmt.Printf("Looking for canonical key: %s\n", hex.EncodeToString(canonicalKey))
	if hashBytes, err := db.Get(canonicalKey); err == nil {
		hashHex := hex.EncodeToString(hashBytes)
		fmt.Printf("Found canonical hash at %d: %s\n", height, hashHex)
		fmt.Printf("\nTo export for LUX_GENESIS:\n")
		fmt.Printf("export LUX_IMPORTED_HEIGHT=%d\n", height)
		fmt.Printf("export LUX_IMPORTED_BLOCK_ID=%s\n", hashHex)
	} else {
		fmt.Printf("Canonical hash at %d not found: %v\n", height, err)
		
		// Try 9-byte format
		canonicalKey9 := make([]byte, 9)
		canonicalKey9[0] = 'h'
		binary.BigEndian.PutUint64(canonicalKey9[1:], height)
		
		fmt.Printf("\nTrying 9-byte format: %s\n", hex.EncodeToString(canonicalKey9))
		if hashBytes, err := db.Get(canonicalKey9); err == nil {
			hashHex := hex.EncodeToString(hashBytes)
			fmt.Printf("Found canonical hash with 9-byte key: %s\n", hashHex)
			fmt.Printf("\nTo export for LUX_GENESIS:\n")
			fmt.Printf("export LUX_IMPORTED_HEIGHT=%d\n", height)
			fmt.Printf("export LUX_IMPORTED_BLOCK_ID=%s\n", hashHex)
		}
	}
}

func runCheckStatus(cmd *cobra.Command, args []string) {
	dbPath := args[0]

	// Open database
	db, err := pebbledb.New(dbPath, 0, 0, "", false)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	fmt.Printf("Database Status for: %s\n", dbPath)
	fmt.Println("=====================================")

	// Check for Height key
	if heightBytes, err := db.Get([]byte("Height")); err == nil {
		height := binary.BigEndian.Uint64(heightBytes)
		fmt.Printf("Height key: %d\n", height)
	} else {
		fmt.Printf("Height key: NOT FOUND\n")
	}

	// Scan for canonical blocks
	iter := db.NewIterator()
	defer iter.Release()

	canonicalCount := 0
	minHeight := uint64(^uint64(0))
	maxHeight := uint64(0)
	
	for iter.Next() {
		key := iter.Key()
		if len(key) == 10 && key[0] == 'h' && key[9] == 'n' {
			blockNum := binary.BigEndian.Uint64(key[1:9])
			canonicalCount++
			if blockNum < minHeight {
				minHeight = blockNum
			}
			if blockNum > maxHeight {
				maxHeight = blockNum
			}
		}
	}

	fmt.Printf("\nCanonical blocks found: %d\n", canonicalCount)
	if canonicalCount > 0 {
		fmt.Printf("Block range: %d - %d\n", minHeight, maxHeight)
		
		// Show last few blocks
		fmt.Println("\nLast 5 blocks:")
		for i := maxHeight; i > maxHeight-5 && i >= minHeight; i-- {
			canonicalKey := make([]byte, 10)
			canonicalKey[0] = 'h'
			binary.BigEndian.PutUint64(canonicalKey[1:9], i)
			canonicalKey[9] = 'n'
			
			if hashBytes, err := db.Get(canonicalKey); err == nil {
				fmt.Printf("  Block %d: %s\n", i, hex.EncodeToString(hashBytes))
			}
		}
	}

	// Check for other key types
	fmt.Println("\nKey type statistics:")
	keyTypes := make(map[byte]int)
	iter2 := db.NewIterator()
	defer iter2.Release()
	
	totalKeys := 0
	for iter2.Next() {
		if totalKeys < 100000 { // Sample first 100k keys
			key := iter2.Key()
			if len(key) > 0 {
				keyTypes[key[0]]++
			}
			totalKeys++
		}
	}

	for prefix, count := range keyTypes {
		fmt.Printf("  Prefix '%c' (0x%02x): %d keys\n", prefix, prefix, count)
	}
	fmt.Printf("  Total keys sampled: %d\n", totalKeys)
}

func runPrepareMigration(cmd *cobra.Command, args []string) {
	dbPath := args[0]
	var height uint64
	if _, err := fmt.Sscanf(args[1], "%d", &height); err != nil {
		log.Fatalf("Invalid height: %s", args[1])
	}

	// Open database
	db, err := pebbledb.New(dbPath, 0, 0, "", false)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	fmt.Printf("Preparing database for LUX_GENESIS migration at height %d...\n", height)

	// Step 1: Write Height key
	heightBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(heightBytes, height)
	
	if err := db.Put([]byte("Height"), heightBytes); err != nil {
		log.Fatalf("Failed to write Height key: %v", err)
	}
	fmt.Printf("✓ Wrote Height=%d\n", height)

	// Step 2: Get canonical hash at that height
	canonicalKey := make([]byte, 10)
	canonicalKey[0] = 'h'
	binary.BigEndian.PutUint64(canonicalKey[1:9], height)
	canonicalKey[9] = 'n'
	
	var blockHash string
	if hashBytes, err := db.Get(canonicalKey); err == nil {
		blockHash = hex.EncodeToString(hashBytes)
		fmt.Printf("✓ Found canonical hash: %s\n", blockHash)
	} else {
		// Try 9-byte format
		canonicalKey9 := make([]byte, 9)
		canonicalKey9[0] = 'h'
		binary.BigEndian.PutUint64(canonicalKey9[1:], height)
		
		if hashBytes, err := db.Get(canonicalKey9); err == nil {
			blockHash = hex.EncodeToString(hashBytes)
			fmt.Printf("✓ Found canonical hash (9-byte format): %s\n", blockHash)
		} else {
			log.Fatalf("Could not find canonical hash at height %d", height)
		}
	}

	// Step 3: Generate launch script snippet
	fmt.Println("\n=====================================")
	fmt.Println("Add to your launch script:")
	fmt.Println("=====================================")
	fmt.Printf("# Set environment for genesis replay\n")
	fmt.Printf("export LUX_GENESIS=1\n")
	fmt.Printf("export LUX_IMPORTED_HEIGHT=%d\n", height)
	fmt.Printf("export LUX_IMPORTED_BLOCK_ID=%s\n", blockHash)
	fmt.Println("=====================================")

	fmt.Println("\nDatabase is ready for migration!")
}