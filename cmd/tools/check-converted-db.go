package main

import (
    "encoding/hex"
    "fmt"
    "log"
    
    "github.com/cockroachdb/pebble"
)

func main() {
    db, err := pebble.Open("/tmp/converted-genesis-db", &pebble.Options{ReadOnly: true})
    if err \!= nil {
        log.Fatal("Failed to open database:", err)
    }
    defer db.Close()
    
    // Check for H keys (canonical mappings)
    fmt.Println("Checking for canonical hash at block 0...")
    hKey := []byte{'H'}
    hKey = append(hKey, 0, 0, 0, 0, 0, 0, 0, 0) // block 0
    
    if val, closer, err := db.Get(hKey); err == nil {
        closer.Close()
        fmt.Printf("✅ Found H key for block 0: %s\n", hex.EncodeToString(val))
        
        // Now check for the header
        headerKey := append([]byte{'h'}, val...)
        if header, closer, err := db.Get(headerKey); err == nil {
            closer.Close()
            fmt.Printf("✅ Found header for block 0: %d bytes\n", len(header))
        } else {
            fmt.Printf("❌ No header found for hash: %s\n", hex.EncodeToString(val))
        }
    } else {
        fmt.Println("❌ No H key found for block 0")
    }
    
    // List first 20 keys
    fmt.Println("\nFirst 20 keys in database:")
    iter, _ := db.NewIter(&pebble.IterOptions{})
    count := 0
    for iter.First(); iter.Valid() && count < 20; iter.Next() {
        key := iter.Key()
        val := iter.Value()
        
        keyStr := hex.EncodeToString(key)
        if len(key) > 0 && key[0] >= 32 && key[0] <= 126 {
            keyStr = fmt.Sprintf("%c %s", key[0], hex.EncodeToString(key[1:]))
        }
        
        fmt.Printf("  %s (len=%d) -> %d bytes\n", keyStr, len(key), len(val))
        count++
    }
    iter.Close()
}
