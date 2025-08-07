package main

import (
    "bytes"
    "encoding/hex"
    "fmt"
    "github.com/cockroachdb/pebble"
)

func main() {
    db, _ := pebble.Open("/home/z/work/lux/state/chaindata/lux-mainnet-96369/db/pebbledb", &pebble.Options{ReadOnly: true})
    defer db.Close()
    
    namespace := []byte{
        0x33, 0x7f, 0xb7, 0x3f, 0x9b, 0xcd, 0xac, 0x8c,
        0x31, 0xa2, 0xd5, 0xf7, 0xb8, 0x77, 0xab, 0x1e,
        0x8a, 0x2b, 0x7f, 0x2a, 0x1e, 0x9b, 0xf0, 0x2a,
        0x0a, 0x0e, 0x6c, 0x6f, 0xd1, 0x64, 0xf1, 0xd1,
    }
    
    fmt.Println("Listing unique RLP blocks")
    fmt.Println("=========================")
    
    // Properly copy keys
    headers := []struct{
        hash []byte
        size int
    }{}
    
    iter, _ := db.NewIter(&pebble.IterOptions{})
    defer iter.Close()
    
    count := 0
    for iter.First(); iter.Valid() && count < 50; iter.Next() {
        key := iter.Key()
        value := iter.Value()
        
        // Check for RLP headers at namespace+32bytes
        if len(key) == 64 && bytes.HasPrefix(key, namespace) {
            if len(value) > 200 && (value[0] == 0xf8 || value[0] == 0xf9) {
                // Make a copy of the hash
                hash := make([]byte, 32)
                copy(hash, key[32:])
                
                headers = append(headers, struct{
                    hash []byte
                    size int
                }{hash, len(value)})
                
                count++
            }
        }
    }
    
    fmt.Printf("Found %d RLP headers\n\n", len(headers))
    
    // Display unique hashes
    fmt.Println("First 20 unique block hashes:")
    for i, h := range headers {
        if i >= 20 {
            break
        }
        
        hashStr := hex.EncodeToString(h.hash)
        fmt.Printf("%d. %s (size: %d)\n", i, hashStr, h.size)
        
        // Decode potential block number
        if len(h.hash) >= 3 {
            blockNum3 := uint64(h.hash[0])<<16 | uint64(h.hash[1])<<8 | uint64(h.hash[2])
            blockNum2 := uint64(h.hash[0])<<8 | uint64(h.hash[1])
            fmt.Printf("   Potential block numbers: 3-byte=%d, 2-byte=%d\n", blockNum3, blockNum2)
        }
    }
    
    // Try to load block 0 by trying different hash patterns
    fmt.Println("\nSearching for block 0...")
    
    // Generate potential hash for block 0
    for padLen := 1; padLen <= 8; padLen++ {
        testHash := make([]byte, 32)
        // Try block 0 encoded in first N bytes
        // All zeros for block 0
        
        testKey := append(namespace, testHash...)
        
        if val, closer, err := db.Get(testKey); err == nil {
            closer.Close()
            fmt.Printf("  Found with %d-byte padding! Size: %d\n", padLen, len(val))
            if len(val) > 0 && (val[0] == 0xf8 || val[0] == 0xf9) {
                fmt.Println("    ✓ This is an RLP header for block 0!")
                break
            }
        }
    }
    
    // Also check if there's a pattern in the collected headers
    fmt.Println("\nChecking for sequential patterns...")
    
    // Sort headers by interpreting first bytes as block number
    type blockHeader struct {
        blockNum uint64
        hash     []byte
        size     int
    }
    
    blocks := []blockHeader{}
    for _, h := range headers {
        if len(h.hash) >= 3 {
            // Try 3-byte block number
            blockNum := uint64(h.hash[0])<<16 | uint64(h.hash[1])<<8 | uint64(h.hash[2])
            if blockNum < 2000000 {
                blocks = append(blocks, blockHeader{blockNum, h.hash, h.size})
            }
        }
    }
    
    fmt.Printf("\nInterpreted block numbers from hashes:\n")
    for i, b := range blocks {
        if i >= 10 {
            break
        }
        fmt.Printf("  Block %d: hash=%s\n", b.blockNum, hex.EncodeToString(b.hash))
    }
}