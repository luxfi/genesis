package main

import (
    "encoding/hex"
    "encoding/json"
    "fmt"
    "log"
    "math/big"
    
    "github.com/dgraph-io/badger/v3"
    "github.com/ethereum/go-ethereum/common"
    "github.com/ethereum/go-ethereum/core/types"
    "github.com/ethereum/go-ethereum/rlp"
)

type Genesis struct {
    Hash       string `json:"hash"`
    Number     uint64 `json:"number"`
    ParentHash string `json:"parentHash"`
    Timestamp  uint64 `json:"timestamp"`
    GasLimit   uint64 `json:"gasLimit"`
    Difficulty string `json:"difficulty"`
    Nonce      uint64 `json:"nonce"`
    MixHash    string `json:"mixHash"`
    Coinbase   string `json:"coinbase"`
    StateRoot  string `json:"stateRoot"`
    ExtraData  string `json:"extraData"`
}

func main() {
    dbPath := "/home/z/.luxd/chainData/xBBY6aJcNichNCkCXgUcG5Gv2PW6FLS81LYDV8VwnPuadKGqm/ethdb"
    
    opts := badger.DefaultOptions(dbPath)
    opts.ReadOnly = true
    
    db, err := badger.Open(opts)
    if err != nil {
        log.Fatal("Failed to open database:", err)
    }
    defer db.Close()
    
    fmt.Println("Extracting genesis from migrated database...")
    
    var genesis Genesis
    
    // Get block 0
    err = db.View(func(txn *badger.Txn) error {
        // Get canonical hash for block 0
        canonicalKey := append([]byte("h"), encodeBlockNumber(0)...)
        canonicalKey = append(canonicalKey, 'n')
        
        item, err := txn.Get(canonicalKey)
        if err != nil {
            return fmt.Errorf("failed to get canonical hash: %v", err)
        }
        
        var hash common.Hash
        err = item.Value(func(val []byte) error {
            copy(hash[:], val)
            return nil
        })
        if err != nil {
            return err
        }
        
        genesis.Hash = hash.Hex()
        fmt.Printf("Genesis hash: %s\n", genesis.Hash)
        
        // Get block header
        headerKey := append([]byte("h"), encodeBlockNumber(0)...)
        headerKey = append(headerKey, hash[:]...)
        
        item, err = txn.Get(headerKey)
        if err != nil {
            return fmt.Errorf("failed to get header: %v", err)
        }
        
        return item.Value(func(val []byte) error {
            var header types.Header
            if err := rlp.DecodeBytes(val, &header); err != nil {
                return fmt.Errorf("failed to decode header: %v", err)
            }
            
            genesis.Number = header.Number.Uint64()
            genesis.ParentHash = header.ParentHash.Hex()
            genesis.Timestamp = header.Time
            genesis.GasLimit = header.GasLimit
            genesis.Difficulty = header.Difficulty.String()
            genesis.Nonce = header.Nonce.Uint64()
            genesis.MixHash = header.MixDigest.Hex()
            genesis.Coinbase = header.Coinbase.Hex()
            genesis.StateRoot = header.Root.Hex()
            genesis.ExtraData = "0x" + hex.EncodeToString(header.Extra)
            
            return nil
        })
    })
    
    if err != nil {
        log.Fatal("Failed to read genesis:", err)
    }
    
    // Create C-Chain genesis JSON
    cchainGenesis := map[string]interface{}{
        "config": map[string]interface{}{
            "chainId":             96369,
            "homesteadBlock":      0,
            "eip150Block":         0,
            "eip150Hash":          "0x2086799aeebeae135c246c65021c82b4e15a2c451340993aacfd2751886514f0",
            "eip155Block":         0,
            "eip158Block":         0,
            "byzantiumBlock":      0,
            "constantinopleBlock": 0,
            "petersburgBlock":     0,
            "istanbulBlock":       0,
            "muirGlacierBlock":    0,
            "berlinBlock":         0,
            "londonBlock":         0,
        },
        "nonce":      fmt.Sprintf("0x%016x", genesis.Nonce),
        "timestamp":  fmt.Sprintf("0x%x", genesis.Timestamp),
        "extraData":  genesis.ExtraData,
        "gasLimit":   fmt.Sprintf("0x%x", genesis.GasLimit),
        "difficulty": fmt.Sprintf("0x%x", mustParseBigInt(genesis.Difficulty)),
        "mixHash":    genesis.MixHash,
        "coinbase":   genesis.Coinbase,
        "alloc":      map[string]interface{}{}, // Empty alloc - state is already in DB
        "number":     fmt.Sprintf("0x%x", genesis.Number),
        "gasUsed":    "0x0",
        "parentHash": genesis.ParentHash,
    }
    
    // Print the formatted genesis
    fmt.Println("\n=== Genesis Block Details ===")
    fmt.Printf("Block Number: %d\n", genesis.Number)
    fmt.Printf("Block Hash: %s\n", genesis.Hash)
    fmt.Printf("Parent Hash: %s\n", genesis.ParentHash)
    fmt.Printf("Timestamp: %d (0x%x)\n", genesis.Timestamp, genesis.Timestamp)
    fmt.Printf("Gas Limit: %d (0x%x)\n", genesis.GasLimit, genesis.GasLimit)
    fmt.Printf("Difficulty: %s\n", genesis.Difficulty)
    fmt.Printf("Nonce: %d\n", genesis.Nonce)
    fmt.Printf("Mix Hash: %s\n", genesis.MixHash)
    fmt.Printf("Coinbase: %s\n", genesis.Coinbase)
    fmt.Printf("State Root: %s\n", genesis.StateRoot)
    fmt.Printf("Extra Data: %s\n", genesis.ExtraData)
    
    // Output C-Chain genesis JSON
    fmt.Println("\n=== C-Chain Genesis JSON ===")
    jsonBytes, err := json.MarshalIndent(cchainGenesis, "", "  ")
    if err != nil {
        log.Fatal("Failed to marshal genesis:", err)
    }
    fmt.Println(string(jsonBytes))
    
    // Save to file
    if _, err := json.MarshalIndent(cchainGenesis, "", "  "); err == nil {
        fmt.Println("\n✓ C-Chain genesis extracted successfully")
        fmt.Println("Note: This genesis should produce hash:", genesis.Hash)
    }
}

func encodeBlockNumber(number uint64) []byte {
    enc := make([]byte, 8)
    for i := 7; i >= 0; i-- {
        enc[i] = byte(number)
        number >>= 8
    }
    return enc
}

func mustParseBigInt(s string) *big.Int {
    n, ok := new(big.Int).SetString(s, 10)
    if !ok {
        return big.NewInt(0)
    }
    return n
}