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
    
    fmt.Println("Checking canonical hash keys...")
    
    // Check for canonical hashes using standard format 'H' + number
    for i := uint64(0); i <= 10; i++ {
        key := make([]byte, 9)
        key[0] = 'H'
        binary.BigEndian.PutUint64(key[1:], i)
        
        err = db.View(func(txn *badger.Txn) error {
            item, err := txn.Get(key)
            if err == nil {
                val, _ := item.ValueCopy(nil)
                if len(val) == 32 {
                    var hash common.Hash
                    copy(hash[:], val)
                    fmt.Printf("Block %d: key=%x value=%x\n", i, key, hash)
                } else {
                    fmt.Printf("Block %d: key=%x value_len=%d (unexpected)\n", i, key, len(val))
                }
            } else {
                fmt.Printf("Block %d: key=%x NOT FOUND\n", i, key)
            }
            return nil
        })
    }
    
    // Also check block 1082780
    key := make([]byte, 9)
    key[0] = 'H'
    binary.BigEndian.PutUint64(key[1:], 1082780)
    
    err = db.View(func(txn *badger.Txn) error {
        item, err := txn.Get(key)
        if err == nil {
            val, _ := item.ValueCopy(nil)
            if len(val) == 32 {
                var hash common.Hash
                copy(hash[:], val)
                fmt.Printf("\nBlock 1082780: key=%x value=%x\n", key, hash)
            }
        } else {
            fmt.Printf("\nBlock 1082780: key=%x NOT FOUND\n", key)
        }
        return nil
    })
}