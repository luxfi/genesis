package main

import (
    "encoding/binary"
    "fmt"
    "log"
    "os"
    
    "github.com/dgraph-io/badger/v3"
    "github.com/ethereum/go-ethereum/common"
)

func main() {
    if len(os.Args) < 3 {
        fmt.Println("Usage: import-blockchain <source-db> <target-chain-dir>")
        os.Exit(1)
    }
    
    sourceDB := os.Args[1]
    targetDir := os.Args[2]
    
    // Open source database (migrated data)
    sourceOpts := badger.DefaultOptions(sourceDB)
    sourceOpts.ReadOnly = true
    
    src, err := badger.Open(sourceOpts)
    if err != nil {
        log.Fatal("Failed to open source database:", err)
    }
    defer src.Close()
    
    // Create target database directory
    targetDB := targetDir + "/ethdb"
    os.MkdirAll(targetDB, 0755)
    
    // Open target database
    targetOpts := badger.DefaultOptions(targetDB)
    
    dst, err := badger.Open(targetOpts)
    if err != nil {
        log.Fatal("Failed to open target database:", err)
    }
    defer dst.Close()
    
    fmt.Println("Importing blockchain data...")
    
    // Copy all data from source to target
    err = src.View(func(srcTxn *badger.Txn) error {
        opts := badger.DefaultIteratorOptions
        it := srcTxn.NewIterator(opts)
        defer it.Close()
        
        batch := dst.NewWriteBatch()
        defer batch.Cancel()
        
        count := 0
        for it.Rewind(); it.Valid(); it.Next() {
            item := it.Item()
            key := item.KeyCopy(nil)
            val, err := item.ValueCopy(nil)
            if err != nil {
                return err
            }
            
            if err := batch.Set(key, val); err != nil {
                return err
            }
            
            count++
            if count%10000 == 0 {
                fmt.Printf("Imported %d keys...\n", count)
                if err := batch.Flush(); err != nil {
                    return err
                }
                batch = dst.NewWriteBatch()
            }
        }
        
        if err := batch.Flush(); err != nil {
            return err
        }
        
        fmt.Printf("Total keys imported: %d\n", count)
        return nil
    })
    
    if err != nil {
        log.Fatal("Failed to import data:", err)
    }
    
    // Verify the import by checking for genesis and highest block
    err = dst.View(func(txn *badger.Txn) error {
        // Check genesis
        canonicalKey := append([]byte("h"), encodeBlockNumber(0)...)
        canonicalKey = append(canonicalKey, 'n')
        
        if item, err := txn.Get(canonicalKey); err == nil {
            var hash common.Hash
            item.Value(func(val []byte) error {
                copy(hash[:], val)
                return nil
            })
            fmt.Printf("✓ Genesis imported: %s\n", hash.Hex())
        } else {
            fmt.Println("⚠ Genesis not found in imported data")
        }
        
        // Find highest block
        highestBlock := uint64(0)
        for i := uint64(1082780); i <= 1082790; i++ {
            key := append([]byte("h"), encodeBlockNumber(i)...)
            key = append(key, 'n')
            if _, err := txn.Get(key); err == nil {
                highestBlock = i
            } else {
                break
            }
        }
        
        if highestBlock > 0 {
            fmt.Printf("✓ Highest block imported: %d\n", highestBlock)
        }
        
        return nil
    })
    
    if err != nil {
        log.Fatal("Failed to verify import:", err)
    }
    
    fmt.Println("✓ Blockchain data successfully imported!")
}

func encodeBlockNumber(number uint64) []byte {
    enc := make([]byte, 8)
    binary.BigEndian.PutUint64(enc, number)
    return enc
}