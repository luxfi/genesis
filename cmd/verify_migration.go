package main

import (
    "encoding/binary"
    "fmt"
    "log"
    "path/filepath"
    
    "github.com/luxfi/geth/common"
    "github.com/luxfi/geth/core/rawdb"
    "github.com/luxfi/geth/ethdb/badgerdb"
    
    luxdb "github.com/luxfi/database/badgerdb"
    "github.com/prometheus/client_golang/prometheus"
)

func main() {
    dbPath := "/Users/z/.luxd/network-96369/chains/X6CU5qgMJfzsTB9UWxj2ZY5hd68x41HfZ4m4hCBWbHuj1Ebc3/ethdb"
    vmPath := "/Users/z/.luxd/network-96369/chains/X6CU5qgMJfzsTB9UWxj2ZY5hd68x41HfZ4m4hCBWbHuj1Ebc3/vm"
    
    tipHash := common.HexToHash("0x32dede1fc8e0f11ecde12fb42aef7933fc6c5fcf863bc277b5eac08ae4d461f0")
    tipHeight := uint64(1082780)
    genesisHash := common.HexToHash("0x3f4fa2a0b0ce089f52bf0ae9199c75ffdd76ecafc987794050cb0d286f1ec61e")
    
    fmt.Println("=== Verifying Migration Invariants ===\n")
    
    // Open database
    db, err := badgerdb.New(filepath.Clean(dbPath), 0, 0, "", false)
    if err != nil {
        log.Fatalf("Failed to open database: %v", err)
    }
    defer db.Close()
    
    passed := 0
    failed := 0
    
    // 1. Check head header hash
    fmt.Print("1. ReadHeadHeaderHash: ")
    if h := rawdb.ReadHeadHeaderHash(db); h != tipHash {
        fmt.Printf("❌ FAILED (got %s, want %s)\n", h.Hex(), tipHash.Hex())
        failed++
    } else {
        fmt.Printf("✓ %s\n", h.Hex())
        passed++
    }
    
    // 2. Check header number mapping
    fmt.Print("2. ReadHeaderNumber(tipHash): ")
    numKey := append([]byte("H"), tipHash.Bytes()...)
    if numBytes, err := db.Get(numKey); err != nil {
        fmt.Printf("❌ FAILED (not found: %v)\n", err)
        failed++
    } else if binary.BigEndian.Uint64(numBytes) != tipHeight {
        fmt.Printf("❌ FAILED (got %d, want %d)\n", binary.BigEndian.Uint64(numBytes), tipHeight)
        failed++
    } else {
        fmt.Printf("✓ %d\n", tipHeight)
        passed++
    }
    
    // 3. Check header exists
    fmt.Print("3. Header at tip exists: ")
    headerKey := append([]byte("h"), be8(tipHeight)...)
    headerKey = append(headerKey, tipHash.Bytes()...)
    if headerBytes, err := db.Get(headerKey); err != nil {
        fmt.Printf("❌ FAILED (not found: %v)\n", err)
        failed++
    } else {
        fmt.Printf("✓ %d bytes\n", len(headerBytes))
        passed++
    }
    
    // 4. Check canonical hash at tip
    fmt.Print("4. ReadCanonicalHash(1082780): ")
    if h := rawdb.ReadCanonicalHash(db, tipHeight); h != tipHash {
        fmt.Printf("❌ FAILED (got %s, want %s)\n", h.Hex(), tipHash.Hex())
        failed++
    } else {
        fmt.Printf("✓ %s\n", h.Hex())
        passed++
    }
    
    // 5. Check genesis
    fmt.Print("5. ReadCanonicalHash(0): ")
    if h := rawdb.ReadCanonicalHash(db, 0); h != genesisHash {
        fmt.Printf("❌ FAILED (got %s, want %s)\n", h.Hex(), genesisHash.Hex())
        failed++
    } else {
        fmt.Printf("✓ %s\n", h.Hex())
        passed++
    }
    
    // 6. Check body exists
    fmt.Print("6. Body at tip exists: ")
    bodyKey := append([]byte("b"), be8(tipHeight)...)
    bodyKey = append(bodyKey, tipHash.Bytes()...)
    if bodyBytes, err := db.Get(bodyKey); err != nil {
        fmt.Printf("⚠️  Not found (may be empty block)\n")
    } else {
        fmt.Printf("✓ %d bytes\n", len(bodyBytes))
        passed++
    }
    
    // 7. Check VM metadata
    fmt.Println("\n=== VM Metadata ===")
    
    vmDB, err := luxdb.New(vmPath, nil, "", prometheus.DefaultRegisterer)
    if err != nil {
        fmt.Printf("❌ Failed to open VM DB: %v\n", err)
        failed++
    } else {
        defer vmDB.Close()
        
        fmt.Print("7. lastAccepted: ")
        if val, err := vmDB.Get([]byte("lastAccepted")); err != nil {
            fmt.Printf("❌ FAILED (not found: %v)\n", err)
            failed++
        } else if common.BytesToHash(val) != tipHash {
            fmt.Printf("❌ FAILED (got %x, want %s)\n", val, tipHash.Hex())
            failed++
        } else {
            fmt.Printf("✓ %s\n", tipHash.Hex())
            passed++
        }
        
        fmt.Print("8. lastAcceptedHeight: ")
        if val, err := vmDB.Get([]byte("lastAcceptedHeight")); err != nil {
            fmt.Printf("❌ FAILED (not found: %v)\n", err)
            failed++
        } else if binary.BigEndian.Uint64(val) != tipHeight {
            fmt.Printf("❌ FAILED (got %d, want %d)\n", binary.BigEndian.Uint64(val), tipHeight)
            failed++
        } else {
            fmt.Printf("✓ %d\n", tipHeight)
            passed++
        }
        
        fmt.Print("9. initialized: ")
        if val, err := vmDB.Get([]byte("initialized")); err != nil {
            fmt.Printf("❌ FAILED (not found: %v)\n", err)
            failed++
        } else if len(val) == 1 && val[0] == 1 {
            fmt.Printf("✓ true\n")
            passed++
        } else {
            fmt.Printf("❌ FAILED (got %x)\n", val)
            failed++
        }
    }
    
    fmt.Printf("\n=== Summary ===\n")
    fmt.Printf("Passed: %d\n", passed)
    fmt.Printf("Failed: %d\n", failed)
    
    if failed > 0 {
        fmt.Println("\n⚠️  Some invariants failed. Fix these before starting the node.")
    } else {
        fmt.Println("\n✅ All invariants passed! Ready to start the node.")
    }
}

func be8(n uint64) []byte {
    var b [8]byte
    binary.BigEndian.PutUint64(b[:], n)
    return b[:]
}