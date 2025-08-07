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
    
    fmt.Println("Comprehensive key scan")
    fmt.Println("======================")
    
    // Categories of keys
    categories := make(map[string]int)
    rlpHeaders := []struct{
        key []byte
        val []byte
    }{}
    
    iter, _ := db.NewIter(&pebble.IterOptions{})
    defer iter.Close()
    
    totalKeys := 0
    for iter.First(); iter.Valid() && totalKeys < 1000000; iter.Next() {
        key := iter.Key()
        value := iter.Value()
        
        keyLen := len(key)
        valLen := len(value)
        
        // Categorize by key structure
        category := fmt.Sprintf("keyLen=%d", keyLen)
        
        if bytes.HasPrefix(key, namespace) {
            remainder := key[32:]
            
            if len(remainder) == 0 {
                category = "namespace-only"
            } else if len(remainder) == 32 {
                category = "namespace+32bytes"
                
                // If value is RLP header, save it
                if valLen > 200 && (value[0] == 0xf8 || value[0] == 0xf9) {
                    if len(rlpHeaders) < 20 {
                        rlpHeaders = append(rlpHeaders, struct{
                            key []byte
                            val []byte
                        }{key, value})
                    }
                }
            } else if len(remainder) == 33 {
                prefix := remainder[0]
                if prefix >= 32 && prefix < 127 {
                    category = fmt.Sprintf("namespace+%c+32bytes", prefix)
                } else {
                    category = fmt.Sprintf("namespace+0x%02x+32bytes", prefix)
                }
            } else if len(remainder) == 8 {
                category = "namespace+8bytes"
            } else if len(remainder) == 9 {
                prefix := remainder[0]
                if prefix >= 32 && prefix < 127 {
                    category = fmt.Sprintf("namespace+%c+8bytes", prefix)
                } else {
                    category = fmt.Sprintf("namespace+0x%02x+8bytes", prefix)
                }
            } else {
                category = fmt.Sprintf("namespace+%dbytes", len(remainder))
            }
        }
        
        // Also categorize by value type
        if valLen == 8 {
            category += " val=8bytes"
        } else if valLen > 200 && (value[0] == 0xf8 || value[0] == 0xf9) {
            category += " val=RLP"
        }
        
        categories[category]++
        totalKeys++
    }
    
    fmt.Printf("Scanned %d keys\n\n", totalKeys)
    
    fmt.Println("Key categories:")
    for cat, count := range categories {
        if count > 100 {
            fmt.Printf("  %s: %d\n", cat, count)
        }
    }
    
    fmt.Printf("\n\nFound %d RLP headers at namespace+32bytes\n", len(rlpHeaders))
    fmt.Println("Analyzing their hashes:")
    
    for i, h := range rlpHeaders {
        if i >= 10 {
            break
        }
        
        hash := h.key[32:]
        fmt.Printf("\n%d. Hash: %s\n", i, hex.EncodeToString(hash))
        
        // Try different decodings
        if len(hash) >= 8 {
            // First 8 bytes as block number
            blockNum1 := binary.BigEndian.Uint64(hash[0:8])
            // First 4 bytes
            blockNum2 := binary.BigEndian.Uint32(hash[0:4])
            // First 3 bytes
            blockNum3 := uint64(hash[0])<<16 | uint64(hash[1])<<8 | uint64(hash[2])
            // First 2 bytes
            blockNum4 := uint64(hash[0])<<8 | uint64(hash[1])
            
            fmt.Printf("   First 8 bytes as uint64: %d\n", blockNum1)
            fmt.Printf("   First 4 bytes as uint32: %d\n", blockNum2)
            fmt.Printf("   First 3 bytes: %d\n", blockNum3)
            fmt.Printf("   First 2 bytes: %d\n", blockNum4)
        }
        
        fmt.Printf("   Header size: %d bytes\n", len(h.val))
    }
    
    // Look for patterns in the H keys
    fmt.Println("\n\nAnalyzing H keys (canonical mapping):")
    
    // Get a few H keys
    hKeys := []struct{
        hash []byte
        blockNum uint64
    }{}
    
    iter2, _ := db.NewIter(&pebble.IterOptions{})
    for iter2.First(); iter2.Valid(); iter2.Next() {
        key := iter2.Key()
        
        if len(key) == 65 && bytes.HasPrefix(key, namespace) && key[32] == 'H' {
            hash := key[33:]
            value := iter2.Value()
            
            if len(value) == 8 {
                blockNum := binary.BigEndian.Uint64(value)
                hKeys = append(hKeys, struct{
                    hash []byte
                    blockNum uint64
                }{hash, blockNum})
                
                if len(hKeys) >= 100 {
                    break
                }
            }
        }
    }
    iter2.Close()
    
    // Display some H keys
    fmt.Printf("Sample H keys:\n")
    for i := 0; i < 10 && i < len(hKeys); i++ {
        fmt.Printf("  Block %d -> H-hash %s\n", hKeys[i].blockNum, hex.EncodeToString(hKeys[i].hash))
    }
    
    // Check if any RLP headers match H-hashes
    fmt.Println("\nChecking if RLP header hashes match H-hashes...")
    matches := 0
    for _, rlp := range rlpHeaders {
        hash := rlp.key[32:]
        for _, h := range hKeys {
            if bytes.Equal(hash, h.hash) {
                fmt.Printf("  MATCH! Block %d has RLP header\n", h.blockNum)
                matches++
                break
            }
        }
    }
    
    if matches == 0 {
        fmt.Println("  No direct matches between RLP hashes and H-hashes")
        fmt.Println("  The H-hashes and block hashes are different!")
    }
}