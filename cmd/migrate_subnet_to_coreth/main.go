package main

import (
    "encoding/binary"
    "encoding/hex"
    "flag"
    "fmt"
    "log"
    "math/big"
    "path/filepath"

    "github.com/cockroachdb/pebble" // read old SubnetEVM Pebble
    "github.com/luxfi/database/badgerdb" // write new C-chain Badger

    "github.com/ethereum/go-ethereum/common"
    "github.com/ethereum/go-ethereum/core/types"
    "github.com/ethereum/go-ethereum/core/rawdb"
    "github.com/ethereum/go-ethereum/params"
    "github.com/ethereum/go-ethereum/rlp"
    "github.com/ethereum/go-ethereum/ethdb"
)

var (
    srcPath  = flag.String("src", "", "SubnetEVM Pebble path (old)")
    dstPath  = flag.String("dst", "", "C-chain ethdb path (new)")
    tipHex   = flag.String("tip", "0x32dede1fc8e0f11ecde12fb42aef7933fc6c5fcf863bc277b5eac08ae4d461f0", "tip hash")
    tipNum   = flag.Uint64("height", 1082780, "tip height")
    nsHex    = flag.String("ns", "", "32-byte namespace hex (optional; auto-detect if empty)")
    writeTD  = flag.Bool("td", true, "write TD entries (td=height+1)")
)

type kv struct{ *badgerdb.Database }

func (k kv) Get(key []byte) ([]byte, error) { return k.Database.Get(key) }
func (k kv) Has(key []byte) (bool, error) {
    if v, _ := k.Database.Get(key); v != nil { return true, nil }
    return false, nil
}
func (k kv) Put(key, val []byte) error { return k.Database.Put(key, val) }
func (k kv) Delete(key []byte) error   { return k.Database.Delete(key) }
func (k kv) NewBatch() ethdb.Batch      { panic("not implemented") }
func (k kv) NewBatchWithSize(int) ethdb.Batch { panic("not implemented") }
func (k kv) NewIterator(prefix []byte, start []byte) ethdb.Iterator { panic("not implemented") }
func (k kv) Stat(property string) (string, error) { return "", nil }
func (k kv) Compact(start []byte, limit []byte) error { return nil }
func (k kv) Close() error { return k.Database.Close() }

// Ancient methods - not used but needed for interface
func (k kv) HasAncient(kind string, number uint64) (bool, error) { return false, nil }
func (k kv) Ancient(kind string, number uint64) ([]byte, error) { return nil, nil }
func (k kv) AncientRange(kind string, start, count, maxBytes uint64) ([][]byte, error) { return nil, nil }
func (k kv) Ancients() (uint64, error) { return 0, nil }
func (k kv) Tail() (uint64, error) { return 0, nil }
func (k kv) AncientSize(kind string) (uint64, error) { return 0, nil }
func (k kv) ReadAncients(fn func(ethdb.AncientReaderOp) error) (err error) { return nil }

var _ ethdb.KeyValueReader = kv{}
var _ ethdb.KeyValueWriter = kv{}

func mustHash(s string) common.Hash {
    b, err := hex.DecodeString(s[2:])
    if err != nil || len(b) != 32 { log.Fatalf("bad hash %s", s) }
    var h common.Hash; copy(h[:], b); return h
}

func be8(n uint64) []byte { var b [8]byte; binary.BigEndian.PutUint64(b[:], n); return b[:] }

// SubnetEVM header key pattern in Pebble (namespaced):
// ns(32) || 'h' || num(8BE) || hash(32) -> RLP(Header)
func makeSrcHeaderKey(ns []byte, num uint64, hash common.Hash) []byte {
    buf := make([]byte, 0, 32+1+8+32)
    buf = append(buf, ns...)
    buf = append(buf, 'h')
    buf = append(buf, be8(num)...)
    buf = append(buf, hash[:]...)
    return buf
}

