package main

import (
    "context"
    "fmt"
    "math/big"
    "net/http"
    "os"
    "path/filepath"
    "time"
    
    "github.com/luxfi/database/badgerdb"
    "github.com/luxfi/geth/common"
    "github.com/luxfi/geth/consensus"
    "github.com/luxfi/geth/consensus/clique"
    "github.com/luxfi/geth/core"
    "github.com/luxfi/geth/core/txpool"
    "github.com/luxfi/geth/core/txpool/legacypool"
    "github.com/luxfi/geth/core/types"
    "github.com/luxfi/geth/eth/ethconfig"
    "github.com/luxfi/geth/ethdb"
    "github.com/luxfi/geth/params"
    "github.com/luxfi/geth/rpc"
    "github.com/luxfi/geth/triedb"
)

// EthDBWrapper wraps badgerdb.Database to implement ethdb.Database interface
type EthDBWrapper struct {
    *badgerdb.Database
}

func (w *EthDBWrapper) Ancient(kind string, number uint64) ([]byte, error) {
    return nil, ethdb.ErrNotSupported
}

func (w *EthDBWrapper) AncientRange(kind string, start, count, maxBytes uint64) ([][]byte, error) {
    return nil, ethdb.ErrNotSupported
}

func (w *EthDBWrapper) Ancients() (uint64, error) {
    return 0, nil
}

func (w *EthDBWrapper) Tail() (uint64, error) {
    return 0, nil
}

func (w *EthDBWrapper) AncientSize(kind string) (uint64, error) {
    return 0, ethdb.ErrNotSupported
}

func (w *EthDBWrapper) ReadAncients(fn func(reader ethdb.AncientReaderOp) error) (err error) {
    return fn(w)
}

func main() {
    fmt.Println("=== Starting Lux Node with Migrated Data ===")
    
    // Open the migrated database
    ethdbPath := "/Users/z/.luxd/network-96369/chains/EWi9aPkGe6EfJ3SobCAmSUXRPLa4brF3cThwPwmHTrD1y13jy/ethdb"
    bdb, err := badgerdb.New(filepath.Clean(ethdbPath), nil, "", nil)
    if err != nil {
        panic(fmt.Sprintf("Failed to open ethdb: %v", err))
    }
    defer bdb.Close()
    
    db := &EthDBWrapper{Database: bdb}
    
    // Create chain config
    chainConfig := &params.ChainConfig{
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
    
    // Create consensus engine
    engine := clique.New(chainConfig.Clique, db)
    
    // Create blockchain
    fmt.Println("Creating blockchain...")
    
    // Check if we have existing data
    if headHash, err := db.Get([]byte("LastBlock")); err == nil && len(headHash) == 32 {
        fmt.Printf("Found existing head: %x\n", headHash)
    }
    
    // Create blockchain without genesis (use existing)
    blockchain, err := core.NewBlockChain(db, nil, engine, core.BlockChainOptions{
        Config:          chainConfig,
        TrieDB:          triedb.NewDatabase(db, &triedb.Config{}),
        SkipBadBlocks:   true,
    })
    if err != nil {
        // Try with a minimal genesis if needed
        genesis := &core.Genesis{
            Config:     chainConfig,
            Difficulty: big.NewInt(1),
            GasLimit:   12000000,
            Timestamp:  1730446786,
        }
        
        blockchain, err = core.NewBlockChain(db, genesis, engine, core.BlockChainOptions{
            Config:          chainConfig,
            TrieDB:          triedb.NewDatabase(db, &triedb.Config{}),
            SkipBadBlocks:   true,
        })
        if err != nil {
            panic(fmt.Sprintf("Failed to create blockchain: %v", err))
        }
    }
    defer blockchain.Stop()
    
    fmt.Printf("✓ Blockchain created\n")
    fmt.Printf("  Current block: #%d\n", blockchain.CurrentBlock().Number.Uint64())
    fmt.Printf("  Current hash: %s\n", blockchain.CurrentBlock().Hash().Hex())
    
    // Create transaction pool
    legacyPool := legacypool.New(legacypool.DefaultConfig, blockchain)
    txPool, err := txpool.New(new(big.Int).SetUint64(1), blockchain, []txpool.SubPool{legacyPool})
    if err != nil {
        panic(fmt.Sprintf("Failed to create tx pool: %v", err))
    }
    defer txPool.Close()
    
    // Create RPC server
    fmt.Println("\nStarting RPC server on :9630...")
    
    rpcServer := rpc.NewServer()
    
    // Register a simple eth_blockNumber method
    rpcServer.RegisterName("eth", &EthAPI{blockchain: blockchain})
    
    // Start HTTP server
    httpServer := &http.Server{
        Addr:    ":9630",
        Handler: rpcServer,
    }
    
    go func() {
        if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
            fmt.Printf("HTTP server error: %v\n", err)
        }
    }()
    
    fmt.Println("✓ Node started successfully!")
    fmt.Println("  RPC endpoint: http://localhost:9630")
    fmt.Println("\nTest with:")
    fmt.Println(`  curl -X POST -H "Content-Type: application/json" -d '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' http://localhost:9630`)
    
    // Keep running
    select {}
}

// Simple Eth API
type EthAPI struct {
    blockchain *core.BlockChain
}

func (api *EthAPI) BlockNumber() (string, error) {
    return fmt.Sprintf("0x%x", api.blockchain.CurrentBlock().Number.Uint64()), nil
}

func (api *EthAPI) GetBlockByNumber(number string, full bool) (map[string]interface{}, error) {
    var blockNum uint64
    if number == "latest" {
        blockNum = api.blockchain.CurrentBlock().Number.Uint64()
    } else {
        fmt.Sscanf(number, "0x%x", &blockNum)
    }
    
    block := api.blockchain.GetBlockByNumber(blockNum)
    if block == nil {
        return nil, fmt.Errorf("block not found")
    }
    
    return map[string]interface{}{
        "number":     fmt.Sprintf("0x%x", block.NumberU64()),
        "hash":       block.Hash().Hex(),
        "parentHash": block.ParentHash().Hex(),
        "timestamp":  fmt.Sprintf("0x%x", block.Time()),
        "gasLimit":   fmt.Sprintf("0x%x", block.GasLimit()),
        "gasUsed":    fmt.Sprintf("0x%x", block.GasUsed()),
    }, nil
}