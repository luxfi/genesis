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
    
    fmt.Println("Analyzing 32-byte keys after namespace")
    fmt.Println("======================================")
    
    // Get samples of namespace + 32-byte keys
    iter, _ := db.NewIter(&pebble.IterOptions{})
    defer iter.Close()
    
    samples := 0
    valueTypes := make(map[string]int)
    
    // Look for keys that are exactly namespace + 32 bytes (64 bytes total)
    for iter.First(); iter.Valid() && samples < 20; iter.Next() {
        key := iter.Key()
        
        if len(key) == 64 && bytes.HasPrefix(key, namespace) {
            hash := key[32:]
            value := iter.Value()
            
            fmt.Printf("\nKey %d:\n", samples)
            fmt.Printf("  Hash: %x\n", hash)
            fmt.Printf("  Value size: %d bytes\n", len(value))
            
            // Analyze value
            if len(value) == 8 {
                blockNum := binary.BigEndian.Uint64(value)
                fmt.Printf("  Value as uint64: %d\n", blockNum)
                valueTypes["uint64"]++
                
                // This could be the canonical mapping! hash -> block number
                // Let's verify by checking if this hash exists as a header
                headerKey := append(append([]byte{}, namespace...), 'h')
                headerKey = append(headerKey, hash...)
                if header, closer, err := db.Get(headerKey); err == nil {
                    closer.Close()
                    fmt.Printf("  ✓ Header exists with this hash! Size: %d bytes\n", len(header))
                } else {
                    fmt.Printf("  ✗ No header with prefix 'h' and this hash\n")
                }
                
                // Try uppercase H
                hKey := append(append([]byte{}, namespace...), 'H')
                hKey = append(hKey, hash...)
                if val, closer, err := db.Get(hKey); err == nil {
                    closer.Close()
                    fmt.Printf("  ✓ Found with 'H' prefix! Value: %x\n", val)
                }
                
            } else if len(value) >= 100 {
                fmt.Printf("  Value preview: %x...\n", value[:50])
                valueTypes["large"]++
            } else {
                fmt.Printf("  Value: %x\n", value)
                valueTypes[fmt.Sprintf("%d-bytes", len(value))]++
            }
            
            samples++
        }
    }
    
    fmt.Printf("\nValue types found:\n")
    for typ, count := range valueTypes {
        fmt.Printf("  %s: %d\n", typ, count)
    }
    
    // Now let's try the reverse - for block 0, generate possible hashes
    fmt.Println("\nSearching for block 0 by value:")
    
    iter2, _ := db.NewIter(&pebble.IterOptions{})
    defer iter2.Close()
    
    found := 0
    for iter2.First(); iter2.Valid() && found < 5; iter2.Next() {
        key := iter2.Key()
        value := iter2.Value()
        
        if len(key) == 64 && bytes.HasPrefix(key, namespace) && len(value) == 8 {
            blockNum := binary.BigEndian.Uint64(value)
            if blockNum == 0 {
                hash := key[32:]
                fmt.Printf("\nFound block 0!\n")
                fmt.Printf("  Hash: %s\n", hex.EncodeToString(hash))
                
                // Get the header
                headerKey := append(append([]byte{}, namespace...), 'h')
                headerKey = append(headerKey, hash...)
                if header, closer, err := db.Get(headerKey); err == nil {
                    closer.Close()
                    fmt.Printf("  ✓ Header found! Size: %d bytes\n", len(header))
                    fmt.Printf("  Header preview: %x...\n", header[:32])
                } else {
                    fmt.Printf("  ✗ No header with this hash\n")
                }
                
                // Get the body
                bodyKey := append(append([]byte{}, namespace...), 'b')
                bodyKey = append(bodyKey, hash...)
                if body, closer, err := db.Get(bodyKey); err == nil {
                    closer.Close()
                    fmt.Printf("  ✓ Body found! Size: %d bytes\n", len(body))
                } else {
                    fmt.Printf("  Body: empty\n")
                }
                
                found++
            }
            
            if blockNum <= 10 {
                hash := key[32:]
                fmt.Printf("\nBlock %d hash: %s\n", blockNum, hex.EncodeToString(hash))
            }
        }
    }
}