package main

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"log"
	"math/big"
	
	"github.com/dgraph-io/badger/v4"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
)

// Account structure from geth
type Account struct {
	Nonce    uint64
	Balance  *big.Int
	Root     common.Hash // merkle root of the storage trie
	CodeHash []byte
}

func main() {
	// Open the ethdb database
	ethdbPath := "/Users/z/.luxd/network-96369/chains/EWi9aPkGe6EfJ3SobCAmSUXRPLa4brF3cThwPwmHTrD1y13jy/ethdb"
	
	db, err := badger.Open(badger.DefaultOptions(ethdbPath))
	if err != nil {
		log.Fatal("Failed to open ethdb:", err)
	}
	defer db.Close()
	
	fmt.Println("Querying balances at block 1,082,780")
	fmt.Println("=====================================")
	
	// Target addresses
	addresses := []struct{
		name string
		addr common.Address
	}{
		{"luxdefi.eth", common.HexToAddress("0x9011E888251AB053B7bD1cdB598Db4f9DEd94714")},
		{"Address 2", common.HexToAddress("0xEAbCC110fAcBfebabC66Ad6f9E7B67288e720B59")},
	}
	
	// Get the state root at block 1,082,780
	targetBlock := uint64(1082780)
	
	// First get the block hash
	var blockHash []byte
	err = db.View(func(txn *badger.Txn) error {
		// Get canonical hash at this height
		canonicalKey := make([]byte, 9)
		canonicalKey[0] = 'H'
		binary.BigEndian.PutUint64(canonicalKey[1:], targetBlock)
		
		item, err := txn.Get(canonicalKey)
		if err != nil {
			return fmt.Errorf("canonical hash not found for block %d", targetBlock)
		}
		
		blockHash, err = item.ValueCopy(nil)
		if err != nil {
			return err
		}
		
		fmt.Printf("Block %d hash: %s\n", targetBlock, hex.EncodeToString(blockHash))
		
		// Now get the header for this block
		headerKey := make([]byte, 41)
		headerKey[0] = 'h'
		binary.BigEndian.PutUint64(headerKey[1:9], targetBlock)
		copy(headerKey[9:41], blockHash)
		
		item, err = txn.Get(headerKey)
		if err != nil {
			return fmt.Errorf("header not found for block %d", targetBlock)
		}
		
		headerData, err := item.ValueCopy(nil)
		if err != nil {
			return err
		}
		
		// Parse the header to get state root
		// The header is RLP encoded, state root is at position 3 in the header
		// For now, just try to find accounts directly
		fmt.Printf("Header data length: %d bytes\n", len(headerData))
		
		return nil
	})
	
	if err != nil {
		log.Fatal("Error getting block data:", err)
	}
	
	// Try to find account data
	// Account keys are typically: sha3(address) for path-based or 
	// 0x26 (account leaf prefix) + hash for newer formats
	fmt.Println("\nSearching for account data...")
	
	err = db.View(func(txn *badger.Txn) error {
		// Look for account data with different possible key formats
		for _, target := range addresses {
			fmt.Printf("\nSearching for %s (%s):\n", target.name, target.addr.Hex())
			
			// Try different key formats
			// Format 1: Account leaf with prefix 0x26
			addrHash := crypto.Keccak256Hash(target.addr.Bytes())
			accountKey1 := append([]byte{0x26}, addrHash.Bytes()...)
			if item, err := txn.Get(accountKey1); err == nil {
				val, _ := item.ValueCopy(nil)
				fmt.Printf("  Found with key format 1 (0x26 prefix): %d bytes\n", len(val))
				
				// Try to decode as RLP
				var acc Account
				if err := rlp.DecodeBytes(val, &acc); err == nil {
					fmt.Printf("  Balance: %s wei\n", acc.Balance.String())
					fmt.Printf("  Balance: %.6f LUX\n", new(big.Float).Quo(new(big.Float).SetInt(acc.Balance), big.NewFloat(1e18)))
					fmt.Printf("  Nonce: %d\n", acc.Nonce)
				}
			}
			
			// Format 2: Direct address hash
			accountKey2 := addrHash.Bytes()
			if item, err := txn.Get(accountKey2); err == nil {
				val, _ := item.ValueCopy(nil)
				fmt.Printf("  Found with key format 2 (direct hash): %d bytes\n", len(val))
			}
			
			// Format 3: Look for any key containing the address
			addrBytes := target.addr.Bytes()
			opts := badger.DefaultIteratorOptions
			opts.PrefetchSize = 10
			it := txn.NewIterator(opts)
			defer it.Close()
			
			found := false
			count := 0
			for it.Rewind(); it.Valid() && count < 100000; it.Next() {
				key := it.Item().Key()
				count++
				
				// Check if key contains address bytes
				if containsBytes(key, addrBytes) {
					if !found {
						fmt.Printf("  Found keys containing address:\n")
						found = true
					}
					fmt.Printf("    Key: %s\n", hex.EncodeToString(key))
					if len(key) < 100 { // Only show value for small keys
						val, _ := it.Item().ValueCopy(nil)
						if len(val) < 200 {
							fmt.Printf("    Value: %s\n", hex.EncodeToString(val))
						}
					}
				}
			}
			
			if !found && count >= 100000 {
				fmt.Printf("  (Stopped after checking %d keys)\n", count)
			}
		}
		
		return nil
	})
	
	if err != nil {
		log.Fatal("Error searching for accounts:", err)
	}
	
	fmt.Println("\n=====================================")
	fmt.Println("Balance query complete")
}

func containsBytes(haystack, needle []byte) bool {
	if len(needle) == 0 {
		return true
	}
	if len(haystack) < len(needle) {
		return false
	}
	for i := 0; i <= len(haystack)-len(needle); i++ {
		match := true
		for j := 0; j < len(needle); j++ {
			if haystack[i+j] != needle[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}