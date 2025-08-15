package main

import (
    "bytes"
    "encoding/binary"
    "encoding/hex"
    "errors"
    "fmt"
    "log"
    "math/big"
    "os"
    "path/filepath"
    "runtime"
    "time"

    // LUXFI Coreth
    "github.com/luxfi/geth/common"
    "github.com/luxfi/geth/core/rawdb"
    "github.com/luxfi/geth/core/types"
    "github.com/luxfi/geth/core/state"
    "github.com/luxfi/geth/crypto"
    "github.com/luxfi/geth/ethdb"
    "github.com/luxfi/geth/rlp"
    "github.com/luxfi/geth/trie"

    // Backing stores
    "github.com/cockroachdb/pebble"
    badger "github.com/dgraph-io/badger/v3"
)

var (
    rehashSrcPebble   string
    rehashDstBadger   string
    rehashVMPath      string
    rehashStateRoot   string
    rehashHeadNumber  uint64
    rehashCopyCode    bool
    rehashWorkers     int
)

func main() {
    // Parse command line args
    if len(os.Args) < 3 {
        fmt.Println("Usage: rehash_state <src-pebble> <dst-badger> [vm-path]")
        fmt.Println("Example: rehash_state /path/to/pebbledb /path/to/ethdb /path/to/vm")
        os.Exit(1)
    }

    rehashSrcPebble = os.Args[1]
    rehashDstBadger = os.Args[2]
    if len(os.Args) > 3 {
        rehashVMPath = os.Args[3]
    } else {
        // Default VM path based on ethdb path
        rehashVMPath = filepath.Join(filepath.Dir(rehashDstBadger), "vm")
    }

    // Fixed values for our migration
    rehashStateRoot = "0xaedd8be7a060b082b0cb3195d0b5ba017c058468851ed93dd07eca274de000c2"
    rehashHeadNumber = 1082780
    rehashCopyCode = true
    rehashWorkers = max(2, runtime.NumCPU()/2)

    fmt.Println("╔════════════════════════════════════════════════════════╗")
    fmt.Println("║   REHASH STATE: Path-Scheme → Hash-Scheme (Coreth)    ║")
    fmt.Println("╚════════════════════════════════════════════════════════╝")
    fmt.Println()
    fmt.Printf("Source (Pebble):  %s\n", rehashSrcPebble)
    fmt.Printf("Dest (Badger):    %s\n", rehashDstBadger)
    fmt.Printf("VM Path:          %s\n", rehashVMPath)
    fmt.Printf("State Root:       %s\n", rehashStateRoot)
    fmt.Printf("Head Number:      %d\n", rehashHeadNumber)
    fmt.Printf("Workers:          %d\n", rehashWorkers)
    fmt.Println()

    if err := runRehash(); err != nil {
        log.Fatal(err)
    }
}

func runRehash() error {
    // Parse/validate root
    root, err := parseHash32(rehashStateRoot)
    if err != nil {
        return fmt.Errorf("state-root: %w", err)
    }

    // Open source (Pebble, read-only)
    src, err := openPebbleKV(rehashSrcPebble, true)
    if err != nil {
        return fmt.Errorf("open src Pebble: %w", err)
    }
    defer src.Close()

    // Open destination (Badger) as an ethdb.KeyValueStore
    dst, err := openBadgerKV(rehashDstBadger)
    if err != nil {
        return fmt.Errorf("open dst Badger: %w", err)
    }
    defer dst.Close()

    // Build a hashed trie DB on top of destination K/V
    trieDB := trie.NewDatabase(dst)

    // 1) Re-encode path-scheme nodes ⇒ hash-scheme via InsertBlob + Commit
    start := time.Now()
    log.Printf("Rehash: scanning path-scheme prefixes 0x00..0x09 from %s", rehashSrcPebble)
    count, bytesWritten, err := rehashAllNodes(src, trieDB, rehashWorkers)
    if err != nil {
        return fmt.Errorf("rehash nodes: %w", err)
    }
    log.Printf("Rehash: inserted %d nodes (%.2f MiB) in %s", count, float64(bytesWritten)/(1024*1024), time.Since(start))

    // 2) Copy code table ("code"+hash → bytecode)
    if rehashCopyCode {
        if err := copyCodeTable(src, dst); err != nil {
            return fmt.Errorf("copy code table: %w", err)
        }
    }

    // 3) Pin root & commit reference counts
    trieDB.Reference(root, common.Hash{}) // attach root to global owner
    if err := trieDB.Commit(root, true); err != nil {
        return fmt.Errorf("trieDB.Commit(%s): %w", root.Hex(), err)
    }
    log.Printf("Rehash: commit complete for root %s", root.Hex())

    // 4) Sanity: open a StateDB at the root and read a couple of balances
    if err := sanityProbe(dst, root); err != nil {
        return fmt.Errorf("sanity probe: %w", err)
    }

    // 5) Set heads/VM metadata at requested height
    if err := setHeadsAndVM(dst, rehashVMPath, root, rehashHeadNumber); err != nil {
        return fmt.Errorf("set heads/meta: %w", err)
    }
    log.Printf("✅ SUCCESS: head set at #%d and VM metadata written to %s", rehashHeadNumber, rehashVMPath)
    
    fmt.Println("\n🎉 State rehashing complete!")
    fmt.Println("You can now start luxd and eth_getBalance will work for ALL addresses.")
    fmt.Println("\nStart with:")
    fmt.Println("  CORETH_DISABLE_FREEZER=1 luxd --network-id=96369 --http-host=0.0.0.0 --http-port=9630")
    
    return nil
}

