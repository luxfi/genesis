package main

import (
    "bytes"
    "encoding/binary"
    "encoding/hex"
    "flag"
    "fmt"
    "log"
    "math/big"
    "path/filepath"
    "time"
    
    "github.com/cockroachdb/pebble"
    
    // Only luxfi/geth types
    "github.com/luxfi/geth/common"
    "github.com/luxfi/geth/core/rawdb"
    "github.com/luxfi/geth/ethdb/badgerdb"
    "github.com/luxfi/geth/params"
    "github.com/luxfi/geth/rlp"
    
    // For VM metadata
    luxdb "github.com/luxfi/database/badgerdb"
    "github.com/prometheus/client_golang/prometheus"
)

var (
    srcPath = flag.String("src", "/Users/z/work/lux/state/chaindata/lux-mainnet-96369/db/pebbledb", "SubnetEVM PebbleDB path")
    dstPath = flag.String("dst", "/Users/z/.luxd/network-96369/chains/X6CU5qgMJfzsTB9UWxj2ZY5hd68x41HfZ4m4hCBWbHuj1Ebc3/ethdb", "Coreth ethdb path")
    vmPath  = flag.String("vm", "/Users/z/.luxd/network-96369/chains/X6CU5qgMJfzsTB9UWxj2ZY5hd68x41HfZ4m4hCBWbHuj1Ebc3/vm", "VM metadata path")
    nsHex   = flag.String("ns", "337fb73f9bcdac8c31a2d5f7b877ab1e8a2b7f2a1e9bf02a0a0e6c6fd164f1d1", "namespace")
)

func be8(n uint64) []byte {
    var b [8]byte
    binary.BigEndian.PutUint64(b[:], n)
    return b[:]
}

// Read hash→number mapping from ns||'H'||hash
func readHashToNumber(pdb *pebble.DB, ns []byte, hash common.Hash) (uint64, error) {
    key := make([]byte, 0, 65)
    key = append(key, ns...)
    key = append(key, 'H')
    key = append(key, hash.Bytes()...)
    
    val, closer, err := pdb.Get(key)
    if err != nil {
        return 0, err
    }
    defer closer.Close()
    
    if len(val) == 8 {
        return binary.BigEndian.Uint64(val), nil
    }
    
    // Try RLP-encoded number as fallback
    var num *big.Int
    if err := rlp.DecodeBytes(val, &num); err != nil {
        return 0, fmt.Errorf("unknown H value encoding")
    }
    return num.Uint64(), nil
}

// Build plain key: namespace + prefix + be8(number) + hash
func plainKey(ns []byte, prefix byte, n uint64, h common.Hash) []byte {
    key := make([]byte, 0, 73)
    key = append(key, ns...)
    key = append(key, prefix)
    key = append(key, be8(n)...)
    key = append(key, h.Bytes()...)
    return key
}

// Extract parent hash from header RLP without full decode
func extractParentHash(headerRLP []byte) (common.Hash, error) {
    // Use RLP stream to read just the first field
    s := rlp.NewStream(bytes.NewReader(headerRLP), 0)
    
    // Read list start
    if _, err := s.List(); err != nil {
        return common.Hash{}, fmt.Errorf("header not a list: %v", err)
    }
    
    // Read parent hash (first field)
    var parent common.Hash
    if err := s.Decode(&parent); err != nil {
        return common.Hash{}, fmt.Errorf("failed to decode parent: %v", err)
    }
    
    return parent, nil
}

