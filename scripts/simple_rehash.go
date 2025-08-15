package main

import (
    "encoding/binary"
    "fmt"
    "log"
    "math/big"
    "path/filepath"
    "time"

    "github.com/cockroachdb/pebble"
    "github.com/luxfi/database/badgerdb"
    "github.com/luxfi/geth/common"
    "github.com/luxfi/geth/core/rawdb"
    "github.com/luxfi/geth/rlp"
    "golang.org/x/crypto/sha3"
)

// Account struct
type Account struct {
    Nonce    uint64
    Balance  *big.Int
    Root     common.Hash
    CodeHash []byte
}

func keccak256(data []byte) []byte {
    hasher := sha3.NewLegacyKeccak256()
    hasher.Write(data)
    return hasher.Sum(nil)
}

func main() {
    srcPath := "/Users/z/work/lux/state/chaindata/lux-mainnet-96369/db/pebbledb"
    dstPath := "/Users/z/.luxd/network-96369/chains/EWi9aPkGe6EfJ3SobCAmSUXRPLa4brF3cThwPwmHTrD1y13jy/ethdb"
    vmPath := "/Users/z/.luxd/network-96369/chains/EWi9aPkGe6EfJ3SobCAmSUXRPLa4brF3cThwPwmHTrD1y13jy/vm"
    
    wantRoot := common.HexToHash("0xaedd8be7a060b082b0cb3195d0b5ba017c058468851ed93dd07eca274de000c2")
    tipHash := common.HexToHash("0x32dede1fc8e0f11ecde12fb42aef7933fc6c5fcf863bc277b5eac08ae4d461f0")
    height := uint64(1082780)
    
    fmt.Println("╔════════════════════════════════════════════════════════╗")
    fmt.Println("║        Simple Rehash: Path → Hash Scheme              ║")
    fmt.Println("╚════════════════════════════════════════════════════════╝")
    fmt.Println()
    
    // Open source
    src, err := pebble.Open(filepath.Clean(srcPath), &pebble.Options{ReadOnly: true})
    if err \!= nil {
        log.Fatalf("open pebble: %v", err)
    }
    defer src.Close()
    
    // Open destination
    dst, err := badgerdb.New(filepath.Clean(dstPath), nil, "", nil)
    if err \!= nil {
        log.Fatalf("open badger: %v", err)
    }
    defer dst.Close()
    
    startTime := time.Now()
    
    // Phase 1: Add hash-scheme keys for all trie nodes
    fmt.Println("📝 Phase 1: Adding hash-scheme keys...")
    nodeCount := 0
    
    // Process trie nodes (prefixes 0x00-0x09)
    for prefix := byte(0x00); prefix <= 0x09; prefix++ {
        it, _ := src.NewIter(&pebble.IterOptions{
            LowerBound: []byte{prefix},
            UpperBound: []byte{prefix + 1},
        })
        
        batch := dst.NewBatch()
        for it.First(); it.Valid(); it.Next() {
            key := it.Key()
            val := it.Value()
            
            // Copy path-scheme key as-is
            batch.Put(key, val)
            
            // Also add hash-scheme key (keccak256(value) -> value)
            hashKey := keccak256(val)
            batch.Put(hashKey, val)
            
            nodeCount++
            if batch.Size() > 10*1024*1024 {
                batch.Write()
                batch.Reset()
                batch = dst.NewBatch()
            }
        }
        
        if batch.Size() > 0 {
            batch.Write()
        }
        it.Close()
        
        if nodeCount > 0 {
            fmt.Printf("  Prefix 0x%02x: %d nodes\n", prefix, nodeCount)
        }
    }
    
    // Phase 2: Copy code table
    fmt.Println("\n📝 Phase 2: Copy bytecode...")
    codeCount := 0
    it, _ := src.NewIter(&pebble.IterOptions{
        LowerBound: []byte{0x02},
        UpperBound: []byte{0x03},
    })
    for it.First(); it.Valid(); it.Next() {
        key := it.Key()
        if len(key) == 33 { // 1 byte prefix + 32 byte hash
            var h common.Hash
            copy(h[:], key[1:])
            rawdb.WriteCode(dst, h, it.Value())
            codeCount++
        }
    }
    it.Close()
    fmt.Printf("  ✓ %d code entries\n", codeCount)
    
    // Write heads
    rawdb.WriteHeadHeaderHash(dst, tipHash)
    rawdb.WriteHeadBlockHash(dst, tipHash)
    rawdb.WriteHeadFastBlockHash(dst, tipHash)
    
    // Write VM metadata
    vm, _ := badgerdb.New(filepath.Clean(vmPath), nil, "", nil)
    vm.Put([]byte("lastAccepted"), tipHash[:])
    heightBytes := make([]byte, 8)
    binary.BigEndian.PutUint64(heightBytes, height)
    vm.Put([]byte("lastAcceptedHeight"), heightBytes)
    vm.Put([]byte("initialized"), []byte{1})
    vm.Close()
    
    elapsed := time.Since(startTime)
    fmt.Printf("\n✅ Complete in %s\n", elapsed.Round(time.Second))
    fmt.Printf("  Added %d hash-scheme nodes\n", nodeCount)
    fmt.Println("\n🚀 Start with:")
    fmt.Println("  CORETH_DISABLE_FREEZER=1 luxd --network-id=96369 --http-host=0.0.0.0 --http-port=9630")
}
EOF < /dev/null