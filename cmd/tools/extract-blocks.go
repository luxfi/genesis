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
        0x33, 0x7f, 0xb7, 0x3f, 0xb8, 0x77, 0xab, 0x1e,
        0x8a, 0x2b, 0x7f, 0x2a, 0x1e, 0x9b, 0xf0, 0x2a,
        0x0a, 0x0e, 0x6c, 0x6f, 0xd1, 0x64, 0xf1, 0xd1,
    }
    
    fmt.Println("Extracting blocks from database")
    fmt.Println("================================")
    
    // Collect all RLP headers with their interpreted block numbers
    blocks := make(map[uint64]struct {
        hash   []byte
        header []byte
    })
    
    iter, _ := db.NewIter(&pebble.IterOptions{})
    defer iter.Close()
    
    for iter.First(); iter.Valid(); iter.Next() {
        key := iter.Key()
        value := iter.Value()
        
        // Look for namespace + 32-byte hash
        if len(key) == 64 && bytes.HasPrefix(key, namespace) {
            hash := key[32:]
            
            // Check if value is RLP header
            if len(value) > 100 && (value[0] == 0xf8 || value[0] == 0xf9) {
                // Decode block number from hash
                // Try first 3 bytes as block number
                blockNum := uint64(0)
                if len(hash) >= 3 {
                    // Convert first 3 bytes to uint64
                    blockNum = uint64(hash[0])<<16 | uint64(hash[1])<<8 | uint64(hash[2])
                }
                
                // Store if reasonable block number
                if blockNum < 2000000 {
                    blocks[blockNum] = struct {
                        hash   []byte
                        header []byte
                    }{hash, value}
                }
            }
        }
    }
    
    fmt.Printf("Found %d blocks\n\n", len(blocks))
    
    // Display first 10 blocks
    for i := uint64(0); i < 10; i++ {
        if block, exists := blocks[i]; exists {
            fmt.Printf("Block %d:\n", i)
            fmt.Printf("  Hash: %s\n", hex.EncodeToString(block.hash))
            fmt.Printf("  Header size: %d bytes\n", len(block.header))
        } else {
            fmt.Printf("Block %d: NOT FOUND\n", i)
        }
    }
    
    // Check for gaps
    fmt.Println("\nChecking for continuity:")
    lastFound := uint64(0)
    gaps := 0
    for i := uint64(0); i < 1100000; i++ {
        if _, exists := blocks[i]; exists {
            if i > lastFound+1 {
                fmt.Printf("  Gap: blocks %d-%d missing\n", lastFound+1, i-1)
                gaps++
                if gaps > 10 {
                    break
                }
            }
            lastFound = i
        }
    }
    
    fmt.Printf("\nHighest block found: %d\n", lastFound)
}