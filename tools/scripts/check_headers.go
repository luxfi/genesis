package main

import (
    "encoding/binary"
    "fmt"
    "log"
    
    "github.com/dgraph-io/badger/v4"
    "github.com/luxfi/geth/common"
)

func main() {
    // Open the ethdb database
    dbPath := "/Users/z/.luxd/network-96369/chains/EWi9aPkGe6EfJ3SobCAmSUXRPLa4brF3cThwPwmHTrD1y13jy/ethdb"
    
    opts := badger.DefaultOptions(dbPath).WithReadOnly(true)
    db, err := badger.Open(opts)
    if err != nil {
        log.Fatal("Failed to open database:", err)
    }
    defer db.Close()
    
    fmt.Println("Checking for block headers...")
    
    // Check for block 1082780 header
    // The key format should be 'h' + num(8) + hash(32)
    err = db.View(func(txn *badger.Txn) error {
        // Get the canonical hash first
        canonicalKey := make([]byte, 9)
        canonicalKey[0] = 'H'
        binary.BigEndian.PutUint64(canonicalKey[1:], 1082780)
        
        item, err := txn.Get(canonicalKey)
        if err != nil {
            fmt.Printf("Failed to get canonical hash for block 1082780: %v\n", err)
            return nil
        }
        
        hashBytes, _ := item.ValueCopy(nil)
        var hash common.Hash
        copy(hash[:], hashBytes)
        fmt.Printf("Canonical hash for block 1082780: %x\n", hash)
        
        // Now check for the header with key 'h' + num + hash
        headerKey := make([]byte, 41)
        headerKey[0] = 'h'
        binary.BigEndian.PutUint64(headerKey[1:9], 1082780)
        copy(headerKey[9:], hash[:])
        
        item, err = txn.Get(headerKey)
        if err != nil {
            fmt.Printf("Header key %x not found: %v\n", headerKey, err)
            
            // Try lowercase 'h' + just number
            headerKey2 := make([]byte, 9)
            headerKey2[0] = 'h'
            binary.BigEndian.PutUint64(headerKey2[1:], 1082780)
            
            item, err = txn.Get(headerKey2)
            if err != nil {
                fmt.Printf("Header key %x not found either: %v\n", headerKey2, err)
            } else {
                val, _ := item.ValueCopy(nil)
                fmt.Printf("Found header with simple key %x: %d bytes\n", headerKey2, len(val))
            }
        } else {
            val, _ := item.ValueCopy(nil)
            fmt.Printf("Found header for block 1082780: %d bytes\n", len(val))
        }
        
        // Also check for block body
        bodyKey := make([]byte, 41)
        bodyKey[0] = 'b'
        binary.BigEndian.PutUint64(bodyKey[1:9], 1082780)
        copy(bodyKey[9:], hash[:])
        
        item, err = txn.Get(bodyKey)
        if err != nil {
            fmt.Printf("Body not found with key %x\n", bodyKey[:20])
        } else {
            val, _ := item.ValueCopy(nil)
            fmt.Printf("Found body for block 1082780: %d bytes\n", len(val))
        }
        
        return nil
    })
    
    if err != nil {
        log.Fatal("Error:", err)
    }
}