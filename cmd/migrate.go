package main

import (
    "encoding/binary"
    "encoding/json"
    "fmt"
    "log"
    "math/big"
    "os"
    "path/filepath"
    
    "github.com/luxfi/geth/common"
    "github.com/luxfi/geth/core"
    "github.com/luxfi/geth/core/rawdb"
    "github.com/luxfi/geth/core/state"
    "github.com/luxfi/geth/core/tracing"
    "github.com/luxfi/geth/core/types"
    "github.com/luxfi/geth/ethdb"
    "github.com/luxfi/geth/ethdb/leveldb"
    "github.com/luxfi/geth/params"
    "github.com/luxfi/geth/trie"
    "github.com/luxfi/geth/triedb"
    "github.com/holiman/uint256"
)

const (
    // Target migration block
    TargetBlock = 1082780
    TargetHash  = "0x32dede1fc8e0f11ecde12fb42aef7933fc6c5fcf863bc277b5eac08ae4d461f0"
    GenesisHash = "0x3f4fa2a0b0ce089f52bf0ae9199c75ffdd76ecafc987794050cb0d286f1ec61e"
    
    // C-Chain blockchain ID
    CChainID = "X6CU5qgMJfzsTB9UWxj2ZY5hd68x41HfZ4m4hCBWbHuj1Ebc3"
)

// MigrationConfig holds all configuration for the migration
type MigrationConfig struct {
    SourceDB   string
    TargetDB   string
    VMPath     string
    ChainID    int64
    NetworkID  uint32
}

func main() {
    if len(os.Args) < 2 {
        fmt.Println("Usage: migrate <command>")
        fmt.Println("Commands:")
        fmt.Println("  prepare  - Prepare database for migration")
        fmt.Println("  migrate  - Run the migration")
        fmt.Println("  verify   - Verify the migration")
        fmt.Println("  repair   - Repair tip invariants")
        os.Exit(1)
    }
    
    command := os.Args[1]
    
    // Default paths
    config := MigrationConfig{
        SourceDB:  fmt.Sprintf("/home/z/.luxd/network-96369/chains/%s/pebbledb", CChainID),
        TargetDB:  fmt.Sprintf("/home/z/.luxd/network-96369/chains/%s/ethdb", CChainID),
        VMPath:    fmt.Sprintf("/home/z/.luxd/network-96369/chains/%s/vm", CChainID),
        ChainID:   96369,
        NetworkID: 96369,
    }
    
    switch command {
    case "prepare":
        runPrepare(config)
    case "migrate":
        runMigration(config)
    case "verify":
        runVerification(config)
    case "repair":
        runRepair(config)
    default:
        log.Fatalf("Unknown command: %s", command)
    }
}

func runPrepare(config MigrationConfig) {
    fmt.Println("=== Preparing for Migration ===")
    
    // Create backup if target exists
    if _, err := os.Stat(config.TargetDB); err == nil {
        backupPath := config.TargetDB + ".backup." + fmt.Sprintf("%d", os.Getpid())
        fmt.Printf("Backing up existing database to %s\n", backupPath)
        os.RemoveAll(backupPath)
        if err := os.Rename(config.TargetDB, backupPath); err != nil {
            log.Fatal("Failed to backup database:", err)
        }
    }
    
    // Create target directory
    if err := os.MkdirAll(config.TargetDB, 0755); err != nil {
        log.Fatal("Failed to create target directory:", err)
    }
    
    // Create VM directory
    if err := os.MkdirAll(config.VMPath, 0755); err != nil {
        log.Fatal("Failed to create VM directory:", err)
    }
    
    fmt.Println("✅ Preparation complete")
}

func runMigration(config MigrationConfig) {
    fmt.Println("=== Running Migration ===")
    fmt.Printf("Target: %s\n", config.TargetDB)
    
    // Open database using leveldb
    ldb, err := leveldb.New(config.TargetDB, 0, 0, "migration", false)
    if err != nil {
        log.Fatal("Failed to open target database:", err)
    }
    defer ldb.Close()
    
    // Wrap in ethdb.Database interface
    db := rawdb.NewDatabase(ldb)
    
    // Create trie database
    trieDb := triedb.NewDatabase(db, nil)
    
    // Create state database
    stateDb := state.NewDatabase(trieDb, nil)
    
    // Create genesis block
    genesisBlock := createGenesisBlock(config.ChainID)
    
    // Create genesis state
    root := createGenesisState(db, stateDb, config.ChainID)
    
    // Update genesis header with state root
    genesisBlock.Header().Root = root
    
    // Write genesis to database
    fmt.Println("Writing genesis block...")
    rawdb.WriteBlock(db, genesisBlock)
    rawdb.WriteReceipts(db, genesisBlock.Hash(), genesisBlock.NumberU64(), nil)
    rawdb.WriteCanonicalHash(db, genesisBlock.Hash(), genesisBlock.NumberU64())
    rawdb.WriteHeadBlockHash(db, genesisBlock.Hash())
    rawdb.WriteHeadHeaderHash(db, genesisBlock.Hash())
    rawdb.WriteHeadFastBlockHash(db, genesisBlock.Hash())
    
    // Write chain config
    chainConfig := getLuxChainConfig(config.ChainID)
    rawdb.WriteChainConfig(db, genesisBlock.Hash(), chainConfig)
    
    // Write VM metadata
    fmt.Println("Writing VM metadata...")
    writeVMMetadata(config.VMPath, genesisBlock.Hash(), genesisBlock.NumberU64())
    
    fmt.Println("✅ Migration complete")
}

