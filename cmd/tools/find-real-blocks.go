package main

import (
    "bytes"
    "encoding/binary"
    "encoding/hex"
    "fmt"
    "github.com/cockroachdb/pebble"
)

func main() {
    db, _ := pebble.Open("/home/z/work/lux/state/chaindata/lux-mainnet-96369/db/pebbledb", &pebble.Options{ReadOnly: true})
    defer db.Close()
    
    namespace := []byte{
        0x33, 0x7f, 0xb7, 0x3f, 0x9b, 0xcd, 0xac, 0x8c,
        0x31, 0xa2, 0xd5, 0xf7, 0xb8, 0x77, 0xab, 0x1e,
        0x8a, 0x2b, 0x7f, 0x2a, 0x1e, 0x9b, 0xf0, 0x2a,
        0x0a, 0x0e, 0x6c, 0x6f, 0xd1, 0x64, 0xf1, 0xd1,
    }
    
    fmt.Println("Finding real blockchain data")
    fmt.Println("=============================")
    
    // Build inverted H mapping: blockNum -> H-hash
    hMapping := make(map[uint64][]byte)
    
    iter, _ := db.NewIter(&pebble.IterOptions{})
    for iter.First(); iter.Valid(); iter.Next() {
        key := iter.Key()
        
        if len(key) == 65 && bytes.Equal(key[:32], namespace) && key[32] == 'H' {
            hash := key[33:65]
            value := iter.Value()
            
            if len(value) == 8 {
                blockNum := binary.BigEndian.Uint64(value)
                hMapping[blockNum] = hash
            }
        }
    }
    iter.Close()
    
    fmt.Printf("Found %d H mappings (canonical)\n\n", len(hMapping))
    
    // Now let's see what the H-hashes actually reference
    fmt.Println("Checking what H-hashes reference:")
    
    for blockNum := uint64(0); blockNum <= 5; blockNum++ {
        hHash := hMapping[blockNum]
        if hHash == nil {
            continue
        }
        
        fmt.Printf("\nBlock %d:\n", blockNum)
        fmt.Printf("  H-hash: %s\n", hex.EncodeToString(hHash))
        
        // The H-hash must be a reference to the actual block data
        // Let's check if there's data at namespace + H-hash
        dataKey := append(namespace, hHash...)
        
        if val, closer, err := db.Get(dataKey); err == nil {
            closer.Close()
            fmt.Printf("  Found data at namespace+H-hash: %d bytes\n", len(val))
            if len(val) > 0 {
                fmt.Printf("    First byte: 0x%02x\n", val[0])
                if val[0] == 0xf8 || val[0] == 0xf9 {
                    fmt.Println("    ✓ This is RLP data (likely header)!")
                }
            }
        }
        
        // Check with various prefixes
        prefixes := []byte{'h', 'H', 'b', 'B', 'r', 'R'}
        for _, prefix := range prefixes {
            testKey := append(namespace, prefix)
            testKey = append(testKey, hHash...)
            
            if val, closer, err := db.Get(testKey); err == nil {
                closer.Close()
                fmt.Printf("  Found with prefix '%c': %d bytes\n", prefix, len(val))
                if len(val) > 0 && (val[0] == 0xf8 || val[0] == 0xf9) {
                    fmt.Printf("    ✓ RLP data!\n")
                }
            }
        }
    }
    
    // Let's also scan for actual RLP headers and see their key patterns
    fmt.Println("\n\nScanning for RLP headers in database:")
    
    iter2, _ := db.NewIter(&pebble.IterOptions{})
    rlpCount := 0
    for iter2.First(); iter2.Valid() && rlpCount < 10; iter2.Next() {
        key := iter2.Key()
        value := iter2.Value()
        
        // Look for RLP headers
        if len(value) > 200 && (value[0] == 0xf8 || value[0] == 0xf9) {
            if bytes.HasPrefix(key, namespace) {
                remainder := key[32:]
                
                fmt.Printf("\nRLP Header %d:\n", rlpCount)
                fmt.Printf("  Key length: %d\n", len(key))
                fmt.Printf("  Key after namespace: %x\n", remainder)
                
                if len(remainder) == 32 {
                    // This is namespace + 32-byte hash
                    fmt.Printf("  This is a 32-byte hash: %s\n", hex.EncodeToString(remainder))
                    
                    // Check if this hash is in our H-mapping
                    for blockNum, hHash := range hMapping {
                        if bytes.Equal(hHash, remainder) {
                            fmt.Printf("  ✓ This is block %d!\n", blockNum)
                            break
                        }
                    }
                } else if len(remainder) == 33 {
                    fmt.Printf("  Prefix: '%c' (0x%02x)\n", remainder[0], remainder[0])
                    hash := remainder[1:]
                    fmt.Printf("  Hash: %x\n", hash)
                }
                
                fmt.Printf("  Value size: %d bytes\n", len(value))
                
                rlpCount++
            }
        }
    }
    iter2.Close()
}