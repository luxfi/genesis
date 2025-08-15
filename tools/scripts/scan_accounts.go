package main

import (
    "encoding/hex"
    "fmt"
    "math/big"
    "path/filepath"
    
    "github.com/cockroachdb/pebble"
    "github.com/luxfi/geth/rlp"
)

type Account struct {
    Nonce    uint64
    Balance  *big.Int
    Root     [32]byte
    CodeHash []byte
}

func main() {
    dbPath := "/Users/z/work/lux/state/chaindata/lux-mainnet-96369/db/pebbledb"
    
    db, err := pebble.Open(filepath.Clean(dbPath), &pebble.Options{ReadOnly: true})
    if err \!= nil {
        panic(err)
    }
    defer db.Close()
    
    fmt.Println("Scanning for accounts with balance...")
    
    // Scan accounts (prefix 0x00)
    it, _ := db.NewIter(&pebble.IterOptions{
        LowerBound: []byte{0x00},
        UpperBound: []byte{0x01},
    })
    defer it.Close()
    
    count := 0
    withBalance := 0
    largeBalances := 0
    
    minBalance := new(big.Int)
    minBalance.SetString("1000000000000000000", 10) // 1 LUX
    
    for it.First(); it.Valid(); it.Next() {
        key := it.Key()
        if len(key) \!= 21 { // 1 byte prefix + 20 byte address
            continue
        }
        
        count++
        addr := key[1:]
        
        var acc Account
        if err := rlp.DecodeBytes(it.Value(), &acc); err \!= nil {
            continue
        }
        
        if acc.Balance \!= nil && acc.Balance.Sign() > 0 {
            withBalance++
            
            if acc.Balance.Cmp(minBalance) > 0 {
                largeBalances++
                if largeBalances <= 10 {
                    balanceEth := new(big.Float).Quo(new(big.Float).SetInt(acc.Balance), big.NewFloat(1e18))
                    fmt.Printf("  0x%x: %.4f LUX\n", addr, balanceEth)
                }
            }
        }
        
        if count%10000 == 0 {
            fmt.Printf("Scanned %d accounts...\n", count)
        }
    }
    
    fmt.Printf("\nTotal accounts: %d\n", count)
    fmt.Printf("Accounts with balance: %d\n", withBalance)
    fmt.Printf("Accounts with > 1 LUX: %d\n", largeBalances)
}
EOF < /dev/null