package main

import (
    "encoding/binary"
    "fmt"
    "path/filepath"
    
    "github.com/luxfi/database/badgerdb"
    "github.com/luxfi/geth/common"
    "github.com/luxfi/geth/core/types"
    "github.com/luxfi/geth/rlp"
)

func main() {
    fmt.Println("=== Query Blocks from Migrated Database ===")
    
    // Open the migrated database
    ethdbPath := "/Users/z/.luxd/network-96369/chains/EWi9aPkGe6EfJ3SobCAmSUXRPLa4brF3cThwPwmHTrD1y13jy/ethdb"
    db, err := badgerdb.New(filepath.Clean(ethdbPath), nil, "", nil)
    if err != nil {
        panic(fmt.Sprintf("Failed to open ethdb: %v", err))
    }
    defer db.Close()
    
    // Query function
    queryBlock := func(number uint64) {
        fmt.Printf("\n--- Block #%d ---\n", number)
        
        // Get canonical hash
        canonKey := make([]byte, 10)
        canonKey[0] = 'h'
        binary.BigEndian.PutUint64(canonKey[1:9], number)
        canonKey[9] = 'n'
        
        hashBytes, err := db.Get(canonKey)
        if err != nil {
            fmt.Printf("  ✗ Not found: %v\n", err)
            return
        }
        
        var hash common.Hash
        copy(hash[:], hashBytes)
        fmt.Printf("  Hash: %s\n", hash.Hex())
        
        // Get header
        headerKey := make([]byte, 41)
        headerKey[0] = 'h'
        binary.BigEndian.PutUint64(headerKey[1:9], number)
        copy(headerKey[9:], hash[:])
        
        headerData, err := db.Get(headerKey)
        if err != nil {
            fmt.Printf("  ✗ Header not found: %v\n", err)
            return
        }
        
        // Decode header
        var header types.Header
        if err := rlp.DecodeBytes(headerData, &header); err != nil {
            // Try old header format
            fmt.Printf("  ! Header decode with new format failed, trying compatibility\n")
            fmt.Printf("  Header size: %d bytes\n", len(headerData))
        } else {
            fmt.Printf("  ✓ Header decoded:\n")
            fmt.Printf("    Parent: %s\n", header.ParentHash.Hex())
            fmt.Printf("    Time: %d\n", header.Time)
            fmt.Printf("    GasLimit: %d\n", header.GasLimit)
            fmt.Printf("    GasUsed: %d\n", header.GasUsed)
            fmt.Printf("    Number: %d\n", header.Number.Uint64())
        }
        
        // Get body
        bodyKey := make([]byte, 41)
        bodyKey[0] = 'b'
        binary.BigEndian.PutUint64(bodyKey[1:9], number)
        copy(bodyKey[9:], hash[:])
        
        bodyData, err := db.Get(bodyKey)
        if err != nil {
            fmt.Printf("  Body: not found\n")
        } else {
            fmt.Printf("  Body: %d bytes\n", len(bodyData))
            
            // Try to decode body
            var body types.Body
            if err := rlp.DecodeBytes(bodyData, &body); err != nil {
                fmt.Printf("    ! Body decode failed: %v\n", err)
            } else {
                fmt.Printf("    Transactions: %d\n", len(body.Transactions))
                fmt.Printf("    Uncles: %d\n", len(body.Uncles))
            }
        }
    }
    
    // Query specific blocks
    blocks := []uint64{0, 1, 2, 100, 1000, 10000, 100000, 500000, 1000000, 1082780}
    
    for _, blockNum := range blocks {
        queryBlock(blockNum)
    }
    
    fmt.Println("\n=== Query Complete ===")
}