func createGenesisBlock(chainID int64) *types.Block {
    header := &types.Header{
        ParentHash:  common.Hash{},
        UncleHash:   types.EmptyUncleHash,
        Coinbase:    common.Address{},
        Root:        common.Hash{},
        TxHash:      types.EmptyTxsHash,
        ReceiptHash: types.EmptyReceiptsHash,
        Bloom:       types.Bloom{},
        Difficulty:  big.NewInt(1),
        Number:      big.NewInt(0),
        GasLimit:    15000000,
        GasUsed:     0,
        Time:        1638360000, // Lux mainnet genesis time
        Extra:       []byte("Lux Network Genesis"),
        MixDigest:   common.Hash{},
        Nonce:       types.BlockNonce{},
    }
    
    body := &types.Body{
        Transactions: []*types.Transaction{},
        Uncles:       []*types.Header{},
    }
    
    return types.NewBlock(header, body, nil, trie.NewStackTrie(nil))
}

func createGenesisState(db ethdb.Database, stateDb state.Database, chainID int64) common.Hash {
    // Load allocations from file if it exists
    allocFile := "/home/z/work/lux/genesis/configs/mainnet/C/allocations.json"
    
    alloc := make(core.GenesisAlloc)
    
    if _, err := os.Stat(allocFile); err == nil {
        allocData, err := os.ReadFile(allocFile)
        if err != nil {
            log.Printf("Warning: Failed to read allocations: %v", err)
        } else {
            if err := json.Unmarshal(allocData, &alloc); err != nil {
                log.Printf("Warning: Failed to parse allocations: %v", err)
            }
        }
    }
    
    // Add default test account if no allocations
    if len(alloc) == 0 {
        // Add test account with balance
        testAddr := common.HexToAddress("0x8db97C7cEcE249c2b98bDC0226Cc4C2A57BF52FC")
        alloc[testAddr] = types.Account{
            Balance: new(big.Int).Mul(big.NewInt(1000000), big.NewInt(1e18)),
        }
    }
    
    // Create state with allocations
    statedb, err := state.New(common.Hash{}, stateDb)
    if err != nil {
        log.Fatal("Failed to create state:", err)
    }
    
    for addr, account := range alloc {
        // Convert big.Int to uint256
        balance := uint256.MustFromBig(account.Balance)
        statedb.AddBalance(addr, balance, tracing.BalanceChangeUnspecified)
        statedb.SetCode(addr, account.Code)
        statedb.SetNonce(addr, account.Nonce, tracing.NonceChangeUnspecified)
        for key, value := range account.Storage {
            statedb.SetState(addr, key, value)
        }
    }
    
    root, err := statedb.Commit(0, false, false)
    if err != nil {
        log.Fatal("Failed to commit genesis state:", err)
    }
    
    // Commit to trie database
    if err := stateDb.TrieDB().Commit(root, false); err != nil {
        log.Fatal("Failed to commit trie:", err)
    }
    
    return root
}

func getLuxChainConfig(chainID int64) *params.ChainConfig {
    return &params.ChainConfig{
        ChainID:             big.NewInt(chainID),
        HomesteadBlock:      big.NewInt(0),
        DAOForkBlock:        nil,
        DAOForkSupport:      false,
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
        MergeNetsplitBlock:  nil,
        ShanghaiTime:        nil,
        CancunTime:          nil,
        PragueTime:          nil,
        VerkleTime:          nil,
        TerminalTotalDifficulty: nil,
    }
}

func writeVMMetadata(vmPath string, hash common.Hash, number uint64) {
    // Write lastAccepted
    lastAcceptedPath := filepath.Join(vmPath, "lastAccepted")
    if err := os.WriteFile(lastAcceptedPath, hash.Bytes(), 0644); err != nil {
        log.Fatal("Failed to write lastAccepted:", err)
    }
    
    // Write lastAcceptedHeight
    lastHeightPath := filepath.Join(vmPath, "lastAcceptedHeight")
    heightBytes := make([]byte, 8)
    binary.BigEndian.PutUint64(heightBytes, number)
    if err := os.WriteFile(lastHeightPath, heightBytes, 0644); err != nil {
        log.Fatal("Failed to write lastAcceptedHeight:", err)
    }
    
    // Write initialized flag
    initializedPath := filepath.Join(vmPath, "initialized")
    if err := os.WriteFile(initializedPath, []byte{0x01}, 0644); err != nil {
        log.Fatal("Failed to write initialized:", err)
    }
}

