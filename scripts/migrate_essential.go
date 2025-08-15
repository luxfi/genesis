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
	
	fmt.Printf("Migrating essential data from PebbleDB to BadgerDB...\n")
	fmt.Printf("Source: %s\n", sourcePath)
	fmt.Printf("Target: %s\n", targetPath)
	
	// Open source PebbleDB
	sourceDB, err := pebble.New(sourcePath, 0, 0, "", true)
	if err != nil {
		log.Fatal("Failed to open source PebbleDB:", err)
	}
	defer sourceDB.Close()
	
	// Open target BadgerDB with minimal settings
	opts := badger.DefaultOptions(targetPath)
	opts.SyncWrites = false // Faster writes
	opts.ValueLogFileSize = 256 << 20 // 256MB smaller files
	
	targetDB, err := badger.Open(opts)
	if err != nil {
		log.Fatal("Failed to open target BadgerDB:", err)
	}
	defer func() {
		// Ensure clean close
		targetDB.Close()
	}()
	
	// Track what we find
	var totalKeys int64
	var lastBlock uint64
	var lastHash common.Hash
	var genesisHash common.Hash
	
	// Target block we want to migrate up to
	targetBlock := uint64(1082780)
	
	fmt.Printf("Migrating blocks 0 to %d...\n", targetBlock)
	
	// First pass: Just migrate canonical hashes and find genesis
	fmt.Println("Pass 1: Migrating canonical block hashes...")
	
	batch := targetDB.NewWriteBatch()
	defer batch.Cancel()
	
	for blockNum := uint64(0); blockNum <= targetBlock; blockNum++ {
		// Build canonical hash key
		canonicalKey := make([]byte, 9)
		canonicalKey[0] = 'H'
		binary.BigEndian.PutUint64(canonicalKey[1:], blockNum)
		
		// Read from source
		hashBytes, err := sourceDB.Get(canonicalKey)
		if err != nil {
			if blockNum == 0 {
				log.Fatal("Genesis block not found in source database")
			}
			// Skip missing blocks
			continue
		}
		
		// Write to target
		err = batch.Set(canonicalKey, hashBytes)
		if err != nil {
			log.Printf("Failed to write canonical hash for block %d: %v", blockNum, err)
			continue
		}
		
		// Track important blocks
		if blockNum == 0 {
			copy(genesisHash[:], hashBytes)
			fmt.Printf("Genesis hash: %x\n", genesisHash)
		}
		if blockNum == targetBlock {
			copy(lastHash[:], hashBytes)
			lastBlock = blockNum
			fmt.Printf("Target block %d hash: %x\n", blockNum, lastHash[:8])
		}
		
		totalKeys++
		
		// Progress
		if blockNum%10000 == 0 && blockNum > 0 {
			fmt.Printf("  Migrated canonical hashes up to block %d\n", blockNum)
			// Flush periodically
			if err := batch.Flush(); err != nil {
				log.Fatal("Failed to flush batch:", err)
			}
			batch = targetDB.NewWriteBatch()
		}
	}
	
	// Final flush
	if err := batch.Flush(); err != nil {
		log.Fatal("Failed to flush final batch:", err)
	}
	
	fmt.Printf("Migrated %d canonical block hashes\n", totalKeys)
	
	// Second pass: Migrate headers and bodies for key blocks
	fmt.Println("\nPass 2: Migrating block headers and bodies...")
	
	keyBlocks := []uint64{0, targetBlock} // Just genesis and target for now
	
	for _, blockNum := range keyBlocks {
		// Get the hash first
		canonicalKey := make([]byte, 9)
		canonicalKey[0] = 'H'
		binary.BigEndian.PutUint64(canonicalKey[1:], blockNum)
		
		hashBytes, err := sourceDB.Get(canonicalKey)
		if err != nil {
			fmt.Printf("  Block %d: canonical hash not found\n", blockNum)
			continue
		}
		
		var blockHash common.Hash
		copy(blockHash[:], hashBytes)
		
		// Try to migrate the header (format: 'h' + num(8) + hash(32))
		headerKey := make([]byte, 41)
		headerKey[0] = 'h'
		binary.BigEndian.PutUint64(headerKey[1:9], blockNum)
		copy(headerKey[9:], blockHash[:])
		
		headerData, err := sourceDB.Get(headerKey)
		if err != nil {
			fmt.Printf("  Block %d: header not found (key: %x)\n", blockNum, headerKey[:20])
		} else {
			// Write header to target
			err = targetDB.Update(func(txn *badger.Txn) error {
				return txn.Set(headerKey, headerData)
			})
			if err != nil {
				log.Printf("Failed to write header for block %d: %v", blockNum, err)
			} else {
				fmt.Printf("  Block %d: header migrated (%d bytes)\n", blockNum, len(headerData))
			}
		}
		
		// Try to migrate the body (format: 'b' + num(8) + hash(32))
		bodyKey := make([]byte, 41)
		bodyKey[0] = 'b'
		binary.BigEndian.PutUint64(bodyKey[1:9], blockNum)
		copy(bodyKey[9:], blockHash[:])
		
		bodyData, err := sourceDB.Get(bodyKey)
		if err != nil {
			fmt.Printf("  Block %d: body not found\n", blockNum)
		} else {
			// Write body to target
			err = targetDB.Update(func(txn *badger.Txn) error {
				return txn.Set(bodyKey, bodyData)
			})
			if err != nil {
				log.Printf("Failed to write body for block %d: %v", blockNum, err)
			} else {
				fmt.Printf("  Block %d: body migrated (%d bytes)\n", blockNum, len(bodyData))
			}
		}
	}
	
	// Set head pointers
	fmt.Println("\nSetting head pointers...")
	
	err = targetDB.Update(func(txn *badger.Txn) error {
		// Write various head markers
		headKeys := map[string][]byte{
			"LastBlock":   lastHash[:],
			"LastHeader":  lastHash[:],
			"LastFast":    lastHash[:],
		}
		
		for key, value := range headKeys {
			if err := txn.Set([]byte(key), value); err != nil {
				return err
			}
			fmt.Printf("  Set %s to %x\n", key, value[:8])
		}
		
		// Also write height
		heightBytes := make([]byte, 8)
		binary.BigEndian.PutUint64(heightBytes, lastBlock)
		if err := txn.Set([]byte("Height"), heightBytes); err != nil {
			return err
		}
		fmt.Printf("  Set Height to %d\n", lastBlock)
		
		return nil
	})
	
	if err != nil {
		log.Printf("Warning: Failed to set some head pointers: %v", err)
	}
	
	// Create a minimal test to verify the database works
	fmt.Println("\nVerifying database...")
	
	err = targetDB.View(func(txn *badger.Txn) error {
		// Check genesis
		key0 := make([]byte, 9)
		key0[0] = 'H'
		binary.BigEndian.PutUint64(key0[1:], 0)
		
		if item, err := txn.Get(key0); err == nil {
			val, _ := item.ValueCopy(nil)
			fmt.Printf("✓ Genesis block found: %x\n", val[:8])
		} else {
			fmt.Printf("✗ Genesis block not found\n")
		}
		
		// Check target block
		keyTarget := make([]byte, 9)
		keyTarget[0] = 'H'
		binary.BigEndian.PutUint64(keyTarget[1:], targetBlock)
		
		if item, err := txn.Get(keyTarget); err == nil {
			val, _ := item.ValueCopy(nil)
			fmt.Printf("✓ Block %d found: %x\n", targetBlock, val[:8])
		} else {
			fmt.Printf("✗ Block %d not found\n", targetBlock)
		}
		
		return nil
	})
	
	if err != nil {
		log.Printf("Verification error: %v", err)
	}
	
	fmt.Println("\nMigration completed!")
	fmt.Printf("Database location: %s\n", targetPath)
	fmt.Printf("Genesis hash: %x\n", genesisHash)
	fmt.Printf("Last block: %d (hash: %x)\n", lastBlock, lastHash[:8])
	fmt.Println("\nTo start the node:")
	fmt.Printf("export LUX_IMPORTED_HEIGHT=%d\n", lastBlock)
	fmt.Printf("export LUX_IMPORTED_BLOCK_ID=%x\n", lastHash)
}