// Bodies: ns || 'b' || num || hash -> RLP(Body)
// Receipts: ns || 'r' || num || hash -> RLP([]*Receipt)
func makeSrcBodyKey(ns []byte, num uint64, hash common.Hash) []byte {
    buf := make([]byte, 0, 32+1+8+32)
    buf = append(buf, ns...); buf = append(buf, 'b')
    buf = append(buf, be8(num)...); buf = append(buf, hash[:]...)
    return buf
}
func makeSrcReceiptsKey(ns []byte, num uint64, hash common.Hash) []byte {
    buf := make([]byte, 0, 32+1+8+32)
    buf = append(buf, ns...); buf = append(buf, 'r')
    buf = append(buf, be8(num)...); buf = append(buf, hash[:]...)
    return buf
}

func autoDetectNamespace(db *pebble.DB) []byte {
    it, _ := db.NewIter(&pebble.IterOptions{})
    defer it.Close()
    // scan first ~100 keys, find a 32-byte prefix where next byte is one of {'h','b','r'}
    count := 0
    for it.First(); it.Valid() && count < 1000; it.Next() {
        k := it.Key()
        if len(k) >= 32+1 && (k[32] == 'h' || k[32] == 'b' || k[32] == 'r') {
            ns := append([]byte(nil), k[:32]...)
            fmt.Printf("Auto-detected namespace: %x\n", ns)
            return ns
        }
        count++
    }
    // No namespace detected, assume no namespace
    fmt.Println("No namespace detected, assuming non-namespaced database")
    return nil
}

func readHeaderPebble(pdb *pebble.DB, ns []byte, num uint64, hash common.Hash) *types.Header {
    val, closer, err := pdb.Get(makeSrcHeaderKey(ns, num, hash))
    if err != nil { 
        // Try without namespace
        key := makeSrcHeaderKey(nil, num, hash)[len(ns):]
        val, closer, err = pdb.Get(key)
        if err != nil {
            log.Fatalf("missing header at %d %x: %v", num, hash[:8], err)
        }
    }
    defer closer.Close()
    var hdr types.Header
    if err := rlp.DecodeBytes(val, &hdr); err != nil { log.Fatalf("decode header: %v", err) }
    return &hdr
}

func readBodyPebble(pdb *pebble.DB, ns []byte, num uint64, hash common.Hash) *types.Body {
    val, closer, err := pdb.Get(makeSrcBodyKey(ns, num, hash))
    if err != nil { 
        // Try without namespace
        key := makeSrcBodyKey(nil, num, hash)[len(ns):]
        val, closer, err = pdb.Get(key)
        if err != nil {
            return &types.Body{} // tolerate missing, state is king
        }
    }
    defer closer.Close()
    var body types.Body
    if err := rlp.DecodeBytes(val, &body); err != nil { log.Fatalf("decode body: %v", err) }
    return &body
}

func readReceiptsPebble(pdb *pebble.DB, ns []byte, num uint64, hash common.Hash) types.Receipts {
    val, closer, err := pdb.Get(makeSrcReceiptsKey(ns, num, hash))
    if err != nil {
        // Try without namespace
        key := makeSrcReceiptsKey(nil, num, hash)[len(ns):]
        val, closer, err = pdb.Get(key)
        if err != nil {
            return nil
        }
    }
    defer closer.Close()
    var receipts types.Receipts
    if err := rlp.DecodeBytes(val, &receipts); err != nil { log.Fatalf("decode receipts: %v", err) }
    return receipts
}

