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
    
    iter, _ := db.NewIter(&pebble.IterOptions{})
    defer iter.Close()
    
    fmt.Println("Comprehensive scan for ALL 'H' prefix keys...")
    
    hKeys := [][]byte{}
    keyLengths := make(map[int]int)
    
    for iter.First(); iter.Valid(); iter.Next() {
        key := iter.Key()
        
        // Check for 'H' at ANY position that could be a prefix
        foundH := false
        
        // Position 0: No namespace
        if len(key) > 0 && key[0] == 'H' {
            foundH = true
            keyLengths[len(key)]++
            if len(hKeys) < 10 {
                hKeys = append(hKeys, append([]byte{}, key...))
            }
        }
        
        // Position 32: After standard namespace
        if len(key) > 32 && key[32] == 'H' {
            foundH = true
            keyLengths[len(key)]++
            if len(hKeys) < 10 {
                hKeys = append(hKeys, append([]byte{}, key...))
            }
        }
        
        // Try interpreting different key lengths
        if foundH && len(key) == 9 {
            // Format: H + blocknum (8 bytes)
            blockNum := binary.BigEndian.Uint64(key[1:])
            if len(hKeys) <= 5 {
                fmt.Printf("9-byte H key: block=%d, hash=%x\n", blockNum, iter.Value())
            }
        }
        
        if foundH && len(key) == 41 {
            // Format: namespace(32) + H + blocknum(8)
            blockNum := binary.BigEndian.Uint64(key[33:])
            if len(hKeys) <= 5 {
                fmt.Printf("41-byte H key: namespace=%x, block=%d, hash=%x\n", 
                    key[:32], blockNum, iter.Value())
            }
        }
    }
    
    fmt.Printf("\nFound %d keys with 'H' prefix\n", len(hKeys))
    fmt.Println("\nKey length distribution for H keys:")
    for length, count := range keyLengths {
        fmt.Printf("  Length %d: %d keys\n", length, count)
    }
    
    // Analyze the keys we found
    if len(hKeys) > 0 {
        fmt.Println("\nFirst few H keys in detail:")
        for i, key := range hKeys {
            if i >= 10 {
                break
            }
            fmt.Printf("\nKey %d (len=%d):\n", i, len(key))
            fmt.Printf("  Raw: %x\n", key)
            
            // Try different interpretations
            if len(key) == 9 && key[0] == 'H' {
                blockNum := binary.BigEndian.Uint64(key[1:])
                fmt.Printf("  Interpretation: H + blockNum(%d)\n", blockNum)
                
                // Try to get the value
                value, closer, err := db.Get(key)
                if err == nil {
                    closer.Close()
                    fmt.Printf("  Value (hash): %x\n", value)
                    
                    // Try to get header with this hash
                    headerKey := append([]byte{'h'}, value...)
                    headerData, closer, err := db.Get(headerKey)
                    if err == nil {
                        closer.Close()
                        fmt.Printf("  Header found: %d bytes\n", len(headerData))
                    }
                }
            }
        }
    }
    
    // Let's also check if there's a different namespace pattern
    fmt.Println("\nChecking for unique namespace patterns...")
    namespaces := make(map[string]int)
    
    iter2, _ := db.NewIter(&pebble.IterOptions{})
    count := 0
    for iter2.First(); iter2.Valid() && count < 1000000; iter2.Next() {
        key := iter2.Key()
        if len(key) >= 32 {
            ns := fmt.Sprintf("%x", key[:32])
            namespaces[ns]++
        }
        count++
    }
    iter2.Close()
    
    fmt.Println("\nTop namespaces:")
    for ns, cnt := range namespaces {
        if cnt > 100000 {
            fmt.Printf("  %s: %d keys\n", ns, cnt)
        }
    }
}