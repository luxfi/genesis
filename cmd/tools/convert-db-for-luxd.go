package main

import (
    "bytes"
    "encoding/binary"
    "encoding/hex"
    "fmt"
    "log"
    "os"
    
    "github.com/cockroachdb/pebble"
)

const (
    SOURCE_DB = "/home/z/work/lux/state/chaindata/lux-mainnet-96369/db/pebbledb"
    TARGET_DB = "/tmp/converted-genesis-db"
)

var namespace = []byte{
    0x33, 0x7f, 0xb7, 0x3f, 0x9b, 0xcd, 0xac, 0x8c,
    0x31, 0xa2, 0xd5, 0xf7, 0xb8, 0x77, 0xab, 0x1e,
    0x8a, 0x2b, 0x7f, 0x2a, 0x1e, 0x9b, 0xf0, 0x2a,
    0x0a, 0x0e, 0x6c, 0x6f, 0xd1, 0x64, 0xf1, 0xd1,
}

func main() {
    fmt.Println("=============================================================")
    fmt.Println("    Converting Subnet-EVM DB to Standard Format for luxd    ")
    fmt.Println("=============================================================")
    fmt.Println()
    fmt.Println("Source: " + SOURCE_DB)
    fmt.Println("Target: " + TARGET_DB)
    fmt.Println()
    
    // Remove target if exists
    os.RemoveAll(TARGET_DB)
    
    // Open source database
    srcDB, err := pebble.Open(SOURCE_DB, &pebble.Options{ReadOnly: true})
    if err != nil {
        log.Fatal("Failed to open source database:", err)
    }
    defer srcDB.Close()
    
    // Create target database
    targetDB, err := pebble.Open(TARGET_DB, &pebble.Options{})
    if err != nil {
        log.Fatal("Failed to create target database:", err)
    }
    defer targetDB.Close()
    
    fmt.Println("Phase 1: Converting block data...")
    fmt.Println("==================================")
    
    // First, collect all blocks with proper canonical mapping
    blocks := make(map[uint64][]byte) // blockNum -> hash
    headers := make(map[string][]byte) // hash -> header
    bodies := make(map[string][]byte)  // hash -> body
    
    iter, _ := srcDB.NewIter(&pebble.IterOptions{})
    defer iter.Close()
    
    totalKeys := 0
    for iter.First(); iter.Valid(); iter.Next() {
        key := make([]byte, len(iter.Key()))
        copy(key, iter.Key())
        value := make([]byte, len(iter.Value()))
        copy(value, iter.Value())
        
        totalKeys++
        
        // Process namespaced keys
        if len(key) == 64 && bytes.HasPrefix(key, namespace) {
            hash := key[32:]
            
            // Check if it's a header (RLP data)
            if len(value) > 100 && (value[0] == 0xf8 || value[0] == 0xf9) {
                // Extract block number from hash
                blockNum := uint64(hash[0])<<16 | uint64(hash[1])<<8 | uint64(hash[2])
                
                if blockNum < 1500000 {
                    blocks[blockNum] = hash
                    headers[hex.EncodeToString(hash)] = value
                    
                    if len(blocks)%100000 == 0 {
                        fmt.Printf("  Processed %d blocks...\n", len(blocks))
                    }
                }
            }
        } else if len(key) == 65 && bytes.HasPrefix(key, namespace) {
            // Check for bodies with 'b' prefix
            if key[32] == 'b' {
                hash := key[33:]
                bodies[hex.EncodeToString(hash)] = value
            }
        }
        
        if totalKeys%1000000 == 0 {
            fmt.Printf("  Scanned %d keys...\n", totalKeys)
        }
    }
    
    fmt.Printf("\n✅ Found %d blocks to convert\n", len(blocks))
    fmt.Printf("   Headers: %d\n", len(headers))
    fmt.Printf("   Bodies: %d\n\n", len(bodies))
    
    // Phase 2: Write to target database in standard format
    fmt.Println("Phase 2: Writing standard format database...")
    fmt.Println("============================================")
    
    batch := targetDB.NewBatch()
    written := 0
    
    for blockNum, hash := range blocks {
        // H key: 'H' + blockNum(8) -> hash (canonical mapping)
        hKey := []byte{'H'}
        blockNumBytes := make([]byte, 8)
        binary.BigEndian.PutUint64(blockNumBytes, blockNum)
        hKey = append(hKey, blockNumBytes...)
        
        if err := batch.Set(hKey, hash, nil); err != nil {
            log.Printf("Failed to write H key for block %d: %v", blockNum, err)
            continue
        }
        
        // h key: 'h' + hash -> header
        hashStr := hex.EncodeToString(hash)
        if header, exists := headers[hashStr]; exists {
            headerKey := append([]byte{'h'}, hash...)
            if err := batch.Set(headerKey, header, nil); err != nil {
                log.Printf("Failed to write header for block %d: %v", blockNum, err)
                continue
            }
        }
        
        // b key: 'b' + hash -> body (if exists)
        if body, exists := bodies[hashStr]; exists {
            bodyKey := append([]byte{'b'}, hash...)
            if err := batch.Set(bodyKey, body, nil); err != nil {
                log.Printf("Failed to write body for block %d: %v", blockNum, err)
                continue
            }
        }
        
        // n key: 'n' + blockNum(8) -> hash (alternate canonical)
        nKey := []byte{'n'}
        nKey = append(nKey, blockNumBytes...)
        if err := batch.Set(nKey, hash, nil); err != nil {
            log.Printf("Failed to write n key for block %d: %v", blockNum, err)
            continue
        }
        
        written++
        
        // Commit batch every 10000 blocks
        if written%10000 == 0 {
            if err := batch.Commit(nil); err != nil {
                log.Fatal("Failed to commit batch:", err)
            }
            batch = targetDB.NewBatch()
            fmt.Printf("  Written %d blocks...\n", written)
        }
    }
    
    // Final batch commit
    if err := batch.Commit(nil); err != nil {
        log.Fatal("Failed to commit final batch:", err)
    }
    
    fmt.Printf("\n✅ Successfully converted %d blocks\n", written)
    
    // Verify the conversion
    fmt.Println("\nPhase 3: Verifying conversion...")
    fmt.Println("=================================")
    
    // Check if we can read block 0
    hKey := []byte{'H'}
    blockNumBytes := make([]byte, 8)
    binary.BigEndian.PutUint64(blockNumBytes, 0)
    hKey = append(hKey, blockNumBytes...)
    
    if hash, closer, err := targetDB.Get(hKey); err == nil {
        closer.Close()
        fmt.Printf("✅ Block 0 canonical hash: %s\n", hex.EncodeToString(hash))
        
        // Try to get the header
        headerKey := append([]byte{'h'}, hash...)
        if header, closer, err := targetDB.Get(headerKey); err == nil {
            closer.Close()
            fmt.Printf("✅ Block 0 header found: %d bytes\n", len(header))
        }
    } else {
        fmt.Println("❌ Block 0 not found in converted database")
    }
    
    fmt.Println("\n=============================================================")
    fmt.Println("Conversion complete!")
    fmt.Println()
    fmt.Println("To use with luxd:")
    fmt.Printf("  luxd --genesis-db=%s --genesis-db-type=pebbledb\n", TARGET_DB)
    fmt.Println("=============================================================")
}