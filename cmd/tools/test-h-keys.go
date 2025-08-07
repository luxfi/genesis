package main

import (
    "bytes"
    "encoding/binary"
    "encoding/hex"
    "fmt"
    "log"
    
    "github.com/cockroachdb/pebble"
)

const DB_PATH = "/home/z/work/lux/state/chaindata/lux-mainnet-96369/db/pebbledb"

func main() {
    db, err := pebble.Open(DB_PATH, &pebble.Options{ReadOnly: true})
    if err != nil {
        log.Fatal("Failed to open database:", err)
    }
    defer db.Close()
    
    fmt.Println("Searching for H keys (canonical mapping)...")
    
    // The namespace we found
    namespace := []byte{
        0x33, 0x7f, 0xb7, 0x3f, 0x9b, 0xcd, 0xac, 0x8c,
        0x31, 0xa2, 0xd5, 0xf7, 0xb8, 0x77, 0xab, 0x1e,
        0x8a, 0x2b, 0x7f, 0x2a, 0x1e, 0x9b, 0xf0, 0x2a,
        0x0a, 0x0e, 0x6c, 0x6f, 0xd1, 0x64, 0xf1, 0xd1,
    }
    
    // Try to find H keys with different formats
    
    // Test 1: namespace + 'H' + blocknum (41 bytes total)
    fmt.Println("\nTest 1: namespace + 'H' + blocknum")
    for i := uint64(0); i < 10; i++ {
        key := make([]byte, 41)
        copy(key, namespace)
        key[32] = 'H'
        binary.BigEndian.PutUint64(key[33:], i)
        
        value, closer, err := db.Get(key)
        if err == nil {
            closer.Close()
            fmt.Printf("  Found block %d: hash=%x\n", i, value)
        }
    }
    
    // Test 2: Just 'H' + blocknum (9 bytes, no namespace)
    fmt.Println("\nTest 2: 'H' + blocknum (no namespace)")
    for i := uint64(0); i < 10; i++ {
        key := make([]byte, 9)
        key[0] = 'H'
        binary.BigEndian.PutUint64(key[1:], i)
        
        value, closer, err := db.Get(key)
        if err == nil {
            closer.Close()
            fmt.Printf("  Found block %d: hash=%x\n", i, value)
        }
    }
    
    // Test 3: Scan for any keys starting with 'H'
    fmt.Println("\nTest 3: Scanning for 'H' prefix keys...")
    iter, _ := db.NewIter(&pebble.IterOptions{})
    defer iter.Close()
    
    hCount := 0
    minBlock := uint64(^uint64(0))
    maxBlock := uint64(0)
    
    for iter.First(); iter.Valid(); iter.Next() {
        key := iter.Key()
        
        // Check if key starts with 'H' (no namespace)
        if len(key) == 9 && key[0] == 'H' {
            blockNum := binary.BigEndian.Uint64(key[1:])
            if hCount < 5 {
                fmt.Printf("  Block %d: hash=%x\n", blockNum, iter.Value())
            }
            if blockNum < minBlock {
                minBlock = blockNum
            }
            if blockNum > maxBlock {
                maxBlock = blockNum
            }
            hCount++
        }
        
        // Also check with namespace
        if len(key) == 41 && bytes.Equal(key[:32], namespace) && key[32] == 'H' {
            blockNum := binary.BigEndian.Uint64(key[33:])
            if hCount < 5 {
                fmt.Printf("  [With namespace] Block %d: hash=%x\n", blockNum, iter.Value())
            }
            hCount++
        }
    }
    
    if hCount > 0 {
        fmt.Printf("\nFound %d 'H' canonical entries\n", hCount)
        fmt.Printf("Block range: %d to %d\n", minBlock, maxBlock)
        fmt.Printf("Expected: 1,082,781 blocks\n")
        
        // Try to access block 0 and its components
        fmt.Println("\nTrying to access block 0 components:")
        
        // Get block 0 hash
        key0 := make([]byte, 9)
        key0[0] = 'H'
        binary.BigEndian.PutUint64(key0[1:], 0)
        
        hash0, closer, err := db.Get(key0)
        if err == nil {
            closer.Close()
            fmt.Printf("Block 0 hash: %s\n", hex.EncodeToString(hash0))
            
            // Try to get header (h + hash)
            headerKey := append([]byte{'h'}, hash0...)
            headerData, closer, err := db.Get(headerKey)
            if err == nil {
                closer.Close()
                fmt.Printf("Header found: %d bytes\n", len(headerData))
            } else {
                // Try with namespace
                headerKeyNS := append(namespace, headerKey...)
                headerData, closer, err = db.Get(headerKeyNS)
                if err == nil {
                    closer.Close()
                    fmt.Printf("Header found (with namespace): %d bytes\n", len(headerData))
                } else {
                    fmt.Println("Header not found")
                }
            }
            
            // Try to get body (b + hash)
            bodyKey := append([]byte{'b'}, hash0...)
            bodyData, closer, err := db.Get(bodyKey)
            if err == nil {
                closer.Close()
                fmt.Printf("Body found: %d bytes\n", len(bodyData))
            } else {
                // Try with namespace
                bodyKeyNS := append(namespace, bodyKey...)
                bodyData, closer, err = db.Get(bodyKeyNS)
                if err == nil {
                    closer.Close()
                    fmt.Printf("Body found (with namespace): %d bytes\n", len(bodyData))
                } else {
                    fmt.Println("Body not found (likely empty block)")
                }
            }
        }
    } else {
        fmt.Println("No 'H' canonical blocks found")
    }
}