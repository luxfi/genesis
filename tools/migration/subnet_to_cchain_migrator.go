package main

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/cockroachdb/pebble"
	badger "github.com/dgraph-io/badger/v4"
)

const (
	// SubnetEVM namespace from the logs
	namespace = "337fb73f9bcdac8c31a2d5f7b877ab1e8a2b7f2a1e9bf02a0a0e6c6fd164f1d1"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Println("Usage: migrate_subnet_to_cchain <source_pebbledb> <target_badgerdb>")
		os.Exit(1)
	}

	sourcePath := os.Args[1]
	targetPath := os.Args[2]

	log.Printf("Starting migration from %s to %s", sourcePath, targetPath)
	
	// Open PebbleDB source
	pebbleOpts := &pebble.Options{
		ReadOnly: true,
		Logger:   pebble.DefaultLogger,
	}
	
	sourceDB, err := pebble.Open(sourcePath, pebbleOpts)
	if err != nil {
		log.Fatalf("Failed to open source PebbleDB: %v", err)
	}
	defer sourceDB.Close()

	// Create BadgerDB target
	os.RemoveAll(targetPath)
	os.MkdirAll(targetPath, 0755)
	
	badgerOpts := badger.DefaultOptions(targetPath)
	badgerOpts.Logger = nil // Disable logs for speed
	badgerOpts.SyncWrites = false
	badgerOpts.NumVersionsToKeep = 1
	badgerOpts.NumCompactors = 4
	badgerOpts.BlockCacheSize = 256 << 20 // 256 MB
	
	targetDB, err := badger.Open(badgerOpts)
	if err != nil {
		log.Fatalf("Failed to open target BadgerDB: %v", err)
	}
	defer targetDB.Close()

	// Decode namespace
	namespaceBytes, err := hex.DecodeString(namespace)
	if err != nil {
		log.Fatalf("Failed to decode namespace: %v", err)
	}
	
	// Migration statistics
	var totalKeys, migratedKeys, skippedKeys int64
	startTime := time.Now()
	lastReport := time.Now()
	
	// Create iterator for PebbleDB
	iter := sourceDB.NewIter(nil)
	defer iter.Close()
	
	// Batch for BadgerDB writes
	batch := targetDB.NewWriteBatch()
	batchSize := 0
	const maxBatchSize = 10000
	
	log.Println("Starting key migration...")
	
	for iter.First(); iter.Valid(); iter.Next() {
		totalKeys++
		
		key := iter.Key()
		value := iter.Value()
		
		// Check if key has namespace prefix
		var newKey []byte
		if bytes.HasPrefix(key, namespaceBytes) {
			// Remove namespace prefix
			newKey = key[len(namespaceBytes):]
			
			// Special handling for certain key types
			if len(newKey) > 0 {
				switch newKey[0] {
				case 'h': // header
					// Headers are stored with prefix 'h' + block number (8 bytes) + block hash (32 bytes)
					// Keep as-is for C-Chain
				case 'b': // body
					// Block bodies: 'b' + block number (8 bytes) + block hash (32 bytes)
					// Keep as-is
				case 'r': // receipts
					// Receipts: 'r' + block number (8 bytes) + block hash (32 bytes)
					// Keep as-is
				case 'H': // canonical hash
					// Canonical hash: 'H' + block number (8 bytes)
					// Keep as-is
				case 'n': // account trie node
					// State trie nodes - keep as-is
				case 'c': // code
					// Contract code - keep as-is
				default:
					// All other keys - keep as-is
				}
			}
		} else {
			// Key doesn't have namespace, copy as-is
			newKey = make([]byte, len(key))
			copy(newKey, key)
		}
		
		// Copy value
		valueCopy := make([]byte, len(value))
		copy(valueCopy, value)
		
		// Write to batch
		err = batch.Set(newKey, valueCopy)
		if err != nil {
			log.Printf("Error writing key: %v", err)
			skippedKeys++
			continue
		}
		
		migratedKeys++
		batchSize++
		
		// Flush batch periodically
		if batchSize >= maxBatchSize {
			err = batch.Flush()
			if err != nil {
				log.Fatalf("Failed to flush batch: %v", err)
			}
			batch = targetDB.NewWriteBatch()
			batchSize = 0
		}
		
		// Progress report every 5 seconds
		if time.Since(lastReport) > 5*time.Second {
			elapsed := time.Since(startTime)
			keysPerSec := float64(migratedKeys) / elapsed.Seconds()
			log.Printf("Progress: %d/%d keys migrated (%.0f keys/sec, skipped: %d)",
				migratedKeys, totalKeys, keysPerSec, skippedKeys)
			lastReport = time.Now()
		}
	}
	
	// Flush final batch
	if batchSize > 0 {
		err = batch.Flush()
		if err != nil {
			log.Fatalf("Failed to flush final batch: %v", err)
		}
	}
	
	// Check for iterator errors
	if err := iter.Error(); err != nil {
		log.Fatalf("Iterator error: %v", err)
	}
	
	// Final statistics
	elapsed := time.Since(startTime)
	log.Printf("\n=== Migration Complete ===")
	log.Printf("Total keys processed: %d", totalKeys)
	log.Printf("Keys migrated: %d", migratedKeys)
	log.Printf("Keys skipped: %d", skippedKeys)
	log.Printf("Time elapsed: %v", elapsed)
	log.Printf("Average speed: %.0f keys/sec", float64(migratedKeys)/elapsed.Seconds())
	
	// Sync BadgerDB to ensure all data is persisted
	log.Println("Syncing BadgerDB...")
	err = targetDB.Sync()
	if err != nil {
		log.Printf("Warning: sync failed: %v", err)
	}
	
	log.Println("Migration completed successfully!")
}