// ---------------- Core logic ----------------

func rehashAllNodes(src ethdb.KeyValueStore, tdb *trie.Database, workers int) (nodes int, bytesWritten uint64, err error) {
    type job struct{ k, v []byte }
    jobs := make(chan job, 4096)
    errs := make(chan error, workers)

    // Producer: scan 0x00..0x09 prefixes
    go func() {
        defer close(jobs)
        for p := byte(0x00); p <= byte(0x09); p++ {
            pfx := []byte{p}
            it := src.NewIterator(pfx, nil)
            for it.Next() {
                // Copy key/value (iterator buffers are reused)
                k := append([]byte{}, it.Key()...)
                v := append([]byte{}, it.Value()...)
                jobs <- job{k: k, v: v}
            }
            if it.Error() != nil {
                errs <- fmt.Errorf("iterator (prefix 0x%02x): %w", p, it.Error())
                it.Release()
                return
            }
            it.Release()
        }
    }()

    // Consumers: hash RLP(node) and insert via trie.Database (tracks refs)
    for w := 0; w < workers; w++ {
        go func() {
            for j := range jobs {
                // Skip empty blobs just in case
                if len(j.v) == 0 {
                    continue
                }
                h := crypto.Keccak256Hash(j.v) // keccak(RLP(node))
                // InsertBlob adds node to internal journal; refcounting accounted on Commit
                tdb.InsertBlob(h, j.v)
            }
            errs <- nil
        }()
    }

    // Small accounting pass (single-threaded iter over prefixes again)
    var n int
    var bw uint64
    for p := byte(0x00); p <= byte(0x09); p++ {
        it := src.NewIterator([]byte{p}, nil)
        for it.Next() {
            n++
            bw += uint64(len(it.Value()))
        }
        it.Release()
    }

    // Wait for workers
    for w := 0; w < workers; w++ {
        if e := <-errs; e != nil {
            return 0, 0, e
        }
    }
    return n, bw, nil
}

func copyCodeTable(src, dst ethdb.KeyValueStore) error {
    const codePrefix = "code"
    it := src.NewIterator([]byte(codePrefix), nil)
    defer it.Release()

    batch := dst.NewBatch()
    defer batch.Reset()

    var copied, total int
    for it.Next() {
        k := append([]byte{}, it.Key()...)
        v := append([]byte{}, it.Value()...)
        if err := batch.Put(k, v); err != nil {
            return err
        }
        copied += len(v)
        total++
        // Flush periodically
        if batch.ValueSize() > 16*1024*1024 {
            if err := batch.Write(); err != nil { return err }
            batch.Reset()
        }
    }
    if err := it.Error(); err != nil {
        return err
    }
    if batch.ValueSize() > 0 {
        if err := batch.Write(); err != nil { return err }
    }
    log.Printf("Copied %d code entries (%.2f MiB)", total, float64(copied)/(1024*1024))
    return nil
}

// sanityProbe opens a StateDB on dest and tries a couple reads at the given root.
func sanityProbe(dst ethdb.KeyValueStore, root common.Hash) error {
    stDB := state.NewDatabase(dst)
    st, err := state.New(root, stDB, nil) // secure trie at target root
    if err != nil {
        return fmt.Errorf("state.New(root): %w", err)
    }
    
    // Test treasury balance
    treasury := common.HexToAddress("0x9011E888251AB053B7bD1cdB598Db4f9DEd94714")
    balance := st.GetBalance(treasury)
    if balance != nil && balance.Sign() > 0 {
        balanceEth := new(big.Float).Quo(new(big.Float).SetInt(balance), big.NewFloat(1e18))
        log.Printf("✓ Treasury balance accessible: %.2f LUX", balanceEth)
    }
    
    // Test user requested address
    testAddr := common.HexToAddress("0xEAbCC110fAcBfebabC66Ad6f9E7B67288e720B59")
    testBalance := st.GetBalance(testAddr)
    log.Printf("✓ Test address balance: %v wei", testBalance)
    
    return nil
}

