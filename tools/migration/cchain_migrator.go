package main

import (
    "fmt"
    "log"
    "os"
    "time"
    "encoding/hex"
    
    "github.com/dgraph-io/badger/v3"
    "github.com/cockroachdb/pebble"
)

// Key prefixes used in the database
var (
    // State trie related
    headerPrefix       = []byte("h") // headerPrefix + num (uint64 big endian) + hash -> header
    headerHashSuffix   = []byte("n") // headerPrefix + num (uint64 big endian) + headerHashSuffix -> hash
    headerNumberPrefix = []byte("H") // headerNumberPrefix + hash -> num (uint64 big endian)
    
    blockBodyPrefix     = []byte("b") // blockBodyPrefix + num (uint64 big endian) + hash -> block body
    blockReceiptsPrefix = []byte("r") // blockReceiptsPrefix + num (uint64 big endian) + hash -> receipts
    
    txLookupPrefix  = []byte("l") // txLookupPrefix + hash -> transaction/receipt lookup metadata
    
    // State trie nodes - CRITICAL for balance queries
    trieNodePrefix = []byte("t") // trieNodePrefix + hash -> trie node
    
    // Account snapshot 
    snapshotAccountPrefix = []byte("a") // snapshotAccountPrefix + hash -> account
    snapshotStoragePrefix = []byte("s") // snapshotStoragePrefix + account hash + storage hash -> storage
    
    // Code storage
    codePrefix = []byte("c") // codePrefix + code hash -> contract code
    
    // Metadata
    databaseVersionKey = []byte("DatabaseVersion")
    headHeaderKey      = []byte("LastHeader")
    headBlockKey       = []byte("LastBlock")
    headFastBlockKey   = []byte("LastFast")
    fastTrieProgressKey = []byte("TrieSync")
)

