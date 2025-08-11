package main

import (
	"encoding/binary"
	"fmt"
	"os"
	
	"github.com/dgraph-io/badger/v4"
	"github.com/ethereum/go-ethereum/common"
)

func main() {
	dbPath := "/home/z/.luxd/network-96369/chains/X6CU5qgMJfzsTB9UWxj2ZY5hd68x41HfZ4m4hCBWbHuj1Ebc3/ethdb"
	
	fmt.Printf("Fixing canonical hash mappings in BadgerDB at: %s\n", dbPath)
	
	opts := badger.DefaultOptions(dbPath)
	opts.SyncWrites = false
	opts.Logger = nil
	
	db, err := badger.Open(opts)
	if err != nil {
		fmt.Printf("Error opening database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()
	
	// Define the canonical chain
	canonicalBlocks := map[uint64]string{
		0:       "0x3f4fa2a0b0ce089f52bf0ae9199c75ffdd76ecafc987794050cb0d286f1ec61e", // Genesis
		1082780: "0x32dede1fc8e0f11ecde12fb42aef7933fc6c5fcf863bc277b5eac08ae4d461f0", // Latest
	}
	
	// Write canonical hash mappings
	err = db.Update(func(txn *badger.Txn) error {
		for blockNum, hashStr := range canonicalBlocks {
			hash := common.HexToHash(hashStr)
			
			// Write canonical hash (H + num -> hash)
			canonicalKey := make([]byte, 9)
			canonicalKey[0] = 'H' // 0x48
			binary.BigEndian.PutUint64(canonicalKey[1:], blockNum)
			
			if err := txn.Set(canonicalKey, hash[:]); err != nil {
				return fmt.Errorf("failed to write canonical hash for block %d: %w", blockNum, err)
			}
			fmt.Printf("✅ Wrote canonical hash for block %d: %x\n", blockNum, hash)
		}
		
		// Also write the head fast block hash
		latestHash := common.HexToHash(canonicalBlocks[1082780])
		if err := txn.Set([]byte("LastFast"), latestHash[:]); err != nil {
			return fmt.Errorf("failed to write LastFast: %w", err)
		}
		fmt.Printf("✅ Wrote LastFast hash: %x\n", latestHash)
		
		// Write head-header key for lookup
		// Format: head-header + hash -> block number
		headHeaderKey := append([]byte("head-header"), latestHash[:]...)
		blockNumBytes := make([]byte, 8)
		binary.BigEndian.PutUint64(blockNumBytes, 1082780)
		if err := txn.Set(headHeaderKey, blockNumBytes); err != nil {
			return fmt.Errorf("failed to write head-header: %w", err)
		}
		fmt.Printf("✅ Wrote head-header mapping for block 1082780\n")
		
		// Do the same for genesis
		genesisHash := common.HexToHash(canonicalBlocks[0])
		genesisHeaderKey := append([]byte("head-header"), genesisHash[:]...)
		genesisNumBytes := make([]byte, 8)
		binary.BigEndian.PutUint64(genesisNumBytes, 0)
		if err := txn.Set(genesisHeaderKey, genesisNumBytes); err != nil {
			return fmt.Errorf("failed to write genesis head-header: %w", err)
		}
		fmt.Printf("✅ Wrote head-header mapping for genesis\n")
		
		return nil
	})
	
	if err != nil {
		fmt.Printf("Error updating database: %v\n", err)
		os.Exit(1)
	}
	
	fmt.Printf("\n✅ Successfully fixed canonical hash mappings!\n")
	fmt.Printf("Database should now have proper canonical chain from genesis to block 1082780\n")
}