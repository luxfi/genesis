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
    
    fmt.Println("Analyzing the mismatch between H keys and headers...")
    
    // Get a sample header hash
    var sampleHeaderHash []byte
    iter, _ := db.NewIter(&pebble.IterOptions{})
    for iter.First(); iter.Valid(); iter.Next() {
        key := iter.Key()
        if len(key) == 65 && bytes.Equal(key[:32], namespace) && key[32] == 'h' {
            sampleHeaderHash = key[33:]
            break
        }
    }
    iter.Close()
    
    if sampleHeaderHash != nil {
        fmt.Printf("Sample header hash: %x\n", sampleHeaderHash)
        
        // Check if there's an H key with this hash
        hKey := append(namespace, 'H')
        hKey = append(hKey, sampleHeaderHash...)
        
        value, closer, err := db.Get(hKey)
        if err == nil {
            closer.Close()
            if len(value) == 8 {
                blockNum := binary.BigEndian.Uint64(value)
                fmt.Printf("Found H key for this hash! Block number: %d\n", blockNum)
            } else {
                fmt.Printf("Found H key but value is not 8 bytes: %x\n", value)
            }
        } else {
            fmt.Printf("No H key found for this header hash\n")
        }
    }
    
    // Let's check if the H keys might be transaction hashes instead
    fmt.Println("\nChecking if H keys are transaction hashes...")
    
    // Get block 0's H key hash
    block0HHash, _ := hex.DecodeString("3f4fa2a0b0ce089f52bf0ae9199c75ffdd76ecafc987794050cb0d286f1ec61e")
    
    // Check if this exists as a receipt
    receiptKey := append(namespace, 'r')
    receiptKey = append(receiptKey, block0HHash...)
    
    _, closer, err := db.Get(receiptKey)
    if err == nil {
        closer.Close()
        fmt.Printf("Found receipt with block 0's H hash!\n")
    } else {
        fmt.Printf("No receipt found with block 0's H hash\n")
    }
    
    // Let's look at what the 64-byte H keys are (not 65)
    fmt.Println("\nAnalyzing 64-byte H keys (namespace + hash only):")
    iter2, _ := db.NewIter(&pebble.IterOptions{})
    count64 := 0
    for iter2.First(); iter2.Valid() && count64 < 5; iter2.Next() {
        key := iter2.Key()
        if len(key) == 64 && bytes.Equal(key[:32], namespace) {
            // This is namespace + 32-byte value
            data := key[32:]
            
            // Check if first byte could be 'H'
            if data[0] == 'H' || data[0] == 0x48 {
                fmt.Printf("Found 64-byte key starting with H: %x\n", data)
                fmt.Printf("  Value: %x\n", iter2.Value())
                count64++
            }
        }
    }
    iter2.Close()
}