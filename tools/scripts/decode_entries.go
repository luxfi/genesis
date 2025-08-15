package main

import (
    "bytes"
    "fmt"
    "log"
    "math/big"
    
    "github.com/cockroachdb/pebble"
    "github.com/ethereum/go-ethereum/common"
    "github.com/ethereum/go-ethereum/rlp"
)

// SubnetEVM namespace prefix (32 bytes)
var subnetNamespace = []byte{
    0x33, 0x7f, 0xb7, 0x3f, 0x9b, 0xcd, 0xac, 0x8c,
    0x31, 0xa2, 0xd5, 0xf7, 0xb8, 0x77, 0xab, 0x1e,
    0x8a, 0x2b, 0x7f, 0x2a, 0x1e, 0x9b, 0xf0, 0x2a,
    0x0a, 0x0e, 0x6c, 0x6f, 0xd1, 0x64, 0xf1, 0xd1,
}

// Account represents an Ethereum account
type Account struct {
    Nonce    uint64
    Balance  *big.Int
    Root     common.Hash // merkle root of the storage trie
    CodeHash []byte
}

func main() {
    sourcePath := "/Users/z/work/lux/state/chaindata/lux-mainnet-96369/db/pebbledb"
    
    fmt.Println("Decoding Database Entries")
    fmt.Println("=========================")
    
    // Open source PebbleDB
    db, err := pebble.Open(sourcePath, &pebble.Options{
        ReadOnly: true,
    })
    if err != nil {
        log.Fatal("Failed to open database:", err)
    }
    defer db.Close()
    
    // Target address - luxdefi.eth
    targetAddr := common.HexToAddress("0x9011E888251AB053B7bD1cdB598Db4f9DEd94714")
    fmt.Printf("Looking for address: %s (luxdefi.eth)\n\n", targetAddr.Hex())
    
    iter, err := db.NewIter(nil)
    if err != nil {
        log.Fatal("Failed to create iterator:", err)
    }
    defer iter.Close()
    
    // Statistics
    totalEntries := 0
    accountEntries := 0
    possibleAccounts := 0
    foundTarget := false
    
    for iter.First(); iter.Valid() && totalEntries < 1000000; iter.Next() {
        key := iter.Key()
        val := iter.Value()
        totalEntries++
        
        // Check if value might be an account (typical length 70-120 bytes)
        if len(val) >= 70 && len(val) <= 150 {
            // Try to decode as account
            var acc Account
            if err := rlp.DecodeBytes(val, &acc); err == nil && acc.Balance != nil {
                accountEntries++
                
                // Check if this could be our target
                if acc.Balance.Cmp(big.NewInt(0)) > 0 {
                    possibleAccounts++
                    
                    // Show first few accounts with balance
                    if possibleAccounts <= 5 {
                        fmt.Printf("Found account with balance:\n")
                        fmt.Printf("  Key: %x\n", key[32:min(len(key), 64)])
                        fmt.Printf("  Balance: %s wei\n", acc.Balance.String())
                        fmt.Printf("  Balance: %s LUX\n", formatBalance(acc.Balance))
                        fmt.Printf("  Nonce: %d\n\n", acc.Nonce)
                    }
                    
                    // Check if balance matches expected range for luxdefi.eth
                    // Expected: ~1.9T LUX = 1.9e30 wei
                    expectedMin := new(big.Int).Exp(big.NewInt(10), big.NewInt(30), nil) // 1e30
                    expectedMax := new(big.Int).Mul(expectedMin, big.NewInt(3)) // 3e30
                    
                    if acc.Balance.Cmp(expectedMin) >= 0 && acc.Balance.Cmp(expectedMax) <= 0 {
                        fmt.Printf("⭐ POSSIBLE MATCH for luxdefi.eth:\n")
                        fmt.Printf("  Key: %x\n", key)
                        fmt.Printf("  Balance: %s wei\n", acc.Balance.String())
                        fmt.Printf("  Balance: %s LUX\n", formatBalance(acc.Balance))
                        fmt.Printf("  Nonce: %d\n\n", acc.Nonce)
                        foundTarget = true
                    }
                }
            }
        }
        
        // Also check if key contains the target address
        if bytes.Contains(key, targetAddr.Bytes()) || bytes.Contains(val, targetAddr.Bytes()) {
            fmt.Printf("Found entry containing target address:\n")
            fmt.Printf("  Key: %x\n", key[:min(len(key), 80)])
            fmt.Printf("  Value length: %d\n\n", len(val))
        }
        
        // Progress indicator
        if totalEntries%100000 == 0 {
            fmt.Printf("Scanned %d entries...\n", totalEntries)
        }
    }
    
    fmt.Printf("\nSummary:\n")
    fmt.Printf("Total entries scanned: %d\n", totalEntries)
    fmt.Printf("Account entries found: %d\n", accountEntries)
    fmt.Printf("Accounts with balance: %d\n", possibleAccounts)
    
    if foundTarget {
        fmt.Println("\n✓ Found possible match for luxdefi.eth!")
    } else {
        fmt.Println("\n✗ Did not find luxdefi.eth in this database")
        fmt.Println("Note: This appears to be a state database, not a block database")
        fmt.Println("Block 1,000,000 may not exist in this dataset")
    }
}

func formatBalance(balance *big.Int) string {
    if balance == nil {
        return "0"
    }
    
    // Convert from wei to LUX (18 decimals)
    ether := new(big.Float).SetInt(balance)
    divisor := new(big.Float).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil))
    ether.Quo(ether, divisor)
    
    return fmt.Sprintf("%s", ether.Text('f', 6))
}

func min(a, b int) int {
    if a < b {
        return a
    }
    return b
}