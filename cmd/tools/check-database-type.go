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
    
    fmt.Println("Database Analysis")
    fmt.Println("=================")
    fmt.Printf("Namespace: %x\n\n", namespace)
    
    // Count key types
    prefixCounts := make(map[byte]int)
    totalKeys := 0
    hKeyExample := []byte{}
    
    iter, _ := db.NewIter(&pebble.IterOptions{})
    defer iter.Close()
    
    for iter.First(); iter.Valid() && totalKeys < 10000000; iter.Next() {
        key := iter.Key()
        
        if bytes.HasPrefix(key, namespace) && len(key) > 32 {
            prefix := key[32]
            prefixCounts[prefix]++
            
            if prefix == 'H' && len(hKeyExample) == 0 {
                hKeyExample = key
            }
        }
        totalKeys++
    }
    
    fmt.Printf("Scanned %d keys\n\n", totalKeys)
    fmt.Println("Prefix counts:")
    
    // Sort and display prefixes
    for prefix := byte(32); prefix < 127; prefix++ {
        if count := prefixCounts[prefix]; count > 0 {
            if prefix >= 32 && prefix < 127 {
                fmt.Printf("  '%c' (0x%02x): %d\n", prefix, prefix, count)
            } else {
                fmt.Printf("  0x%02x: %d\n", prefix, count)
            }
        }
    }
    
    // Check what an H key actually contains
    if len(hKeyExample) > 0 {
        fmt.Printf("\nExample H key:\n")
        fmt.Printf("  Full key: %x\n", hKeyExample)
        
        value, closer, _ := db.Get(hKeyExample)
        closer.Close()
        
        fmt.Printf("  Value: %x\n", value)
        if len(value) == 8 {
            blockNum := binary.BigEndian.Uint64(value)
            fmt.Printf("  Value as uint64: %d\n", blockNum)
        }
        
        // Decode the hash part
        hashPart := hKeyExample[33:]
        fmt.Printf("  Hash part: %x\n", hashPart)
        fmt.Printf("  Hash as string: %s\n", string(hashPart))
    }
    
    // Check for standard geth/coreth prefixes WITHOUT namespace
    fmt.Println("\nChecking for keys without namespace:")
    
    // Check for canonical number mapping
    canonicalKey := []byte{'h'}
    blockNumBytes := make([]byte, 8)
    binary.BigEndian.PutUint64(blockNumBytes, 0)
    canonicalKey = append(canonicalKey, blockNumBytes...)
    canonicalKey = append(canonicalKey, []byte("n")...)
    
    if val, closer, err := db.Get(canonicalKey); err == nil {
        closer.Close()
        fmt.Printf("  Found 'h' + blocknum + 'n' key: %x\n", val)
    }
    
    // Try standard geth canonical
    canonicalKey = []byte("h")
    canonicalKey = append(canonicalKey, blockNumBytes...)
    canonicalKey = append(canonicalKey, []byte("n")...)
    
    if val, closer, err := db.Get(canonicalKey); err == nil {
        closer.Close()
        fmt.Printf("  Found standard canonical: %x\n", val)
    }
    
    // Look for "LastHeader" or similar
    lastHeaderKey := []byte("LastHeader")
    if val, closer, err := db.Get(lastHeaderKey); err == nil {
        closer.Close()
        fmt.Printf("  Found LastHeader (no namespace): %x\n", val)
    }
    
    // Check the actual content type
    fmt.Println("\nSampling actual data (first 10 keys with namespace):")
    
    iter2, _ := db.NewIter(&pebble.IterOptions{})
    count := 0
    for iter2.First(); iter2.Valid() && count < 10; iter2.Next() {
        key := iter2.Key()
        
        if bytes.HasPrefix(key, namespace) {
            value := iter2.Value()
            
            fmt.Printf("\n  Key %d:\n", count)
            fmt.Printf("    Key length: %d\n", len(key))
            fmt.Printf("    Key (after namespace): %x\n", key[32:])
            
            if len(key) > 32 && key[32] >= 32 && key[32] < 127 {
                fmt.Printf("    First byte after namespace: '%c' (0x%02x)\n", key[32], key[32])
            }
            
            fmt.Printf("    Value length: %d\n", len(value))
            if len(value) < 50 {
                fmt.Printf("    Value: %x\n", value)
            } else {
                fmt.Printf("    Value (first 50): %x...\n", value[:50])
            }
            
            // Check if value looks like RLP
            if len(value) > 0 {
                firstByte := value[0]
                if firstByte >= 0xc0 && firstByte <= 0xff {
                    fmt.Printf("    Looks like RLP (first byte: 0x%02x)\n", firstByte)
                }
            }
            
            count++
        }
    }
    iter2.Close()
    
    // Try to find blocks using different approaches
    fmt.Println("\nSearching for block data patterns:")
    
    // Pattern 1: Look for RLP-encoded headers
    iter3, _ := db.NewIter(&pebble.IterOptions{})
    rlpHeaders := 0
    for iter3.First(); iter3.Valid() && rlpHeaders < 5; iter3.Next() {
        value := iter3.Value()
        
        // Check for RLP list starting with 0xf9 (common for headers)
        if len(value) > 100 && value[0] == 0xf9 {
            key := iter3.Key()
            fmt.Printf("\n  Potential RLP header found:\n")
            fmt.Printf("    Key: %x\n", key)
            if bytes.HasPrefix(key, namespace) {
                fmt.Printf("    Key after namespace: %x\n", key[32:])
            }
            fmt.Printf("    Value size: %d bytes\n", len(value))
            fmt.Printf("    Value preview: %x...\n", value[:32])
            
            // Try to decode as header
            if len(value) > 200 && value[0] == 0xf9 {
                // This looks like a header
                fmt.Printf("    Likely a block header!\n")
                
                // Check if we can find this in headers
                if bytes.HasPrefix(key, namespace) && len(key) == 64 {
                    hash := key[32:]
                    fmt.Printf("    Block hash: %s\n", hex.EncodeToString(hash))
                }
            }
            
            rlpHeaders++
        }
    }
    iter3.Close()
}