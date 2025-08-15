package main

import (
    "encoding/binary"
    "encoding/hex"
    "fmt"
    "log"
    "math/big"
    "os"
    "path/filepath"
    "time"

    "github.com/cockroachdb/pebble"
    "github.com/luxfi/database/badgerdb"
    "github.com/luxfi/geth/common"
    "github.com/luxfi/geth/core/rawdb"
    "github.com/luxfi/geth/rlp"
    "github.com/luxfi/geth/trie"
    "github.com/luxfi/geth/triedb"
)

// Account struct
type Account struct {
    Nonce    uint64
    Balance  *big.Int
    Root     common.Hash
    CodeHash []byte
}

func main() {
    srcPath := "/Users/z/work/lux/state/chaindata/lux-mainnet-96369/db/pebbledb"
    dstPath := "/Users/z/.luxd/network-96369/chains/EWi9aPkGe6EfJ3SobCAmSUXRPLa4brF3cThwPwmHTrD1y13jy/ethdb"
    vmPath := "/Users/z/.luxd/network-96369/chains/EWi9aPkGe6EfJ3SobCAmSUXRPLa4brF3cThwPwmHTrD1y13jy/vm"
    
    wantRoot := common.HexToHash("0xaedd8be7a060b082b0cb3195d0b5ba017c058468851ed93dd07eca274de000c2")
    tipHash := common.HexToHash("0x32dede1fc8e0f11ecde12fb42aef7933fc6c5fcf863bc277b5eac08ae4d461f0")
    height := uint64(1082780)
    
    fmt.Println("╔════════════════════════════════════════════════════════╗")
    fmt.Println("║   REHASH STATE: Path-Scheme → Hash-Scheme (Local)     ║")
    fmt.Println("╚════════════════════════════════════════════════════════╝")
    fmt.Println()
    
    // Open source
    src, err := pebble.Open(filepath.Clean(srcPath), &pebble.Options{ReadOnly: true})
    if err != nil {
        log.Fatalf("open pebble: %v", err)
    }
    defer src.Close()
    
    // Open destination
    dst, err := badgerdb.New(filepath.Clean(dstPath), nil, "", nil)
    if err != nil {
        log.Fatalf("open badger: %v", err)
    }
    defer dst.Close()
    
    // Create trie database
    trieDB := triedb.NewDatabase(dst, triedb.HashDefaults)
    
    // Create account trie  
    accTrie, err := trie.NewStateTrie(
        trie.StateTrieID(common.Hash{}),
        trieDB,
    )
    if err != nil {
        log.Fatalf("new trie: %v", err)
    }
    
    startTime := time.Now()
    
    // Phase 1: Copy code
    fmt.Println("📝 Phase 1: Copy bytecode...")
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
    
    // Phase 2: Process accounts
    fmt.Println("\n📊 Phase 2: Process accounts...")
    accCount := 0
    it, _ = src.NewIter(&pebble.IterOptions{
        LowerBound: []byte{0x00},
        UpperBound: []byte{0x01},
    })
    
    for it.First(); it.Valid(); it.Next() {
        key := it.Key()
        if len(key) != 21 { // 1 byte prefix + 20 byte address
            continue
        }
        
        var addr common.Address
        copy(addr[:], key[1:])
        
        // Decode account
        var acc Account
        if err := rlp.DecodeBytes(it.Value(), &acc); err != nil {
            log.Printf("decode account %x: %v", addr, err)
            continue
        }
        
        // Process storage if needed
        if acc.Root != (common.Hash{}) && acc.Root != common.HexToHash("0x56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421") {
            stTrie, _ := trie.NewStateTrie(
                trie.StateTrieID(common.Hash{}),
                trieDB,
            )
            
            // Copy storage for this account
            storPrefix := append([]byte{0x01}, addr.Bytes()...)
            storIt, _ := src.NewIter(&pebble.IterOptions{
                LowerBound: storPrefix,
                UpperBound: append(storPrefix, 0xff),
            })
            
            for storIt.First(); storIt.Valid(); storIt.Next() {
                sk := storIt.Key()
                if len(sk) == 53 { // 1 + 20 + 32
                    slot := sk[21:]
                    stTrie.UpdateStorage(addr, slot, storIt.Value())
                }
            }
            storIt.Close()
            
            newStorageRoot, _ := stTrie.Commit(false)
            acc.Root = newStorageRoot
        }
        
        // Encode and update account
        accEnc, _ := rlp.EncodeToBytes(&acc)
        accTrie.UpdateAccount(addr, accEnc)
        
        accCount++
        if accCount%10000 == 0 {
            fmt.Printf("  Processed %d accounts\n", accCount)
        }
    }
    it.Close()
    
    // Commit trie
    newRoot, _ := accTrie.Commit(false)
    
    fmt.Printf("\n✓ Processed %d accounts\n", accCount)
    fmt.Printf("  Computed root: %s\n", newRoot.Hex())
    fmt.Printf("  Expected root: %s\n", wantRoot.Hex())
    
    if newRoot != wantRoot {
        log.Fatalf("❌ Root mismatch!")
    }
    
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
    fmt.Println("\n🚀 Start with:")
    fmt.Println("  CORETH_DISABLE_FREEZER=1 luxd --network-id=96369 --http-host=0.0.0.0 --http-port=9630")
}