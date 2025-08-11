package main

import (
	"fmt"
	"log"
	"math/big"
	"path/filepath"

	"github.com/luxfi/geth/common"
	"github.com/luxfi/geth/core/rawdb"
	"github.com/luxfi/geth/ethdb"
	"github.com/luxfi/database/manager"
)

func main() {
	// Open the PHYSICAL ethdb path the VM uses
	ethdbPath := filepath.Clean(filepath.Join(
		"/home/z/.luxd", "network-96369", "chains",
		"X6CU5qgMJfzsTB9UWxj2ZY5hd68x41HfZ4m4hCBWbHuj1Ebc3", "ethdb",
	))
	
	fmt.Printf("=== Repairing Total Difficulty ===\n")
	fmt.Printf("Database: %s\n\n", ethdbPath)
	
	// Open using the Lux database manager which properly handles BadgerDB
	// The VM uses prefixdb wrapper, but we need direct access
	dbManager, err := manager.NewRocksDB(ethdbPath, nil, "", nil, nil)
	if err != nil {
		// Try BadgerDB if RocksDB fails
		dbManager, err = manager.NewMemDB(nil)
		if err != nil {
			log.Fatalf("Failed to open database: %v", err)
		}
		
		// Actually, let's use the PebbleDB since that's what the migrated data uses
		fmt.Printf("Opening as PebbleDB...\n")
		db, err := rawdb.NewPebbleDBDatabase(ethdbPath, 0, 0, "", false)
		if err != nil {
			log.Fatalf("Failed to open PebbleDB: %v", err)
		}
		defer db.Close()
		
		repairTD(db)
		return
	}
	defer dbManager.Close()
	
	// Wrap in ethdb interface
	var db ethdb.Database = dbManager
	repairTD(db)
}

func repairTD(db ethdb.Database) {
	// First, find the tip by checking what we know
	tip := uint64(1082780) // We know this from the migration
	
	// Verify we have canonical hash at tip
	tipHash := rawdb.ReadCanonicalHash(db, tip)
	if tipHash == (common.Hash{}) {
		fmt.Printf("No canonical hash at known tip %d, scanning...\n", tip)
		// Scan to find actual tip
		for n := uint64(0); n <= 2000000; n++ {
			if n%100000 == 0 && n > 0 {
				fmt.Printf("  Scanned up to block %d...\n", n)
			}
			h := rawdb.ReadCanonicalHash(db, n)
			if h != (common.Hash{}) {
				tip = n // Update tip to last found
			} else if n > tip {
				break // Stop if we've gone past expected tip
			}
		}
	}
	
	fmt.Printf("Tip found at: %d (hash: %s)\n", tip, tipHash.Hex())
	
	// Write TD for each canonical block: TD = height + 1
	fmt.Printf("\nWriting TD for blocks 0 to %d...\n", tip)
	
	batch := db.NewBatch()
	written := 0
	missing := 0
	
	for n := uint64(0); n <= tip; n++ {
		h := rawdb.ReadCanonicalHash(db, n)
		if h == (common.Hash{}) {
			missing++
			if n == 0 {
				fmt.Printf("  ⚠️  Genesis canonical hash missing\n")
			}
			continue
		}
		
		// Check if header exists
		if hdr := rawdb.ReadHeader(db, h, n); hdr == nil {
			fmt.Printf("  ⚠️  Missing header at block %d\n", n)
			missing++
			continue
		}
		
		// TD = height + 1 for this chain
		td := new(big.Int).SetUint64(n + 1)
		
		// Write TD using rawdb (it handles the encoding)
		rawdb.WriteTd(batch, h, n, td)
		written++
		
		// Commit batch periodically
		if written%10000 == 0 {
			fmt.Printf("  TD written for %d blocks...\n", written)
			if err := batch.Write(); err != nil {
				log.Fatalf("Failed to write batch: %v", err)
			}
			batch.Reset()
		}
	}
	
	// Write final batch
	if err := batch.Write(); err != nil {
		log.Fatalf("Failed to write final batch: %v", err)
	}
	
	fmt.Printf("✅ TD written for %d blocks\n", written)
	if missing > 0 {
		fmt.Printf("⚠️  %d blocks were missing canonical hash or header\n", missing)
	}
	
	// Set heads to the canonical tip
	if tipHash == (common.Hash{}) {
		tipHash = rawdb.ReadCanonicalHash(db, tip)
	}
	
	if tipHash != (common.Hash{}) {
		fmt.Printf("\nSetting head pointers to block %d (hash: %s)...\n", tip, tipHash.Hex())
		rawdb.WriteHeadHeaderHash(db, tipHash)
		rawdb.WriteHeadBlockHash(db, tipHash)
		rawdb.WriteHeadFastBlockHash(db, tipHash)
		fmt.Printf("✅ Head pointers set\n")
	} else {
		fmt.Printf("❌ Could not find canonical hash for tip!\n")
	}
	
	// Verify TD at key heights
	fmt.Printf("\nVerifying TD...\n")
	
	// Check genesis
	if h := rawdb.ReadCanonicalHash(db, 0); h != (common.Hash{}) {
		if td := rawdb.ReadTd(db, h, 0); td != nil {
			fmt.Printf("  Genesis TD: %v (expected 1)\n", td)
		}
	}
	
	// Check tip
	if tipHash != (common.Hash{}) {
		if td := rawdb.ReadTd(db, tipHash, tip); td != nil {
			expectedTD := new(big.Int).SetUint64(tip + 1)
			if td.Cmp(expectedTD) == 0 {
				fmt.Printf("  Tip TD: %v ✅\n", td)
			} else {
				fmt.Printf("  Tip TD: %v (expected %v) ⚠️\n", td, expectedTD)
			}
		} else {
			fmt.Printf("  Tip TD: NOT FOUND ❌\n")
		}
	}
	
	fmt.Printf("\n✅ Done. Heads -> %s at #%d\n", tipHash.Hex(), tip)
}