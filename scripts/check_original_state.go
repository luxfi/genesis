package main

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"log"
	"math/big"
	
	"github.com/cockroachdb/pebble"
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
	// Open the original PebbleDB database
	dbPath := "/Users/z/work/lux/state/chaindata/lux-mainnet-96369/db/pebbledb"
	
	db, err := pebble.Open(dbPath, &pebble.Options{
		ReadOnly: true,
	})
	if err != nil {
		log.Fatal("Failed to open pebbledb:", err)
	}
	defer db.Close()
	
	fmt.Println("Checking original PebbleDB for state data...")
	fmt.Println("=============================================")
	
	// Target addresses
	addresses := []struct{
		name string
		addr common.Address
	}{
		{"luxdefi.eth", common.HexToAddress("0x9011E888251AB053B7bD1cdB598Db4f9DEd94714")},
		{"Address 2", common.HexToAddress("0xEAbCC110fAcBfebabC66Ad6f9E7B67288e720B59")},
	}
	
	// First, let's analyze key types in the database
	keyTypes := make(map[string]int)
	stateKeys := [][]byte{}
	
	iter, err := db.NewIter(nil)
	if err != nil {
		log.Fatal("Failed to create iterator:", err)
	}
	defer iter.Close()
	
	count := 0
	for iter.First(); iter.Valid() && count < 100000; iter.Next() {
		key := iter.Key()
		count++
		
		// Categorize by prefix
		if len(key) > 0 {
			prefix := key[0]
			switch prefix {
			case 'H':
				keyTypes["H-canonical"]++
			case 'h':
				keyTypes["h-header"]++
			case 'b':
				keyTypes["b-body"]++
			case 'r':
				keyTypes["r-receipt"]++
			case 't':
				keyTypes["t-transaction"]++
			case 'l':
				keyTypes["l-lookup"]++
			case 'n': // State node
				keyTypes["n-state-node"]++
				if len(stateKeys) < 10 {
					stateKeys = append(stateKeys, bytes.Clone(key))
				}
			case 's': // Snapshot
				keyTypes["s-snapshot"]++
			default:
				if len(key) == 32 {
					keyTypes["32-byte-hash"]++
					// Could be state trie node
					if len(stateKeys) < 10 {
						stateKeys = append(stateKeys, bytes.Clone(key))
					}
				} else {
					keyTypes[fmt.Sprintf("0x%02x-other", prefix)]++
				}
			}
		}
		
		if count%10000 == 0 {
			fmt.Printf("Processed %d keys...\n", count)
		}
	}
	
	fmt.Println("\nKey Type Summary:")
	for keyType, cnt := range keyTypes {
		fmt.Printf("  %s: %d\n", keyType, cnt)
	}
	
	// Check for state at block 1082780
	blockNum := uint64(1082780)
	fmt.Printf("\nLooking for state at block %d...\n", blockNum)
	
	// Get block hash first
	canonicalKey := make([]byte, 9)
	canonicalKey[0] = 'H'
	binary.BigEndian.PutUint64(canonicalKey[1:], blockNum)
	
	blockHash, closer, err := db.Get(canonicalKey)
	if err != nil {
		fmt.Printf("Could not find canonical hash for block %d: %v\n", blockNum, err)
	} else {
		fmt.Printf("Block %d hash: %s\n", blockNum, hex.EncodeToString(blockHash))
		closer.Close()
		
		// Try to get header to find state root
		headerKey := make([]byte, 41)
		headerKey[0] = 'h'
		binary.BigEndian.PutUint64(headerKey[1:9], blockNum)
		copy(headerKey[9:41], blockHash)
		
		headerData, closer2, err := db.Get(headerKey)
		if err != nil {
			fmt.Printf("Could not find header for block %d: %v\n", blockNum, err)
		} else {
			fmt.Printf("Found header for block %d: %d bytes\n", blockNum, len(headerData))
			closer2.Close()
			
			// The state root is at a specific position in the RLP-encoded header
			// For now, let's just note we have the header
		}
	}
	
	// Try to find account data for our addresses
	fmt.Println("\nSearching for account data...")
	
	for _, target := range addresses {
		fmt.Printf("\nSearching for %s (%s):\n", target.name, target.addr.Hex())
		
		// Calculate the account key (keccak256 of address)
		addrHash := crypto.Keccak256Hash(target.addr.Bytes())
		
		// Try different possible keys
		// 1. Direct hash (state trie node)
		if val, closer, err := db.Get(addrHash.Bytes()); err == nil {
			fmt.Printf("  Found with direct hash key: %d bytes\n", len(val))
			closer.Close()
		}
		
		// 2. With 'n' prefix (state node)
		stateKey := append([]byte{'n'}, addrHash.Bytes()...)
		if val, closer, err := db.Get(stateKey); err == nil {
			fmt.Printf("  Found with 'n' prefix: %d bytes\n", len(val))
			closer.Close()
		}
		
		// 3. Look for snapshot data ('s' prefix)
		snapKey := append([]byte{'s'}, target.addr.Bytes()...)
		if val, closer, err := db.Get(snapKey); err == nil {
			fmt.Printf("  Found snapshot data: %d bytes\n", len(val))
			
			// Try to decode as account
			var acc Account
			if err := rlp.DecodeBytes(val, &acc); err == nil {
				fmt.Printf("  Balance: %s wei\n", acc.Balance.String())
				luxBalance := new(big.Float).Quo(new(big.Float).SetInt(acc.Balance), big.NewFloat(1e18))
				fmt.Printf("  Balance: %.18f LUX\n", luxBalance)
				fmt.Printf("  Nonce: %d\n", acc.Nonce)
			}
			closer.Close()
		}
	}
	
	fmt.Println("\n=============================================")
	fmt.Println("Analysis complete")
}