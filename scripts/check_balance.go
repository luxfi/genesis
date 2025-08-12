package main

import (
	"fmt"
	"log"
	"math/big"
	
	"github.com/dgraph-io/badger/v4"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

func main() {
	// Treasury address
	address := common.HexToAddress("0x9011E888251AB053B7bD1cdB598Db4f9DEd94714")
	
	// Open BadgerDB
	opts := badger.DefaultOptions("migrated/badgerdb")
	opts.Logger = nil
	db, err := badger.Open(opts)
	if err != nil {
		log.Fatal("Failed to open database:", err)
	}
	defer db.Close()
	
	// Compute storage key for balance
	// In EVM, account balance is stored with a specific key format
	// Key format: evm + hash(address) for account state
	
	addressHash := crypto.Keccak256(address.Bytes())
	
	// Try different key formats
	keyFormats := [][]byte{
		append([]byte("evm"), addressHash...),
		append([]byte("s"), addressHash...),
		append([]byte("accounts"), addressHash...),
		addressHash,
	}
	
	fmt.Printf("Checking balance for treasury address: %s\n", address.Hex())
	fmt.Printf("Address hash: %x\n", addressHash)
	
	for i, key := range keyFormats {
		err = db.View(func(txn *badger.Txn) error {
			item, err := txn.Get(key)
			if err != nil {
				if err == badger.ErrKeyNotFound {
					fmt.Printf("Format %d: Key not found (key=%x)\n", i, key)
					return nil
				}
				return err
			}
			
			val, err := item.ValueCopy(nil)
			if err != nil {
				return err
			}
			
			fmt.Printf("Format %d: Found value (key=%x, value=%x)\n", i, key, val)
			
			// Try to interpret as balance
			if len(val) >= 32 {
				balance := new(big.Int).SetBytes(val[:32])
				fmt.Printf("  Possible balance: %s wei\n", balance.String())
				
				// Convert to LUX (assuming 18 decimals)
				luxBalance := new(big.Float).SetInt(balance)
				divisor := new(big.Float).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil))
				luxBalance.Quo(luxBalance, divisor)
				fmt.Printf("  Possible balance: %s LUX\n", luxBalance.String())
			}
			
			return nil
		})
		
		if err != nil && err != badger.ErrKeyNotFound {
			fmt.Printf("Error reading key: %v\n", err)
		}
	}
	
	// Also try to scan for keys containing the address
	fmt.Println("\nScanning for keys containing address bytes...")
	count := 0
	err = db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchSize = 10
		it := txn.NewIterator(opts)
		defer it.Close()
		
		addressBytes := address.Bytes()
		
		for it.Rewind(); it.Valid() && count < 1000000; it.Next() {
			item := it.Item()
			key := item.Key()
			
			// Check if key contains address
			if contains(key, addressBytes) {
				val, _ := item.ValueCopy(nil)
				fmt.Printf("Found key containing address: key=%x, value=%x\n", key, val)
				count++
				if count >= 10 {
					fmt.Println("(stopping after 10 matches)")
					break
				}
			}
		}
		return nil
	})
	
	if err != nil {
		log.Fatal("Error scanning database:", err)
	}
	
	if count == 0 {
		fmt.Println("No keys found containing the address bytes")
	}
}

func contains(haystack, needle []byte) bool {
	if len(needle) == 0 {
		return true
	}
	if len(haystack) < len(needle) {
		return false
	}
	for i := 0; i <= len(haystack)-len(needle); i++ {
		if equal(haystack[i:i+len(needle)], needle) {
			return true
		}
	}
	return false
}

func equal(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}