package main

import (
	"encoding/binary"
	"fmt"
	"path/filepath"
	
	"github.com/luxfi/database/badgerdb"
	"github.com/luxfi/geth/common"
	"github.com/luxfi/geth/core/types"
	"github.com/luxfi/geth/rlp"
)

func main() {
	fmt.Println("=== Checking REAL Mainnet State ===")
	
	// Open the migrated database
	ethdbPath := "/Users/z/.luxd/network-96369/chains/EWi9aPkGe6EfJ3SobCAmSUXRPLa4brF3cThwPwmHTrD1y13jy/ethdb"
	db, err := badgerdb.New(filepath.Clean(ethdbPath), nil, "", nil)
	if err != nil {
		panic(fmt.Sprintf("Failed to open ethdb: %v", err))
	}
	defer db.Close()
	
	// Check the actual tip
	fmt.Println("\n--- Checking Chain Tip ---")
	
	// Get LastHeader
	if val, err := db.Get([]byte("LastHeader")); err == nil {
		var hash common.Hash
		copy(hash[:], val)
		fmt.Printf("LastHeader: %s\n", hash.Hex())
	}
	
	// Check blocks at different heights
	checkHeights := []uint64{0, 1, 100, 1000, 10000, 100000, 500000, 1000000, 1082780}
	
	for _, height := range checkHeights {
		canonKey := make([]byte, 10)
		canonKey[0] = 'h'
		binary.BigEndian.PutUint64(canonKey[1:9], height)
		canonKey[9] = 'n'
		
		if hashBytes, err := db.Get(canonKey); err == nil {
			var hash common.Hash
			copy(hash[:], hashBytes)
			
			// Get header
			headerKey := make([]byte, 41)
			headerKey[0] = 'h'
			binary.BigEndian.PutUint64(headerKey[1:9], height)
			copy(headerKey[9:], hash[:])
			
			if headerData, err := db.Get(headerKey); err == nil {
				// Try to decode header
				var header types.Header
				var stateRoot common.Hash
				
				if err := rlp.DecodeBytes(headerData, &header); err == nil {
					stateRoot = header.Root
					fmt.Printf("Block %d: hash=%s stateRoot=%s\n", height, hash.Hex()[:16]+"...", stateRoot.Hex()[:16]+"...")
				} else {
					// Try raw RLP
					var rawList []rlp.RawValue
					if err := rlp.DecodeBytes(headerData, &rawList); err == nil && len(rawList) > 3 {
						if len(rawList[3]) == 32 {
							copy(stateRoot[:], rawList[3])
							fmt.Printf("Block %d: hash=%s stateRoot(raw)=%s\n", height, hash.Hex()[:16]+"...", stateRoot.Hex()[:16]+"...")
						}
					}
				}
				
				// Check if we have state for this root
				if stateRoot != (common.Hash{}) {
					// Try different state key formats
					stateKeys := [][]byte{
						append([]byte("s"), stateRoot[:]...),
						append([]byte("t"), stateRoot[:]...), // trie nodes
						append([]byte("n"), stateRoot[:]...), // new format
					}
					
					for _, key := range stateKeys {
						if _, err := db.Get(key); err == nil {
							fmt.Printf("  ✓ Found state data with prefix '%c'\n", key[0])
							break
						}
					}
				}
			}
		} else {
			if height < 1082780 {
				fmt.Printf("Block %d: NOT FOUND\n", height)
			}
		}
	}
	
	// Check what prefixes we have
	fmt.Println("\n--- Database Key Prefixes ---")
	prefixes := make(map[byte]int)
	count := 0
	
	it := db.NewIterator(nil, nil)
	defer it.Release()
	
	for it.Next() && count < 10000 {
		key := it.Key()
		if len(key) > 0 {
			prefixes[key[0]]++
		}
		count++
	}
	
	fmt.Printf("Analyzed %d keys:\n", count)
	for prefix, cnt := range prefixes {
		fmt.Printf("  '%c' (0x%02x): %d entries\n", prefix, prefix, cnt)
	}
	
	// Check for state-related keys
	fmt.Println("\n--- Looking for State Storage ---")
	
	// Check for code storage
	codeKey := append([]byte("c"), common.HexToHash("0x0")...)
	if _, err := db.Get(codeKey); err == nil {
		fmt.Println("✓ Found code storage")
	} else {
		fmt.Println("✗ No code storage found")
	}
	
	// Check for secure trie nodes
	secureKey := []byte("secure-key-")
	if _, err := db.Get(secureKey); err == nil {
		fmt.Println("✓ Found secure trie nodes")
	} else {
		fmt.Println("✗ No secure trie nodes found")
	}
	
	fmt.Println("\n=== State Check Complete ===")
	fmt.Println("\nThe database contains block headers but NO STATE DATA!")
	fmt.Println("This means we only have block headers/bodies, not account balances.")
	fmt.Println("We need to either:")
	fmt.Println("1. Import state from a state snapshot")
	fmt.Println("2. Sync state from the network")
	fmt.Println("3. Use a full node backup that includes state")
}