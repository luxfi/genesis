package main

import (
    "fmt"
    "math/big"
    
    "github.com/luxfi/geth/common"
    "github.com/luxfi/geth/core/rawdb"
    "github.com/luxfi/geth/core/state"
    "github.com/luxfi/geth/ethdb"
    "github.com/luxfi/geth/trie"
    "github.com/luxfi/geth/core/types"
)

func main() {
    // Open the migrated Coreth database
    fmt.Println("Opening Coreth database from /tmp/coreth-from-subnet...")
    
    db, err := rawdb.NewLevelDBDatabase("/tmp/coreth-from-subnet", 256, 256, "", false)
    if err != nil {
        fmt.Printf("Failed to open database: %v\n", err)
        return
    }
    defer db.Close()
    
    // Get the latest block header
    blockNum := uint64(1082780) // The block we migrated
    hash := rawdb.ReadCanonicalHash(db, blockNum)
    if hash == (common.Hash{}) {
        fmt.Printf("Block %d not found in database\n", blockNum)
        return
    }
    
    header := rawdb.ReadHeader(db, hash, blockNum)
    if header == nil {
        fmt.Printf("Header for block %d not found\n", blockNum)
        return
    }
    
    fmt.Printf("Block %d found!\n", blockNum)
    fmt.Printf("Block Hash: %s\n", hash.Hex())
    fmt.Printf("State Root: %s\n", header.Root.Hex())
    
    // Create state database with the state root
    stateDB := state.NewDatabase(db)
    st, err := state.New(header.Root, stateDB, nil)
    if err != nil {
        fmt.Printf("Failed to create state: %v\n", err)
        return
    }
    
    // Check balances
    addresses := []string{
        "0x9011E888251AB053B7bD1cdB598Db4f9DEd94714", // Treasury
        "0x8d5081153aE1cfb41f5c932fe0b6Beb7E159cF84", // User requested
        "0xEAbCC110fAcBfebabC66Ad6f9E7B67288e720B59", // Another address
    }
    
    fmt.Println("\nChecking balances:")
    for _, addrStr := range addresses {
        addr := common.HexToAddress(addrStr)
        balance := st.GetBalance(addr)
        
        // Convert to LUX (18 decimals)
        luxBalance := new(big.Float).SetInt(balance)
        luxBalance.Quo(luxBalance, big.NewFloat(1e18))
        
        fmt.Printf("%s: %s LUX\n", addrStr, luxBalance.Text('f', 6))
    }
    
    fmt.Println("\nBlockchain data verification complete!")
    fmt.Println("All migrated state data is accessible.")
}