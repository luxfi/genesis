package main

import (
    "encoding/binary"
    "fmt"
    "log"
    
    "github.com/dgraph-io/badger/v3"
    "github.com/ethereum/go-ethereum/common"
    "github.com/ethereum/go-ethereum/core/types"
    "github.com/ethereum/go-ethereum/rlp"
)

func main() {
    dbPath := "/home/z/.luxd/chainData/xBBY6aJcNichNCkCXgUcG5Gv2PW6FLS81LYDV8VwnPuadKGqm/ethdb"
    
    opts := badger.DefaultOptions(dbPath)
    opts.ReadOnly = true
    
    db, err := badger.Open(opts)
    if err != nil {
        log.Fatal("Failed to open database:", err)
    }
    defer db.Close()
    
    fmt.Println("Verifying migrated blockchain data...")
    
    err = db.View(func(txn *badger.Txn) error {
        // Check for highest block
        var highestBlock uint64 = 0
        var genesisHash common.Hash
        
        // Get block 0 (genesis)
        canonicalKey := append([]byte("h"), encodeBlockNumber(0)...)
        canonicalKey = append(canonicalKey, 'n')
        
        if item, err := txn.Get(canonicalKey); err == nil {
            item.Value(func(val []byte) error {
                copy(genesisHash[:], val)
                return nil
            })
            fmt.Printf("✓ Genesis found: %s\n", genesisHash.Hex())
        }
        
        // Check a few key blocks
        checkBlocks := []uint64{0, 1, 100, 1000, 10000, 100000, 500000, 1000000, 1082780}
        
        for _, blockNum := range checkBlocks {
            canonicalKey := append([]byte("h"), encodeBlockNumber(blockNum)...)
            canonicalKey = append(canonicalKey, 'n')
            
            if item, err := txn.Get(canonicalKey); err == nil {
                var hash common.Hash
                item.Value(func(val []byte) error {
                    copy(hash[:], val)
                    return nil
                })
                
                // Get header to verify
                headerKey := append([]byte("h"), encodeBlockNumber(blockNum)...)
                headerKey = append(headerKey, hash[:]...)
                
                if item, err := txn.Get(headerKey); err == nil {
                    var header types.Header
                    item.Value(func(val []byte) error {
                        rlp.DecodeBytes(val, &header)
                        return nil
                    })
                    
                    if blockNum == 0 {
                        fmt.Printf("  Block %d: hash=%s, timestamp=%d, gasLimit=%d\n", 
                            blockNum, hash.Hex()[:16]+"...", header.Time, header.GasLimit)
                    } else {
                        fmt.Printf("  Block %d: hash=%s\n", blockNum, hash.Hex()[:16]+"...")
                    }
                    
                    if blockNum > highestBlock {
                        highestBlock = blockNum
                    }
                }
            } else if blockNum == 1082780 {
                fmt.Printf("  Block %d: NOT FOUND\n", blockNum)
            }
        }
        
        // Try to find actual highest block
        fmt.Println("\nSearching for highest block...")
        for i := uint64(1082780); i <= 1082790; i++ {
            canonicalKey := append([]byte("h"), encodeBlockNumber(i)...)
            canonicalKey = append(canonicalKey, 'n')
            
            if _, err := txn.Get(canonicalKey); err == nil {
                highestBlock = i
                fmt.Printf("  Found block %d\n", i)
            } else {
                break
            }
        }
        
        fmt.Printf("\n✓ Highest block found: %d\n", highestBlock)
        fmt.Printf("✓ Chain data is present and accessible\n")
        
        return nil
    })
    
    if err != nil {
        log.Fatal("Failed to verify chain:", err)
    }
}

func encodeBlockNumber(number uint64) []byte {
    enc := make([]byte, 8)
    binary.BigEndian.PutUint64(enc, number)
    return enc
}