func main() {
    flag.Parse()
    
    fmt.Println("=============================================================")
    fmt.Println("    SubnetEVM to Coreth Migration (Final)                   ")
    fmt.Println("=============================================================")
    fmt.Printf("Source: %s\n", *srcPath)
    fmt.Printf("Destination: %s\n", *dstPath)
    fmt.Printf("VM Path: %s\n", *vmPath)
    
    // Parse namespace
    namespace, err := hex.DecodeString(*nsHex)
    if err != nil || len(namespace) != 32 {
        log.Fatal("Invalid namespace")
    }
    fmt.Printf("Namespace: %x...\n", namespace[:8])
    
    // Open source PebbleDB
    sdb, err := pebble.Open(filepath.Clean(*srcPath), &pebble.Options{ReadOnly: true})
    if err != nil {
        log.Fatalf("Failed to open source: %v", err)
    }
    defer sdb.Close()
    
    // Read acceptorTipKey to get the tip hash
    acceptorKey := append(namespace, []byte("AcceptorTipKey")...)
    tipHashBytes, closer, err := sdb.Get(acceptorKey)
    if err != nil {
        log.Fatalf("Failed to read AcceptorTipKey: %v", err)
    }
    tip := common.BytesToHash(tipHashBytes)
    closer.Close()
    fmt.Printf("Tip hash from AcceptorTipKey: %s\n", tip.Hex())
    
    // Get tip height from H mapping
    tipNum, err := readHashToNumber(sdb, namespace, tip)
    if err != nil {
        log.Fatalf("Failed to read tip number: %v", err)
    }
    fmt.Printf("Tip height from H mapping: %d\n", tipNum)
    
    // Open destination using badgerdb adapter
    ddb, err := badgerdb.New(filepath.Clean(*dstPath), 0, 0, "", false)
    if err != nil {
        log.Fatalf("Failed to open destination: %v", err)
    }
    defer ddb.Close()
    
    fmt.Println("\nStarting migration...")
    startTime := time.Now()
    lastLog := time.Now()
    copied := uint64(0)
    
    n := tipNum
    h := tip
    var genesisHash common.Hash
    
    // Walk backwards from tip to genesis
    for {
        // Build plain keys for SubnetEVM
        headerKey := plainKey(namespace, 'h', n, h)
        bodyKey := plainKey(namespace, 'b', n, h)
        receiptKey := plainKey(namespace, 'r', n, h)
        
        // Read header RLP
        headerRLP, closer, err := sdb.Get(headerKey)
        if err != nil {
            log.Fatalf("Missing header at block %d hash %s: %v", n, h.Hex(), err)
        }
        headerBytes := append([]byte(nil), headerRLP...) // copy
        closer.Close()
        
        // Extract parent hash without full decode
        parent, err := extractParentHash(headerBytes)
        if err != nil {
            log.Fatalf("Failed to extract parent at block %d: %v", n, err)
        }
        
        // Read body RLP (may not exist)
        var bodyRLP []byte
        if val, closer, err := sdb.Get(bodyKey); err == nil {
            bodyRLP = append([]byte(nil), val...)
            closer.Close()
        }
        
        // Read receipts RLP (may not exist)
        var receiptRLP []byte
        if val, closer, err := sdb.Get(receiptKey); err == nil {
            receiptRLP = append([]byte(nil), val...)
            closer.Close()
        }
        
        // Write to destination - use raw keys to avoid struct issues
        // Write header
        headerDbKey := append([]byte("h"), be8(n)...)
        headerDbKey = append(headerDbKey, h.Bytes()...)
        if err := ddb.Put(headerDbKey, headerBytes); err != nil {
            log.Fatalf("Failed to write header: %v", err)
        }
        
        // Write body if exists
        if len(bodyRLP) > 0 {
            bodyDbKey := append([]byte("b"), be8(n)...)
            bodyDbKey = append(bodyDbKey, h.Bytes()...)
            if err := ddb.Put(bodyDbKey, bodyRLP); err != nil {
                log.Fatalf("Failed to write body: %v", err)
            }
        }
        
        // Write receipts if exists
        if len(receiptRLP) > 0 {
            receiptDbKey := append([]byte("r"), be8(n)...)
            receiptDbKey = append(receiptDbKey, h.Bytes()...)
            if err := ddb.Put(receiptDbKey, receiptRLP); err != nil {
                log.Fatalf("Failed to write receipts: %v", err)
            }
        }
        
        // Write canonical hash mapping
        rawdb.WriteCanonicalHash(ddb, h, n)
        
        // Write header number mapping
        rawdb.WriteHeaderNumber(ddb, h, n)
        
        // Write TD
        td := new(big.Int).SetUint64(n + 1)
        tdKey := append([]byte("t"), be8(n)...)
        tdKey = append(tdKey, h.Bytes()...)
        tdBytes, _ := rlp.EncodeToBytes(td)
        if err := ddb.Put(tdKey, tdBytes); err != nil {
            log.Fatalf("Failed to write TD: %v", err)
        }
        
        copied++
        
        // Progress logging
        if time.Since(lastLog) > 5*time.Second {
            rate := float64(copied) / time.Since(startTime).Seconds()
            remaining := float64(n) / rate
            fmt.Printf("Progress: %d blocks copied (%.1f blocks/sec), at block %d, ETA: %.1f min\n", 
                copied, rate, n, remaining/60)
            lastLog = time.Now()
        }
        
        // Check for genesis
        if n == 0 {
            genesisHash = h
            fmt.Printf("\nReached genesis: %s\n", h.Hex())
            break
        }
        
        // Move to parent
        n--
        h = parent
    }
    
    // Write head pointers
    fmt.Println("\nWriting head pointers...")
    rawdb.WriteHeadHeaderHash(ddb, tip)
    rawdb.WriteHeadBlockHash(ddb, tip)
    rawdb.WriteHeadFastBlockHash(ddb, tip)
    
    // Write chain config
    fmt.Println("Writing chain config...")
    chainConfig := &params.ChainConfig{
        ChainID:                 big.NewInt(96369),
        HomesteadBlock:          big.NewInt(0),
        EIP150Block:            big.NewInt(0),
        EIP155Block:            big.NewInt(0),
        EIP158Block:            big.NewInt(0),
        ByzantiumBlock:         big.NewInt(0),
        ConstantinopleBlock:    big.NewInt(0),
        PetersburgBlock:        big.NewInt(0),
        IstanbulBlock:          big.NewInt(0),
        MuirGlacierBlock:       big.NewInt(0),
        BerlinBlock:            big.NewInt(0),
        LondonBlock:            big.NewInt(0),
        ArrowGlacierBlock:      big.NewInt(0),
        GrayGlacierBlock:       big.NewInt(0),
        MergeNetsplitBlock:     big.NewInt(0),
        ShanghaiTime:           new(uint64), // 0
        CancunTime:             nil,         // Not activated
        TerminalTotalDifficulty: nil,
    }
    rawdb.WriteChainConfig(ddb, genesisHash, chainConfig)
    
    // Write VM metadata
    fmt.Println("Writing VM metadata...")
    vmDB, err := luxdb.New(*vmPath, nil, "", prometheus.DefaultRegisterer)
    if err != nil {
        log.Fatalf("Failed to open VM DB: %v", err)
    }
    defer vmDB.Close()
    
    // Write lastAccepted
    if err := vmDB.Put([]byte("lastAccepted"), tip.Bytes()); err != nil {
        log.Fatalf("Failed to write lastAccepted: %v", err)
    }
    
    // Write lastAcceptedHeight
    if err := vmDB.Put([]byte("lastAcceptedHeight"), be8(tipNum)); err != nil {
        log.Fatalf("Failed to write lastAcceptedHeight: %v", err)
    }
    
    // Write initialized
    if err := vmDB.Put([]byte("initialized"), []byte{1}); err != nil {
        log.Fatalf("Failed to write initialized: %v", err)
    }
    
    // Verify invariants
    fmt.Println("\n=== Verifying invariants ===")
    
    // 1. Check head header hash
    if h := rawdb.ReadHeadHeaderHash(ddb); h != tip {
        log.Fatalf("Head header hash mismatch: got %s, want %s", h.Hex(), tip.Hex())
    }
    fmt.Printf("✓ Head header hash: %s\n", tip.Hex())
    
    // 2. Check header number
    numKey := append([]byte("H"), tip.Bytes()...)
    if numBytes, err := ddb.Get(numKey); err == nil {
        var num uint64
        if err := rlp.DecodeBytes(numBytes, &num); err == nil && num == tipNum {
            fmt.Printf("✓ Header number: %d\n", tipNum)
        } else {
            log.Printf("Warning: Header number check failed")
        }
    } else {
        log.Printf("Warning: Header number not found")
    }
    
    // 3. Check canonical hash
    if h := rawdb.ReadCanonicalHash(ddb, tipNum); h != tip {
        log.Fatalf("Canonical hash mismatch at tip")
    }
    fmt.Printf("✓ Canonical hash at %d: %s\n", tipNum, tip.Hex())
    
    // 4. Check genesis
    if h := rawdb.ReadCanonicalHash(ddb, 0); h != genesisHash {
        log.Fatalf("Genesis hash mismatch")
    }
    fmt.Printf("✓ Genesis hash: %s\n", genesisHash.Hex())
    
    elapsed := time.Since(startTime)
    fmt.Println("\n=============================================================")
    fmt.Printf("✅ Migration complete in %s\n", elapsed)
    fmt.Printf("   Blocks copied: %d (genesis to %d)\n", copied, tipNum)
    fmt.Printf("   Genesis hash: %s\n", genesisHash.Hex())
    fmt.Printf("   Tip hash: %s\n", tip.Hex())
    fmt.Printf("   Average rate: %.1f blocks/sec\n", float64(copied)/elapsed.Seconds())
    fmt.Println("=============================================================")
    fmt.Println("\nNext steps:")
    fmt.Println("1. Boot node with k=1 for testing:")
    fmt.Println("   cd ~/work/lux/node")
    fmt.Println("   ./build/luxd --network-id=96369 --consensus-k=1")
    fmt.Println("\n2. Query balances via RPC:")
    fmt.Println("   curl -X POST -H \"Content-Type: application/json\" \\")
    fmt.Println("        --data '{\"jsonrpc\":\"2.0\",\"method\":\"eth_getBalance\",\"params\":[\"0x9011E888251AB053B7bD1cdB598Db4f9DEd94714\",\"0x10859c\"],\"id\":1}' \\")
    fmt.Println("        http://localhost:9650/ext/bc/C/rpc")
}