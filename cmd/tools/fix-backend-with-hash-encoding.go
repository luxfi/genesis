// Fix for backend.go to handle subnet-EVM database with block numbers encoded in hash

package main

import (
    "bytes"
    "encoding/binary"
    "fmt"
)

// Subnet-EVM namespace for LUX mainnet
var subnetNamespace = []byte{
    0x33, 0x7f, 0xb7, 0x3f, 0x9b, 0xcd, 0xac, 0x8c,
    0x31, 0xa2, 0xd5, 0xf7, 0xb8, 0x77, 0xab, 0x1e,
    0x8a, 0x2b, 0x7f, 0x2a, 0x1e, 0x9b, 0xf0, 0x2a,
    0x0a, 0x0e, 0x6c, 0x6f, 0xd1, 0x64, 0xf1, 0xd1,
}

// FindBlockByNumber finds a block by scanning for headers where the first 3 bytes encode the block number
func FindBlockByNumber(db ethdb.Database, blockNum uint64) (hash []byte, header []byte, err error) {
    // In subnet-EVM, blocks are stored as namespace + 32-byte hash -> RLP header
    // The first 3 bytes of the hash encode the block number
    
    // We need to iterate through the database to find blocks
    iter := db.NewIterator(nil, nil)
    defer iter.Release()
    
    for iter.Next() {
        key := iter.Key()
        value := iter.Value()
        
        // Check if this is a namespaced header
        if len(key) == 64 && bytes.HasPrefix(key, subnetNamespace) {
            hash := key[32:]
            
            // Check if value is RLP header (starts with 0xf8 or 0xf9)
            if len(value) > 100 && (value[0] == 0xf8 || value[0] == 0xf9) {
                // Decode block number from first 3 bytes of hash
                hashBlockNum := uint64(hash[0])<<16 | uint64(hash[1])<<8 | uint64(hash[2])
                
                if hashBlockNum == blockNum {
                    // Found the block!
                    hashCopy := make([]byte, 32)
                    copy(hashCopy, hash)
                    headerCopy := make([]byte, len(value))
                    copy(headerCopy, value)
                    return hashCopy, headerCopy, nil
                }
            }
        }
    }
    
    return nil, nil, fmt.Errorf("block %d not found", blockNum)
}

// NewMigratedBackendFixed creates a backend that properly handles subnet-EVM format
func NewMigratedBackendFixed(db ethdb.Database, migratedHeight uint64) (*MinimalEthBackend, error) {
    fmt.Printf("Creating migrated backend for subnet-EVM data at height %d\n", migratedHeight)
    
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
        BerlinBlock:             big.NewInt(0),
        LondonBlock:             big.NewInt(0),
        TerminalTotalDifficulty: common.Big0,
    }
    
    // Create dummy consensus engine
    engine := &dummyEngine{}
    
    // Find the head block using the hash encoding
    fmt.Printf("Looking for block at height %d using hash encoding...\n", migratedHeight)
    
    headHash, headerData, err := FindBlockByNumber(db, migratedHeight)
    if err != nil {
        // Try block 0 if the requested height doesn't exist
        fmt.Printf("Block %d not found, trying block 0...\n", migratedHeight)
        headHash, headerData, err = FindBlockByNumber(db, 0)
        if err != nil {
            return nil, fmt.Errorf("could not find any blocks in database: %w", err)
        }
        migratedHeight = 0
    }
    
    fmt.Printf("Found block %d with hash: %x\n", migratedHeight, headHash)
    fmt.Printf("Header size: %d bytes\n", len(headerData))
    
    // Convert hash to common.Hash
    var commonHash common.Hash
    copy(commonHash[:], headHash)
    
    // Write the canonical mappings in standard format
    rawdb.WriteCanonicalHash(db, commonHash, migratedHeight)
    rawdb.WriteHeadBlockHash(db, commonHash)
    rawdb.WriteHeadHeaderHash(db, commonHash)
    rawdb.WriteHeadFastBlockHash(db, commonHash)
    rawdb.WriteLastPivotNumber(db, migratedHeight)
    
    // Create blockchain without genesis
    options := &gethcore.BlockChainConfig{
        TrieCleanLimit: 256,
        NoPrefetch:     false,
        StateScheme:    rawdb.HashScheme,
    }
    
    blockchain, err := gethcore.NewBlockChain(db, nil, engine, options)
    if err != nil {
        return nil, fmt.Errorf("failed to create blockchain: %w", err)
    }
    
    currentBlock := blockchain.CurrentBlock()
    fmt.Printf("Blockchain initialized at height: %d\n", currentBlock.Number.Uint64())
    
    // Create transaction pool
    legacyPool := legacypool.New(ethconfig.Defaults.TxPool, blockchain)
    txPool, err := txpool.New(ethconfig.Defaults.TxPool.PriceLimit, blockchain, []txpool.SubPool{legacyPool})
    if err != nil {
        return nil, err
    }
    
    return &MinimalEthBackend{
        chainConfig: chainConfig,
        blockchain:  blockchain,
        txPool:      txPool,
        chainDb:     db,
        engine:      engine,
        networkID:   96369,
    }, nil
}