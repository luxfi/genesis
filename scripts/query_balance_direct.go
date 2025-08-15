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
	targetBlock := uint64(1000000) // Block 1 million
	
	// Target address - luxdefi.eth
	targetAddr := common.HexToAddress("0x9011E888251AB053B7bD1cdB598Db4f9DEd94714")
	
	fmt.Println("Query Balance at Block 1,000,000")
	fmt.Println("=================================")
	fmt.Printf("Database: %s\n", sourcePath)
	fmt.Printf("Block: %d\n", targetBlock)
	fmt.Printf("Address: %s (luxdefi.eth)\n", targetAddr.Hex())
	fmt.Println()
	
	// Open source PebbleDB
	db, err := pebble.Open(sourcePath, &pebble.Options{
		ReadOnly: true,
	})
	if err != nil {
		log.Fatal("Failed to open database:", err)
	}
	defer db.Close()
	
	// First, find the block header at height 1,000,000
	fmt.Printf("Searching for block %d...\n", targetBlock)
	
	var blockHash []byte
	var stateRoot []byte
	
	// Iterate through database to find the block
	iter, err := db.NewIter(nil)
	if err != nil {
		log.Fatal("Failed to create iterator:", err)
	}
	defer iter.Close()
	
	blocksFound := 0
	for iter.First(); iter.Valid(); iter.Next() {
		key := iter.Key()
		val := iter.Value()
		
		// Check if this is a namespaced key
		if len(key) == 64 && bytes.HasPrefix(key, subnetNamespace) {
			actualKey := key[32:]
			
			// Check if this looks like a block header
			if len(val) > 500 && (val[0] == 0xf8 || val[0] == 0xf9) {
				// Try to decode as RLP to see if it's a header
				var headerFields []interface{}
				if err := rlp.DecodeBytes(val, &headerFields); err == nil && len(headerFields) >= 15 {
					// Field 8 is the block number
					if numberBytes, ok := headerFields[8].([]byte); ok {
						blockNum := new(big.Int).SetBytes(numberBytes).Uint64()
						
						if blockNum == targetBlock {
							blockHash = actualKey
							// Field 3 is the state root
							if sr, ok := headerFields[3].([]byte); ok {
								stateRoot = sr
								fmt.Printf("Found block %d!\n", targetBlock)
								fmt.Printf("  Block hash: 0x%x\n", blockHash)
								fmt.Printf("  State root: 0x%x\n", stateRoot)
								break
							}
						}
						
						if blockNum > 999900 && blockNum < 1000100 {
							blocksFound++
							if blocksFound < 10 {
								fmt.Printf("  Found nearby block %d\n", blockNum)
							}
						}
					}
				}
			}
		}
	}
	
	if len(stateRoot) == 0 {
		fmt.Printf("\n⚠️  Could not find block %d in database\n", targetBlock)
		fmt.Printf("Found %d blocks near target height\n", blocksFound)
		
		// Try to find the highest block
		fmt.Println("\nSearching for highest block...")
		highestBlock := uint64(0)
		
		iter2, _ := db.NewIter(nil)
		defer iter2.Close()
		
		for iter2.First(); iter2.Valid(); iter2.Next() {
			key := iter2.Key()
			val := iter2.Value()
			
			if len(key) == 64 && bytes.HasPrefix(key, subnetNamespace) {
				if len(val) > 500 && (val[0] == 0xf8 || val[0] == 0xf9) {
					var headerFields []interface{}
					if err := rlp.DecodeBytes(val, &headerFields); err == nil && len(headerFields) >= 15 {
						if numberBytes, ok := headerFields[8].([]byte); ok {
							blockNum := new(big.Int).SetBytes(numberBytes).Uint64()
							if blockNum > highestBlock {
								highestBlock = blockNum
							}
						}
					}
				}
			}
		}
		
		fmt.Printf("Highest block found: %d\n", highestBlock)
		
		if highestBlock < targetBlock {
			fmt.Printf("\n✗ Block %d is beyond the migrated data (highest: %d)\n", targetBlock, highestBlock)
		}
		return
	}
	
	// Now look for the account state
	fmt.Printf("\nSearching for account state...\n")
	
	// Calculate the account key in the state trie
	accountHash := crypto.Keccak256(targetAddr.Bytes())
	fmt.Printf("Account hash: 0x%x\n", accountHash)
	
	// Look for state entries
	stateFound := false
	iter3, _ := db.NewIter(nil)
	defer iter3.Close()
	
	for iter3.First(); iter3.Valid(); iter3.Next() {
		key := iter3.Key()
		val := iter3.Value()
		
		// Check for account data
		if bytes.Contains(key, accountHash) || bytes.Contains(val, targetAddr.Bytes()) {
			fmt.Printf("Found potential account data:\n")
			fmt.Printf("  Key: 0x%x\n", key[:32])
			fmt.Printf("  Value length: %d\n", len(val))
			
			// Try to decode as account
			var acc Account
			if err := rlp.DecodeBytes(val, &acc); err == nil && acc.Balance != nil {
				fmt.Printf("\n✓ Found account balance!\n")
				fmt.Printf("  Balance: %s wei\n", acc.Balance.String())
				fmt.Printf("  Balance: %s LUX\n", formatBalance(acc.Balance))
				fmt.Printf("  Nonce: %d\n", acc.Nonce)
				stateFound = true
				break
			}
		}
	}
	
	if !stateFound {
		fmt.Printf("\n⚠️  Could not find account state for luxdefi.eth at block %d\n", targetBlock)
		fmt.Println("The account may not have existed at this block height")
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