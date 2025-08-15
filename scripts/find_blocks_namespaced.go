package main

import (
    "bytes"
    "encoding/binary"
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
    
    fmt.Println("Finding Blocks with Namespace")
    fmt.Println("==============================")
    
    // Open source PebbleDB
    db, err := pebble.Open(sourcePath, &pebble.Options{
        ReadOnly: true,
    })
    if err != nil {
        log.Fatal("Failed to open database:", err)
    }
    defer db.Close()
    
    // Try to get canonical hash for block 0 with namespace
    key0 := make([]byte, 41) // 32 namespace + 9 canonical key
    copy(key0[:32], subnetNamespace)
    key0[32] = 'H'
    binary.BigEndian.PutUint64(key0[33:], 0)
    
    val, closer, err := db.Get(key0)
    if err == nil {
        defer closer.Close()
        fmt.Printf("Block 0 canonical hash (namespaced): %x\n", val)
    } else {
        fmt.Printf("Block 0 not found with namespaced key\n")
    }
    
    // Try to get canonical hash for block 1000000 with namespace
    key1M := make([]byte, 41)
    copy(key1M[:32], subnetNamespace)
    key1M[32] = 'H'
    binary.BigEndian.PutUint64(key1M[33:], 1000000)
    
    val2, closer2, err := db.Get(key1M)
    if err == nil {
        defer closer2.Close()
        fmt.Printf("Block 1000000 canonical hash (namespaced): %x\n", val2)
    } else {
        fmt.Printf("Block 1000000 not found with namespaced key\n")
    }
    
    // Try to scan for headers and decode block numbers
    fmt.Println("\nScanning for block headers...")
    
    iter, err := db.NewIter(nil)
    if err != nil {
        log.Fatal("Failed to create iterator:", err)
    }
    defer iter.Close()
    
    blocksFound := make(map[uint64]bool)
    samplesShown := 0
    totalScanned := 0
    
    for iter.First(); iter.Valid() && totalScanned < 100000; iter.Next() {
        key := iter.Key()
        _ = iter.Value() // Not used but needed to advance iterator
        totalScanned++
        
        // Check if this is a namespaced hash key (64 bytes)
        if len(key) == 64 && bytes.HasPrefix(key, subnetNamespace) {
            hash := key[32:]
            
            // Try to read the header for this hash
            headerKey := make([]byte, 65) // namespace + 'h' + hash
            copy(headerKey[:32], subnetNamespace)
            headerKey[32] = 'h'
            copy(headerKey[33:], hash)
            
            headerVal, closer, err := db.Get(headerKey)
            if err == nil {
                defer closer.Close()
                
                // Try to decode as RLP header
                var header Header
                if err := rlp.DecodeBytes(headerVal, &header); err == nil {
                    if len(header.Number) > 0 && len(header.Number) <= 8 {
                        blockNum := uint64(0)
                        for _, b := range header.Number {
                            blockNum = (blockNum << 8) | uint64(b)
                        }
                        
                        if blockNum < 10000000 { // Reasonable block number
                            blocksFound[blockNum] = true
                            
                            if samplesShown < 10 || blockNum == 1000000 || blockNum == 1082780 {
                                fmt.Printf("Found block %d: hash=%x\n", blockNum, hash[:8])
                                samplesShown++
                            }
                        }
                    }
                }
            }
        }
    }
    
    fmt.Printf("\nTotal blocks found: %d\n", len(blocksFound))
    
    if len(blocksFound) > 0 {
        // Find min and max
        minBlock := uint64(^uint64(0))
        maxBlock := uint64(0)
        
        for height := range blocksFound {
            if height < minBlock {
                minBlock = height
            }
            if height > maxBlock {
                maxBlock = height
            }
        }
        
        fmt.Printf("Block range: %d to %d\n", minBlock, maxBlock)
        
        // Check for specific blocks
        checkBlocks := []uint64{0, 1, 100, 1000, 10000, 100000, 500000, 1000000, 1082780}
        fmt.Println("\nChecking specific blocks:")
        for _, block := range checkBlocks {
            if blocksFound[block] {
                fmt.Printf("  ✓ Block %d found\n", block)
            } else if block <= maxBlock {
                fmt.Printf("  ✗ Block %d missing (within range)\n", block)
            } else {
                fmt.Printf("  - Block %d beyond range\n", block)
            }
        }
    }
}