package main

import (
    "fmt"
    "github.com/cockroachdb/pebble"
)

func main() {
    db, _ := pebble.Open("/home/z/work/lux/state/chaindata/lux-mainnet-96369/db/pebbledb", &pebble.Options{ReadOnly: true})
    defer db.Close()
    
    iter, _ := db.NewIter(&pebble.IterOptions{})
    defer iter.Close()
    
    count := 0
    hCount := 0
    
    fmt.Println("Dumping keys that contain 'H' (0x48):")
    for iter.First(); iter.Valid() && count < 10000000; iter.Next() {
        key := iter.Key()
        
        // Look for 'H' anywhere in the key
        for i, b := range key {
            if b == 0x48 { // 'H'
                if hCount < 20 {
                    fmt.Printf("Key[%d]: ", hCount)
                    for j := 0; j < len(key); j++ {
                        fmt.Printf("%02x", key[j])
                    }
                    fmt.Printf(" (H at position %d)\n", i)
                    
                    // If H is at beginning or after 32-byte namespace
                    if i == 0 || i == 32 {
                        fmt.Printf("  Potential canonical key!\n")
                    }
                }
                hCount++
                break
            }
        }
        count++
    }
    
    fmt.Printf("\nTotal keys scanned: %d\n", count)
    fmt.Printf("Keys containing 'H': %d\n", hCount)
}