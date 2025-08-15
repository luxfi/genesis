package main

import (
    "encoding/binary"
    "encoding/hex"
    "errors"
    "flag"
    "fmt"
    "log"
    "math/big"
    "os"
    "path/filepath"

    "github.com/cockroachdb/pebble"
    "github.com/luxfi/geth/common"
    "github.com/luxfi/geth/core/rawdb"
    "github.com/luxfi/geth/params"
    "github.com/luxfi/geth/rlp"

    // Coreth adapters – use these instead of dgraph badger directly
    "github.com/luxfi/database/badgerdb"
    "github.com/luxfi/geth/ethdb"
)

// EthDBWrapper wraps badgerdb.Database to implement ethdb.Database interface
type EthDBWrapper struct {
    *badgerdb.Database
}

// Ancient implements ethdb.AncientReader (returns error - no ancients)
func (w *EthDBWrapper) Ancient(kind string, number uint64) ([]byte, error) {
    return nil, errors.New("ancients not supported")
}

// AncientRange implements ethdb.AncientReader
func (w *EthDBWrapper) AncientRange(kind string, start, count, maxBytes uint64) ([][]byte, error) {
    return nil, errors.New("ancients not supported")
}

// Ancients implements ethdb.AncientReader
func (w *EthDBWrapper) Ancients() (uint64, error) {
    return 0, nil
}

// Tail implements ethdb.AncientReader
func (w *EthDBWrapper) Tail() (uint64, error) {
    return 0, nil
}

// AncientSize implements ethdb.AncientReader
func (w *EthDBWrapper) AncientSize(kind string) (uint64, error) {
    return 0, errors.New("ancients not supported")
}

// ReadAncients implements ethdb.AncientReaderOp
func (w *EthDBWrapper) ReadAncients(fn func(reader ethdb.AncientReaderOp) error) (err error) {
    return fn(w)
}

var (
    srcPath     = flag.String("src", "", "source Pebble path (SubnetEVM chaindata .../db/pebbledb)")
    dstEthPath  = flag.String("dst-eth", "", "destination Coreth ethdb path (~/.luxd/.../ethdb)")
    dstVmPath   = flag.String("dst-vm", "", "destination Coreth vm path (~/.luxd/.../vm)")
    chainCfg    = flag.String("chain-config", "", "Coreth chain config JSON file (matches luxd genesis)")
    tipHex      = flag.String("tip", "", "optional tip hash (0x...) to start from; otherwise auto-detect")
    writeTD     = flag.Bool("td", true, "write TD entries (td=num+1)")
    batchEvery  = flag.Int("batch", 10000, "flush interval")
)

func be8(n uint64) []byte { var b [8]byte; binary.BigEndian.PutUint64(b[:], n); return b[:] }

func keySrcHeader(ns []byte, n uint64, h common.Hash) []byte { // ns|'h'|num|hash
    k := make([]byte, 0, 32+1+8+32)
    k = append(k, ns...); k = append(k, 'h'); k = append(k, be8(n)...); k = append(k, h[:]...)
    return k
}
func keySrcBody(ns []byte, n uint64, h common.Hash) []byte { // ns|'b'|num|hash
    k := make([]byte, 0, 32+1+8+32)
    k = append(k, ns...); k = append(k, 'b'); k = append(k, be8(n)...); k = append(k, h[:]...)
    return k
}
func keySrcReceipts(ns []byte, n uint64, h common.Hash) []byte { // ns|'r'|num|hash
    k := make([]byte, 0, 32+1+8+32)
    k = append(k, ns...); k = append(k, 'r'); k = append(k, be8(n)...); k = append(k, h[:]...)
    return k
}
func keySrcHtoN(ns []byte, h common.Hash) []byte { // ns|'H'|hash -> num(8)
    k := make([]byte, 0, 32+1+32)
    k = append(k, ns...); k = append(k, 'H'); k = append(k, h[:]...)
    return k
}

func keyDstHeader(n uint64, h common.Hash) []byte { // 'h'|num|hash
    k := make([]byte, 41)
    k[0] = 'h'; binary.BigEndian.PutUint64(k[1:9], n); copy(k[9:], h[:])
    return k
}
func keyDstBody(n uint64, h common.Hash) []byte { // 'b'|num|hash
    k := make([]byte, 41)
    k[0] = 'b'; binary.BigEndian.PutUint64(k[1:9], n); copy(k[9:], h[:])
    return k
}
func keyDstReceipts(n uint64, h common.Hash) []byte { // 'r'|num|hash
    k := make([]byte, 41)
    k[0] = 'r'; binary.BigEndian.PutUint64(k[1:9], n); copy(k[9:], h[:])
    return k
}
func keyDstCanon(n uint64) []byte { // 'h'|num|'n' -> hash
    k := make([]byte, 10)
    k[0] = 'h'; binary.BigEndian.PutUint64(k[1:9], n); k[9] = 'n'
    return k
}
func keyDstHtoN(h common.Hash) []byte { // 'H'|hash -> num
    k := make([]byte, 33)
    k[0] = 'H'; copy(k[1:], h[:])
    return k
}
func keyDstTD(n uint64, h common.Hash) []byte { // 't'|num|hash -> RLP(td)
    k := make([]byte, 41)
    k[0] = 't'; binary.BigEndian.PutUint64(k[1:9], n); copy(k[9:], h[:])
    return k
}

