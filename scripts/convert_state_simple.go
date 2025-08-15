package main

import (
    "fmt"
    "log"
    "path/filepath"
    "time"
    
    "github.com/cockroachdb/pebble"
    "github.com/luxfi/database/badgerdb"
)

func main() {
    srcPath := "/Users/z/work/lux/state/chaindata/lux-mainnet-96369/db/pebbledb"
    dstPath := "/Users/z/.luxd/network-96369/chains/EWi9aPkGe6EfJ3SobCAmSUXRPLa4brF3cThwPwmHTrD1y13jy/ethdb"
    
    fmt.Println("╔════════════════════════════════════════════════════════╗")
    fmt.Println("║        Simple State Conversion: PebbleDB → BadgerDB      ║")
    fmt.Println("╚════════════════════════════════════════════════════════╝")
    fmt.Println()
    fmt.Printf("Source: %s\n", srcPath)
    fmt.Printf("Dest:   %s\n", dstPath)
    fmt.Println()
    
    // Open source PebbleDB (read-only)
    src, err := pebble.Open(filepath.Clean(srcPath), &pebble.Options{ReadOnly: true})
    if err != nil {
        log.Fatalf("open pebble: %v", err)
    }
    defer src.Close()
    
    // Open destination BadgerDB
    dst, err := badgerdb.New(filepath.Clean(dstPath), nil, "", nil)
    if err != nil {
        log.Fatalf("open badger: %v", err)
    }
    defer dst.Close()
    
    startTime := time.Now()
    
    // Copy all state data (accounts, storage, code)
    fmt.Println("📝 Copying state data...")
    
    // Process each prefix type
    prefixes := []struct{
        prefix byte
        name string
    }{
        {0x00, "accounts"},
        {0x01, "storage"},
        {0x02, "code"},
    }
    
    for _, p := range prefixes {
        fmt.Printf("\nProcessing %s (prefix 0x%02x)...\n", p.name, p.prefix)
        
        lowerBound := []byte{p.prefix}
        upperBound := []byte{p.prefix + 1}
        
        it, err := src.NewIter(&pebble.IterOptions{
            LowerBound: lowerBound,
            UpperBound: upperBound,
        })
        if err != nil {
            log.Fatalf("create iterator: %v", err)
        }
        
        batch := dst.NewBatch()
        count := 0
        bytes := 0
        
        for it.First(); it.Valid(); it.Next() {
            key := append([]byte{}, it.Key()...)
            val := append([]byte{}, it.Value()...)
            
            // Direct copy
            batch.Put(key, val)
            
            count++
            bytes += len(key) + len(val)
            
            // Flush batch periodically
            if batch.Size() > 10*1024*1024 { // 10MB batches
                if err := batch.Write(); err != nil {
                    log.Fatalf("write batch: %v", err)
                }
                batch.Reset()
                batch = dst.NewBatch()
            }
            
            if count%100000 == 0 {
                fmt.Printf("  Processed %d entries (%.2f MB)\n", count, float64(bytes)/(1024*1024))
            }
        }
        
        // Final batch write
        if batch.Size() > 0 {
            if err := batch.Write(); err != nil {
                log.Fatalf("write final batch: %v", err)
            }
        }
        
        it.Close()
        fmt.Printf("  ✓ Copied %d %s entries (%.2f MB)\n", count, p.name, float64(bytes)/(1024*1024))
    }
    
    // Also copy any trie nodes that might exist
    fmt.Println("\n📊 Copying trie nodes (0x03-0x09)...")
    totalNodes := 0
    totalBytes := 0
    
    for prefix := byte(0x03); prefix <= 0x09; prefix++ {
        it, err := src.NewIter(&pebble.IterOptions{
            LowerBound: []byte{prefix},
            UpperBound: []byte{prefix + 1},
        })
        if err != nil {
            continue
        }
        
        batch := dst.NewBatch()
        count := 0
        
        for it.First(); it.Valid(); it.Next() {
            key := append([]byte{}, it.Key()...)
            val := append([]byte{}, it.Value()...)
            
            batch.Put(key, val)
            count++
            totalNodes++
            totalBytes += len(key) + len(val)
            
            if batch.Size() > 10*1024*1024 {
                batch.Write()
                batch.Reset()
                batch = dst.NewBatch()
            }
        }
        
        if batch.Size() > 0 {
            batch.Write()
        }
        
        it.Close()
        if count > 0 {
            fmt.Printf("  Prefix 0x%02x: %d nodes\n", prefix, count)
        }
    }
    
    elapsed := time.Since(startTime)
    
    fmt.Println("\n✅ State conversion complete!")
    fmt.Printf("  Total time: %s\n", elapsed.Round(time.Second))
    fmt.Printf("  Trie nodes: %d (%.2f MB)\n", totalNodes, float64(totalBytes)/(1024*1024))
    fmt.Println("\n🚀 The state data has been copied to the destination.")
    fmt.Println("However, this is still in path-scheme format.")
    fmt.Println("To convert to hash-scheme, you'll need to run the rehash tool.")
}