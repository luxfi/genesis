package main

// Rehash a path/plain state snapshot -> Coreth hash-scheme using Coreth trie.Database
// Uses luxfi forks only.

import (
    "encoding/binary"
    "encoding/hex"
    "flag"
    "fmt"
    "log"
    "math/big"
    "os"
    "path/filepath"
    "time"

    "github.com/cockroachdb/pebble" // source: Pebble (SubnetEVM state snapshot)

    "github.com/luxfi/database/badgerdb" // destination: Badger (Coreth ethdb)
    "github.com/luxfi/geth/common"
    "github.com/luxfi/geth/core/rawdb"
    "github.com/luxfi/geth/rlp"
    "github.com/luxfi/geth/trie"
    "github.com/luxfi/geth/triedb"
)

// ---- CLI flags ----
var (
    srcPath       = flag.String("src", "", "Path to Pebble path/plain state DB (read-only)")
    dstEthdbPath  = flag.String("dst-ethdb", "", "Path to Coreth ethdb/ (Badger)")
    dstVMPath     = flag.String("dst-vm", "", "Path to Coreth vm/ (Badger)")
    stateRootHex  = flag.String("state-root", "", "Expected state root (hex)")
    height        = flag.Uint64("height", 1082780, "lastAcceptedHeight to stamp in vm/")
    tipHashHex    = flag.String("tip-hash", "", "lastAccepted (tip) block hash to stamp in vm/")

    accPrefixHex  = flag.String("acc-prefix",  "00", "hex byte: account key prefix (default 0x00)")
    storPrefixHex = flag.String("stor-prefix", "01", "hex byte: storage key prefix (default 0x01)")
    codePrefixHex = flag.String("code-prefix", "02", "hex byte: code key prefix (default 0x02)")

    verifyOnly    = flag.Bool("verify-only", false, "Only compute root; don't write to dst")
)

// ---- helpers ----

func mustHexHash(s string) common.Hash {
    h := common.Hash{}
    if s == "" {
        return h
    }
    b, err := hex.DecodeString(strip0x(s))
    if err != nil || len(b) != 32 {
        log.Fatalf("bad hash %q", s)
    }
    copy(h[:], b)
    return h
}

func strip0x(s string) string {
    if len(s) >= 2 && (s[0:2] == "0x" || s[0:2] == "0X") {
        return s[2:]
    }
    return s
}

func mustOneByte(hexByte string) byte {
    bs, err := hex.DecodeString(strip0x(hexByte))
    if err != nil || len(bs) != 1 {
        log.Fatalf("prefix %q must be a single byte in hex", hexByte)
    }
    return bs[0]
}

func be8(n uint64) []byte {
    var b [8]byte
    binary.BigEndian.PutUint64(b[:], n)
    return b[:]
}

// minimal wrapper so Badger implements the ethdb interfaces that trie/rawdb expect
type kvdb struct{ *badgerdb.Database }

func openBadgerKV(path string) kvdb {
    db, err := badgerdb.New(filepath.Clean(path), nil, "", nil)
    if err != nil {
        log.Fatalf("open badger: %v", err)
    }
    return kvdb{db}
}
func (k kvdb) Close() { _ = k.Database.Close() }

// read an item from Pebble (panic if missing when must=true)
func pebbleGet(p *pebble.DB, key []byte, must bool) []byte {
    val, closer, err := p.Get(key)
    if err != nil {
        if must {
            log.Fatalf("missing source key %x: %v", key, err)
        }
        return nil
    }
    defer closer.Close()
    out := make([]byte, len(val))
    copy(out, val)
    return out
}

// RLP account struct (compatible with geth/core/state Account)
type Account struct {
    Nonce    uint64
    Balance  *big.Int
    Root     common.Hash
    CodeHash []byte
}

