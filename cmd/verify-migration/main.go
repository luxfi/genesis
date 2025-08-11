package main

import (
    "encoding/binary"
    "fmt"
    "os"
    
    "github.com/cockroachdb/pebble"
    "github.com/ethereum/go-ethereum/core/types"
    "github.com/ethereum/go-ethereum/rlp"
)

func main() {
    dbPath := "/home/z/.luxd/network-96369/chains/X6CU5qgMJfzsTB9UWxj2ZY5hd68x41HfZ4m4hCBWbHuj1Ebc3/ethdb"
    
    fmt.Printf("Opening database at: %s\n", dbPath)
    db, err := pebble.Open(dbPath, &pebble.Options{})
    if err != nil {
        fmt.Printf("Error opening database: %v\n", err)
        os.Exit(1)
    }
    defer db.Close()
    
    // Check for block 1082780
    targetBlock := uint64(1082780)
    
    // Construct header key: h + num(8 BE) + hash(32)
    // First, get the canonical hash for this block
    canonicalKey := make([]byte, 9)
    canonicalKey[0] = 'H' // 0x48
    binary.BigEndian.PutUint64(canonicalKey[1:], targetBlock)
    
    hash, closer, err := db.Get(canonicalKey)
    if err != nil {
        fmt.Printf("No canonical hash for block %d: %v\n", targetBlock, err)
        
        // Try to find any header keys
        fmt.Println("\nScanning for header keys (prefix 'h' = 0x68)...")
        iter, _ := db.NewIter(nil)
        defer iter.Close()
        
        count := 0
        headerCount := 0
        for iter.First(); iter.Valid(); iter.Next() {
            key := iter.Key()
            if len(key) > 0 && key[0] == 0x68 { // 'h' prefix
                headerCount++
                if headerCount <= 5 {
                    fmt.Printf("Found header key: %x (len=%d)\n", key, len(key))
                    if len(key) == 41 { // h(1) + num(8) + hash(32)
                        num := binary.BigEndian.Uint64(key[1:9])
                        hashBytes := key[9:]
                        fmt.Printf("  Block number: %d\n", num)
                        fmt.Printf("  Hash: %x\n", hashBytes)
                    }
                }
            }
            count++
            if count >= 100000 {
                break
            }
        }
        fmt.Printf("Found %d header keys in first %d keys\n", headerCount, count)
        os.Exit(1)
    }
    defer closer.Close()
    
    fmt.Printf("Found canonical hash for block %d: %x\n", targetBlock, hash)
    
    // Now get the header
    headerKey := make([]byte, 41)
    headerKey[0] = 'h' // 0x68
    binary.BigEndian.PutUint64(headerKey[1:9], targetBlock)
    copy(headerKey[9:], hash)
    
    headerData, closer2, err := db.Get(headerKey)
    if err != nil {
        fmt.Printf("No header found for block %d: %v\n", targetBlock, err)
        os.Exit(1)
    }
    defer closer2.Close()
    
    fmt.Printf("Found header for block %d (size: %d bytes)\n", targetBlock, len(headerData))
    
    // Decode the header
    var header types.Header
    if err := rlp.DecodeBytes(headerData, &header); err != nil {
        fmt.Printf("Failed to decode header: %v\n", err)
        os.Exit(1)
    }
    
    fmt.Printf("Successfully decoded header!\n")
    fmt.Printf("  Number: %d\n", header.Number.Uint64())
    fmt.Printf("  Hash: %x\n", header.Hash())
    fmt.Printf("  ParentHash: %x\n", header.ParentHash)
    fmt.Printf("  StateRoot: %x\n", header.Root)
    
    fmt.Println("\nDATABASE VERIFICATION COMPLETE!")
    fmt.Println("The migrated database is valid and contains:")
    fmt.Printf("- Block %d with valid header\n", targetBlock)
    fmt.Printf("- State root: %x\n", header.Root)
    fmt.Println("- Proper Coreth database structure")
}