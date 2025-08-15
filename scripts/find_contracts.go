package main

import (
    "fmt"
    "path/filepath"
    "github.com/cockroachdb/pebble"
)

func main() {
    dbPath := "/Users/z/work/lux/state/chaindata/lux-mainnet-96369/db/pebbledb"
    
    db, err := pebble.Open(filepath.Clean(dbPath), &pebble.Options{ReadOnly: true})
    if err \!= nil {
        panic(err)
    }
    defer db.Close()
    
    fmt.Println("Scanning for deployed contracts (accounts with code)...")
    
    // Scan code table (prefix 0x02)
    it, _ := db.NewIter(&pebble.IterOptions{
        LowerBound: []byte{0x02},
        UpperBound: []byte{0x03},
    })
    defer it.Close()
    
    count := 0
    for it.First(); it.Valid(); it.Next() {
        key := it.Key()
        val := it.Value()
        if len(key) == 33 && len(val) > 0 { // 1 byte prefix + 32 byte code hash
            count++
            if count <= 10 {
                fmt.Printf("  Code hash %x: %d bytes\n", key[1:], len(val))
            }
        }
    }
    
    fmt.Printf("\nTotal contracts with code: %d\n", count)
}