func autoNamespace(pdb *pebble.DB) []byte {
    it, err := pdb.NewIter(&pebble.IterOptions{})
    if err != nil { log.Fatalf("iter: %v", err) }
    defer it.Close()

    for it.First(); it.Valid(); it.Next() {
        k := it.Key()
        if len(k) >= 33 && (k[32] == 'h' || k[32] == 'b' || k[32] == 'r' || k[32] == 'H') {
            ns := append([]byte(nil), k[:32]...)
            return ns
        }
    }
    log.Fatalf("could not auto-detect 32B namespace; provide manually")
    return nil
}

func mustGet(pdb *pebble.DB, key []byte) []byte {
    val, closer, err := pdb.Get(key)
    if err != nil {
        log.Fatalf("missing key %x: %v", key, err)
    }
    defer closer.Close()
    out := make([]byte, len(val))
    copy(out, val)
    return out
}

func readHeaderRLP(pdb *pebble.DB, ns []byte, n uint64, h common.Hash) []byte {
    return mustGet(pdb, keySrcHeader(ns, n, h))
}
func readBodyRLP(pdb *pebble.DB, ns []byte, n uint64, h common.Hash) []byte {
    val, closer, err := pdb.Get(keySrcBody(ns, n, h))
    if err != nil { return nil }
    defer closer.Close()
    out := make([]byte, len(val)); copy(out, val); return out
}
func readReceiptsRLP(pdb *pebble.DB, ns []byte, n uint64, h common.Hash) []byte {
    val, closer, err := pdb.Get(keySrcReceipts(ns, n, h))
    if err != nil { return nil }
    defer closer.Close()
    out := make([]byte, len(val)); copy(out, val); return out
}

