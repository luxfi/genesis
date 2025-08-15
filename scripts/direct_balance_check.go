package main

import (
	"fmt"
	"math/big"
	"path/filepath"
	
	"github.com/luxfi/database/badgerdb"
	"github.com/luxfi/geth/common"
	"github.com/luxfi/geth/rlp"
	"golang.org/x/crypto/sha3"
)

type Account struct {
	Nonce    uint64
	Balance  *big.Int
	Root     common.Hash
	CodeHash []byte
}

func keccak256(data []byte) []byte {
	hasher := sha3.NewLegacyKeccak256()
	hasher.Write(data)
	return hasher.Sum(nil)
}

func main() {
	dbPath := "/Users/z/.luxd/network-96369/chains/EWi9aPkGe6EfJ3SobCAmSUXRPLa4brF3cThwPwmHTrD1y13jy/ethdb"
	db, _ := badgerdb.New(filepath.Clean(dbPath), nil, "", nil)
	defer db.Close()
	
	// Check specific addresses
	addresses := []string{
		"0x9011E888251AB053B7bD1cdB598Db4f9DEd94714", // Treasury
		"0xEAbCC110fAcBfebabC66Ad6f9E7B67288e720B59", // User requested
		"0x8d5081153aE1cfb41f5c932fe0b6Beb7E159cF84", // Another test
	}
	
	fmt.Println("Checking account balances from raw data:")
	fmt.Println("=========================================")
	
	for _, addrStr := range addresses {
		addr := common.HexToAddress(addrStr)
		addrHash := keccak256(addr.Bytes())
		
		// Try with 'a' prefix (account data)
		accountKey := append([]byte{'a'}, addrHash...)
		
		if accountData, err := db.Get(accountKey); err == nil {
			// Try to decode as account
			var acc Account
			if err := rlp.DecodeBytes(accountData, &acc); err == nil {
				balanceEth := new(big.Float).Quo(new(big.Float).SetInt(acc.Balance), big.NewFloat(1e18))
				fmt.Printf("\n%s:\n", addrStr)
				fmt.Printf("  Balance: %.18f LUX\n", balanceEth)
				fmt.Printf("  Nonce: %d\n", acc.Nonce)
			} else {
				// Try raw balance (20 bytes = address only means empty account)
				if len(accountData) == 20 {
					fmt.Printf("\n%s: Empty account (0 LUX)\n", addrStr)
				} else {
					fmt.Printf("\n%s: Data found but decode failed (%d bytes)\n", addrStr, len(accountData))
				}
			}
		} else {
			fmt.Printf("\n%s: No account data found\n", addrStr)
		}
	}
}
