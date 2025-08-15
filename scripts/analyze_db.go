package main

import (
    "bytes"
    "encoding/binary"
    "fmt"
    "log"
    
    "github.com/cockroachdb/pebble"
)

func main() {
    sourcePath := "/Users/z/work/lux/state/chaindata/lux-mainnet-96369/db/pebbledb"
    
    fmt.Println("Analyzing Database Structure")
    fmt.Println("============================")
    
    // Open source PebbleDB
    db, err := pebble.Open(sourcePath, &pebble.Options{
        ReadOnly: true,
    })
    if err != nil {
        log.Fatal("Failed to open database:", err)
    }
    defer db.Close()
    
    // SubnetEVM namespace prefix (32 bytes)
    subnetNamespace := []byte{
        0x33, 0x7f, 0xb7, 0x3f, 0x9b, 0xcd, 0xac, 0x8c,
        0x31, 0xa2, 0xd5, 0xf7, 0xb8, 0x77, 0xab, 0x1e,
        0x8a, 0x2b, 0x7f, 0x2a, 0x1e, 0x9b, 0xf0, 0x2a,
        0x0a, 0x0e, 0x6c, 0x6f, 0xd1, 0x64, 0xf1, 0xd1,
    }
    
    // Count different key types
    keyCounts := make(map[string]int)
    samples := make(map[string][]byte)
    totalKeys := 0
    
    iter, err := db.NewIter(nil)
    if err != nil {
        log.Fatal("Failed to create iterator:", err)
    }
    defer iter.Close()
    
    for iter.First(); iter.Valid() && totalKeys < 10000; iter.Next() {
        key := iter.Key()
        totalKeys++
        
        keyType := "unknown"
        
        // Check if it's a namespaced key
        if len(key) == 64 && bytes.HasPrefix(key, subnetNamespace) {
            actualKey := key[32:]
            
            // Categorize by key pattern
            if len(actualKey) == 9 && actualKey[0] == 'H' {
                keyType = "canonical-hash"
                if keyCounts[keyType] == 0 {
                    samples[keyType] = key
                }
            } else if len(actualKey) == 33 && actualKey[0] == 'h' {
                keyType = "header"
                if keyCounts[keyType] == 0 {
                    samples[keyType] = key
                }
            } else if len(actualKey) == 33 && actualKey[0] == 'b' {
                keyType = "body"
            } else if len(actualKey) == 33 && actualKey[0] == 'r' {
                keyType = "receipt"
            } else if len(actualKey) == 32 {
                keyType = "hash-key"
            } else {
                keyType = fmt.Sprintf("namespaced-%d", len(actualKey))
            }
        } else if len(key) == 41 {
            // Non-namespaced keys with prefix
            if key[0] == 'h' {
                keyType = "header-direct"
                if keyCounts[keyType] == 0 {
                    samples[keyType] = key
                }
            } else if key[0] == 'b' {
                keyType = "body-direct"
            } else if key[0] == 'r' {
                keyType = "receipt-direct"
            } else {
                keyType = fmt.Sprintf("prefixed-%d", len(key))
            }
        } else if len(key) == 9 && key[0] == 'H' {
            keyType = "canonical-direct"
            if keyCounts[keyType] == 0 {
                samples[keyType] = key
                // Decode the block number
                blockNum := binary.BigEndian.Uint64(key[1:])
                fmt.Printf("  Found canonical block %d\n", blockNum)
            }
        } else {
            keyType = fmt.Sprintf("length-%d", len(key))
        }
        
        keyCounts[keyType]++
    }
    
    fmt.Printf("\nKey Type Distribution (first %d keys):\n", totalKeys)
    fmt.Println("--------------------------------------")
    for keyType, count := range keyCounts {
        fmt.Printf("%-20s: %d\n", keyType, count)
        
        // Show sample key for debugging
        if sample, ok := samples[keyType]; ok {
            fmt.Printf("  Sample: %x\n", sample[:min(len(sample), 40)])
        }
    }
    
    // Look specifically for canonical hashes
    fmt.Println("\nSearching for canonical block hashes...")
    
    // Try block 0
    key0 := make([]byte, 9)
    key0[0] = 'H'
    binary.BigEndian.PutUint64(key0[1:], 0)
    
    val, closer, err := db.Get(key0)
    if err == nil {
        defer closer.Close()
        fmt.Printf("Block 0 canonical hash: %x\n", val)
    } else {
        fmt.Printf("Block 0 not found with key %x: %v\n", key0, err)
    }
    
    // Try block 1000000
    key1M := make([]byte, 9)
    key1M[0] = 'H'
    binary.BigEndian.PutUint64(key1M[1:], 1000000)
    
    val2, closer2, err := db.Get(key1M)
    if err == nil {
        defer closer2.Close()
        fmt.Printf("Block 1000000 canonical hash: %x\n", val2)
    } else {
        fmt.Printf("Block 1000000 not found with key %x: %v\n", key1M, err)
    }
}

func min(a, b int) int {
    if a < b {
        return a
    }
    return b
}