func runVerification(config MigrationConfig) {
    fmt.Println("=== Verifying Migration ===")
    
    // Open database
    ldb, err := leveldb.New(config.TargetDB, 0, 0, "verification", true)
    if err != nil {
        log.Fatal("Failed to open database:", err)
    }
    defer ldb.Close()
    
    db := rawdb.NewDatabase(ldb)
    
    // Check genesis
    genesisHash := rawdb.ReadCanonicalHash(db, 0)
    fmt.Printf("Genesis hash: %s\n", genesisHash.Hex())
    
    if genesisHash == (common.Hash{}) {
        fmt.Println("❌ Genesis not found")
        os.Exit(1)
    }
    
    // Check genesis header
    genesisHeader := rawdb.ReadHeader(db, genesisHash, 0)
    if genesisHeader == nil {
        fmt.Println("❌ Genesis header not found")
        os.Exit(1)
    }
    
    fmt.Printf("Genesis state root: %s\n", genesisHeader.Root.Hex())
    
    // Check heads
    headBlock := rawdb.ReadHeadBlockHash(db)
    headHeader := rawdb.ReadHeadHeaderHash(db)
    headFast := rawdb.ReadHeadFastBlockHash(db)
    
    fmt.Printf("Head block: %s\n", headBlock.Hex())
    fmt.Printf("Head header: %s\n", headHeader.Hex())
    fmt.Printf("Head fast: %s\n", headFast.Hex())
    
    // Check chain config
    chainConfig := rawdb.ReadChainConfig(db, genesisHash)
    if chainConfig == nil {
        fmt.Println("❌ Chain config not found")
        os.Exit(1)
    }
    
    fmt.Printf("Chain ID: %s\n", chainConfig.ChainID.String())
    
    // Check VM metadata
    checkVMMetadata(config.VMPath, genesisHash, 0)
    
    fmt.Println("✅ Verification passed")
}

func checkVMMetadata(vmPath string, expectedHash common.Hash, expectedHeight uint64) {
    // Check lastAccepted
    lastAcceptedPath := filepath.Join(vmPath, "lastAccepted")
    data, err := os.ReadFile(lastAcceptedPath)
    if err != nil || len(data) != 32 {
        fmt.Println("❌ VM lastAccepted missing or invalid")
        os.Exit(1)
    }
    
    hash := common.BytesToHash(data)
    if hash != expectedHash {
        fmt.Printf("❌ VM lastAccepted mismatch: got %s, want %s\n", hash.Hex(), expectedHash.Hex())
        os.Exit(1)
    }
    
    // Check lastAcceptedHeight
    lastHeightPath := filepath.Join(vmPath, "lastAcceptedHeight")
    data, err = os.ReadFile(lastHeightPath)
    if err != nil || len(data) != 8 {
        fmt.Println("❌ VM lastAcceptedHeight missing or invalid")
        os.Exit(1)
    }
    
    height := binary.BigEndian.Uint64(data)
    if height != expectedHeight {
        fmt.Printf("❌ VM lastAcceptedHeight mismatch: got %d, want %d\n", height, expectedHeight)
        os.Exit(1)
    }
    
    fmt.Println("✅ VM metadata correct")
}

func runRepair(config MigrationConfig) {
    fmt.Println("=== Repairing Database ===")
    
    // Open database
    ldb, err := leveldb.New(config.TargetDB, 0, 0, "repair", false)
    if err != nil {
        log.Fatal("Failed to open database:", err)
    }
    defer ldb.Close()
    
    db := rawdb.NewDatabase(ldb)
    
    // Find the highest valid block
    var tipHash common.Hash
    var tipNumber uint64
    
    // Check if we have the target block
    targetHash := common.HexToHash(TargetHash)
    targetHeader := rawdb.ReadHeader(db, targetHash, TargetBlock)
    
    if targetHeader != nil {
        tipHash = targetHash
        tipNumber = TargetBlock
        fmt.Printf("Found target block %d\n", tipNumber)
    } else {
        // Find highest contiguous block
        for n := uint64(1); n <= TargetBlock; n++ {
            hash := rawdb.ReadCanonicalHash(db, n)
            if hash == (common.Hash{}) {
                break
            }
            header := rawdb.ReadHeader(db, hash, n)
            if header == nil {
                break
            }
            tipHash = hash
            tipNumber = n
        }
        fmt.Printf("Highest contiguous block: %d\n", tipNumber)
    }
    
    if tipNumber == 0 {
        // Only genesis exists
        genesisHash := rawdb.ReadCanonicalHash(db, 0)
        if genesisHash != (common.Hash{}) {
            tipHash = genesisHash
            tipNumber = 0
        } else {
            log.Fatal("No valid blocks found")
        }
    }
    
    // Update heads
    fmt.Printf("Setting heads to block %d (%s)\n", tipNumber, tipHash.Hex())
    rawdb.WriteHeadBlockHash(db, tipHash)
    rawdb.WriteHeadHeaderHash(db, tipHash)
    rawdb.WriteHeadFastBlockHash(db, tipHash)
    
    // Update VM metadata
    writeVMMetadata(config.VMPath, tipHash, tipNumber)
    
    fmt.Println("✅ Repair complete")
}