func main() {
    flag.Parse()
    if *srcPath == "" || *dstEthPath == "" {
        log.Fatal("--src and --dst-eth are required")
    }
    if *dstVmPath == "" {
        log.Fatal("--dst-vm is required (to set lastAccepted metadata)")
    }

    // open source Pebble (RO)
    pdb, err := pebble.Open(filepath.Clean(*srcPath), &pebble.Options{ReadOnly: true})
    if err != nil { log.Fatalf("open pebble: %v", err) }
    defer pdb.Close()

    // detect namespace
    var ns []byte
    if env := os.Getenv("SUBNET_NS_HEX"); env != "" {
        b, _ := hex.DecodeString(env)
        if len(b) != 32 { log.Fatalf("SUBNET_NS_HEX must be 32 bytes") }
        ns = b
    } else {
        ns = autoNamespace(pdb)
    }
    fmt.Printf("Namespace: %x\n", ns)

    // detect tip hash
    var tipHash common.Hash
    if *tipHex != "" {
        b, _ := hex.DecodeString((*tipHex)[2:])
        copy(tipHash[:], b)
    } else {
        // try AcceptorTipKey first
        val := mustGet(pdb, append(ns, []byte("AcceptorTipKey")...))
        if len(val) != 32 {
            log.Fatalf("AcceptorTipKey invalid len=%d", len(val))
        }
        copy(tipHash[:], val)
    }
    // read number from H[hash]
    numBytes := mustGet(pdb, keySrcHtoN(ns, tipHash))
    if len(numBytes) != 8 { log.Fatalf("H[tip] number invalid") }
    tipNum := binary.BigEndian.Uint64(numBytes)
    fmt.Printf("Tip: #%d %s\n", tipNum, tipHash.Hex())

    // open destination ethdb via Coreth adapter (NOT raw badger)
    bdb, err := badgerdb.New(filepath.Clean(*dstEthPath), nil, "", nil)
    if err != nil { log.Fatalf("open ethdb badger adapter: %v", err) }
    defer bdb.Close()
    
    // Wrap for ethdb interface
    edb := &EthDBWrapper{Database: bdb}

    // write chain config (must match the luxd genesis)
    // For now, we'll use a hardcoded config for network 96369
    // You can replace this with loading from JSON file if needed
    var chainConfig *params.ChainConfig
    if *chainCfg != "" {
        // For now, use hardcoded mainnet config
        // TODO: Load from JSON file when params.ChainConfigFromFile is available
        chainConfig = &params.ChainConfig{
            ChainID:                 big.NewInt(96369),
            HomesteadBlock:          big.NewInt(0),
            EIP150Block:             big.NewInt(0),
            EIP155Block:             big.NewInt(0),
            EIP158Block:             big.NewInt(0),
            ByzantiumBlock:          big.NewInt(0),
            ConstantinopleBlock:     big.NewInt(0),
            PetersburgBlock:         big.NewInt(0),
            IstanbulBlock:           big.NewInt(0),
            MuirGlacierBlock:        big.NewInt(0),
            BerlinBlock:             big.NewInt(0),
            LondonBlock:             big.NewInt(0),
            ShanghaiTime:            func() *uint64 { t := uint64(1607144400); return &t }(),
            CancunTime:              func() *uint64 { t := uint64(253399622400); return &t }(),
            TerminalTotalDifficulty: common.Big0,
        }
        rawdb.WriteChainConfig(edb, common.Hash{}, chainConfig)
    }

    // stream backwards: tip -> 0
    hash := tipHash
    var processed uint64
    batch := edb.NewBatch()
    defer batch.Reset()

    for n := tipNum; ; n-- {
        // raw RLP fetch from source
        hdrRLP := readHeaderRLP(pdb, ns, n, hash)
        bodyRLP := readBodyRLP(pdb, ns, n, hash)
        rcptRLP := readReceiptsRLP(pdb, ns, n, hash)

        // write header/body/receipts directly (raw keys)
        if err := batch.Put(keyDstHeader(n, hash), hdrRLP); err != nil { log.Fatal(err) }
        if bodyRLP != nil {
            if err := batch.Put(keyDstBody(n, hash), bodyRLP); err != nil { log.Fatal(err) }
        }
        if rcptRLP != nil {
            if err := batch.Put(keyDstReceipts(n, hash), rcptRLP); err != nil { log.Fatal(err) }
        }

        // canonical + H→num
        if err := batch.Put(keyDstCanon(n), hash[:]); err != nil { log.Fatal(err) }
        var nb [8]byte; binary.BigEndian.PutUint64(nb[:], n)
        if err := batch.Put(keyDstHtoN(hash), nb[:]); err != nil { log.Fatal(err) }

        // optional TD for convenience
        if *writeTD {
            td := new(big.Int).SetUint64(n + 1)
            tdRLP, _ := rlp.EncodeToBytes(td)
            if err := batch.Put(keyDstTD(n, hash), tdRLP); err != nil { log.Fatal(err) }
        }

        processed++
        if processed%uint64(*batchEvery) == 0 {
            if err := batch.Write(); err != nil { log.Fatal(err) }
            batch.Reset()
            fmt.Printf("... wrote %d blocks (up to #%d)\n", processed, n)
        }

        // step to parent - extract ParentHash directly from RLP
        // The header RLP structure starts with [parentHash, uncleHash, coinbase, ...]
        // We need to decode just the first field
        if n == 0 { break }
        
        // Decode as raw RLP list to get parent hash
        var rawList []rlp.RawValue
        if err := rlp.DecodeBytes(hdrRLP, &rawList); err != nil {
            log.Fatalf("decode header RLP @%d: %v", n, err)
        }
        if len(rawList) < 1 {
            log.Fatalf("header RLP has no fields @%d", n)
        }
        
        // First field is parent hash
        var parentHash common.Hash
        if err := rlp.DecodeBytes(rawList[0], &parentHash); err != nil {
            log.Fatalf("decode parent hash @%d: %v", n, err)
        }
        hash = parentHash
    }

    // final flush
    if err := batch.Write(); err != nil { log.Fatal(err) }

    // set heads
    rawdb.WriteHeadHeaderHash(edb, tipHash)
    rawdb.WriteHeadBlockHash(edb, tipHash)
    rawdb.WriteHeadFastBlockHash(edb, tipHash)

    // now that canonical[0] is known, ensure chain config is stored under genesis hash
    genHash := rawdb.ReadCanonicalHash(edb, 0)
    if chainConfig != nil {
        rawdb.WriteChainConfig(edb, genHash, chainConfig)
    }

    // set VM metadata
    if *dstVmPath != "" {
        vdb, err := pebble.Open(filepath.Clean(*dstVmPath), &pebble.Options{})
        if err != nil { log.Fatalf("open vm pebble: %v", err) }
        defer vdb.Close()
        if err := vdb.Set([]byte("lastAccepted"), tipHash[:], pebble.Sync); err != nil { log.Fatal(err) }
        var nb [8]byte; binary.BigEndian.PutUint64(nb[:], tipNum)
        if err := vdb.Set([]byte("lastAcceptedHeight"), nb[:], pebble.Sync); err != nil { log.Fatal(err) }
        if err := vdb.Set([]byte("initialized"), []byte{1}, pebble.Sync); err != nil { log.Fatal(err) }
    }

    fmt.Printf("✅ Done. Wrote %d blocks 0..%d into %s\n", processed, tipNum, *dstEthPath)
}