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
        0x33, 0x7f, 0xb7, 0x3f, 0x9b, 0xcd, 0xac, 0x8c,
        0x31, 0xa2, 0xd5, 0xf7, 0xb8, 0x77, 0xab, 0x1e,
        0x8a, 0x2b, 0x7f, 0x2a, 0x1e, 0x9b, 0xf0, 0x2a,
        0x0a, 0x0e, 0x6c, 0x6f, 0xd1, 0x64, 0xf1, 0xd1,
    }
    
    // Block 0 hash from our H key
    block0Hash, _ := hex.DecodeString("3f4fa2a0b0ce089f52bf0ae9199c75ffdd76ecafc987794050cb0d286f1ec61e")
    
    fmt.Printf("Testing header access for block 0 hash: %x\n\n", block0Hash)
    
    // Try namespace + h + hash
    key1 := append(namespace, 'h')
    key1 = append(key1, block0Hash...)
    fmt.Printf("Try 1 - namespace+h+hash (len=%d): ", len(key1))
    _, closer, err := db.Get(key1)
    if err == nil {
        closer.Close()
        fmt.Println("FOUND")
    } else {
        fmt.Println("NOT FOUND")
    }
    
    // Try just h + hash
    key2 := append([]byte{'h'}, block0Hash...)
    fmt.Printf("Try 2 - h+hash (len=%d): ", len(key2))
    _, closer, err = db.Get(key2)
    if err == nil {
        closer.Close()
        fmt.Println("FOUND")
    } else {
        fmt.Println("NOT FOUND")
    }
    
    // Search for any header keys that contain this hash
    fmt.Println("\nSearching for headers containing this hash...")
    iter, _ := db.NewIter(&pebble.IterOptions{})
    count := 0
    found := 0
    
    for iter.First(); iter.Valid() && count < 10000000; iter.Next() {
        key := iter.Key()
        
        // Check if this could be a header key
        if (len(key) == 65 && bytes.Equal(key[:32], namespace) && key[32] == 'h') ||
           (len(key) == 33 && key[0] == 'h') {
            
            var hash []byte
            if len(key) == 65 {
                hash = key[33:]
            } else {
                hash = key[1:]
            }
            
            if bytes.Equal(hash, block0Hash) {
                fmt.Printf("FOUND header key: %x\n", key)
                fmt.Printf("  Value size: %d bytes\n", len(iter.Value()))
                found++
            }
        }
        count++
    }
    iter.Close()
    
    if found == 0 {
        fmt.Printf("No header found for this hash in %d keys\n", count)
    }
    
    // Let's check what hash format the headers actually use
    fmt.Println("\nSample header keys:")
    iter2, _ := db.NewIter(&pebble.IterOptions{})
    samples := 0
    for iter2.First(); iter2.Valid() && samples < 5; iter2.Next() {
        key := iter2.Key()
        if len(key) == 65 && bytes.Equal(key[:32], namespace) && key[32] == 'h' {
            hash := key[33:]
            fmt.Printf("  Header: namespace+h+%x\n", hash)
            samples++
        }
    }
    iter2.Close()
}
