package main

import (
    "encoding/binary"
    "fmt"
    "log"
    
    "github.com/dgraph-io/badger/v4"
)

func main() {
    // Open the VM database
    vmDB, err := badger.Open(badger.DefaultOptions("/Users/z/.luxd/network-96369/chains/EWi9aPkGe6EfJ3SobCAmSUXRPLa4brF3cThwPwmHTrD1y13jy/vm"))
    if err != nil {
        log.Fatal("Failed to open VM database:", err)
    }
    defer vmDB.Close()
    
    // Mark the database as initialized
    err = vmDB.Update(func(txn *badger.Txn) error {
        // Write "initialized" flag
        if err := txn.Set([]byte("initialized"), []byte{1}); err != nil {
            return err
        }
        
        // Write last accepted block hash
        lastAccepted := []byte{
            0x32, 0xde, 0xde, 0x1f, 0xc8, 0xe0, 0xf1, 0x1e,
            0xcd, 0xe1, 0x2f, 0xb4, 0x2a, 0xef, 0x79, 0x33,
            0xfc, 0x6c, 0x5f, 0xcf, 0x86, 0x3b, 0xc2, 0x77,
            0xb5, 0xea, 0xc0, 0x8a, 0xe4, 0xd4, 0x61, 0xf0,
        }
        if err := txn.Set([]byte("lastAccepted"), lastAccepted); err != nil {
            return err
        }
        
        // Write height
        heightBytes := make([]byte, 8)
        binary.BigEndian.PutUint64(heightBytes, 1082780)
        if err := txn.Set([]byte("height"), heightBytes); err != nil {
            return err
        }
        
        fmt.Println("Marked VM database as initialized with migrated data")
        return nil
    })
    
    if err != nil {
        log.Fatal("Failed to mark database:", err)
    }
    
    fmt.Println("Successfully marked database for migration mode")
}