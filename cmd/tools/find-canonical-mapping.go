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
    
    namespace := []byte{
        0x33, 0x7f, 0xb7, 0x3f, 0x9b, 0xcd, 0xac, 0x8c,
        0x31, 0xa2, 0xd5, 0xf7, 0xb8, 0x77, 0xab, 0x1e,
        0x8a, 0x2b, 0x7f, 0x2a, 0x1e, 0x9b, 0xf0, 0x2a,
        0x0a, 0x0e, 0x6c, 0x6f, 0xd1, 0x64, 0xf1, 0xd1,
    }
    
    fmt.Println("Finding the canonical mapping pattern")
    fmt.Println("======================================")
    
    // Count different key patterns
    patterns := make(map[string]int)
    sampleKeys := make(map[string][][]byte)
    
    iter, _ := db.NewIter(&pebble.IterOptions{})
    defer iter.Close()
    
    count := 0
    for iter.First(); iter.Valid() && count < 5000000; iter.Next() {
        key := iter.Key()
        
        if bytes.HasPrefix(key, namespace) {
            remaining := key[32:]
            
            pattern := ""
            if len(remaining) == 0 {
                pattern = "namespace-only"
            } else if len(remaining) == 1 {
                pattern = fmt.Sprintf("prefix-%c", remaining[0])
            } else if len(remaining) == 9 {
                // Prefix + 8 bytes (possibly block number)
                pattern = fmt.Sprintf("prefix-%c-8bytes", remaining[0])
                if patterns[pattern] < 5 {
                    if sampleKeys[pattern] == nil {
                        sampleKeys[pattern] = [][]byte{}
                    }
                    sampleKeys[pattern] = append(sampleKeys[pattern], key)
                }
            } else if len(remaining) == 32 {
                pattern = "32-bytes"
            } else if len(remaining) == 33 {
                pattern = fmt.Sprintf("prefix-%c-32bytes", remaining[0])
                if patterns[pattern] < 5 {
                    if sampleKeys[pattern] == nil {
                        sampleKeys[pattern] = [][]byte{}
                    }
                    sampleKeys[pattern] = append(sampleKeys[pattern], key)
                }
            } else {
                pattern = fmt.Sprintf("%d-bytes", len(remaining))
            }
            
            patterns[pattern]++
        }
        count++
    }
    
    fmt.Printf("\nScanned %d keys\n", count)
    fmt.Println("\nKey patterns found:")
    for pattern, cnt := range patterns {
        fmt.Printf("  %s: %d\n", pattern, cnt)
    }
    
    // Examine the 9-byte patterns (likely canonical)
    fmt.Println("\nExamining prefix-?-8bytes patterns (likely canonical):")
    for pattern, keys := range sampleKeys {
        if len(keys) > 0 && len(pattern) >= 7 && pattern[7] == '8' {
            fmt.Printf("\n%s:\n", pattern)
            for i, key := range keys {
                if i >= 3 {
                    break
                }
                remaining := key[32:]
                prefix := remaining[0]
                numBytes := remaining[1:9]
                blockNum := binary.BigEndian.Uint64(numBytes)
                
                value := []byte{}
                if val, closer, err := db.Get(key); err == nil {
                    value = val
                    closer.Close()
                }
                
                fmt.Printf("  Key: prefix='%c'(0x%02x) blockNum=%d\n", prefix, prefix, blockNum)
                if len(value) == 32 {
                    fmt.Printf("    Value (32 bytes - hash): %x\n", value)
                    
                    // Check if this hash exists as a header
                    headerKey := append(namespace, 'h')
                    headerKey = append(headerKey, value...)
                    if header, closer, err := db.Get(headerKey); err == nil {
                        closer.Close()
                        fmt.Printf("    ✓ Header exists! Size: %d bytes\n", len(header))
                    } else {
                        fmt.Printf("    ✗ No header with this hash\n")
                    }
                } else {
                    fmt.Printf("    Value (%d bytes): %x\n", len(value), value)
                }
            }
        }
    }
    
    // Look for the canonical mapping for block 0 specifically
    fmt.Println("\nSearching for block 0 canonical entry:")
    
    // Try different prefixes with block number 0
    prefixes := []byte{'c', 'C', 'n', 'N', 'h', 'H', 'l', 'L', 'a'}
    
    for _, prefix := range prefixes {
        testKey := append(namespace, prefix)
        blockNumBytes := make([]byte, 8)
        binary.BigEndian.PutUint64(blockNumBytes, 0)
        testKey = append(testKey, blockNumBytes...)
        
        if value, closer, err := db.Get(testKey); err == nil {
            closer.Close()
            fmt.Printf("  Found with prefix '%c': value=%x\n", prefix, value)
            
            if len(value) == 32 {
                // Try as header hash
                headerKey := append(namespace, 'h')
                headerKey = append(headerKey, value...)
                if header, closer, err := db.Get(headerKey); err == nil {
                    closer.Close()
                    fmt.Printf("    ✓ This is a valid block hash! Header size: %d\n", len(header))
                }
            }
        }
    }
    
    // Try looking for "Last" prefix (common in some implementations)
    fmt.Println("\nChecking for 'Last' canonical header:")
    lastKey := append(namespace, []byte("LastHeader")...)
    if value, closer, err := db.Get(lastKey); err == nil {
        closer.Close()
        fmt.Printf("  Found LastHeader: %x\n", value)
        if len(value) == 8 {
            lastBlock := binary.BigEndian.Uint64(value)
            fmt.Printf("  Last block number: %d\n", lastBlock)
        }
    }
    
    lastKey = append(namespace, []byte("LastBlock")...)
    if value, closer, err := db.Get(lastKey); err == nil {
        closer.Close()
        fmt.Printf("  Found LastBlock: %x\n", value)
    }
    
    // Check if there's a number-to-hash mapping without prefix
    fmt.Println("\nTrying direct number-to-hash mapping (no prefix):")
    for blockNum := uint64(0); blockNum <= 5; blockNum++ {
        testKey := namespace
        blockNumBytes := make([]byte, 8)
        binary.BigEndian.PutUint64(blockNumBytes, blockNum)
        testKey = append(testKey, blockNumBytes...)
        
        if value, closer, err := db.Get(testKey); err == nil {
            closer.Close()
            fmt.Printf("  Block %d -> %x\n", blockNum, value)
            
            if len(value) == 32 {
                // Check if it's a valid header hash
                headerKey := append(namespace, 'h')
                headerKey = append(headerKey, value...)
                if _, closer, err := db.Get(headerKey); err == nil {
                    closer.Close()
                    fmt.Printf("    ✓ Valid header hash!\n")
                }
            }
        }
    }
}