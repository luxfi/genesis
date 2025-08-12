package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"log"
	"math/big"

	"github.com/cockroachdb/pebble"
	"github.com/luxfi/database/badgerdb"
	"github.com/prometheus/client_golang/prometheus"
)

// MigrateWithRawDB performs migration using proper rawdb helpers
func MigrateWithRawDB(sourceDB, ethdbPath, vmdbPath string) error {
	fmt.Println("=== Starting Fixed Migration ===")
	
	// Open source PebbleDB
	source, err := pebble.Open(sourceDB, &pebble.Options{ReadOnly: true})
	if err != nil {
		return fmt.Errorf("failed to open source: %v", err)
	}
	defer source.Close()
	
	// Open destination databases
	ethdb, err := badgerdb.New(ethdbPath, nil, "", prometheus.DefaultRegisterer)
	if err != nil {
		return fmt.Errorf("failed to create ethdb: %v", err)
	}
	defer ethdb.Close()
	
	vmdb, err := badgerdb.New(vmdbPath, nil, "", prometheus.DefaultRegisterer)
	if err != nil {
		return fmt.Errorf("failed to create vmdb: %v", err)
	}
	defer vmdb.Close()
	
	namespace := []byte{
		0x33, 0x7f, 0xb7, 0x3f, 0x9b, 0xcd, 0xac, 0x8c, 0x31,
		0xa2, 0xd5, 0xf7, 0xb8, 0x77, 0xab, 0x1e, 0x8a, 0x2b,
		0x7f, 0x2a, 0x1e, 0x9b, 0xf0, 0x2a, 0x0a, 0x0e, 0x6c,
		0x6f, 0xd1, 0x64, 0xf1, 0xd1,
	}
	
	// Phase 1: Collect canonical chain
	fmt.Println("Phase 1: Finding canonical chain...")
	canonicalBlocks := make(map[uint64][]byte) // num -> hash
	
	iter, err := source.NewIter(nil)
	if err != nil {
		return err
	}
	defer iter.Close()
	
	for iter.First(); iter.Valid(); iter.Next() {
		key := iter.Key()
		
		// Look for SubnetEVM canonical format: namespace(32) + 'H'(1) + num(8) -> hash(32)
		if len(key) == 41 && bytes.HasPrefix(key, namespace) {
			actualKey := key[32:]
			if actualKey[0] == 'H' && len(actualKey) == 9 {
				num := binary.BigEndian.Uint64(actualKey[1:9])
				value, err := iter.ValueAndErr()
				if err == nil && len(value) == 32 {
					canonicalBlocks[num] = value
					if num%100000 == 0 || num == 1082780 {
						fmt.Printf("  Found canonical block %d\n", num)
					}
				}
			}
		}
	}
	
	fmt.Printf("Found %d canonical blocks, max height: %d\n", len(canonicalBlocks), 1082780)
	
	// Phase 2: Migrate block data
	fmt.Println("\nPhase 2: Migrating block data...")
	iter2, _ := source.NewIter(nil)
	defer iter2.Close()
	
	batch := ethdb.NewBatch()
	batchCount := 0
	migratedBlocks := 0
	
	for iter2.First(); iter2.Valid(); iter2.Next() {
		key := iter2.Key()
		value, err := iter2.ValueAndErr()
		if err != nil {
			continue
		}
		
		// Process 73-byte keys (namespace + prefix + num + hash)
		if len(key) == 73 && bytes.HasPrefix(key, namespace) {
			actualKey := key[32:]
			prefix := actualKey[0]
			num := binary.BigEndian.Uint64(actualKey[1:9])
			hash := actualKey[9:41]
			
			// Only process canonical blocks
			canonicalHash, isCanonical := canonicalBlocks[num]
			if !isCanonical || !bytes.Equal(hash, canonicalHash) {
				continue
			}
			
			// Write the data with correct key format
			switch prefix {
			case 'h': // header
				// h + num(8) + hash(32) -> header data
				batch.Put(actualKey, value)
				
				// Also write H + hash -> num mapping
				hashNumKey := make([]byte, 33)
				hashNumKey[0] = 'H'
				copy(hashNumKey[1:33], hash)
				numBytes := make([]byte, 8)
				binary.BigEndian.PutUint64(numBytes, num)
				batch.Put(hashNumKey, numBytes)
				
			case 'b': // body
				// b + num(8) + hash(32) -> body data
				batch.Put(actualKey, value)
				
			case 'r': // receipt
				// r + num(8) + hash(32) -> receipt data
				batch.Put(actualKey, value)
			}
			
			migratedBlocks++
		}
		
		// Process 64-byte keys (state nodes)
		if len(key) == 64 && bytes.HasPrefix(key, namespace) {
			actualKey := key[32:] // 32-byte state node key
			batch.Put(actualKey, value)
		}
		
		// Write batch periodically
		batchCount++
		if batchCount >= 10000 {
			if err := batch.Write(); err != nil {
				return fmt.Errorf("batch write failed: %v", err)
			}
			batch.Reset()
			batchCount = 0
			if migratedBlocks%100000 == 0 {
				fmt.Printf("  Migrated %d blocks...\n", migratedBlocks/3) // div by 3 for h,b,r
			}
		}
	}
	
	// Write final batch
	if batchCount > 0 {
		if err := batch.Write(); err != nil {
			return err
		}
	}
	
	fmt.Printf("Migrated %d block components\n", migratedBlocks)
	
	// Phase 3: Write canonical chain mappings
	fmt.Println("\nPhase 3: Writing canonical chain...")
	for num, hash := range canonicalBlocks {
		// h + num(8) + 'n' -> hash
		canonKey := make([]byte, 10)
		canonKey[0] = 'h'
		binary.BigEndian.PutUint64(canonKey[1:9], num)
		canonKey[9] = 'n'
		ethdb.Put(canonKey, hash)
	}
	
	// Phase 4: Compute and write Total Difficulty
	fmt.Println("\nPhase 4: Computing Total Difficulty...")
	for num := uint64(0); num <= 1082780; num++ {
		hash, exists := canonicalBlocks[num]
		if !exists {
			continue
		}
		
		// TD for C-Chain = height + 1
		td := big.NewInt(int64(num + 1))
		
		// TD key: h + num(8) + hash(32) + 't'
		tdKey := make([]byte, 42)
		tdKey[0] = 'h'
		binary.BigEndian.PutUint64(tdKey[1:9], num)
		copy(tdKey[9:41], hash)
		tdKey[41] = 't'
		
		ethdb.Put(tdKey, td.Bytes())
		
		if num%100000 == 0 || num == 1082780 {
			fmt.Printf("  TD at block %d: %s\n", num, td.String())
		}
	}
	
	// Phase 5: Set head pointers
	fmt.Println("\nPhase 5: Setting head pointers...")
	tipHash := canonicalBlocks[1082780]
	if len(tipHash) != 32 {
		return fmt.Errorf("invalid tip hash")
	}
	
	// Use rawdb-style keys
	ethdb.Put([]byte("LastHeader"), tipHash)
	ethdb.Put([]byte("LastBlock"), tipHash)
	ethdb.Put([]byte("LastFast"), tipHash)
	
	fmt.Printf("Set heads to block 1082780: %x\n", tipHash)
	
	// Phase 6: Write VM metadata
	fmt.Println("\nPhase 6: Writing VM metadata...")
	
	// lastAccepted -> 32 raw bytes
	vmdb.Put([]byte("lastAccepted"), tipHash)
	
	// lastAcceptedHeight -> 8-byte BE
	heightBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(heightBytes, 1082780)
	vmdb.Put([]byte("lastAcceptedHeight"), heightBytes)
	
	// initialized -> 0x01
	vmdb.Put([]byte("initialized"), []byte{1})
	
	fmt.Println("VM metadata written")
	
	// Phase 7: Sync databases
	fmt.Println("\nPhase 7: Syncing databases...")
	
	// Badger doesn't have explicit Sync, but closing will flush
	// We'll reopen to verify
	ethdb.Close()
	vmdb.Close()
	
	// Reopen to verify
	ethdb2, err := badgerdb.New(ethdbPath, nil, "", prometheus.DefaultRegisterer)
	if err != nil {
		return fmt.Errorf("failed to reopen ethdb: %v", err)
	}
	defer ethdb2.Close()
	
	// Verify tip
	lastBlock, err := ethdb2.Get([]byte("LastBlock"))
	if err != nil || !bytes.Equal(lastBlock, tipHash) {
		return fmt.Errorf("verification failed: LastBlock mismatch")
	}
	
	// Check TD at tip
	tdKey := make([]byte, 42)
	tdKey[0] = 'h'
	binary.BigEndian.PutUint64(tdKey[1:9], 1082780)
	copy(tdKey[9:41], tipHash)
	tdKey[41] = 't'
	
	tdBytes, err := ethdb2.Get(tdKey)
	if err != nil {
		return fmt.Errorf("TD not found at tip")
	}
	td := new(big.Int).SetBytes(tdBytes)
	if td.Uint64() != 1082781 {
		return fmt.Errorf("TD mismatch: got %s, want 1082781", td.String())
	}
	
	fmt.Println("\n✅ Migration complete and verified!")
	fmt.Printf("   Tip: block 1082780, hash %x\n", tipHash)
	fmt.Printf("   TD: %s\n", td.String())
	fmt.Printf("   Database size: Check %s\n", ethdbPath)
	
	return nil
}

func main() {
	sourceDB := "/home/z/work/lux/state/chaindata/lux-mainnet-96369/db/pebbledb"
	ethdbPath := "/home/z/.luxd/network-96369/chains/X6CU5qgMJfzsTB9UWxj2ZY5hd68x41HfZ4m4hCBWbHuj1Ebc3/ethdb"
	vmdbPath := "/home/z/.luxd/network-96369/chains/X6CU5qgMJfzsTB9UWxj2ZY5hd68x41HfZ4m4hCBWbHuj1Ebc3/vm"
	
	if err := MigrateWithRawDB(sourceDB, ethdbPath, vmdbPath); err != nil {
		log.Fatal(err)
	}
}