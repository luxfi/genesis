package main

import (
    "encoding/binary"
    "fmt"
    "github.com/cockroachdb/pebble"
)

func main() {
    db, _ := pebble.Open("/home/z/work/lux/state/chaindata/lux-mainnet-96369/db/pebbledb", &pebble.Options{ReadOnly: true})
    defer db.Close()
    
    iter, _ := db.NewIter(&pebble.IterOptions{})
    defer iter.Close()
    
    namespace := []byte{
        0x33, 0x7f, 0xb7, 0x3f, 0x9b, 0xcd, 0xac, 0x8c,
        0x31, 0xa2, 0xd5, 0xf7, 0xb8, 0x77, 0xab, 0x1e,
        0x8a, 0x2b, 0x7f, 0x2a, 0x1e, 0x9b, 0xf0, 0x2a,
        0x0a, 0x0e, 0x6c, 0x6f, 0xd1, 0x64, 0xf1, 0xd1,
    }
    
    fmt.Println("Looking for canonical 'H' keys...")
    fmt.Printf("Namespace: %x\n\n", namespace)
    
    count := 0
    hCount := 0
    minBlock := uint64(^uint64(0))
    maxBlock := uint64(0)
    
    // Try constructing keys directly
    fmt.Println("Method 1: Direct key construction (namespace + H + blocknum):")
    for i := uint64(0); i < 10; i++ {
        key := make([]byte, 41)
        copy(key, namespace)
        key[32] = 'H'
        binary.BigEndian.PutUint64(key[33:], i)
        
        value, closer, err := db.Get(key)
        if err == nil {
            closer.Close()
            fmt.Printf("  Block %d: hash=%x\n", i, value)
        }
    }
    
    // Also check different namespace possibilities
    fmt.Println("\nMethod 2: Different namespace (all zeros) + H + blocknum:")
    zeroNamespace := make([]byte, 32)
    for i := uint64(0); i < 10; i++ {
        key := make([]byte, 41)
        copy(key, zeroNamespace)
        key[32] = 'H'
        binary.BigEndian.PutUint64(key[33:], i)
        
        value, closer, err := db.Get(key)
        if err == nil {
            closer.Close()
            fmt.Printf("  Block %d: hash=%x\n", i, value)
        }
    }
    
    fmt.Println("\nMethod 3: Scanning for keys where byte[32] == 'H':")
    for iter.First(); iter.Valid() && count < 5000000; iter.Next() {
        key := iter.Key()
        
        // Check if this is namespace + 'H' + blocknum format
        if len(key) == 41 && key[32] == 'H' {
            blockNum := binary.BigEndian.Uint64(key[33:])
            if hCount < 10 {
                fmt.Printf("  Found: namespace=%x, block=%d, hash=%x\n", 
                    key[:32], blockNum, iter.Value())
            }
            
            if blockNum < minBlock {
                minBlock = blockNum
            }
            if blockNum > maxBlock {
                maxBlock = blockNum
            }
            hCount++
        }
        count++
    }
    
    fmt.Printf("\nScanned %d keys\n", count)
    fmt.Printf("Found %d canonical 'H' keys\n", hCount)
    if hCount > 0 {
        fmt.Printf("Block range: %d to %d\n", minBlock, maxBlock)
    }
}