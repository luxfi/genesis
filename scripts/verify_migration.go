package main

import (
    "encoding/binary"
    "fmt"
    "path/filepath"
    
    "github.com/cockroachdb/pebble"
    "github.com/luxfi/database/badgerdb"
    "github.com/luxfi/geth/common"
    "github.com/luxfi/geth/core/rawdb"
)

func main() {
    fmt.Println("=== Verifying Migrated Database ===")
    
    // Open ethdb
    ethdbPath := "/Users/z/.luxd/network-96369/chains/EWi9aPkGe6EfJ3SobCAmSUXRPLa4brF3cThwPwmHTrD1y13jy/ethdb"
    bdb, err := badgerdb.New(filepath.Clean(ethdbPath), nil, "", nil)
    if err != nil {
        panic(fmt.Sprintf("Failed to open ethdb: %v", err))
    }
    defer bdb.Close()
    
    // Check heads
    headHash := rawdb.ReadHeadBlockHash(bdb)
    fmt.Printf("Head block hash: %s\n", headHash.Hex())
    
    headHeader := rawdb.ReadHeadHeaderHash(bdb)
    fmt.Printf("Head header hash: %s\n", headHeader.Hex())
    
    // Check genesis
    genesisHash := rawdb.ReadCanonicalHash(bdb, 0)
    fmt.Printf("Genesis hash: %s\n", genesisHash.Hex())
    
    // Check chain config
    if chainConfig := rawdb.ReadChainConfig(bdb, genesisHash); chainConfig != nil {
        fmt.Printf("Chain config found with ID: %v\n", chainConfig.ChainID)
    } else {
        fmt.Println("No chain config found")
    }
    
    // Check a few blocks
    for i := uint64(0); i <= 5; i++ {
        hash := rawdb.ReadCanonicalHash(bdb, i)
        if hash != (common.Hash{}) {
            header := rawdb.ReadHeader(bdb, hash, i)
            if header != nil {
                fmt.Printf("Block #%d: %s (timestamp: %d)\n", i, hash.Hex(), header.Time)
            }
        }
    }
    
    // Check block 1000000
    hash1M := rawdb.ReadCanonicalHash(bdb, 1000000)
    if hash1M != (common.Hash{}) {
        fmt.Printf("Block #1000000: %s\n", hash1M.Hex())
    }
    
    // Check VM metadata
    vmPath := "/Users/z/.luxd/network-96369/chains/EWi9aPkGe6EfJ3SobCAmSUXRPLa4brF3cThwPwmHTrD1y13jy/vm"
    vdb, err := pebble.Open(filepath.Clean(vmPath), &pebble.Options{ReadOnly: true})
    if err != nil {
        fmt.Printf("Failed to open VM db: %v\n", err)
    } else {
        defer vdb.Close()
        
        if val, closer, err := vdb.Get([]byte("lastAccepted")); err == nil {
            defer closer.Close()
            var hash common.Hash
            copy(hash[:], val)
            fmt.Printf("VM lastAccepted: %s\n", hash.Hex())
        }
        
        if val, closer, err := vdb.Get([]byte("lastAcceptedHeight")); err == nil {
            defer closer.Close()
            height := binary.BigEndian.Uint64(val)
            fmt.Printf("VM lastAcceptedHeight: %d\n", height)
        }
        
        if val, closer, err := vdb.Get([]byte("initialized")); err == nil {
            defer closer.Close()
            fmt.Printf("VM initialized: %v\n", val[0] == 1)
        }
    }
    
    fmt.Println("=== Verification Complete ===")
}