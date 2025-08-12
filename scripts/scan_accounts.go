package main

import (
	"fmt"
	"log"
	"math/big"
	"sort"
	
	"github.com/dgraph-io/badger/v4"
)

type Account struct {
	Key     []byte
	Balance *big.Int
}

func main() {
	// Open BadgerDB
	opts := badger.DefaultOptions("migrated/badgerdb")
	opts.Logger = nil
	db, err := badger.Open(opts)
	if err \!= nil {
		log.Fatal("Failed to open database:", err)
	}
	defer db.Close()
	
	fmt.Println("Scanning for accounts with balances...")
	
	accounts := []Account{}
	
	err = db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchSize = 100
		it := txn.NewIterator(opts)
		defer it.Close()
		
		// Look for account state keys (they typically start with specific prefixes)
		prefixes := [][]byte{
			[]byte("evms"),     // EVM state
			[]byte("evma"),     // EVM accounts
			[]byte("s"),        // State
			[]byte("accounts"), // Accounts
		}
		
		for _, prefix := range prefixes {
			it.Seek(prefix)
			count := 0
			
			for it.ValidForPrefix(prefix) && count < 100 {
				item := it.Item()
				key := item.Key()
				
				val, err := item.ValueCopy(nil)
				if err \!= nil {
					it.Next()
					continue
				}
				
				// Try to interpret as balance (first 32 bytes)
				if len(val) >= 32 {
					balance := new(big.Int).SetBytes(val[:32])
					
					// Only show non-zero balances
					if balance.Sign() > 0 {
						accounts = append(accounts, Account{
							Key:     key,
							Balance: balance,
						})
						count++
					}
				}
				
				it.Next()
			}
		}
		
		return nil
	})
	
	if err \!= nil {
		log.Fatal("Error scanning database:", err)
	}
	
	// Sort by balance
	sort.Slice(accounts, func(i, j int) bool {
		return accounts[i].Balance.Cmp(accounts[j].Balance) > 0
	})
	
	// Display top accounts
	fmt.Printf("\nFound %d accounts with non-zero balances:\n", len(accounts))
	fmt.Println("Top 10 accounts by balance:")
	
	for i, acc := range accounts {
		if i >= 10 {
			break
		}
		
		// Convert to LUX (assuming 18 decimals)
		luxBalance := new(big.Float).SetInt(acc.Balance)
		divisor := new(big.Float).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil))
		luxBalance.Quo(luxBalance, divisor)
		
		fmt.Printf("%d. Key: %x\n", i+1, acc.Key)
		fmt.Printf("   Balance: %s wei\n", acc.Balance.String())
		fmt.Printf("   Balance: %s LUX\n", luxBalance.String())
	}
	
	// Calculate total balance
	total := new(big.Int)
	for _, acc := range accounts {
		total.Add(total, acc.Balance)
	}
	
	totalLux := new(big.Float).SetInt(total)
	divisor := new(big.Float).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil))
	totalLux.Quo(totalLux, divisor)
	
	fmt.Printf("\nTotal balance across all accounts: %s LUX\n", totalLux.String())
}
