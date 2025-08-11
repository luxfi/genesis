package main

import (
    "fmt"
    "os"
    "github.com/cockroachdb/pebble"
)

func main() {
    srcPath := "/home/z/work/lux/state/chaindata/lux-mainnet-96369/db/pebbledb"
    
    fmt.Printf("Opening PebbleDB at: %s\n", srcPath)
    
    db, err := pebble.Open(srcPath, &pebble.Options{})
    if err != nil {
        fmt.Printf("Error opening database: %v\n", err)
        os.Exit(1)
    }
    defer db.Close()
    
    it, err := db.NewIter(nil)
    if err != nil {
        fmt.Printf("Error creating iterator: %v\n", err)
        os.Exit(1)
    }
    defer it.Close()
    
    count := 0
    samples := make(map[byte]int)
    
    // Check specific keys first
    fmt.Println("\n=== Checking specific keys ===")
    
    // Check for headers (h prefix = 0x68)
    headerKey := []byte{0x68} // 'h'
    val, closer, err := db.Get(headerKey)
    if err == nil {
        fmt.Printf("Found header prefix key: %x\n", val)
        closer.Close()
    } else {
        fmt.Printf("No header prefix (h) found\n")
    }
    
    // Check for bodies (b prefix = 0x62)
    bodyKey := []byte{0x62} // 'b'
    val, closer, err = db.Get(bodyKey)
    if err == nil {
        fmt.Printf("Found body prefix key: %x\n", val)
        closer.Close()
    } else {
        fmt.Printf("No body prefix (b) found\n")
    }
    
    // Check canonical hash for block 0 (genesis)
    canonicalKey := []byte{72} // 'H'
    val, closer, err = db.Get(canonicalKey)
    if err == nil {
        fmt.Printf("Canonical block 0 key (H): %x\n", val)
        closer.Close()
    } else {
        fmt.Printf("No canonical block 0 key found\n")
    }
    
    fmt.Println("\n=== Scanning all key prefixes ===")
    
    for it.First(); it.Valid(); it.Next() {
        key := it.Key()
        if len(key) > 0 {
            samples[key[0]]++
        }
        
        if count < 20 {
            fmt.Printf("Key %d: len=%d prefix=%02x", count+1, len(key), key[0])
            if len(key) <= 32 {
                fmt.Printf(" full=%x", key)
            }
            
            // Check value size
            val := it.Value()
            fmt.Printf(" val_len=%d", len(val))
            
            fmt.Println()
        }
        count++
        
        // Stop after 100000 to get better sample
        if count >= 100000 {
            fmt.Println("... (stopped at 100000 keys)")
            break
        }
    }
    
    fmt.Printf("\nTotal keys scanned: %d\n", count)
    fmt.Println("Key prefixes distribution:")
    for prefix, cnt := range samples {
        fmt.Printf("  0x%02x ('%c'): %d keys\n", prefix, prefix, cnt)
    }
}