func main() {
    log.SetFlags(0)
    flag.Parse()

    // Set defaults for our specific migration
    if *srcPath == "" {
        *srcPath = "/Users/z/work/lux/state/chaindata/lux-mainnet-96369/db/pebbledb"
    }
    if *dstEthdbPath == "" {
        *dstEthdbPath = "/Users/z/.luxd/network-96369/chains/EWi9aPkGe6EfJ3SobCAmSUXRPLa4brF3cThwPwmHTrD1y13jy/ethdb"
    }
    if *dstVMPath == "" {
        *dstVMPath = "/Users/z/.luxd/network-96369/chains/EWi9aPkGe6EfJ3SobCAmSUXRPLa4brF3cThwPwmHTrD1y13jy/vm"
    }
    if *stateRootHex == "" {
        *stateRootHex = "0xaedd8be7a060b082b0cb3195d0b5ba017c058468851ed93dd07eca274de000c2"
    }
    if *tipHashHex == "" {
        *tipHashHex = "0x32dede1fc8e0f11ecde12fb42aef7933fc6c5fcf863bc277b5eac08ae4d461f0"
    }

    wantRoot := mustHexHash(*stateRootHex)
    tipHash := mustHexHash(*tipHashHex)
    accPrefix := mustOneByte(*accPrefixHex)
    storPrefix := mustOneByte(*storPrefixHex)
    codePrefix := mustOneByte(*codePrefixHex)

    fmt.Println("╔════════════════════════════════════════════════════════╗")
    fmt.Println("║   REHASH STATE: Path-Scheme → Hash-Scheme (Coreth)    ║")
    fmt.Println("╚════════════════════════════════════════════════════════╝")
    fmt.Println()
    fmt.Printf("  Source:      %s\n", *srcPath)
    fmt.Printf("  Dest ethdb:  %s\n", *dstEthdbPath)
    fmt.Printf("  Dest VM:     %s\n", *dstVMPath)
    fmt.Printf("  State root:  %s\n", wantRoot.Hex())
    fmt.Printf("  Height:      %d\n", *height)
    fmt.Printf("  Tip hash:    %s\n", tipHash.Hex())
    fmt.Println()

    // ---- open source Pebble (read-only) ----
    src, err := pebble.Open(filepath.Clean(*srcPath), &pebble.Options{ReadOnly: true})
    if err != nil {
        log.Fatalf("open pebble: %v", err)
    }
    defer src.Close()

    // ---- open destination Coreth DBs ----
    dstEth := openBadgerKV(*dstEthdbPath)
    defer dstEth.Close()

    // Create a trie database using the new API
    trieDB := triedb.NewDatabase(dstEth, triedb.HashDefaults)
    
    // account trie (secure = keys hashed by keccak)
    // NewStateTrie is the new API
    accTrie, err := trie.NewStateTrie(
        trie.StateTrieID(common.Hash{}), // empty root for new trie
        trieDB,
    )
    if err != nil {
        log.Fatalf("new account trie: %v", err)
    }

    // ---- Phase 1: write bytecode table (safe to do up-front) ----
    fmt.Println("📝 Phase 1/3: Copy bytecode table...")
    codeLB := []byte{codePrefix}
    codeUB := []byte{codePrefix + 1}
    it, err := src.NewIter(&pebble.IterOptions{LowerBound: codeLB, UpperBound: codeUB})
    if err != nil {
        log.Fatalf("pebble.NewIter(code): %v", err)
    }
    codeCount := 0
    for it.First(); it.Valid(); it.Next() {
        key := append([]byte{}, it.Key()...)
        val := append([]byte{}, it.Value()...)
        // key layout: 1B prefix + 32B codehash
        if len(key) != 1+32 {
            continue
        }
        var h common.Hash
        copy(h[:], key[1:])
        // store via rawdb so Coreth sees it
        rawdb.WriteCode(dstEth, h, val)
        codeCount++
    }
    it.Close()
    fmt.Printf("  ✓ %d code entries written\n", codeCount)

    // ---- Phase 2: stream storage grouped by address, build per-account storage tries ----
    fmt.Println("\n🔄 Phase 2/3: Rebuild per-account storage tries...")
    stLB := []byte{storPrefix}
    stUB := []byte{storPrefix + 1}
    stIt, err := src.NewIter(&pebble.IterOptions{LowerBound: stLB, UpperBound: stUB})
    if err != nil {
        log.Fatalf("pebble.NewIter(storage): %v", err)
    }
    defer stIt.Close()

    var curAddr common.Address
    var haveCur bool
    var stTrie *trie.StateTrie
    var wroteAccounts uint64
    var wroteSlots uint64

    flushAccount := func() {
        if !haveCur || stTrie == nil {
            return
        }
        // commit storage trie nodes and compute root
        stRoot, _ := stTrie.Commit(false)
        
        // read account RLP from source, swap Root, write to account trie
        accKey := append([]byte{accPrefix}, curAddr.Bytes()...)
        accVal := pebbleGet(src, accKey, true)
        var acc Account
        if err := rlp.DecodeBytes(accVal, &acc); err != nil {
            log.Fatalf("decode account %s: %v", curAddr.Hex(), err)
        }
        acc.Root = stRoot
        accEnc, _ := rlp.EncodeToBytes(&acc)
        if err := accTrie.UpdateAccount(curAddr, accEnc); err != nil {
            log.Fatalf("accTrie.UpdateAccount: %v", err)
        }
        wroteAccounts++
        // reset
        haveCur = false
        stTrie = nil
    }

    for stIt.First(); stIt.Valid(); stIt.Next() {
        k := stIt.Key()
        v := stIt.Value()
        // expected key: 1B prefix + 20B address + 32B slot
        if len(k) != 1+20+32 {
            continue
        }
        var addr common.Address
        copy(addr[:], k[1:21])
        slot := make([]byte, 32)
        copy(slot, k[21:])

        // group by address
        if !haveCur || addr != curAddr {
            // finalize previous account
            flushAccount()
            curAddr = addr
            haveCur = true
            var err error
            stTrie, err = trie.NewStateTrie(
                trie.StateTrieID(common.Hash{}), // empty root for new storage trie
                trieDB,
            )
            if err != nil {
                log.Fatalf("new storage trie: %v", err)
            }
        }
        // v is RLP(U256) (or empty / zero)
        if len(v) == 0 {
            // implicit delete
            if err := stTrie.DeleteStorage(curAddr, slot); err != nil {
                log.Fatalf("storage DeleteStorage: %v", err)
            }
        } else {
            if err := stTrie.UpdateStorage(curAddr, slot, append([]byte{}, v...)); err != nil {
                log.Fatalf("storage UpdateStorage: %v", err)
            }
            wroteSlots++
        }
    }
    flushAccount() // last one

    // ---- Phase 3: add accounts with *no storage* (still need to exist in the trie)
    fmt.Println("\n📊 Phase 3/3: Insert storage-less accounts...")
    accLB := []byte{accPrefix}
    accUB := []byte{accPrefix + 1}
    accIt, err := src.NewIter(&pebble.IterOptions{LowerBound: accLB, UpperBound: accUB})
    if err != nil {
        log.Fatalf("pebble.NewIter(accounts): %v", err)
    }
    defer accIt.Close()

    var scannedAcc uint64
    for accIt.First(); accIt.Valid(); accIt.Next() {
        key := accIt.Key()
        val := accIt.Value()
        if len(key) != 1+20 {
            continue
        }
        scannedAcc++
        var addr common.Address
        copy(addr[:], key[1:])
        // We already wrote accounts that had storage when we flushed them.
        // To avoid O(N) lookups in src, we optimistically (re)insert; UpdateAccount is idempotent.
        if err := accTrie.UpdateAccount(addr, append([]byte{}, val...)); err != nil {
            log.Fatalf("accTrie.UpdateAccount (storage-less): %v", err)
        }
    }
    
    // commit account trie
    newRoot, _ := accTrie.Commit(false)
    
    fmt.Println("\n📈 Summary:")
    fmt.Printf("  Accounts scanned: %d\n", scannedAcc)
    fmt.Printf("  Accounts written: %d\n", wroteAccounts)
    fmt.Printf("  Storage slots:    %d\n", wroteSlots)
    fmt.Printf("  Computed root:    %s\n", newRoot.Hex())
    
    if wantRoot != (common.Hash{}) && newRoot != wantRoot {
        log.Fatalf("❌ state root mismatch: got %s, want %s", newRoot.Hex(), wantRoot.Hex())
    }
    fmt.Println("  ✓ State root verified!")

    if *verifyOnly {
        fmt.Println("verify-only: not stamping heads/vm; done.")
        return
    }

    // ---- Stamp heads (not strictly required for state-only, but harmless) ----
    if tipHash != (common.Hash{}) {
        rawdb.WriteHeadHeaderHash(dstEth, tipHash)
        rawdb.WriteHeadBlockHash(dstEth, tipHash)
        rawdb.WriteHeadFastBlockHash(dstEth, tipHash)
        fmt.Printf("  ✓ Heads set to tip %s\n", tipHash.Hex())
    }

    // ---- Stamp VM metadata ----
    vm := openBadgerKV(*dstVMPath)
    defer vm.Close()
    if err := vm.Put([]byte("lastAccepted"), tipHash[:]); err != nil {
        log.Fatalf("vm lastAccepted: %v", err)
    }
    if err := vm.Put([]byte("lastAcceptedHeight"), be8(*height)); err != nil {
        log.Fatalf("vm lastAcceptedHeight: %v", err)
    }
    if err := vm.Put([]byte("initialized"), []byte{1}); err != nil {
        log.Fatalf("vm initialized: %v", err)
    }
    fmt.Printf("  ✓ VM metadata: lastAccepted=%s height=%d initialized=1\n", tipHash.Hex(), *height)

    elapsed := time.Since(time.Now())
    fmt.Printf("\n✅ Rehash-state complete! Time: %s\n", elapsed)
    fmt.Println("\n🚀 Next steps:")
    fmt.Println("  1. Start luxd:")
    fmt.Println("     CORETH_DISABLE_FREEZER=1 luxd --network-id=96369 --http-host=0.0.0.0 --http-port=9630")
    fmt.Println("  2. Test balance queries:")
    fmt.Println("     curl -X POST -H 'Content-Type: application/json' \\")
    fmt.Println("       -d '{\"jsonrpc\":\"2.0\",\"method\":\"eth_getBalance\",\"params\":[\"0xEAbCC110fAcBfebabC66Ad6f9E7B67288e720B59\",\"latest\"],\"id\":1}' \\")
    fmt.Println("       http://localhost:9630")
}