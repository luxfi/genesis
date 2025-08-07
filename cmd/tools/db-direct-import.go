package main

import (
    "bytes"
    "encoding/binary"
    "encoding/hex"
    "fmt"
    "log"
    
    "github.com/cockroachdb/pebble"
)

const (
    SOURCE_DB = "/home/z/work/lux/state/chaindata/lux-mainnet-96369/db/pebbledb"
    TARGET_DB = "/tmp/luxd-replay/db/pebbledb" // luxd's database
)

var namespace = []byte{
    0x33, 0x7f, 0xb7, 0x3f, 0x9b, 0xcd, 0xac, 0x8c,
    0x31, 0xa2, 0xd5, 0xf7, 0xb8, 0x77, 0xab, 0x1e,
    0x8a, 0x2b, 0x7f, 0x2a, 0x1e, 0x9b, 0xf0, 0x2a,
    0x0a, 0x0e, 0x6c, 0x6f, 0xd1, 0x64, 0xf1, 0xd1,
}

func main() {
    fmt.Println("=============================================================")
    fmt.Println("          Direct Database Import for LUX C-Chain            ")
    fmt.Println("=============================================================")
    fmt.Println()
    fmt.Println("Source: " + SOURCE_DB)
    fmt.Println("Target: " + TARGET_DB)
    fmt.Println()
    
    // Open source database
    srcDB, err := pebble.Open(SOURCE_DB, &pebble.Options{ReadOnly: true})
    if err != nil {
        log.Fatal("Failed to open source database:", err)
    }
    defer srcDB.Close()
    
    // Open target database
    fmt.Println("⚠️  WARNING: This will modify luxd's database directly!")
    fmt.Println("   Make sure luxd is stopped before running this.")
    fmt.Println("   Press Ctrl+C to abort...")
    fmt.Println()
    
    // Count blocks in source
    fmt.Println("Analyzing source database...")
    blockCount := 0
    blocks := make(map[uint64]struct{
        hash   []byte
        header []byte
    })
    
    iter, _ := srcDB.NewIter(&pebble.IterOptions{})
    for iter.First(); iter.Valid(); iter.Next() {
        key := make([]byte, len(iter.Key()))
        copy(key, iter.Key())
        value := iter.Value()
        
        if len(key) == 64 && bytes.HasPrefix(key, namespace) {
            hash := key[32:]
            
            if len(value) > 100 && (value[0] == 0xf8 || value[0] == 0xf9) {
                blockNum := uint64(hash[0])<<16 | uint64(hash[1])<<8 | uint64(hash[2])
                
                if blockNum < 1500000 {
                    hashCopy := make([]byte, 32)
                    copy(hashCopy, hash)
                    headerCopy := make([]byte, len(value))
                    copy(headerCopy, value)
                    
                    blocks[blockNum] = struct{
                        hash   []byte
                        header []byte
                    }{hashCopy, headerCopy}
                    
                    blockCount++
                    if blockCount%100000 == 0 {
                        fmt.Printf("  Found %d blocks...\n", blockCount)
                    }
                }
            }
        }
    }
    iter.Close()
    
    fmt.Printf("✅ Found %d blocks to import\n\n", blockCount)
    
    // Now we would write to target, but let's check format first
    fmt.Println("Sample blocks to be imported:")
    for i := uint64(0); i < 10; i++ {
        if block, exists := blocks[i]; exists {
            fmt.Printf("  Block %d: hash=%s, header=%d bytes\n", 
                i, hex.EncodeToString(block.hash)[:16]+"...", len(block.header))
        }
    }
    
    fmt.Println("\n📝 Import Plan:")
    fmt.Println("================")
    fmt.Println("For each block, we need to write:")
    fmt.Println("  1. H key: namespace + 'H' + blockNum(8) → blockHash")
    fmt.Println("  2. h key: namespace + 'h' + blockHash → header")
    fmt.Println("  3. b key: namespace + 'b' + blockHash → body (if exists)")
    fmt.Println("  4. n key: namespace + 'n' + blockNum(8) → blockHash")
    fmt.Println()
    
    // Create a batch for first 100 blocks
    fmt.Println("Creating import batch for first 100 blocks...")
    
    batch := make([]struct{
        key   []byte
        value []byte
    }, 0)
    
    for blockNum := uint64(0); blockNum < 100; blockNum++ {
        block, exists := blocks[blockNum]
        if !exists {
            continue
        }
        
        blockNumBytes := make([]byte, 8)
        binary.BigEndian.PutUint64(blockNumBytes, blockNum)
        
        // H key: blockNum → hash
        hKey := append(namespace, 'H')
        hKey = append(hKey, blockNumBytes...)
        batch = append(batch, struct{
            key   []byte
            value []byte
        }{hKey, block.hash})
        
        // h key: hash → header
        headerKey := append(namespace, 'h')
        headerKey = append(headerKey, block.hash...)
        batch = append(batch, struct{
            key   []byte
            value []byte
        }{headerKey, block.header})
        
        // n key: blockNum → hash (alternate canonical mapping)
        nKey := append(namespace, 'n')
        nKey = append(nKey, blockNumBytes...)
        batch = append(batch, struct{
            key   []byte
            value []byte
        }{nKey, block.hash})
    }
    
    fmt.Printf("✅ Prepared %d key-value pairs for import\n", len(batch))
    
    // Display sample of what would be written
    fmt.Println("\nSample of keys to be written:")
    for i := 0; i < 5 && i < len(batch); i++ {
        entry := batch[i]
        prefix := ""
        if len(entry.key) > 32 {
            prefix = string([]byte{entry.key[32]})
        }
        fmt.Printf("  Key[%d]: prefix='%s', key_len=%d, val_len=%d\n", 
            i, prefix, len(entry.key), len(entry.value))
    }
    
    fmt.Println("\n⚠️  To actually import:")
    fmt.Println("   1. Stop luxd: pkill -f luxd")
    fmt.Println("   2. Backup target DB: cp -r " + TARGET_DB + " " + TARGET_DB + ".backup")
    fmt.Println("   3. Run import with --write flag")
    fmt.Println("   4. Restart luxd")
}