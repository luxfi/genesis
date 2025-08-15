package main

import (
	"bytes"
	"fmt"
	"log"
	"math/big"
	
	"github.com/cockroachdb/pebble"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
)

// SubnetEVM namespace
var subnetNamespace = []byte{
	0x33, 0x7f, 0xb7, 0x3f, 0x9b, 0xcd, 0xac, 0x8c,
	0x31, 0xa2, 0xd5, 0xf7, 0xb8, 0x77, 0xab, 0x1e,
	0x8a, 0x2b, 0x7f, 0x2a, 0x1e, 0x9b, 0xf0, 0x2a,
	0x0a, 0x0e, 0x6c, 0x6f, 0xd1, 0x64, 0xf1, 0xd1,
}

// Account structure from geth
type Account struct {
	Nonce    uint64
	Balance  *big.Int
	Root     common.Hash // merkle root of the storage trie
	CodeHash []byte
}

func main() {
	// Open the original PebbleDB database
	dbPath := "/Users/z/work/lux/state/chaindata/lux-mainnet-96369/db/pebbledb"
	
	db, err := pebble.Open(dbPath, &pebble.Options{
		ReadOnly: true,
	})
	if err != nil {
		log.Fatal("Failed to open pebbledb:", err)
	}
	defer db.Close()
	
	fmt.Println("Reading SubnetEVM state data...")
	fmt.Println("================================")
	fmt.Printf("Subnet namespace: %x\n", subnetNamespace)
	
	// Target addresses
	addresses := []struct{
		name string
		addr common.Address
	}{
		{"luxdefi.eth", common.HexToAddress("0x9011E888251AB053B7bD1cdB598Db4f9DEd94714")},
		{"Address 2", common.HexToAddress("0xEAbCC110fAcBfebabC66Ad6f9E7B67288e720B59")},
	}
	
	// First find the last block height
	fmt.Println("\nFinding highest block...")
	
	highestBlock := uint64(0)
	var lastBlockHash []byte
	
	// Iterate through keys to find blocks
	iter, err := db.NewIter(&pebble.IterOptions{
		LowerBound: subnetNamespace,
		UpperBound: append(subnetNamespace, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff),
	})
	if err != nil {
		log.Fatal("Failed to create iterator:", err)
	}
	defer iter.Close()
	
	blockCount := 0
	for iter.First(); iter.Valid() && blockCount < 100; iter.Next() {
		key := iter.Key()
		val := iter.Value()
		
		if len(key) == 64 && bytes.HasPrefix(key, subnetNamespace) {
			// Extract the actual key after namespace
			actualKey := key[32:]
			
			// Check if this looks like a block (large RLP-encoded value)
			if len(val) > 500 && (val[0] == 0xf8 || val[0] == 0xf9) {
				// Try to extract block number from the hash
				// Block numbers are often encoded in the first bytes of the hash
				if len(actualKey) >= 32 {
					// Estimate block number from hash pattern
					blockNum := uint64(actualKey[0])<<16 | uint64(actualKey[1])<<8 | uint64(actualKey[2])
					if blockNum > highestBlock && blockNum < 2000000 { // Sanity check
						highestBlock = blockNum
						lastBlockHash = actualKey
					}
					
					if blockCount < 5 {
						fmt.Printf("Found block-like data: key=%x (block ~%d), value_len=%d\n", 
							actualKey[:8], blockNum, len(val))
					}
					blockCount++
				}
			}
		}
	}
	
	fmt.Printf("\nEstimated highest block: %d\n", highestBlock)
	if lastBlockHash != nil {
		fmt.Printf("Last block hash: %x\n", lastBlockHash[:8])
	}
	
	// Now look for account state
	fmt.Println("\nSearching for account state...")
	
	for _, target := range addresses {
		fmt.Printf("\nSearching for %s (%s):\n", target.name, target.addr.Hex())
		
		// Calculate the account key (keccak256 of address)
		addrHash := crypto.Keccak256Hash(target.addr.Bytes())
		
		// Try SubnetEVM key format: namespace + hash
		subnetKey := append(subnetNamespace, addrHash.Bytes()...)
		
		if val, closer, err := db.Get(subnetKey); err == nil {
			fmt.Printf("  Found with subnet namespace: %d bytes\n", len(val))
			fmt.Printf("  Raw value: %x\n", val[:min(64, len(val))])
			
			// Try to decode as RLP
			var acc Account
			if err := rlp.DecodeBytes(val, &acc); err == nil {
				fmt.Printf("  Successfully decoded account!\n")
				fmt.Printf("  Balance: %s wei\n", acc.Balance.String())
				
				// Convert to LUX (18 decimals)
				luxBalance := new(big.Float).Quo(new(big.Float).SetInt(acc.Balance), big.NewFloat(1e18))
				fmt.Printf("  Balance: %.18f LUX\n", luxBalance)
				fmt.Printf("  Nonce: %d\n", acc.Nonce)
			} else {
				fmt.Printf("  RLP decode error: %v\n", err)
			}
			closer.Close()
		} else {
			// Try without namespace
			if val, closer, err := db.Get(addrHash.Bytes()); err == nil {
				fmt.Printf("  Found without namespace: %d bytes\n", len(val))
				closer.Close()
			}
		}
		
		// Also try the address directly with namespace
		directKey := append(subnetNamespace, target.addr.Bytes()...)
		if val, closer, err := db.Get(directKey); err == nil {
			fmt.Printf("  Found with direct address key: %d bytes\n", len(val))
			closer.Close()
		}
	}
	
	// Let's scan for any account-like data
	fmt.Println("\nScanning for account data patterns...")
	
	iter2, err := db.NewIter(&pebble.IterOptions{
		LowerBound: subnetNamespace,
		UpperBound: append(subnetNamespace, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff),
	})
	if err != nil {
		log.Fatal("Failed to create iterator:", err)
	}
	defer iter2.Close()
	
	accountsFound := 0
	for iter2.First(); iter2.Valid() && accountsFound < 10; iter2.Next() {
		val := iter2.Value()
		
		// Account data is typically RLP encoded and starts with specific patterns
		if len(val) >= 4 && len(val) <= 200 {
			// Try to decode as account
			var acc Account
			if err := rlp.DecodeBytes(val, &acc); err == nil && acc.Balance != nil {
				key := iter2.Key()
				if len(key) == 64 {
					actualKey := key[32:]
					fmt.Printf("\nFound account: key=%x\n", actualKey[:8])
					fmt.Printf("  Balance: %s wei\n", acc.Balance.String())
					
					// Check if this matches our target addresses
					for _, target := range addresses {
						addrHash := crypto.Keccak256Hash(target.addr.Bytes())
						if bytes.Equal(actualKey, addrHash.Bytes()) {
							fmt.Printf("  *** This is %s! ***\n", target.name)
							luxBalance := new(big.Float).Quo(new(big.Float).SetInt(acc.Balance), big.NewFloat(1e18))
							fmt.Printf("  Balance: %.18f LUX\n", luxBalance)
						}
					}
					
					accountsFound++
				}
			}
		}
	}
	
	fmt.Println("\n================================")
	fmt.Println("Scan complete")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}