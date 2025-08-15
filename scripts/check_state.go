package main

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strings"
	
	"github.com/luxfi/database/badgerdb"
	"github.com/luxfi/geth/common"
	"github.com/luxfi/geth/core/types"
	"github.com/luxfi/geth/rlp"
)

func main() {
	fmt.Println("=== Checking State Data in Database ===")
	
	// Open the migrated database
	ethdbPath := "/Users/z/.luxd/network-96369/chains/EWi9aPkGe6EfJ3SobCAmSUXRPLa4brF3cThwPwmHTrD1y13jy/ethdb"
	db, err := badgerdb.New(filepath.Clean(ethdbPath), nil, "", nil)
	if err != nil {
		panic(fmt.Sprintf("Failed to open ethdb: %v", err))
	}
	defer db.Close()
	
	// Check for state trie nodes
	fmt.Println("\n--- Checking for State Trie Data ---")
	
	// Check last header to get state root
	if val, err := db.Get([]byte("LastHeader")); err == nil {
		var lastHash common.Hash
		copy(lastHash[:], val)
		fmt.Printf("LastHeader hash: %s\n", lastHash.Hex())
		
		// Try to get the header at tip
		canonKey := make([]byte, 10)
		canonKey[0] = 'h'
		binary.BigEndian.PutUint64(canonKey[1:9], 1082780)
		canonKey[9] = 'n'
		
		if hashBytes, err := db.Get(canonKey); err == nil {
			var hash common.Hash
			copy(hash[:], hashBytes)
			
			// Get header
			headerKey := make([]byte, 41)
			headerKey[0] = 'h'
			binary.BigEndian.PutUint64(headerKey[1:9], 1082780)
			copy(headerKey[9:], hash[:])
			
			if headerData, err := db.Get(headerKey); err == nil {
				// Try to decode header
				var header types.Header
				if err := rlp.DecodeBytes(headerData, &header); err != nil {
					// Try extracting from raw RLP
					fmt.Printf("Could not decode header as standard format, trying raw RLP...\n")
					var rawList []rlp.RawValue
					if err := rlp.DecodeBytes(headerData, &rawList); err == nil {
						fmt.Printf("Header has %d fields\n", len(rawList))
						if len(rawList) > 3 && len(rawList[3]) == 32 {
							fmt.Printf("State Root (field 3): 0x%x\n", rawList[3])
						}
					}
				} else {
					fmt.Printf("State Root: %s\n", header.Root.Hex())
					
					// Try to find state data for this root
					stateKey := append([]byte("s"), header.Root[:]...)
					if stateData, err := db.Get(stateKey); err == nil {
						fmt.Printf("✓ Found state data for root: %d bytes\n", len(stateData))
					} else {
						fmt.Printf("✗ No state data found for root\n")
					}
				}
			}
		}
	}
	
	// Check for account data
	fmt.Println("\n--- Looking for Account Storage ---")
	
	// Treasury address
	treasury := common.HexToAddress("0x9011E888251AB053B7bD1cdB598Db4f9DEd94714")
	fmt.Printf("Checking treasury: %s\n", treasury.Hex())
	
	// Look for any keys starting with 'a' (accounts)
	accountCount := 0
	err = db.Iterate(func(key, value []byte) bool {
		if len(key) > 0 && key[0] == 'a' {
			accountCount++
			if accountCount <= 5 {
				fmt.Printf("  Account key: %x (value: %d bytes)\n", key[:min(20, len(key))], len(value))
			}
		}
		return accountCount < 100 // Stop after 100 to avoid too much output
	})
	
	if accountCount > 0 {
		fmt.Printf("✓ Found %d account entries\n", accountCount)
	} else {
		fmt.Printf("✗ No account entries found\n")
	}
	
	// Check for snapshot data
	fmt.Println("\n--- Checking for Snapshot Data ---")
	
	// Check snapshot root
	if val, err := db.Get([]byte("SnapshotRoot")); err == nil {
		var snapRoot common.Hash
		copy(snapRoot[:], val)
		fmt.Printf("✓ Snapshot Root: %s\n", snapRoot.Hex())
	} else {
		fmt.Printf("✗ No snapshot root found\n")
	}
	
	// Check for any snapshot account data
	snapCount := 0
	err = db.Iterate(func(key, value []byte) bool {
		keyStr := string(key)
		if strings.HasPrefix(keyStr, "snap") {
			snapCount++
			if snapCount <= 5 {
				fmt.Printf("  Snapshot key: %s... (value: %d bytes)\n", keyStr[:min(20, len(keyStr))], len(value))
			}
		}
		return snapCount < 100
	})
	
	if snapCount > 0 {
		fmt.Printf("✓ Found %d snapshot entries\n", snapCount)
	} else {
		fmt.Printf("✗ No snapshot entries found\n")
	}
	
	// Check total database entries
	fmt.Println("\n--- Database Statistics ---")
	
	totalEntries := 0
	headerCount := 0
	bodyCount := 0
	receiptCount := 0
	
	err = db.Iterate(func(key, value []byte) bool {
		totalEntries++
		if len(key) > 0 {
			switch key[0] {
			case 'h':
				if len(key) == 41 {
					headerCount++
				}
			case 'b':
				if len(key) == 41 {
					bodyCount++
				}
			case 'r':
				if len(key) == 41 {
					receiptCount++
				}
			}
		}
		return true
	})
	
	fmt.Printf("Total entries: %d\n", totalEntries)
	fmt.Printf("Headers: %d\n", headerCount)
	fmt.Printf("Bodies: %d\n", bodyCount)
	fmt.Printf("Receipts: %d\n", receiptCount)
	
	fmt.Println("\n=== Analysis Complete ===")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}