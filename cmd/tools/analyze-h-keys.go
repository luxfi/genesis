package main

import (
    "bytes"
    "encoding/binary"
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
    
    fmt.Println("Analyzing the 1,188,482 'H' keys...")
    
    iter, _ := db.NewIter(&pebble.IterOptions{})
    defer iter.Close()
    
    hCount := 0
    keyLengths := make(map[int]int)
    samples := [][]byte{}
    minBlockNum := uint64(^uint64(0))
    maxBlockNum := uint64(0)
    
    for iter.First(); iter.Valid(); iter.Next() {
        key := iter.Key()
        
        if len(key) >= 33 && bytes.Equal(key[:32], namespace) && key[32] == 'H' {
            hCount++
            keyLengths[len(key)]++
            
            if len(samples) < 10 {
                samples = append(samples, append([]byte{}, key...))
            }
            
            // Try to extract block number from different positions
            if len(key) == 73 {
                // namespace(32) + H(1) + hash(32) + blocknum(8)
                blockNum := binary.BigEndian.Uint64(key[65:])
                if blockNum < minBlockNum {
                    minBlockNum = blockNum
                }
                if blockNum > maxBlockNum {
                    maxBlockNum = blockNum
                }
            } else if len(key) == 41 {
                // namespace(32) + H(1) + blocknum(8)
                blockNum := binary.BigEndian.Uint64(key[33:])
                if blockNum < minBlockNum {
                    minBlockNum = blockNum
                }
                if blockNum > maxBlockNum {
                    maxBlockNum = blockNum
                }
            }
        }
    }
    
    fmt.Printf("Total 'H' keys: %d\n", hCount)
    fmt.Println("\nKey length distribution:")
    for length, count := range keyLengths {
        fmt.Printf("  Length %d: %d keys\n", length, count)
    }
    
    fmt.Println("\nFirst 10 sample 'H' keys:")
    for i, key := range samples {
        fmt.Printf("\nSample %d (len=%d):\n", i, len(key))
        fmt.Printf("  Full key: %x\n", key)
        fmt.Printf("  After namespace: %x\n", key[32:])
        
        if len(key) == 73 {
            // namespace(32) + H(1) + hash(32) + blocknum(8)
            hash := key[33:65]
            blockNum := binary.BigEndian.Uint64(key[65:])
            fmt.Printf("  Interpretation: H + hash(%x...) + blockNum(%d)\n", hash[:8], blockNum)
            
            // Try to get value
            value, closer, err := db.Get(key)
            if err == nil {
                closer.Close()
                fmt.Printf("  Value: %x\n", value)
            }
        } else if len(key) == 65 {
            // namespace(32) + H(1) + hash(32)
            hash := key[33:]
            fmt.Printf("  Interpretation: H + hash(%x...)\n", hash[:8])
            
            // Try to get value
            value, closer, err := db.Get(key)
            if err == nil {
                closer.Close()
                fmt.Printf("  Value: %x\n", value)
            }
        }
    }
    
    if minBlockNum != ^uint64(0) {
        fmt.Printf("\nBlock number range: %d to %d\n", minBlockNum, maxBlockNum)
    }
    
    // Now let's try to build a canonical mapping
    fmt.Println("\nBuilding canonical block mapping...")
    
    // If keys are namespace+H+hash+blocknum, we need to invert this
    blockToHash := make(map[uint64][]byte)
    
    iter2, _ := db.NewIter(&pebble.IterOptions{})
    count := 0
    for iter2.First(); iter2.Valid() && count < 100000; iter2.Next() {
        key := iter2.Key()
        
        if len(key) == 73 && bytes.Equal(key[:32], namespace) && key[32] == 'H' {
            hash := key[33:65]
            blockNum := binary.BigEndian.Uint64(key[65:])
            
            if blockNum < 10 {
                fmt.Printf("  Block %d -> hash %s\n", blockNum, hex.EncodeToString(hash))
                blockToHash[blockNum] = hash
            }
            count++
        }
    }
    iter2.Close()
    
    // Try to access block 0's data
    if hash0, ok := blockToHash[0]; ok {
        fmt.Printf("\nFound block 0 hash: %s\n", hex.EncodeToString(hash0))
        
        // Try to get header with namespace+h+hash
        headerKey := append(namespace, 'h')
        headerKey = append(headerKey, hash0...)
        
        headerData, closer, err := db.Get(headerKey)
        if err == nil {
            closer.Close()
            fmt.Printf("Header found: %d bytes\n", len(headerData))
        } else {
            fmt.Println("Header not found")
        }
        
        // Try to get body with namespace+b+hash
        bodyKey := append(namespace, 'b')
        bodyKey = append(bodyKey, hash0...)
        
        bodyData, closer, err := db.Get(bodyKey)
        if err == nil {
            closer.Close()
            fmt.Printf("Body found: %d bytes\n", len(bodyData))
        } else {
            fmt.Println("Body not found (empty block)")
        }
    }
}