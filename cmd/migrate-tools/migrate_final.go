package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"log"
	"math/big"
	"os"
	"path/filepath"

	"github.com/cockroachdb/pebble"
	"github.com/luxfi/database/badgerdb"
	"github.com/prometheus/client_golang/prometheus"
)

// MigrateFinal performs the complete migration with all fixes
func MigrateFinal(sourceDB, destDir string) error {
	fmt.Println("=== Final Migration Tool ===")
	
	// Setup paths
	chainID := "X6CU5qgMJfzsTB9UWxj2ZY5hd68x41HfZ4m4hCBWbHuj1Ebc3"
	ethdbPath := filepath.Join(destDir, "network-96369", "chains", chainID, "ethdb")
	vmdbPath := filepath.Join(destDir, "network-96369", "chains", chainID, "vm")
	
	fmt.Printf("Source: %s\n", sourceDB)
	fmt.Printf("EthDB: %s\n", ethdbPath)
	fmt.Printf("VMDB: %s\n", vmdbPath)
	
	// Create directories
	if err := os.MkdirAll(ethdbPath, 0755); err != nil {
		return fmt.Errorf("failed to create ethdb dir: %v", err)
	}
	if err := os.MkdirAll(vmdbPath, 0755); err != nil {
		return fmt.Errorf("failed to create vm dir: %v", err)
	}
	
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
	
	// Phase 1: Find canonical chain from h<num>n keys
	fmt.Println("\nPhase 1: Finding canonical chain...")
	canonicalBlocks := make(map[uint64][]byte) // num -> hash
	
	iter, err := source.NewIter(nil)
	if err != nil {
		return err
	}
	defer iter.Close()
	
	for iter.First(); iter.Valid(); iter.Next() {
		key := iter.Key()
		
		// Look for canonical mappings: namespace(32) + 'h'(0x68) + num(8) + 'n'(0x6e)
		if len(key) == 42 && bytes.HasPrefix(key, namespace) {
			actualKey := key[32:]
			if actualKey[0] == 'h' && len(actualKey) == 10 && actualKey[9] == 'n' {
				num := binary.BigEndian.Uint64(actualKey[1:9])
				value, err := iter.ValueAndErr()
				if err == nil && len(value) == 32 {
					canonicalBlocks[num] = value
				}
			}
		}
	}
	
	fmt.Printf("Found %d canonical blocks\n", len(canonicalBlocks))
	if len(canonicalBlocks) == 0 {
		return fmt.Errorf("no canonical blocks found")
	}
	
	// Verify we have the tip
	tipHash, hasTip := canonicalBlocks[1082780]
	if !hasTip {
		return fmt.Errorf("missing tip block 1082780")
	}
	fmt.Printf("Tip hash: %x\n", tipHash)
	
	// Phase 2: Migrate all data
	fmt.Println("\nPhase 2: Migrating blockchain data...")
	
	iter2, _ := source.NewIter(nil)
	defer iter2.Close()
	
	batch := ethdb.NewBatch()
	batchCount := 0
	stats := struct {
		Headers  int
		Bodies   int
		Receipts int
		State    int
		Other    int
	}{}
	
	for iter2.First(); iter2.Valid(); iter2.Next() {
		key := iter2.Key()
		value, err := iter2.ValueAndErr()
		if err != nil {
			continue
		}
		
		// Skip non-namespaced keys
		if !bytes.HasPrefix(key, namespace) {
			continue
		}
		
		actualKey := key[32:]
		
		// Categorize and write
		switch len(key) {
		case 73: // Block data (h/b/r + num + hash)
			prefix := actualKey[0]
			switch prefix {
			case 'h':
				batch.Put(actualKey, value)
				stats.Headers++
			case 'b':
				batch.Put(actualKey, value)
				stats.Bodies++
			case 'r':
				batch.Put(actualKey, value)
				stats.Receipts++
			}
			
		case 42: // Canonical mappings (h + num + n)
			if actualKey[0] == 'h' && actualKey[9] == 'n' {
				batch.Put(actualKey, value)
			}
			
		case 65: // Hash->number mappings (H + hash)
			if actualKey[0] == 'H' {
				batch.Put(actualKey, value)
			}
			
		case 64: // State nodes (32-byte keys)
			batch.Put(actualKey, value)
			stats.State++
			
		default:
			// Write other keys as-is
			if len(actualKey) > 0 {
				batch.Put(actualKey, value)
				stats.Other++
			}
		}
		
		// Write batch periodically
		batchCount++
		if batchCount >= 10000 {
			if err := batch.Write(); err != nil {
				return fmt.Errorf("batch write failed: %v", err)
			}
			batch.Reset()
			batchCount = 0
			if (stats.Headers + stats.Bodies + stats.Receipts) % 100000 == 0 {
				fmt.Printf("  Progress: %d headers, %d bodies, %d receipts, %d state\n",
					stats.Headers, stats.Bodies, stats.Receipts, stats.State)
			}
		}
	}
	
	// Write final batch
	if batchCount > 0 {
		if err := batch.Write(); err != nil {
			return err
		}
	}
	
	fmt.Printf("\nMigrated:\n")
	fmt.Printf("  Headers:  %d\n", stats.Headers)
	fmt.Printf("  Bodies:   %d\n", stats.Bodies)
	fmt.Printf("  Receipts: %d\n", stats.Receipts)
	fmt.Printf("  State:    %d\n", stats.State)
	fmt.Printf("  Other:    %d\n", stats.Other)
	
	// Phase 3: Create hash->number mappings if missing
	fmt.Println("\nPhase 3: Ensuring hash->number mappings...")
	for num, hash := range canonicalBlocks {
		// H + hash -> num
		hashNumKey := make([]byte, 33)
		hashNumKey[0] = 'H'
		copy(hashNumKey[1:33], hash)
		
		numBytes := make([]byte, 8)
		binary.BigEndian.PutUint64(numBytes, num)
		ethdb.Put(hashNumKey, numBytes)
	}
	
	// Phase 4: Compute Total Difficulty
	fmt.Println("\nPhase 4: Computing Total Difficulty...")
	for num := uint64(0); num <= 1082780; num++ {
		hash, exists := canonicalBlocks[num]
		if !exists {
			continue
		}
		
		// TD = height + 1 for C-Chain
		td := big.NewInt(int64(num + 1))
		
		// TD key: h + num(8) + hash(32) + 't'
		tdKey := make([]byte, 42)
		tdKey[0] = 'h'
		binary.BigEndian.PutUint64(tdKey[1:9], num)
		copy(tdKey[9:41], hash)
		tdKey[41] = 't'
		
		ethdb.Put(tdKey, td.Bytes())
		
		if num == 0 || num == 1082780 || num%200000 == 0 {
			fmt.Printf("  TD at block %d: %s\n", num, td.String())
		}
	}
	
	// Phase 5: Set head pointers
	fmt.Println("\nPhase 5: Setting head pointers...")
	
	// Write head pointers using standard keys
	ethdb.Put([]byte("LastHeader"), tipHash)
	ethdb.Put([]byte("LastBlock"), tipHash)
	ethdb.Put([]byte("LastFast"), tipHash)
	ethdb.Put([]byte("LastFinalized"), tipHash)
	
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
	
	// Phase 7: Close and sync
	fmt.Println("\nPhase 7: Syncing databases...")
	ethdb.Close()
	vmdb.Close()
	
	// Phase 8: Verify
	fmt.Println("\nPhase 8: Verifying migration...")
	
	// Reopen to verify
	ethdb2, err := badgerdb.New(ethdbPath, nil, "", prometheus.DefaultRegisterer)
	if err != nil {
		return fmt.Errorf("failed to reopen ethdb: %v", err)
	}
	defer ethdb2.Close()
	
	// Check heads
	checks := []string{"LastHeader", "LastBlock", "LastFast"}
	for _, key := range checks {
		val, err := ethdb2.Get([]byte(key))
		if err != nil {
			return fmt.Errorf("%s not found", key)
		}
		if !bytes.Equal(val, tipHash) {
			return fmt.Errorf("%s mismatch", key)
		}
		fmt.Printf("  ✓ %s correct\n", key)
	}
	
	// Check canonical at tip
	canonKey := make([]byte, 10)
	canonKey[0] = 'h'
	binary.BigEndian.PutUint64(canonKey[1:9], 1082780)
	canonKey[9] = 'n'
	
	canonHash, err := ethdb2.Get(canonKey)
	if err != nil || !bytes.Equal(canonHash, tipHash) {
		return fmt.Errorf("canonical at tip not found or mismatch")
	}
	fmt.Printf("  ✓ Canonical at 1082780 correct\n")
	
	// Check TD at tip
	tdKey := make([]byte, 42)
	tdKey[0] = 'h'
	binary.BigEndian.PutUint64(tdKey[1:9], 1082780)
	copy(tdKey[9:41], tipHash)
	tdKey[41] = 't'
	
	tdBytes, err := ethdb2.Get(tdKey)
	if err != nil {
		return fmt.Errorf("TD at tip not found")
	}
	td := new(big.Int).SetBytes(tdBytes)
	if td.Uint64() != 1082781 {
		return fmt.Errorf("TD mismatch: got %s, want 1082781", td.String())
	}
	fmt.Printf("  ✓ TD at tip: %s\n", td.String())
	
	// Check header exists
	headerKey := make([]byte, 41)
	headerKey[0] = 'h'
	binary.BigEndian.PutUint64(headerKey[1:9], 1082780)
	copy(headerKey[9:41], tipHash)
	
	if _, err := ethdb2.Get(headerKey); err != nil {
		return fmt.Errorf("header at tip not found")
	}
	fmt.Printf("  ✓ Header at tip exists\n")
	
	// Check body exists
	bodyKey := make([]byte, 41)
	bodyKey[0] = 'b'
	binary.BigEndian.PutUint64(bodyKey[1:9], 1082780)
	copy(bodyKey[9:41], tipHash)
	
	if _, err := ethdb2.Get(bodyKey); err != nil {
		return fmt.Errorf("body at tip not found")
	}
	fmt.Printf("  ✓ Body at tip exists\n")
	
	fmt.Println("\n✅ Migration complete and verified!")
	fmt.Println("\nNext steps:")
	fmt.Println("1. Launch luxd with --db-dir=" + destDir)
	fmt.Println("2. Check RPC at http://localhost:9630/ext/bc/C/rpc")
	fmt.Println("3. Query block height and balance")
	
	return nil
}

func main() {
	sourceDB := "/home/z/work/lux/state/chaindata/lux-mainnet-96369/db/pebbledb"
	destDir := "/home/z/.luxd"
	
	if err := MigrateFinal(sourceDB, destDir); err != nil {
		log.Fatal(err)
	}
}