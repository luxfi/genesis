package main

import (
	"encoding/binary"
	"fmt"
	"log"
	"os"
	
	"github.com/dgraph-io/badger/v4"
	"github.com/luxfi/geth/common"
	"github.com/luxfi/geth/ethdb/pebble"
)

func main() {
	sourcePath := "/Users/z/work/lux/genesis/state/chaindata/lux-mainnet-96369/db/pebbledb"
	targetPath := "/Users/z/.luxd/network-96369/chains/EWi9aPkGe6EfJ3SobCAmSUXRPLa4brF3cThwPwmHTrD1y13jy/ethdb"
	
	// Create target directory
	os.MkdirAll(targetPath, 0755)
	
	fmt.Printf("Migrating from PebbleDB to BadgerDB with headers...\n")
	fmt.Printf("Source: %s\n", sourcePath)
	fmt.Printf("Target: %s\n", targetPath)
	
	// Open source PebbleDB
	sourceDB, err := pebble.New(sourcePath, 0, 0, "", true)
	if err != nil {
		log.Fatal("Failed to open source PebbleDB:", err)
	}
	defer sourceDB.Close()
	
	// Open target BadgerDB
	opts := badger.DefaultOptions(targetPath)
	opts.ValueLogFileSize = 1 << 30 // 1GB (max is 2GB-1)
	opts.MemTableSize = 128 << 20   // 128MB
	opts.NumMemtables = 4
	opts.NumLevelZeroTables = 4
	opts.NumLevelZeroTablesStall = 8
	
	targetDB, err := badger.Open(opts)
	if err != nil {
		log.Fatal("Failed to open target BadgerDB:", err)
	}
	defer targetDB.Close()
	
	// Track statistics
	var totalKeys, canonicalBlocks, headers, bodies, receipts, td, states int64
	var lastBlock uint64
	var lastHash common.Hash
	
	fmt.Println("Starting migration...")
	
	// Iterate through source database
	it := sourceDB.NewIterator(nil, nil)
	defer it.Release()
	
	batch := targetDB.NewWriteBatch()
	batchSize := 0
	const maxBatchSize = 10000
	
	for it.Next() {
		key := it.Key()
		value := it.Value()
		
		// Make copies since iterator reuses slices
		keyCopy := make([]byte, len(key))
		copy(keyCopy, key)
		valueCopy := make([]byte, len(value))
		copy(valueCopy, value)
		
		// Track what we're migrating
		if len(keyCopy) > 0 {
			switch keyCopy[0] {
			case 'H': // Canonical hash
				if len(keyCopy) == 9 {
					canonicalBlocks++
					blockNum := binary.BigEndian.Uint64(keyCopy[1:])
					if blockNum > lastBlock {
						lastBlock = blockNum
						copy(lastHash[:], valueCopy)
					}
					if blockNum%10000 == 0 {
						fmt.Printf("  Canonical block %d\n", blockNum)
					}
				}
			case 'h': // Header
				if len(keyCopy) >= 9 {
					headers++
				}
			case 'b': // Body
				if len(keyCopy) >= 9 {
					bodies++
				}
			case 'r': // Receipts
				if len(keyCopy) >= 9 {
					receipts++
				}
			case 't': // Total difficulty
				if len(keyCopy) >= 9 {
					td++
				}
			case 's': // State
				states++
			}
		}
		
		// Special handling for specific keys
		if string(keyCopy) == "LastBlock" {
			// This will help us identify the head
			fmt.Printf("Found LastBlock: %x\n", valueCopy)
		}
		
		// Add to batch
		err = batch.Set(keyCopy, valueCopy)
		if err != nil {
			log.Printf("Failed to set key %x: %v", keyCopy[:min(len(keyCopy), 20)], err)
			continue
		}
		
		totalKeys++
		batchSize++
		
		// Flush batch periodically
		if batchSize >= maxBatchSize {
			if err := batch.Flush(); err != nil {
				log.Fatal("Failed to flush batch:", err)
			}
			batch = targetDB.NewWriteBatch()
			batchSize = 0
			
			if totalKeys%100000 == 0 {
				fmt.Printf("Migrated %d keys...\n", totalKeys)
			}
		}
	}
	
	// Flush final batch
	if batchSize > 0 {
		if err := batch.Flush(); err != nil {
			log.Fatal("Failed to flush final batch:", err)
		}
	}
	
	if it.Error() != nil {
		log.Fatal("Iterator error:", it.Error())
	}
	
	fmt.Printf("\nMigration complete!\n")
	fmt.Printf("Total keys migrated: %d\n", totalKeys)
	fmt.Printf("Canonical blocks: %d\n", canonicalBlocks)
	fmt.Printf("Headers: %d\n", headers)
	fmt.Printf("Bodies: %d\n", bodies)
	fmt.Printf("Receipts: %d\n", receipts)
	fmt.Printf("Total difficulty: %d\n", td)
	fmt.Printf("State entries: %d\n", states)
	fmt.Printf("Last block: %d (hash: %x)\n", lastBlock, lastHash)
	
	// Now ensure critical head pointers are set
	fmt.Println("\nSetting head pointers...")
	
	// Write the head block markers
	headKeys := map[string][]byte{
		"LastBlock":         lastHash[:],
		string([]byte{0x4c, 0x61, 0x73, 0x74, 0x42, 0x6c, 0x6f, 0x63, 0x6b}): lastHash[:], // "LastBlock"
		string([]byte{0x48, 0x65, 0x69, 0x67, 0x68, 0x74}): encodeUint64(lastBlock), // "Height"
	}
	
	// Also write the standard geth head markers
	headHashKey := []byte("LastHeader")
	headBlockKey := []byte("LastBlock")
	headFastKey := []byte("LastFast")
	
	err = targetDB.Update(func(txn *badger.Txn) error {
		for key, value := range headKeys {
			if err := txn.Set([]byte(key), value); err != nil {
				return err
			}
		}
		
		// Standard geth markers
		if err := txn.Set(headHashKey, lastHash[:]); err != nil {
			return err
		}
		if err := txn.Set(headBlockKey, lastHash[:]); err != nil {
			return err
		}
		if err := txn.Set(headFastKey, lastHash[:]); err != nil {
			return err
		}
		
		return nil
	})
	
	if err != nil {
		log.Printf("Warning: Failed to set some head pointers: %v", err)
	}
	
	// Verify we can read back critical data
	fmt.Println("\nVerifying migration...")
	
	// Check canonical blocks
	err = targetDB.View(func(txn *badger.Txn) error {
		// Check block 0
		key0 := make([]byte, 9)
		key0[0] = 'H'
		binary.BigEndian.PutUint64(key0[1:], 0)
		
		if item, err := txn.Get(key0); err == nil {
			val, _ := item.ValueCopy(nil)
			fmt.Printf("Block 0 canonical hash: %x\n", val)
		}
		
		// Check last block
		keyLast := make([]byte, 9)
		keyLast[0] = 'H'
		binary.BigEndian.PutUint64(keyLast[1:], lastBlock)
		
		if item, err := txn.Get(keyLast); err == nil {
			val, _ := item.ValueCopy(nil)
			fmt.Printf("Block %d canonical hash: %x\n", lastBlock, val)
			
			// Now check if we have the header for this block
			headerKey := make([]byte, 41)
			headerKey[0] = 'h'
			binary.BigEndian.PutUint64(headerKey[1:9], lastBlock)
			copy(headerKey[9:], val)
			
			if item, err := txn.Get(headerKey); err == nil {
				val, _ := item.ValueCopy(nil)
				fmt.Printf("Block %d header found: %d bytes\n", lastBlock, len(val))
			} else {
				fmt.Printf("WARNING: Block %d header NOT found\n", lastBlock)
			}
		}
		
		return nil
	})
	
	if err != nil {
		log.Printf("Verification error: %v", err)
	}
	
	fmt.Println("\nMigration completed successfully!")
	fmt.Printf("Database location: %s\n", targetPath)
	fmt.Println("\nTo start the node with this data:")
	fmt.Printf("export LUX_IMPORTED_HEIGHT=%d\n", lastBlock)
	fmt.Printf("export LUX_IMPORTED_BLOCK_ID=%x\n", lastHash)
	fmt.Println("Then run the node with --data-dir=/Users/z/.luxd")
}

func encodeUint64(n uint64) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, n)
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}