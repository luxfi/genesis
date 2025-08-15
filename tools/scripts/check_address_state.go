package main

import (
    "encoding/hex"
    "fmt"
    "path/filepath"
    
    "github.com/cockroachdb/pebble"
)

func main() {
    dbPath := "/Users/z/work/lux/state/chaindata/lux-mainnet-96369/db/pebbledb"
    
    // Target address
    addr, _ := hex.DecodeString("EAbCC110fAcBfebabC66Ad6f9E7B67288e720B59")
    
    db, err := pebble.Open(filepath.Clean(dbPath), &pebble.Options{ReadOnly: true})
    if err != nil {
        panic(err)
    }
    defer db.Close()
    
    // Check account state (prefix 0x00)
    accKey := append([]byte{0x00}, addr...)
    val, closer, err := db.Get(accKey)
    if err != nil {
        fmt.Printf("No account state for address: %x", addr)
        fmt.Printf("Error: %v", err)
    } else {
        fmt.Printf("Found account state for address %x:", addr)
        fmt.Printf("  Raw data: %x", val)
        fmt.Printf("  Length: %d bytes", len(val))
        closer.Close()
    }
    
    // Check storage (prefix 0x01)
    storPrefix := append([]byte{0x01}, addr...)
    it, _ := db.NewIter(&pebble.IterOptions{
        LowerBound: storPrefix,
        UpperBound: append(storPrefix, 0xff),
    })
    
    storCount := 0
    for it.First(); it.Valid() && storCount < 5; it.Next() {
        if storCount == 0 {
            fmt.Println("Storage slots found:")
        }
        key := it.Key()
        slot := key[21:]
        fmt.Printf("  Slot %x: %x", slot, it.Value())
        storCount++
    }
    it.Close()
    
    if storCount == 0 {
        fmt.Println("No storage slots for this address")
    } else if storCount >= 5 {
        fmt.Printf("  ... and more")
    }
}
