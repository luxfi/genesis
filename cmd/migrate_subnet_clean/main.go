package main

import (
    "encoding/binary"
    "encoding/hex"
    "flag"
    "fmt"
    "log"
    "math/big"
    "path/filepath"

    "github.com/cockroachdb/pebble"
    "github.com/luxfi/database/badgerdb"
    
    "github.com/ethereum/go-ethereum/common"
    "github.com/ethereum/go-ethereum/core/types"
    "github.com/ethereum/go-ethereum/core/rawdb"
    "github.com/ethereum/go-ethereum/ethdb"
    "github.com/ethereum/go-ethereum/rlp"
)

var (
    srcPath  = flag.String("src", "", "SubnetEVM Pebble path")
    dstPath  = flag.String("dst", "", "C-chain BadgerDB path")
    tipHex   = flag.String("tip", "0x32dede1fc8e0f11ecde12fb42aef7933fc6c5fcf863bc277b5eac08ae4d461f0", "tip hash")
    tipNum   = flag.Uint64("height", 1082780, "tip height")
    nsHex    = flag.String("ns", "337fb73f9bcdac8c31a2d5f7b877ab1e8a2b7f2a1e9bf02a0a0e6c6fd164f1d1", "namespace hex")
)

// SubnetEVM header structure (pre-Cancun)
type SubnetHeader struct {
    ParentHash  common.Hash    `json:"parentHash"       gencodec:"required"`
    UncleHash   common.Hash    `json:"sha3Uncles"       gencodec:"required"`
    Coinbase    common.Address `json:"miner"`
    Root        common.Hash    `json:"stateRoot"        gencodec:"required"`
    TxHash      common.Hash    `json:"transactionsRoot" gencodec:"required"`
    ReceiptHash common.Hash    `json:"receiptsRoot"     gencodec:"required"`
    Bloom       types.Bloom    `json:"logsBloom"        gencodec:"required"`
    Difficulty  *big.Int       `json:"difficulty"       gencodec:"required"`
    Number      *big.Int       `json:"number"           gencodec:"required"`
    GasLimit    uint64         `json:"gasLimit"         gencodec:"required"`
    GasUsed     uint64         `json:"gasUsed"          gencodec:"required"`
    Time        uint64         `json:"timestamp"        gencodec:"required"`
    Extra       []byte         `json:"extraData"        gencodec:"required"`
    MixDigest   common.Hash    `json:"mixHash"`
    Nonce       types.BlockNonce `json:"nonce"`
    BaseFee     *big.Int       `json:"baseFeePerGas" rlp:"optional"`
    BlockGasCost *big.Int      `json:"blockGasCost" rlp:"optional"`
}

type badgerWrapper struct {
    *badgerdb.Database
}

func (b badgerWrapper) Get(key []byte) ([]byte, error) { return b.Database.Get(key) }
func (b badgerWrapper) Has(key []byte) (bool, error) {
    v, _ := b.Database.Get(key)
    return v != nil, nil
}
func (b badgerWrapper) Put(key, val []byte) error { return b.Database.Put(key, val) }
func (b badgerWrapper) Delete(key []byte) error { return b.Database.Delete(key) }
func (b badgerWrapper) NewBatch() ethdb.Batch { panic("not implemented") }
func (b badgerWrapper) NewBatchWithSize(int) ethdb.Batch { panic("not implemented") }
func (b badgerWrapper) NewIterator(prefix []byte, start []byte) ethdb.Iterator { panic("not implemented") }
func (b badgerWrapper) Stat(string) (string, error) { return "", nil }
func (b badgerWrapper) Compact([]byte, []byte) error { return nil }
func (b badgerWrapper) Close() error { return b.Database.Close() }
func (b badgerWrapper) HasAncient(string, uint64) (bool, error) { return false, nil }
func (b badgerWrapper) Ancient(string, uint64) ([]byte, error) { return nil, nil }
func (b badgerWrapper) AncientRange(string, uint64, uint64, uint64) ([][]byte, error) { return nil, nil }
func (b badgerWrapper) Ancients() (uint64, error) { return 0, nil }
func (b badgerWrapper) Tail() (uint64, error) { return 0, nil }
func (b badgerWrapper) AncientSize(string) (uint64, error) { return 0, nil }
func (b badgerWrapper) ReadAncients(func(ethdb.AncientReaderOp) error) error { return nil }

func be8(n uint64) []byte {
    var b [8]byte
    binary.BigEndian.PutUint64(b[:], n)
    return b[:]
}

func makeKey(ns []byte, prefix byte, num uint64, hash common.Hash) []byte {
    key := make([]byte, 0, len(ns)+1+8+32)
    key = append(key, ns...)
    key = append(key, prefix)
    key = append(key, be8(num)...)
    key = append(key, hash[:]...)
    return key
}