func main() {
    if len(os.Args) < 3 {
        fmt.Println("Usage: migrate_cchain_complete <source_pebbledb_path> <dest_badgerdb_path>")
        os.Exit(1)
    }
    
    sourcePath := os.Args[1]
    destPath := os.Args[2]
    
    log.Printf("Starting complete C-Chain migration")
    log.Printf("Source: %s", sourcePath)
    log.Printf("Destination: %s", destPath)
    
    // Open source PebbleDB
    pdb, err := pebble.Open(sourcePath, &pebble.Options{})
    if err != nil {
        log.Fatalf("Failed to open source PebbleDB: %v", err)
    }
    defer pdb.Close()
    
    // Create destination BadgerDB
    os.MkdirAll(destPath, 0755)
    
    opts := badger.DefaultOptions(destPath)
    opts.Logger = nil // Silence badger logs
    opts.SyncWrites = false // Faster writes during migration
    opts.CompactL0OnClose = false
    
    bdb, err := badger.Open(opts)
    if err != nil {
        log.Fatalf("Failed to open destination BadgerDB: %v", err)
    }
    defer bdb.Close()
    
    // Count total keys
    log.Println("Counting keys in source database...")
    totalKeys := 0
    iter, err := pdb.NewIter(nil)
    if err != nil {
        log.Fatalf("Failed to create iterator: %v", err)
    }
    for iter.First(); iter.Valid(); iter.Next() {
        totalKeys++
    }
    iter.Close()
    log.Printf("Total keys to migrate: %d", totalKeys)
    
    // Migrate all data
    log.Println("Starting migration of all keys...")
    
    copiedKeys := 0
    stateTrieKeys := 0
    headerKeys := 0
    bodyKeys := 0
    receiptKeys := 0
    codeKeys := 0
    snapshotKeys := 0
    otherKeys := 0
    
    startTime := time.Now()
    lastReport := time.Now()
    
    batch := bdb.NewWriteBatch()
    batchSize := 0
    maxBatchSize := 10000
    
    iter, err = pdb.NewIter(nil)
    if err != nil {
        log.Fatalf("Failed to create iterator: %v", err)
    }
    defer iter.Close()
    
    for iter.First(); iter.Valid(); iter.Next() {
        key := iter.Key()
        value := iter.Value()
        
        // Make copies since iterator reuses buffers
        keyCopy := make([]byte, len(key))
        copy(keyCopy, key)
        valueCopy := make([]byte, len(value))
        copy(valueCopy, value)
        
        // Categorize keys
        if len(keyCopy) > 0 {
            switch keyCopy[0] {
            case 't':
                stateTrieKeys++
            case 'h', 'H':
                headerKeys++
            case 'b':
                bodyKeys++
            case 'r':
                receiptKeys++
            case 'c':
                codeKeys++
            case 'a', 's':
                snapshotKeys++
            default:
                otherKeys++
            }
        }
        
        // Write to BadgerDB
        err := batch.Set(keyCopy, valueCopy)
        if err != nil {
            log.Printf("Error writing key: %v", err)
            continue
        }
        
        copiedKeys++
        batchSize++
        
        // Commit batch periodically
        if batchSize >= maxBatchSize {
            err = batch.Flush()
            if err != nil {
                log.Fatalf("Failed to flush batch: %v", err)
            }
            batch = bdb.NewWriteBatch()
            batchSize = 0
        }
        
        // Progress report every 5 seconds
        if time.Since(lastReport) > 5*time.Second {
            elapsed := time.Since(startTime)
            rate := float64(copiedKeys) / elapsed.Seconds()
            remaining := float64(totalKeys-copiedKeys) / rate
            
            log.Printf("Progress: %d/%d keys (%.1f%%) - Rate: %.0f keys/sec - ETA: %s",
                copiedKeys, totalKeys, 
                float64(copiedKeys)*100/float64(totalKeys),
                rate,
                time.Duration(remaining*float64(time.Second)).String())
            
            log.Printf("  State Trie: %d, Headers: %d, Bodies: %d, Receipts: %d, Code: %d, Snapshots: %d, Other: %d",
                stateTrieKeys, headerKeys, bodyKeys, receiptKeys, codeKeys, snapshotKeys, otherKeys)
            
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
    
    // Force sync to disk
    log.Println("Syncing to disk...")
    err = bdb.Sync()
    if err != nil {
        log.Printf("Warning: sync failed: %v", err)
    }
    
    elapsed := time.Since(startTime)
    log.Printf("Migration complete!")
    log.Printf("Total time: %s", elapsed)
    log.Printf("Keys migrated: %d", copiedKeys)
    log.Printf("Average rate: %.0f keys/sec", float64(copiedKeys)/elapsed.Seconds())
    log.Printf("\nKey breakdown:")
    log.Printf("  State Trie Nodes: %d", stateTrieKeys)
    log.Printf("  Headers: %d", headerKeys)
    log.Printf("  Block Bodies: %d", bodyKeys)
    log.Printf("  Receipts: %d", receiptKeys)
    log.Printf("  Contract Code: %d", codeKeys)
    log.Printf("  Snapshots: %d", snapshotKeys)
    log.Printf("  Other: %d", otherKeys)
    
    // Verify we can read back some key data
    log.Println("\nVerifying migration...")
    
    // Check if we have the head block
    err = bdb.View(func(txn *badger.Txn) error {
        item, err := txn.Get(headBlockKey)
        if err == nil {
            var value []byte
            value, err = item.ValueCopy(nil)
            if err == nil {
                log.Printf("✓ Head block found: %s", hex.EncodeToString(value))
            }
        }
        
        // Count state trie nodes
        opts := badger.DefaultIteratorOptions
        opts.Prefix = trieNodePrefix
        iter := txn.NewIterator(opts)
        defer iter.Close()
        
        count := 0
        for iter.Rewind(); iter.Valid(); iter.Next() {
            count++
            if count > 1000000 {
                break // Don't count forever
            }
        }
        log.Printf("✓ State trie nodes in destination: %d%s", count, func() string {
            if count > 1000000 {
                return "+"
            }
            return ""
        }())
        
        return nil
    })
    
    if err != nil {
        log.Printf("Verification error: %v", err)
    }
    
    log.Println("\nMigration completed successfully!")
    log.Printf("Destination database ready at: %s", destPath)
}