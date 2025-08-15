package main

import (
    "bytes"
    "fmt"
    "log"
    
    "github.com/cockroachdb/pebble"
    "github.com/ethereum/go-ethereum/rlp"
)

// SubnetEVM namespace prefix (32 bytes)
var subnetNamespace = []byte{
    0x33, 0x7f, 0xb7, 0x3f, 0x9b, 0xcd, 0xac, 0x8c,
    0x31, 0xa2, 0xd5, 0xf7, 0xb8, 0x77, 0xab, 0x1e,
    0x8a, 0x2b, 0x7f, 0x2a, 0x1e, 0x9b, 0xf0, 0x2a,
    0x0a, 0x0e, 0x6c, 0x6f, 0xd1, 0x64, 0xf1, 0xd1,
}

type Header struct {
    ParentHash  []byte
    UncleHash   []byte
    Coinbase    []byte
    Root        []byte
    TxHash      []byte
    ReceiptHash []byte
    Bloom       []byte
    Difficulty  []byte
    Number      []byte
    GasLimit    []byte
    GasUsed     []byte
    Time        []byte
    Extra       []byte
    MixDigest   []byte
    Nonce       []byte
}

func main() {
    sourcePath := "/Users/z/work/lux/state/chaindata/lux-mainnet-96369/db/pebbledb"
    
    fmt.Println("Scanning Database Values")
    fmt.Println("========================")
    
    // Open source PebbleDB
    db, err := pebble.Open(sourcePath, &pebble.Options{
        ReadOnly: true,
    })
    if err != nil {
        log.Fatal("Failed to open database:", err)
    }
    defer db.Close()
    
    iter, err := db.NewIter(nil)
    if err != nil {
        log.Fatal("Failed to create iterator:", err)
    }
    defer iter.Close()
    
    // Look at first 20 values to understand structure
    count := 0
    headerCount := 0
    blocksFound := make(map[uint64]bool)
    
    for iter.First(); iter.Valid() && count < 100000; iter.Next() {
        key := iter.Key()
        val := iter.Value()
        count++
        
        // Check if this might be a block header (RLP encoded)
        if len(val) > 500 && (val[0] == 0xf8 || val[0] == 0xf9 || val[0] == 0xfa) {
            // Try to decode as header
            var header Header
            if err := rlp.DecodeBytes(val, &header); err == nil {
                if len(header.Number) > 0 && len(header.Number) <= 8 {
                    blockNum := uint64(0)
                    for _, b := range header.Number {
                        blockNum = (blockNum << 8) | uint64(b)
                    }
                    
                    if blockNum < 10000000 { // Reasonable block number
                        blocksFound[blockNum] = true
                        headerCount++
                        
                        if headerCount <= 10 || blockNum == 1000000 || blockNum == 1082780 {
                            fmt.Printf("Found header at block %d:\n", blockNum)
                            fmt.Printf("  Key: %x\n", key[:min(len(key), 40)])
                            fmt.Printf("  Value length: %d bytes\n", len(val))
                            
                            // Check if key is namespaced
                            if len(key) == 64 && bytes.HasPrefix(key, subnetNamespace) {
                                fmt.Printf("  Key is namespaced, actual key: %x\n", key[32:])
                            }
                        }
                    }
                }
            }
        }
        
        // Show a few samples to understand key structure
        if count <= 5 {
            fmt.Printf("\nSample %d:\n", count)
            fmt.Printf("  Key len: %d, Val len: %d\n", len(key), len(val))
            fmt.Printf("  Key: %x\n", key[:min(len(key), 40)])
            
            // Check if key is namespaced
            if len(key) == 64 && bytes.HasPrefix(key, subnetNamespace) {
                fmt.Printf("  Namespaced key, actual: %x\n", key[32:])
            }
            
            if len(val) > 500 {
                fmt.Printf("  Large value, first bytes: %x\n", val[:20])
            }
        }
    }
    
    fmt.Printf("\nScanned %d entries, found %d headers\n", count, headerCount)
    fmt.Printf("Total unique blocks: %d\n", len(blocksFound))
    
    if len(blocksFound) > 0 {
        // Find min and max
        minBlock := uint64(^uint64(0))
        maxBlock := uint64(0)
        hasBlock1M := false
        
        for height := range blocksFound {
            if height < minBlock {
                minBlock = height
            }
            if height > maxBlock {
                maxBlock = height
            }
            if height == 1000000 {
                hasBlock1M = true
            }
        }
        
        fmt.Printf("Block range: %d to %d\n", minBlock, maxBlock)
        
        if hasBlock1M {
            fmt.Println("\n✓ Block 1,000,000 is available in the database!")
        } else if 1000000 <= maxBlock {
            fmt.Println("\n✗ Block 1,000,000 is missing but should be in range")
        } else {
            fmt.Printf("\n✗ Block 1,000,000 is beyond the available range (max: %d)\n", maxBlock)
        }
    }
}

func min(a, b int) int {
    if a < b {
        return a
    }
    return b
}