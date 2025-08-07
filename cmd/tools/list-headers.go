package main

import (
    "bytes"
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
    
    fmt.Println("First 10 header keys:")
    
    iter, _ := db.NewIter(&pebble.IterOptions{})
    defer iter.Close()
    
    count := 0
    for iter.First(); iter.Valid() && count < 10; iter.Next() {
        key := iter.Key()
        
        if len(key) == 65 && bytes.Equal(key[:32], namespace) && key[32] == 'h' {
            hash := key[33:]
            fmt.Printf("Header %d: hash=%x (value: %d bytes)\n", count, hash, len(iter.Value()))
            count++
        }
    }
    
    if count == 0 {
        fmt.Println("No headers found with namespace+h format")
        
        // Try looking for headers without namespace
        iter2, _ := db.NewIter(&pebble.IterOptions{})
        for iter2.First(); iter2.Valid() && count < 10; iter2.Next() {
            key := iter2.Key()
            
            if len(key) == 33 && key[0] == 'h' {
                hash := key[1:]
                fmt.Printf("Header %d (no namespace): hash=%x (value: %d bytes)\n", count, hash, len(iter2.Value()))
                count++
            }
        }
        iter2.Close()
    }
}
