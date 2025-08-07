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
    
    namespace := []byte{
        0x33, 0x7f, 0xb7, 0x3f, 0x9b, 0xcd, 0xac, 0x8c,
        0x31, 0xa2, 0xd5, 0xf7, 0xb8, 0x77, 0xab, 0x1e,
        0x8a, 0x2b, 0x7f, 0x2a, 0x1e, 0x9b, 0xf0, 0x2a,
        0x0a, 0x0e, 0x6c, 0x6f, 0xd1, 0x64, 0xf1, 0xd1,
    }
    
    fmt.Println("Debugging hash format for blocks 0-10...")
    
    // Find H keys for blocks 0-10
    iter, _ := db.NewIter(&pebble.IterOptions{})
    defer iter.Close()
    
    blockToKey := make(map[uint64][]byte)
    blockToValue := make(map[uint64][]byte)
    
    for iter.First(); iter.Valid(); iter.Next() {
        key := iter.Key()
        
        if len(key) == 65 && bytes.Equal(key[:32], namespace) && key[32] == 'H' {
            value := iter.Value()
            
            // Check if value is 8-byte block number
            if len(value) == 8 {
                blockNum := binary.BigEndian.Uint64(value)
                if blockNum <= 10 {
                    blockToKey[blockNum] = key
                    blockToValue[blockNum] = value
                    
                    hash := key[33:65]
                    fmt.Printf("\nBlock %d:\n", blockNum)
                    fmt.Printf("  Full key: %x\n", key)
                    fmt.Printf("  Hash part: %x\n", hash)
                    fmt.Printf("  Hash as string: %s\n", string(hash))
                    fmt.Printf("  Value (should be blocknum): %x (decimal: %d)\n", value, blockNum)
                }
            } else if len(value) == 32 {
                // Maybe the value is the hash and we need to decode the block number from the key?
                hash := key[33:65]
                
                // Try interpreting the hash part as containing a block number
                // Check if any part of it could be a block number
                for i := 0; i <= 24; i++ {
                    if i+8 <= 32 {
                        possibleNum := binary.BigEndian.Uint64(hash[i:i+8])
                        if possibleNum <= 10 {
                            fmt.Printf("\nPossible block %d at offset %d in hash:\n", possibleNum, i)
                            fmt.Printf("  Full key: %x\n", key)
                            fmt.Printf("  Hash: %x\n", hash)
                            fmt.Printf("  Value: %x\n", value)
                        }
                    }
                }
            }
        }
    }
    
    fmt.Println("\n\nSummary of blocks found:")
    for i := uint64(0); i <= 10; i++ {
        if key, exists := blockToKey[i]; exists {
            hash := key[33:65]
            fmt.Printf("Block %d: hash=%x\n", i, hash)
            
            // Now try to find the header with this hash
            headerKey := append(namespace, 'h')
            headerKey = append(headerKey, hash...)
            
            headerData, closer, err := db.Get(headerKey)
            if err == nil {
                closer.Close()
                fmt.Printf("  Header found: %d bytes\n", len(headerData))
            } else {
                fmt.Printf("  Header NOT found with namespace+h+hash\n")
                
                // Try without namespace
                headerKey2 := append([]byte{'h'}, hash...)
                headerData, closer, err = db.Get(headerKey2)
                if err == nil {
                    closer.Close()
                    fmt.Printf("  Header found without namespace: %d bytes\n", len(headerData))
                } else {
                    fmt.Printf("  Header NOT found without namespace either\n")
                }
            }
        } else {
            fmt.Printf("Block %d: NOT FOUND\n", i)
        }
    }
    
    // Let's also look at actual 'h' prefix keys to understand the format
    fmt.Println("\n\nSample 'h' (header) keys:")
    iter2, _ := db.NewIter(&pebble.IterOptions{})
    count := 0
    for iter2.First(); iter2.Valid() && count < 5; iter2.Next() {
        key := iter2.Key()
        if len(key) == 65 && bytes.Equal(key[:32], namespace) && key[32] == 'h' {
            hash := key[33:65]
            fmt.Printf("Header key: namespace+h+%x (value: %d bytes)\n", hash, len(iter2.Value()))
            count++
        }
    }
    iter2.Close()
}