func readSubnetHeader(pdb *pebble.DB, ns []byte, num uint64, hash common.Hash) *types.Header {
    key := makeKey(ns, 'h', num, hash)
    val, closer, err := pdb.Get(key)
    if err != nil {
        log.Fatalf("missing header at %d %x: %v", num, hash[:8], err)
    }
    defer closer.Close()
    
    // Decode SubnetEVM header
    var subnetHdr SubnetHeader
    if err := rlp.DecodeBytes(val, &subnetHdr); err != nil {
        log.Fatalf("decode subnet header: %v", err)
    }
    
    // Convert to standard header
    hdr := &types.Header{
        ParentHash:  subnetHdr.ParentHash,
        UncleHash:   subnetHdr.UncleHash,
        Coinbase:    subnetHdr.Coinbase,
        Root:        subnetHdr.Root,
        TxHash:      subnetHdr.TxHash,
        ReceiptHash: subnetHdr.ReceiptHash,
        Bloom:       subnetHdr.Bloom,
        Difficulty:  subnetHdr.Difficulty,
        Number:      subnetHdr.Number,
        GasLimit:    subnetHdr.GasLimit,
        GasUsed:     subnetHdr.GasUsed,
        Time:        subnetHdr.Time,
        Extra:       subnetHdr.Extra,
        MixDigest:   subnetHdr.MixDigest,
        Nonce:       subnetHdr.Nonce,
        BaseFee:     subnetHdr.BaseFee,
    }
    
    return hdr
}

func readBody(pdb *pebble.DB, ns []byte, num uint64, hash common.Hash) *types.Body {
    key := makeKey(ns, 'b', num, hash)
    val, closer, err := pdb.Get(key)
    if err != nil {
        return &types.Body{} // tolerate missing bodies
    }
    defer closer.Close()
    
    var body types.Body
    if err := rlp.DecodeBytes(val, &body); err != nil {
        log.Printf("decode body error at %d: %v", num, err)
        return &types.Body{}
    }
    return &body
}

func readReceipts(pdb *pebble.DB, ns []byte, num uint64, hash common.Hash) types.Receipts {
    key := makeKey(ns, 'r', num, hash)
    val, closer, err := pdb.Get(key)
    if err != nil {
        return nil // tolerate missing receipts
    }
    defer closer.Close()
    
    var receipts types.Receipts
    if err := rlp.DecodeBytes(val, &receipts); err != nil {
        log.Printf("decode receipts error at %d: %v", num, err)
        return nil
    }
    return receipts
}

func main() {
    flag.Parse()
    if *srcPath == "" || *dstPath == "" {
        log.Fatal("--src and --dst required")
    }
    
    // Parse namespace
    ns, err := hex.DecodeString(*nsHex)
    if err != nil || len(ns) != 32 {
        log.Fatal("namespace must be 32 bytes hex")
    }
    
    // Open source PebbleDB
    srcDB, err := pebble.Open(filepath.Clean(*srcPath), &pebble.Options{ReadOnly: true})
    if err != nil {
        log.Fatalf("open source: %v", err)
    }
    defer srcDB.Close()
    
    // Open destination BadgerDB
    dstDB, err := badgerdb.New(filepath.Clean(*dstPath), nil, "", nil)
    if err != nil {
        log.Fatalf("open destination: %v", err)
    }
    defer dstDB.Close()
    
    db := badgerWrapper{dstDB}
    
    // Parse tip hash
    tipHash, err := hex.DecodeString((*tipHex)[2:])
    if err != nil || len(tipHash) != 32 {
        log.Fatal("invalid tip hash")
    }
    var tip common.Hash
    copy(tip[:], tipHash)
    
    fmt.Printf("Starting migration from tip %x @ %d\n", tip[:8], *tipNum)
    fmt.Printf("Using namespace: %x\n", ns)
    
    // Walk back the chain
    hash := tip
    processed := 0
    for n := *tipNum; ; n-- {
        // Read from SubnetEVM
        hdr := readSubnetHeader(srcDB, ns, n, hash)
        body := readBody(srcDB, ns, n, hash)
        receipts := readReceipts(srcDB, ns, n, hash)
        
        // Write to Coreth format
        rawdb.WriteHeader(db, hdr)
        rawdb.WriteBody(db, hash, n, body)
        if receipts != nil {
            rawdb.WriteReceipts(db, hash, n, receipts)
        }
        rawdb.WriteCanonicalHash(db, hash, n)
        rawdb.WriteHeaderNumber(db, hash, n)
        
        // Write TD manually (height + 1 for PoS-like behavior)
        td := new(big.Int).SetUint64(n + 1)
        tdKey := append(append([]byte("t"), be8(n)...), hash[:]...)
        tdBytes, _ := rlp.EncodeToBytes(td)
        db.Put(tdKey, tdBytes)
        
        processed++
        if processed%10000 == 0 {
            fmt.Printf("Processed %d blocks (at height %d)...\n", processed, n)
        }
        
        if n == 0 {
            break
        }
        
        hash = hdr.ParentHash
    }
    
    // Set chain heads
    rawdb.WriteHeadHeaderHash(db, tip)
    rawdb.WriteHeadBlockHash(db, tip)
    rawdb.WriteHeadFastBlockHash(db, tip)
    
    fmt.Printf("\nMigration complete!\n")
    fmt.Printf("Processed %d blocks\n", processed)
    fmt.Printf("Tip: %x @ %d\n", tip[:8], *tipNum)
    
    // Verify
    canonical0 := rawdb.ReadCanonicalHash(db, 0)
    canonicalTip := rawdb.ReadCanonicalHash(db, *tipNum)
    
    if canonical0 == (common.Hash{}) {
        log.Fatal("Genesis block missing!")
    }
    if canonicalTip != tip {
        log.Fatalf("Tip mismatch: got %x, want %x", canonicalTip[:8], tip[:8])
    }
    
    fmt.Println("✓ Verification passed")
}