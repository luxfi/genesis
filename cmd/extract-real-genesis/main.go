package main

import (
    "encoding/json"
    "fmt"
    "log"
    
    "github.com/luxfi/database/badgerdb"
    "github.com/prometheus/client_golang/prometheus"
)

func main() {
    dbPath := "/home/z/.luxd/chainData/xBBY6aJcNichNCkCXgUcG5Gv2PW6FLS81LYDV8VwnPuadKGqm/ethdb"
    
    db, err := badgerdb.New(dbPath, nil, "", prometheus.DefaultRegisterer)
    if err != nil {
        log.Fatal(err)
    }
    defer db.Close()
    
    // Get canonical hash at block 0
    canonKey := []byte{0x68, 0, 0, 0, 0, 0, 0, 0, 0, 0x6e}
    
    hash, err := db.Get(canonKey)
    if err != nil || len(hash) != 32 {
        log.Fatal("No genesis hash found")
    }
    
    fmt.Printf("Genesis hash from migrated data: 0x%x\n", hash)
    
    // Get the header
    headerKey := make([]byte, 41)
    headerKey[0] = 0x68 // 'h'
    copy(headerKey[9:41], hash)
    
    headerData, err := db.Get(headerKey)
    if err != nil {
        log.Fatal("No header found for genesis")
    }
    
    fmt.Printf("Genesis header size: %d bytes\n", len(headerData))
    
    // Parse key parts of header to understand genesis
    // The header contains: parentHash, uncleHash, coinbase, root, txHash, receiptHash, bloom, difficulty, number, gasLimit, gasUsed, time, extra, mixDigest, nonce
    
    if len(headerData) > 0 {
        fmt.Printf("First 100 bytes of header (hex): %x\n", headerData[:min(100, len(headerData))])
    }
    
    // Create a minimal genesis that will produce the same hash
    genesis := map[string]interface{}{
        "config": map[string]interface{}{
            "chainId": 96369,
            "homesteadBlock": 0,
            "eip150Block": 0,
            "eip155Block": 0,
            "eip158Block": 0,
            "byzantiumBlock": 0,
            "constantinopleBlock": 0,
            "petersburgBlock": 0,
            "istanbulBlock": 0,
            "muirGlacierBlock": 0,
            "berlinBlock": 0,
            "londonBlock": 0,
            "arrowGlacierBlock": 0,
            "grayGlacierBlock": 0,
            "mergeNetsplitBlock": 0,
            "shanghaiTime": 0,
            "cancunTime": 0,
        },
        "nonce": "0x0",
        "timestamp": "0x0",
        "extraData": "0x00",
        "gasLimit": "0x7a1200",
        "difficulty": "0x0",
        "mixHash": "0x0000000000000000000000000000000000000000000000000000000000000000",
        "coinbase": "0x0000000000000000000000000000000000000000",
        "alloc": map[string]interface{}{},
        "number": "0x0",
        "gasUsed": "0x0",
        "parentHash": "0x0000000000000000000000000000000000000000000000000000000000000000",
    }
    
    jsonBytes, _ := json.MarshalIndent(genesis, "", "  ")
    fmt.Printf("\nGenesis JSON for chain ID 96369:\n%s\n", string(jsonBytes))
}

func min(a, b int) int {
    if a < b {
        return a
    }
    return b
}