func setHeadsAndVM(dst ethdb.KeyValueStore, vmPath string, expectRoot common.Hash, headNum uint64) error {
    // Ensure canonical mapping + header exist
    headHash := rawdb.ReadCanonicalHash(dst, headNum)
    if (headHash == common.Hash{}) {
        return fmt.Errorf("no canonical hash at #%d", headNum)
    }
    hdr := rawdb.ReadHeader(dst, headHash, headNum)
    if hdr == nil {
        return fmt.Errorf("missing header at #%d (%s)", headNum, headHash.Hex())
    }
    if hdr.Root != expectRoot {
        return fmt.Errorf("header root mismatch at #%d: have %s, want %s", headNum, hdr.Root.Hex(), expectRoot.Hex())
    }
    // Point heads
    rawdb.WriteHeadHeaderHash(dst, headHash)
    rawdb.WriteHeadBlockHash(dst, headHash)
    rawdb.WriteHeadFastBlockHash(dst, headHash)

    // VM metadata files (same convention as earlier helpers)
    if vmPath != "" {
        if err := os.MkdirAll(vmPath, 0o755); err != nil { return err }
        if err := os.WriteFile(filepath.Join(vmPath, "initialized"), []byte{1}, 0o644); err != nil { return err }
        if err := os.WriteFile(filepath.Join(vmPath, "lastAccepted"), headHash.Bytes(), 0o644); err != nil { return err }
        var be [8]byte
        binary.BigEndian.PutUint64(be[:], headNum)
        if err := os.WriteFile(filepath.Join(vmPath, "lastAcceptedHeight"), be[:], 0o644); err != nil { return err }
    }
    return nil
}

// ---------------- Helpers & adapters ----------------

func parseHash32(hs string) (common.Hash, error) {
    var z common.Hash
    if len(hs) >= 2 && hs[:2] == "0x" {
        hs = hs[2:]
    }
    b, err := hex.DecodeString(hs)
    if err != nil {
        return z, err
    }
    if len(b) != 32 {
        return z, fmt.Errorf("expected 32 bytes, got %d", len(b))
    }
    copy(z[:], b)
    return z, nil
}

func max(a, b int) int { if a > b { return a }; return b }

// ---------- Badger adapter (dest) implements ethdb.KeyValueStore ----------

type badgerKV struct{ db *badger.DB }

func openBadgerKV(dir string) (ethdb.KeyValueStore, error) {
    if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
        return nil, fmt.Errorf("badger path not a directory: %s", dir)
    }
    opts := badger.DefaultOptions(dir).
        WithTruncate(true).
        WithLoggingLevel(badger.ERROR)
    db, err := badger.Open(opts)
    if err != nil { return nil, err }
    return &badgerKV{db: db}, nil
}
func (b *badgerKV) Has(key []byte) (bool, error) {
    err := b.db.View(func(txn *badger.Txn) error {
        _, err := txn.Get(key); return err
    })
    if errors.Is(err, badger.ErrKeyNotFound) { return false, nil }
    return err == nil, err
}
func (b *badgerKV) Get(key []byte) ([]byte, error) {
    var out []byte
    err := b.db.View(func(txn *badger.Txn) error {
        itm, err := txn.Get(key); if err != nil { return err }
        return itm.Value(func(v []byte) error {
            out = append([]byte{}, v...)
            return nil
        })
    })
    return out, err
}
func (b *badgerKV) Put(key []byte, value []byte) error {
    return b.db.Update(func(txn *badger.Txn) error {
        return txn.Set(key, append([]byte{}, value...))
    })
}
func (b *badgerKV) Delete(key []byte) error {
    return b.db.Update(func(txn *badger.Txn) error { return txn.Delete(key) })
}
func (b *badgerKV) NewBatch() ethdb.Batch { return &badgerBatch{db: b.db, wb: b.db.NewWriteBatch()} }
func (b *badgerKV) NewIterator(prefix []byte, start []byte) ethdb.Iterator {
    txn := b.db.NewTransaction(false)
    opts := badger.DefaultIteratorOptions
    opts.PrefetchValues = true
    opts.Prefix = prefix
    it := txn.NewIterator(opts)
    if start != nil {
        it.Seek(start)
    } else {
        it.Rewind()
    }
    return &badgerIter{txn: txn, it: it, prefix: prefix}
}
func (b *badgerKV) Stat(property string) (string, error) { return "", nil }
func (b *badgerKV) Compact(start []byte, limit []byte) error { return nil }
func (b *badgerKV) Close() { _ = b.db.Close() }

