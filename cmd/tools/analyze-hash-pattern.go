package main

import (
    "bytes"
    "encoding/binary"
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
    
    fmt.Println("Analyzing block hash patterns")
    fmt.Println("==============================")
    
    // Get RLP headers and analyze their hashes
    iter, _ := db.NewIter(&pebble.IterOptions{})
    defer iter.Close()
    
    headers := []struct {
        hash  []byte
        value []byte
    }{}
    
    count := 0
    for iter.First(); iter.Valid() && count < 20; iter.Next() {
        key := iter.Key()
        value := iter.Value()
        
        if len(key) == 64 && bytes.HasPrefix(key, namespace) && len(value) > 200 && value[0] == 0xf9 {
            hash := key[32:]
            headers = append(headers, struct {
                hash  []byte
                value []byte
            }{hash, value})
            count++
        }
    }
    
    fmt.Printf("Found %d header samples\n\n", len(headers))
    
    for i, h := range headers {
        fmt.Printf("Header %d:\n", i)
        fmt.Printf("  Hash: %s\n", hex.EncodeToString(h.hash))
        
        // Try different interpretations
        if len(h.hash) >= 8 {
            // First 8 bytes as uint64
            num1 := binary.BigEndian.Uint64(h.hash[0:8])
            fmt.Printf("  First 8 bytes as uint64: %d\n", num1)
            
            // First 4 bytes as uint32
            num2 := binary.BigEndian.Uint32(h.hash[0:4])
            fmt.Printf("  First 4 bytes as uint32: %d\n", num2)
            
            // Last 8 bytes as uint64
            num3 := binary.BigEndian.Uint64(h.hash[24:32])
            fmt.Printf("  Last 8 bytes as uint64: %d\n", num3)
        }
        
        // Check if hash contains ASCII
        hasAscii := false
        for _, b := range h.hash {
            if b >= 32 && b < 127 {
                hasAscii = true
                break
            }
        }
        if hasAscii {
            fmt.Printf("  Contains ASCII: %s\n", string(h.hash))
        }
        
        fmt.Printf("  RLP size: %d bytes\n", len(h.value))
        fmt.Println()
    }
    
    // Now let's check the H keys to understand the mapping
    fmt.Println("Checking H key mappings:")
    
    // Look for the first few H keys and see what they map to
    iter2, _ := db.NewIter(&pebble.IterOptions{})
    defer iter2.Close()
    
    hCount := 0
    for iter2.First(); iter2.Valid() && hCount < 10; iter2.Next() {
        key := iter2.Key()
        
        if len(key) == 65 && bytes.HasPrefix(key, namespace) && key[32] == 'H' {
            hash := key[33:]
            value := iter2.Value()
            
            fmt.Printf("\nH key %d:\n", hCount)
            fmt.Printf("  Hash in key: %x\n", hash)
            
            if len(value) == 8 {
                blockNum := binary.BigEndian.Uint64(value)
                fmt.Printf("  Maps to block: %d\n", blockNum)
                
                // Now try to find a header that corresponds to this block
                // We need to find the relationship between H-hash and actual block hash
            } else {
                fmt.Printf("  Value: %x\n", value)
            }
            
            hCount++
        }
    }
    
    // Try to correlate H mappings with headers
    fmt.Println("\n\nLooking for correlation between H keys and headers...")
    
    // Build H mapping
    hMapping := make(map[uint64][]byte)
    iter3, _ := db.NewIter(&pebble.IterOptions{})
    for iter3.First(); iter3.Valid(); iter3.Next() {
        key := iter3.Key()
        
        if len(key) == 65 && bytes.HasPrefix(key, namespace) && key[32] == 'H' {
            hash := key[33:]
            value := iter3.Value()
            
            if len(value) == 8 {
                blockNum := binary.BigEndian.Uint64(value)
                hMapping[blockNum] = hash
                
                if blockNum < 5 {
                    fmt.Printf("Block %d -> H-hash: %x\n", blockNum, hash)
                }
            }
        }
    }
    iter3.Close()
    
    fmt.Printf("\nTotal H mappings: %d\n", len(hMapping))
    
    // Now check if any headers have hashes that match a pattern for block 0
    fmt.Println("\nSearching for block 0 header...")
    
    block0HHash := hMapping[0]
    if block0HHash != nil {
        fmt.Printf("Block 0 H-hash: %x\n", block0HHash)
        
        // Try to find headers with various transformations of this hash
        // The H-hash might be related to the actual block hash somehow
    }
}