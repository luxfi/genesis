package main

import (
    "bytes"
    "encoding/binary"
    "fmt"
    "github.com/cockroachdb/pebble"
)

func main() {
    db, _ := pebble.Open("/home/z/work/lux/state/chaindata/lux-mainnet-96369/db/pebbledb", &pebble.Options{ReadOnly: true})
    defer db.Close()
    
    // The user said:
    // • 32-byte namespace prefix
    // • uppercase block-data prefixes (H, not h)
    // • 10-byte canonical key (n ... 0x6e) or none
    
    fmt.Println("Following user's exact format description...")
    fmt.Println("Looking for:")
    fmt.Println("- 32-byte namespace prefix")
    fmt.Println("- Uppercase 'H' for canonical mapping")
    fmt.Println("- Uppercase 'B' for body, uppercase 'H' for header (confusing, both H?)")
    fmt.Println("")
    
    // The namespace we found
    knownNamespace := []byte{
        0x33, 0x7f, 0xb7, 0x3f, 0x9b, 0xcd, 0xac, 0x8c,
        0x31, 0xa2, 0xd5, 0xf7, 0xb8, 0x77, 0xab, 0x1e,
        0x8a, 0x2b, 0x7f, 0x2a, 0x1e, 0x9b, 0xf0, 0x2a,
        0x0a, 0x0e, 0x6c, 0x6f, 0xd1, 0x64, 0xf1, 0xd1,
    }
    
    // Count different patterns
    patterns := make(map[string]int)
    canonicalCount := 0
    
    iter, _ := db.NewIter(&pebble.IterOptions{})
    defer iter.Close()
    
    for iter.First(); iter.Valid(); iter.Next() {
        key := iter.Key()
        
        // Check if it has our known namespace
        if len(key) >= 32 && bytes.Equal(key[:32], knownNamespace) {
            if len(key) > 32 {
                prefix := key[32]
                patterns[fmt.Sprintf("Known namespace + '%c' (0x%02x)", prefix, prefix)]++
                
                // Check for canonical patterns
                if len(key) == 41 && prefix == 'H' {
                    // namespace + H + blocknum
                    blockNum := binary.BigEndian.Uint64(key[33:])
                    if canonicalCount < 5 {
                        fmt.Printf("Found canonical: block %d -> hash %x\n", blockNum, iter.Value())
                    }
                    canonicalCount++
                }
            }
        }
        
        // Also check without namespace for 'H' keys
        if len(key) == 9 && key[0] == 'H' {
            blockNum := binary.BigEndian.Uint64(key[1:])
            patterns["No namespace + 'H' + blocknum"]++
            if canonicalCount < 5 {
                fmt.Printf("Found canonical (no namespace): block %d -> hash %x\n", blockNum, iter.Value())
            }
            canonicalCount++
        }
    }
    
    fmt.Printf("\nTotal canonical blocks found: %d\n", canonicalCount)
    fmt.Println("\nPattern distribution:")
    for pattern, count := range patterns {
        if count > 0 {
            fmt.Printf("  %s: %d keys\n", pattern, count)
        }
    }
    
    // The user mentioned the data IS there. Let's try different interpretations
    fmt.Println("\nTrying to find block 0 with different approaches...")
    
    // Try 1: namespace + H + blocknum
    key1 := append(knownNamespace, 'H')
    key1 = append(key1, make([]byte, 8)...)
    binary.BigEndian.PutUint64(key1[33:], 0)
    
    val, closer, err := db.Get(key1)
    if err == nil {
        closer.Close()
        fmt.Printf("Found with namespace+H: %x\n", val)
    } else {
        fmt.Println("Not found with namespace+H")
    }
    
    // Try 2: Just H + blocknum
    key2 := make([]byte, 9)
    key2[0] = 'H'
    binary.BigEndian.PutUint64(key2[1:], 0)
    
    val, closer, err = db.Get(key2)
    if err == nil {
        closer.Close()
        fmt.Printf("Found with H only: %x\n", val)
    } else {
        fmt.Println("Not found with H only")
    }
    
    // Try 3: Different namespace combinations
    fmt.Println("\nSearching for ANY key that could represent block 0...")
    
    iter2, _ := db.NewIter(&pebble.IterOptions{})
    found := false
    count := 0
    
    for iter2.First(); iter2.Valid() && !found && count < 10000000; iter2.Next() {
        key := iter2.Key()
        
        // Look for patterns that could encode block number 0
        if len(key) >= 8 {
            // Check last 8 bytes
            if len(key) >= 8 {
                lastBytes := key[len(key)-8:]
                num := binary.BigEndian.Uint64(lastBytes)
                if num == 0 {
                    // Check if this looks like a canonical key
                    if (len(key) == 9 && key[0] == 'H') ||
                       (len(key) == 41 && key[32] == 'H') ||
                       (len(key) == 10 && key[0] == 'n') ||
                       (len(key) == 42 && key[32] == 'n') {
                        fmt.Printf("Potential block 0 key found: %x\n", key)
                        fmt.Printf("  Value: %x\n", iter2.Value())
                        found = true
                    }
                }
            }
        }
        count++
    }
    iter2.Close()
    
    if !found {
        fmt.Printf("No obvious block 0 key found in first %d keys\n", count)
    }
}