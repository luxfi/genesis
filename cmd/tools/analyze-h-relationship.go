package main

import (
    "bytes"
    "encoding/binary"
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
    
    fmt.Println("Analyzing H key to header relationship...")
    fmt.Println("==========================================")
    
    // Get a few H keys and their values
    fmt.Println("\n1. Sample H keys (namespace + H + 32-byte hash):")
    hKeys := make(map[uint64][]byte)
    
    iter, _ := db.NewIter(&pebble.IterOptions{})
    count := 0
    for iter.First(); iter.Valid() && count < 10; iter.Next() {
        key := iter.Key()
        if len(key) == 65 && bytes.Equal(key[:32], namespace) && key[32] == 'H' {
            hash := key[33:65]
            value := iter.Value()
            
            if len(value) == 8 {
                blockNum := binary.BigEndian.Uint64(value)
                hKeys[blockNum] = hash
                fmt.Printf("  Block %d: H-hash=%x\n", blockNum, hash)
                count++
            }
        }
    }
    iter.Close()
    
    // Get a few header keys
    fmt.Println("\n2. Sample header keys (namespace + h + 32-byte hash):")
    headerHashes := [][]byte{}
    
    iter2, _ := db.NewIter(&pebble.IterOptions{})
    count = 0
    for iter2.First(); iter2.Valid() && count < 10; iter2.Next() {
        key := iter2.Key()
        if len(key) == 65 && bytes.Equal(key[:32], namespace) && key[32] == 'h' {
            hash := key[33:65]
            headerHashes = append(headerHashes, hash)
            fmt.Printf("  Header %d: hash=%x\n", count, hash)
            count++
        }
    }
    iter2.Close()
    
    // Check if H-hashes exist as other types
    fmt.Println("\n3. Checking what H-hashes actually are:")
    for blockNum, hHash := range hKeys {
        fmt.Printf("\n  Block %d H-hash %x:\n", blockNum, hHash)
        
        // Check as header
        headerKey := append(namespace, 'h')
        headerKey = append(headerKey, hHash...)
        if _, closer, err := db.Get(headerKey); err == nil {
            closer.Close()
            fmt.Println("    ✓ Found as header")
        }
        
        // Check as body
        bodyKey := append(namespace, 'b')
        bodyKey = append(bodyKey, hHash...)
        if _, closer, err := db.Get(bodyKey); err == nil {
            closer.Close()
            fmt.Println("    ✓ Found as body")
        }
        
        // Check as receipt
        receiptKey := append(namespace, 'r')
        receiptKey = append(receiptKey, hHash...)
        if _, closer, err := db.Get(receiptKey); err == nil {
            closer.Close()
            fmt.Println("    ✓ Found as receipt")
        }
        
        // Check if it's a transaction hash
        txKey := append(namespace, 't')
        txKey = append(txKey, hHash...)
        if _, closer, err := db.Get(txKey); err == nil {
            closer.Close()
            fmt.Println("    ✓ Found as transaction")
        }
        
        // Check the 64-byte key format (namespace + hash only)
        key64 := append(namespace, hHash...)
        if val, closer, err := db.Get(key64); err == nil {
            closer.Close()
            fmt.Printf("    ✓ Found as 64-byte key, value: %x\n", val[:20])
        }
    }
    
    // Try to find the pattern in header hashes
    fmt.Println("\n4. Analyzing header hash pattern:")
    for i, hash := range headerHashes {
        if i >= 5 {
            break
        }
        
        // Check if the hash encodes block number
        if len(hash) >= 8 {
            // Check first 8 bytes
            possibleNum1 := binary.BigEndian.Uint64(hash[0:8])
            // Check last 8 bytes
            possibleNum2 := binary.BigEndian.Uint64(hash[24:32])
            
            fmt.Printf("  Header %d hash: first8=%d, last8=%d\n", i, possibleNum1, possibleNum2)
        }
    }
    
    // Look for 'n' keys (canonical in lowercase)
    fmt.Println("\n5. Checking for 'n' canonical keys:")
    iter3, _ := db.NewIter(&pebble.IterOptions{})
    nCount := 0
    for iter3.First(); iter3.Valid() && nCount < 5; iter3.Next() {
        key := iter3.Key()
        if len(key) == 41 && bytes.Equal(key[:32], namespace) && key[32] == 'n' {
            blockNum := binary.BigEndian.Uint64(key[33:41])
            value := iter3.Value()
            fmt.Printf("  n-key block %d -> hash %x\n", blockNum, value)
            nCount++
            
            // Check if this hash exists as a header
            headerKey := append(namespace, 'h')
            headerKey = append(headerKey, value...)
            if _, closer, err := db.Get(headerKey); err == nil {
                closer.Close()
                fmt.Println("    ✓ This hash exists as a header!")
            }
        }
    }
    iter3.Close()
    
    // Try looking for block number patterns in keys
    fmt.Println("\n6. Looking for block 0 by trying different key formats:")
    
    // Try namespace + 'n' + block number (41 bytes)
    testKey1 := append(namespace, 'n')
    blockNumBytes := make([]byte, 8)
    binary.BigEndian.PutUint64(blockNumBytes, 0)
    testKey1 = append(testKey1, blockNumBytes...)
    
    if val, closer, err := db.Get(testKey1); err == nil {
        closer.Close()
        fmt.Printf("  Found with n+blocknum format! Hash: %x\n", val)
        
        // Try to get this header
        headerKey := append(namespace, 'h')
        headerKey = append(headerKey, val...)
        if header, closer, err := db.Get(headerKey); err == nil {
            closer.Close()
            fmt.Printf("    Header found: %d bytes\n", len(header))
        }
    }
    
    // Try other formats
    fmt.Println("\n7. Scanning for any key containing block 0 encoding:")
    iter4, _ := db.NewIter(&pebble.IterOptions{})
    scanCount := 0
    for iter4.First(); iter4.Valid() && scanCount < 1000000; iter4.Next() {
        key := iter4.Key()
        
        // Look for keys that might encode block 0
        if len(key) >= 40 && bytes.Equal(key[:32], namespace) {
            remainder := key[32:]
            
            // Check if it contains 8 zero bytes (block 0)
            zeroBlock := make([]byte, 8)
            if bytes.Contains(remainder, zeroBlock) {
                if remainder[0] == 'n' || remainder[0] == 'N' || remainder[0] == 'H' {
                    fmt.Printf("  Found potential block 0 key: prefix='%c', key=%x\n", remainder[0], key)
                    fmt.Printf("    Value: %x\n", iter4.Value()[:32])
                }
            }
        }
        scanCount++
    }
    iter4.Close()
    
    fmt.Printf("\nScanned %d keys\n", scanCount)
}