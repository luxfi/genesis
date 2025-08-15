package main

import (
    "encoding/binary"
    "fmt"
    "path/filepath"
    
    "github.com/cockroachdb/pebble"
    "github.com/luxfi/database/badgerdb"
    "github.com/luxfi/geth/common"
)

func main() {
    fmt.Println("=== Direct Database Boot Test ===")
    
    // Test 1: Check migrated ethdb
    ethdbPath := "/Users/z/.luxd/network-96369/chains/EWi9aPkGe6EfJ3SobCAmSUXRPLa4brF3cThwPwmHTrD1y13jy/ethdb"
    fmt.Printf("\nOpening ethdb at: %s\n", ethdbPath)
    
    bdb, err := badgerdb.New(filepath.Clean(ethdbPath), nil, "", nil)
    if err != nil {
        panic(fmt.Sprintf("Failed to open ethdb: %v", err))
    }
    defer bdb.Close()
    
    // Direct key reads using raw badger
    fmt.Println("\n--- Checking database contents ---")
    
    // Check LastHeader
    if val, err := bdb.Get([]byte("LastHeader")); err == nil {
        var hash common.Hash
        copy(hash[:], val)
        fmt.Printf("✓ LastHeader: %s\n", hash.Hex())
    } else {
        fmt.Printf("✗ LastHeader not found: %v\n", err)
    }
    
    // Check LastBlock
    if val, err := bdb.Get([]byte("LastBlock")); err == nil {
        var hash common.Hash
        copy(hash[:], val)
        fmt.Printf("✓ LastBlock: %s\n", hash.Hex())
    } else {
        fmt.Printf("✗ LastBlock not found: %v\n", err)
    }
    
    // Check LastFast
    if val, err := bdb.Get([]byte("LastFast")); err == nil {
        var hash common.Hash
        copy(hash[:], val)
        fmt.Printf("✓ LastFast: %s\n", hash.Hex())
    } else {
        fmt.Printf("✗ LastFast not found: %v\n", err)
    }
    
    // Check canonical hash at 0
    canonKey := make([]byte, 10)
    canonKey[0] = 'h'
    binary.BigEndian.PutUint64(canonKey[1:9], 0)
    canonKey[9] = 'n'
    
    if val, err := bdb.Get(canonKey); err == nil {
        var hash common.Hash
        copy(hash[:], val)
        fmt.Printf("✓ Genesis (canonical[0]): %s\n", hash.Hex())
        
        // Try to read the genesis header
        headerKey := make([]byte, 41)
        headerKey[0] = 'h'
        binary.BigEndian.PutUint64(headerKey[1:9], 0)
        copy(headerKey[9:], hash[:])
        
        if hdrData, err := bdb.Get(headerKey); err == nil {
            fmt.Printf("✓ Genesis header found: %d bytes\n", len(hdrData))
        } else {
            fmt.Printf("✗ Genesis header not found: %v\n", err)
        }
    } else {
        fmt.Printf("✗ Canonical[0] not found: %v\n", err)
    }
    
    // Check canonical at tip
    canonTipKey := make([]byte, 10)
    canonTipKey[0] = 'h'
    binary.BigEndian.PutUint64(canonTipKey[1:9], 1082780)
    canonTipKey[9] = 'n'
    
    if val, err := bdb.Get(canonTipKey); err == nil {
        var hash common.Hash
        copy(hash[:], val)
        fmt.Printf("✓ Tip (canonical[1082780]): %s\n", hash.Hex())
    } else {
        fmt.Printf("✗ Canonical[1082780] not found: %v\n", err)
    }
    
    // Test 2: Check VM database
    vmPath := "/Users/z/.luxd/network-96369/chains/EWi9aPkGe6EfJ3SobCAmSUXRPLa4brF3cThwPwmHTrD1y13jy/vm"
    fmt.Printf("\n--- Checking VM database ---\n")
    
    vdb, err := pebble.Open(filepath.Clean(vmPath), &pebble.Options{ReadOnly: true})
    if err != nil {
        fmt.Printf("✗ Failed to open VM db: %v\n", err)
    } else {
        defer vdb.Close()
        
        if val, closer, err := vdb.Get([]byte("lastAccepted")); err == nil {
            defer closer.Close()
            var hash common.Hash
            copy(hash[:], val)
            fmt.Printf("✓ VM lastAccepted: %s\n", hash.Hex())
        }
        
        if val, closer, err := vdb.Get([]byte("lastAcceptedHeight")); err == nil {
            defer closer.Close()
            height := binary.BigEndian.Uint64(val)
            fmt.Printf("✓ VM lastAcceptedHeight: %d\n", height)
        }
        
        if val, closer, err := vdb.Get([]byte("initialized")); err == nil {
            defer closer.Close()
            fmt.Printf("✓ VM initialized: %v\n", val[0] == 1)
        }
    }
    
    fmt.Println("\n=== Database Ready for Boot ===")
}