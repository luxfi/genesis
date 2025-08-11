package main

import (
	"encoding/binary"
	"fmt"
	"os"
	
	"github.com/dgraph-io/badger/v4"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"
)

func main() {
	dbPath := "/home/z/.luxd/network-96369/chains/X6CU5qgMJfzsTB9UWxj2ZY5hd68x41HfZ4m4hCBWbHuj1Ebc3/ethdb"
	
	fmt.Printf("Opening BadgerDB at: %s\n", dbPath)
	
	opts := badger.DefaultOptions(dbPath)
	opts.SyncWrites = false
	opts.Logger = nil
	
	db, err := badger.Open(opts)
	if err != nil {
		fmt.Printf("Error opening database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()
	
	targetBlock := uint64(1082780)
	targetHash := common.HexToHash("0x32dede1fc8e0f11ecde12fb42aef7933fc6c5fcf863bc277b5eac08ae4d461f0")
	
	fmt.Printf("\nChecking for block %d with hash %x\n", targetBlock, targetHash)
	
	// Check canonical hash (H + num)
	err = db.View(func(txn *badger.Txn) error {
		// Check canonical hash key
		canonicalKey := make([]byte, 9)
		canonicalKey[0] = 'H' // 0x48
		binary.BigEndian.PutUint64(canonicalKey[1:], targetBlock)
		
		item, err := txn.Get(canonicalKey)
		if err != nil {
			fmt.Printf("❌ No canonical hash for block %d: %v\n", targetBlock, err)
		} else {
			hash, _ := item.ValueCopy(nil)
			fmt.Printf("✅ Found canonical hash for block %d: %x\n", targetBlock, hash)
		}
		
		// Check header key (h + num + hash)
		headerKey := make([]byte, 41)
		headerKey[0] = 'h' // 0x68
		binary.BigEndian.PutUint64(headerKey[1:9], targetBlock)
		copy(headerKey[9:], targetHash[:])
		
		item, err = txn.Get(headerKey)
		if err != nil {
			fmt.Printf("❌ No header for block %d at expected hash: %v\n", targetBlock, err)
			
			// Try to find any header for this block number
			fmt.Printf("\nSearching for any headers at block %d...\n", targetBlock)
			prefix := make([]byte, 9)
			prefix[0] = 'h'
			binary.BigEndian.PutUint64(prefix[1:], targetBlock)
			
			it := txn.NewIterator(badger.DefaultIteratorOptions)
			defer it.Close()
			
			count := 0
			for it.Seek(prefix); it.Valid(); it.Next() {
				key := it.Item().Key()
				if len(key) < 9 || !startsWith(key, prefix) {
					break
				}
				
				if len(key) == 41 {
					blockHash := key[9:]
					fmt.Printf("  Found header at hash: %x\n", blockHash)
					count++
				}
			}
			
			if count == 0 {
				fmt.Printf("  No headers found for block %d\n", targetBlock)
			}
		} else {
			headerData, _ := item.ValueCopy(nil)
			fmt.Printf("✅ Found header for block %d (size: %d bytes)\n", targetBlock, len(headerData))
			
			// Try to decode the header
			var header types.Header
			if err := rlp.DecodeBytes(headerData, &header); err != nil {
				fmt.Printf("❌ Failed to decode header: %v\n", err)
			} else {
				fmt.Printf("✅ Successfully decoded header!\n")
				fmt.Printf("  Number: %d\n", header.Number.Uint64())
				fmt.Printf("  Hash: %x\n", header.Hash())
				fmt.Printf("  ParentHash: %x\n", header.ParentHash)
				fmt.Printf("  StateRoot: %x\n", header.Root)
			}
		}
		
		// Check LastBlock key
		lastBlockKey := []byte("LastBlock")
		item, err = txn.Get(lastBlockKey)
		if err != nil {
			fmt.Printf("\n❌ No LastBlock key: %v\n", err)
		} else {
			hash, _ := item.ValueCopy(nil)
			fmt.Printf("\n✅ LastBlock hash: %x\n", hash)
		}
		
		// Check LastHeader key
		lastHeaderKey := []byte("LastHeader")
		item, err = txn.Get(lastHeaderKey)
		if err != nil {
			fmt.Printf("❌ No LastHeader key: %v\n", err)
		} else {
			hash, _ := item.ValueCopy(nil)
			fmt.Printf("✅ LastHeader hash: %x\n", hash)
		}
		
		// Check for genesis
		genesisKey := make([]byte, 9)
		genesisKey[0] = 'H'
		binary.BigEndian.PutUint64(genesisKey[1:], 0)
		
		item, err = txn.Get(genesisKey)
		if err != nil {
			fmt.Printf("\n❌ No canonical genesis: %v\n", err)
		} else {
			hash, _ := item.ValueCopy(nil)
			fmt.Printf("\n✅ Found genesis canonical hash: %x\n", hash)
			
			// Try to get the genesis header
			genesisHeaderKey := make([]byte, 41)
			genesisHeaderKey[0] = 'h'
			binary.BigEndian.PutUint64(genesisHeaderKey[1:9], 0)
			copy(genesisHeaderKey[9:], hash)
			
			item, err = txn.Get(genesisHeaderKey)
			if err != nil {
				fmt.Printf("❌ No genesis header: %v\n", err)
			} else {
				headerData, _ := item.ValueCopy(nil)
				fmt.Printf("✅ Found genesis header (size: %d bytes)\n", len(headerData))
			}
		}
		
		return nil
	})
	
	if err != nil {
		fmt.Printf("Error reading database: %v\n", err)
	}
}

func startsWith(key, prefix []byte) bool {
	if len(key) < len(prefix) {
		return false
	}
	for i := range prefix {
		if key[i] != prefix[i] {
			return false
		}
	}
	return true
}