func main() {
    flag.Parse()
    if *srcPath == "" || *dstPath == "" { log.Fatal("--src and --dst required") }

    // open source (Pebble)
    sdb, err := pebble.Open(filepath.Clean(*srcPath), &pebble.Options{ReadOnly: true})
    if err != nil { log.Fatalf("open pebble: %v", err) }
    defer sdb.Close()

    // find namespace
    var ns []byte
    if *nsHex != "" {
        bs, _ := hex.DecodeString(*nsHex)
        if len(bs) != 32 { log.Fatalf("ns must be 32 bytes hex") }
        ns = bs
    } else {
        ns = autoDetectNamespace(sdb)
    }

    // open destination (Badger – Coreth ethdb/)
    bdb, err := badgerdb.New(filepath.Clean(*dstPath), nil, "", nil)
    if err != nil { log.Fatalf("open badger: %v", err) }
    defer bdb.Close()
    db := kv{bdb}

    tipHash := mustHash(*tipHex)
    tipN := *tipNum

    fmt.Printf("Migrating canonical from tip %x @ %d\n", tipHash[:8], tipN)

    // Walk back canonical chain by parent linkage
    hash := tipHash
    processedCount := 0
    for n := tipN; ; n-- {
        // 1) Read header/body/receipts from SubnetEVM Pebble (namespaced)
        hdr := readHeaderPebble(sdb, ns, n, hash)
        body := readBodyPebble(sdb, ns, n, hash)
        recs := readReceiptsPebble(sdb, ns, n, hash)

        // 2) Write into Coreth ethdb using rawdb (no manual keys)
        rawdb.WriteHeader(db, hdr)
        rawdb.WriteBody(db, hash, n, body)
        rawdb.WriteReceipts(db, hash, n, recs)
        rawdb.WriteCanonicalHash(db, hash, n)
        rawdb.WriteHeaderNumber(db, hash, n)
        if *writeTD {
            td := new(big.Int).SetUint64(n + 1) // PoS-like TD
            // WriteTd doesn't exist in some versions, write manually
            key := append(append([]byte("h"), be8(n)...), hash[:]...)
            key = append([]byte("t"), key[1:]...) // 't' prefix for TD
            tdBytes, _ := rlp.EncodeToBytes(td)
            db.Put(key, tdBytes)
        }

        processedCount++
        if processedCount % 10000 == 0 {
            fmt.Printf("Processed %d blocks (current: %d)...\n", processedCount, n)
        }

        if n == 0 { break }
        hash = hdr.ParentHash
    }

    // set heads
    rawdb.WriteHeadHeaderHash(db, tipHash)
    rawdb.WriteHeadBlockHash(db, tipHash)
    rawdb.WriteHeadFastBlockHash(db, tipHash)

    // Write genesis configuration
    genesisHash := rawdb.ReadCanonicalHash(db, 0)
    if genesisHash != (common.Hash{}) {
        // Create a basic chain config with Cancun support
        chainConfig := &params.ChainConfig{
            ChainID:             big.NewInt(96369),
            HomesteadBlock:      big.NewInt(0),
            EIP150Block:         big.NewInt(0),
            EIP155Block:         big.NewInt(0),
            EIP158Block:         big.NewInt(0),
            ByzantiumBlock:      big.NewInt(0),
            ConstantinopleBlock: big.NewInt(0),
            PetersburgBlock:     big.NewInt(0),
            IstanbulBlock:       big.NewInt(0),
            MuirGlacierBlock:    big.NewInt(0),
            BerlinBlock:         big.NewInt(0),
            LondonBlock:         big.NewInt(0),
            ArrowGlacierBlock:   big.NewInt(0),
            GrayGlacierBlock:    big.NewInt(0),
            MergeNetsplitBlock:  big.NewInt(0),
            ShanghaiTime:        new(uint64), // 0
            CancunTime:          new(uint64), // 0
        }
        rawdb.WriteChainConfig(db, genesisHash, chainConfig)
    }

    // sanity: canonical[0] and canonical[tip]
    g := rawdb.ReadCanonicalHash(db, 0)
    c := rawdb.ReadCanonicalHash(db, tipN)
    if (g == common.Hash{}) { log.Fatal("canonical[0] missing") }
    if c != tipHash { log.Fatalf("canonical[%d]!=tip (%x!=%x)", tipN, c[:8], tipHash[:8]) }

    fmt.Printf("Done. Canonical+headers+bodies+receipts (+TD=%v) written 0..%d\n", *writeTD, tipN)
    fmt.Printf("Total blocks processed: %d\n", processedCount)
}