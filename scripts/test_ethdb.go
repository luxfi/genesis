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
    
    fmt.Println("Database opened successfully")
    
    // Try to find blocks
    foundBlocks := 0
    
    // Check for standard Coreth keys
    err = db.View(func(txn *badger.Txn) error {
        // Check for last block
        item, err := txn.Get([]byte("LastBlock"))
        if err == nil {
            val, _ := item.ValueCopy(nil)
            if len(val) == 32 {
                var hash common.Hash
                copy(hash[:], val)
                fmt.Printf("Found LastBlock: %x\n", hash)
            }
        }
        
        // Check for specific block headers with 'h' prefix
        for i := uint64(0); i <= 10; i++ {
            // Try 'h' + block number format (migrated)
            key := make([]byte, 9)
            key[0] = 'h'
            binary.BigEndian.PutUint64(key[1:], i)
            
            opts := badger.DefaultIteratorOptions
            opts.PrefetchValues = false
            it := txn.NewIterator(opts)
            defer it.Close()
            
            it.Seek(key)
            if it.Valid() {
                k := it.Item().Key()
                if len(k) >= 9 && k[0] == 'h' {
                    blockNum := binary.BigEndian.Uint64(k[1:9])
                    if blockNum == i {
                        fmt.Printf("Found block %d with key length %d\n", i, len(k))
                        foundBlocks++
                    }
                }
            }
        }
        
        // Check for canonical hash keys 'H' + number
        for i := uint64(0); i <= 10; i++ {
            key := make([]byte, 9)
            key[0] = 'H'
            binary.BigEndian.PutUint64(key[1:], i)
            
            item, err := txn.Get(key)
            if err == nil {
                val, _ := item.ValueCopy(nil)
                if len(val) == 32 {
                    var hash common.Hash
                    copy(hash[:], val)
                    fmt.Printf("Found canonical block %d: %x\n", i, hash)
                    foundBlocks++
                }
            }
        }
        
        return nil
    })
    
    if err != nil {
        log.Fatal("Error reading database:", err)
    }
    
    fmt.Printf("\nTotal blocks found: %d\n", foundBlocks)
    
    // List some keys to see what's actually in the database
    fmt.Println("\nFirst 20 keys in database:")
    count := 0
    err = db.View(func(txn *badger.Txn) error {
        opts := badger.DefaultIteratorOptions
        opts.PrefetchValues = false
        it := txn.NewIterator(opts)
        defer it.Close()
        
        for it.Rewind(); it.Valid() && count < 20; it.Next() {
            key := it.Item().Key()
            fmt.Printf("  Key[%d]: %x (len=%d)\n", count, key, len(key))
            if len(key) > 0 {
                fmt.Printf("    First byte: '%c' (0x%02x)\n", key[0], key[0])
            }
            count++
        }
        return nil
    })
    
    if err != nil {
        log.Fatal("Error listing keys:", err)
    }
}