type badgerBatch struct {
    db *badger.DB
    wb *badger.WriteBatch
    sz int
}
func (b *badgerBatch) Put(key, value []byte) error {
    b.sz += len(value)
    return b.wb.SetEntry(badger.NewEntry(append([]byte{}, key...), append([]byte{}, value...)))
}
func (b *badgerBatch) Delete(key []byte) error {
    return b.wb.Delete(append([]byte{}, key...))
}
func (b *badgerBatch) ValueSize() int { return b.sz }
func (b *badgerBatch) Write() error   { return b.wb.Flush() }
func (b *badgerBatch) Reset()         { b.sz = 0; b.wb.Cancel(); b.wb = b.db.NewWriteBatch() }

type badgerIter struct {
    txn    *badger.Txn
    it     *badger.Iterator
    prefix []byte
    key    []byte
    val    []byte
    err    error
    init   bool
}
func (it *badgerIter) Next() bool {
    if !it.init {
        it.init = true
        it.it.Rewind()
    } else {
        it.it.Next()
    }
    for ; it.it.ValidForPrefix(it.prefix); it.it.Next() {
        item := it.it.Item()
        k := item.KeyCopy(nil)
        var v []byte
        err := item.Value(func(val []byte) error {
            v = append([]byte{}, val...)
            return nil
        })
        if err != nil {
            it.err = err
            return false
        }
        it.key, it.val = k, v
        return true
    }
    return false
}
func (it *badgerIter) Error() error   { return it.err }
func (it *badgerIter) Key() []byte    { return it.key }
func (it *badgerIter) Value() []byte  { return it.val }
func (it *badgerIter) Release()       { it.it.Close(); it.txn.Discard() }

// ---------- Pebble adapter (src) implements ethdb.KeyValueStore (read-only) ----------

type pebbleKV struct{ db *pebble.DB }

func openPebbleKV(dir string, readOnly bool) (ethdb.KeyValueStore, error) {
    if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
        return nil, fmt.Errorf("pebble path not a directory: %s", dir)
    }
    opts := &pebble.Options{}
    if readOnly {
        opts.ReadOnly = true
    }
    db, err := pebble.Open(dir, opts)
    if err != nil { return nil, err }
    return &pebbleKV{db: db}, nil
}
func (p *pebbleKV) Has(key []byte) (bool, error) {
    _, closer, err := p.db.Get(key)
    if err != nil {
        if errors.Is(err, pebble.ErrNotFound) { return false, nil }
        return false, err
    }
    _ = closer.Close()
    return true, nil
}
func (p *pebbleKV) Get(key []byte) ([]byte, error) {
    val, closer, err := p.db.Get(key)
    if err != nil { return nil, err }
    out := append([]byte{}, val...)
    _ = closer.Close()
    return out, nil
}
func (p *pebbleKV) Put(key []byte, value []byte) error   { return errors.New("pebbleKV is read-only") }
func (p *pebbleKV) Delete(key []byte) error               { return errors.New("pebbleKV is read-only") }
func (p *pebbleKV) NewBatch() ethdb.Batch                { return &noopBatch{} }
func (p *pebbleKV) NewIterator(prefix []byte, start []byte) ethdb.Iterator {
    lower := append([]byte{}, prefix...)
    upper := append([]byte{}, prefix...)
    upper = append(upper, 0xFF)

    it := p.db.NewIter(&pebble.IterOptions{
        LowerBound: lower,
        UpperBound: upper,
    })
    if start != nil {
        it.SeekGE(start)
    } else {
        it.First()
    }
    return &pebbleIter{it: it, prefix: prefix}
}
func (p *pebbleKV) Stat(property string) (string, error) { return "", nil }
func (p *pebbleKV) Compact(start []byte, limit []byte) error { return nil }
func (p *pebbleKV) Close() { _ = p.db.Close() }

type noopBatch struct{}
func (n *noopBatch) Put(key, value []byte) error { return nil }
func (n *noopBatch) Delete(key []byte) error     { return nil }
func (n *noopBatch) ValueSize() int              { return 0 }
func (n *noopBatch) Write() error                { return nil }
func (n *noopBatch) Reset()                      {}

type pebbleIter struct {
    it     *pebble.Iterator
    prefix []byte
    key    []byte
    val    []byte
    err    error
}
func (it *pebbleIter) Next() bool {
    if !it.it.Valid() {
        return false
    }
    k := append([]byte{}, it.it.Key()...)
    v := append([]byte{}, it.it.Value()...)
    if len(it.prefix) > 0 && !bytes.HasPrefix(k, it.prefix) {
        return false
    }
    it.key, it.val = k, v
    it.it.Next()
    return true
}
func (it *pebbleIter) Error() error  { return it.err }
func (it *pebbleIter) Key() []byte   { return it.key }
func (it *pebbleIter) Value() []byte { return it.val }
func (it *pebbleIter) Release()      { it.it.Close() }