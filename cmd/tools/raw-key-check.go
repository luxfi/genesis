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
    
    fmt.Println("Raw key examination")
    fmt.Println("===================")
    
    // Get the ACTUAL first H key
    iter, _ := db.NewIter(&pebble.IterOptions{})
    defer iter.Close()
    
    fmt.Println("\nFirst H key found:")
    for iter.First(); iter.Valid(); iter.Next() {
        key := iter.Key()
        if len(key) == 65 && bytes.Equal(key[:32], namespace) && key[32] == 'H' {
            fmt.Printf("  Full key: %x\n", key)
            fmt.Printf("  Namespace: %x\n", key[:32])
            fmt.Printf("  Prefix: '%c' (0x%02x)\n", key[32], key[32])
            fmt.Printf("  Hash part: %x\n", key[33:65])
            fmt.Printf("  Hash as string: %s\n", string(key[33:65]))
            
            value := iter.Value()
            fmt.Printf("  Value (raw): %x\n", value)
            if len(value) == 8 {
                blockNum := binary.BigEndian.Uint64(value)
                fmt.Printf("  Value as block number: %d\n", blockNum)
            }
            
            // Now try to access this specific key again
            fmt.Println("\n  Verifying by direct Get:")
            if val2, closer, err := db.Get(key); err == nil {
                closer.Close()
                fmt.Printf("    Retrieved value: %x\n", val2)
                if bytes.Equal(val2, value) {
                    fmt.Println("    ✓ Values match")
                } else {
                    fmt.Println("    ✗ Values DON'T match!")
                }
            } else {
                fmt.Printf("    Error: %v\n", err)
            }
            
            break
        }
    }
    
    // Now look for the first header
    fmt.Println("\nFirst header key found:")
    iter2, _ := db.NewIter(&pebble.IterOptions{})
    defer iter2.Close()
    
    for iter2.First(); iter2.Valid(); iter2.Next() {
        key := iter2.Key()
        if len(key) == 65 && bytes.Equal(key[:32], namespace) && key[32] == 'h' {
            fmt.Printf("  Full key: %x\n", key)
            fmt.Printf("  Prefix: '%c' (0x%02x)\n", key[32], key[32])
            fmt.Printf("  Hash part: %x\n", key[33:65])
            
            value := iter2.Value()
            fmt.Printf("  Value size: %d bytes\n", len(value))
            fmt.Printf("  Value (first 32 bytes): %x\n", value[:32])
            break
        }
    }
    
    // Count total keys of each type
    fmt.Println("\nCounting key types (first 1 million):")
    iter3, _ := db.NewIter(&pebble.IterOptions{})
    defer iter3.Close()
    
    counts := make(map[string]int)
    total := 0
    
    for iter3.First(); iter3.Valid() && total < 1000000; iter3.Next() {
        key := iter3.Key()
        
        if len(key) == 65 && bytes.Equal(key[:32], namespace) {
            prefix := string([]byte{key[32]})
            counts[prefix]++
        } else if len(key) == 64 && bytes.Equal(key[:32], namespace) {
            counts["64-byte"]++
        } else if len(key) == 41 && bytes.Equal(key[:32], namespace) {
            counts["41-byte"]++
        }
        
        total++
    }
    
    fmt.Println("Key type counts:")
    for typ, count := range counts {
        if count > 0 {
            fmt.Printf("  %s: %d\n", typ, count)
        }
    }
    
    // Let's specifically look for block 0
    fmt.Println("\nSearching for block 0 in H keys:")
    
    // Build the key for block 0
    iter4, _ := db.NewIter(&pebble.IterOptions{})
    defer iter4.Close()
    
    found := false
    for iter4.First(); iter4.Valid(); iter4.Next() {
        key := iter4.Key()
        
        if len(key) == 65 && bytes.Equal(key[:32], namespace) && key[32] == 'H' {
            value := iter4.Value()
            if len(value) == 8 {
                blockNum := binary.BigEndian.Uint64(value)
                if blockNum == 0 {
                    fmt.Printf("  Found block 0!\n")
                    fmt.Printf("  Key: %x\n", key)
                    fmt.Printf("  Hash in key: %x\n", key[33:65])
                    fmt.Printf("  Hash as hex string: %s\n", hex.EncodeToString(key[33:65]))
                    found = true
                    
                    // Now look for a header with this hash
                    hash := key[33:65]
                    headerKey := append(namespace, 'h')
                    headerKey = append(headerKey, hash...)
                    
                    if _, closer, err := db.Get(headerKey); err == nil {
                        closer.Close()
                        fmt.Println("  ✓ Header found with this hash!")
                    } else {
                        fmt.Println("  ✗ No header with this hash")
                    }
                    
                    break
                }
            }
        }
    }
    
    if !found {
        fmt.Println("  Block 0 not found in H keys")
    }
}