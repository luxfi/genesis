package main

import (
    "fmt"
    "github.com/luxfi/database/badgerdb"
)

func main() {
    db, err := badgerdb.New("/home/z/.luxd", nil, "", nil)
    if err != nil {
        panic(err)
    }
    defer db.Close()
    
    it := db.NewIterator()
    defer it.Release()
    
    count := 0
    samples := make(map[byte]int)
    
    for it.Next() && count < 1000 {
        key := it.Key()
        if len(key) > 0 {
            samples[key[0]]++
        }
        count++
        
        if count <= 20 {
            fmt.Printf("Key %d: len=%d prefix=%02x", count, len(key), key[0])
            if len(key) <= 32 {
                fmt.Printf(" full=%x", key)
            }
            fmt.Println()
        }
    }
    
    fmt.Printf("\nTotal keys sampled: %d\n", count)
    fmt.Println("Key prefixes found:")
    for prefix, cnt := range samples {
        fmt.Printf("  0x%02x: %d keys\n", prefix, cnt)
    }
}