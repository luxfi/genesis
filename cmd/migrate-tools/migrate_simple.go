package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"log"
	"math/big"
	"os"
	"path/filepath"
	"time"

	"github.com/cockroachdb/pebble"
	"github.com/luxfi/database/badgerdb"
	"github.com/prometheus/client_golang/prometheus"
)

func main() {
	fmt.Println("=== Simple Direct Migration ===")
	
	sourceDB := "/home/z/work/lux/state/chaindata/lux-mainnet-96369/db/pebbledb"
	destDir := "/home/z/.luxd"
	chainID := "X6CU5qgMJfzsTB9UWxj2ZY5hd68x41HfZ4m4hCBWbHuj1Ebc3"
	
	ethdbPath := filepath.Join(destDir, "network-96369", "chains", chainID, "ethdb")
	vmdbPath := filepath.Join(destDir, "network-96369", "chains", chainID, "vm")
	
	// Clean and create directories
	os.RemoveAll(filepath.Join(destDir, "network-96369"))
	os.MkdirAll(ethdbPath, 0755)
	os.MkdirAll(vmdbPath, 0755)
	
	// Open databases
	source, err := pebble.Open(sourceDB, &pebble.Options{ReadOnly: true})
	if err != nil {
		log.Fatal(err)
	}
	defer source.Close()
	
	ethdb, err := badgerdb.New(ethdbPath, nil, "", prometheus.DefaultRegisterer)
	if err != nil {
		log.Fatal(err)
	}
	
	namespace := []byte{
		0x33, 0x7f, 0xb7, 0x3f, 0x9b, 0xcd, 0xac, 0x8c, 0x31,
		0xa2, 0xd5, 0xf7, 0xb8, 0x77, 0xab, 0x1e, 0x8a, 0x2b,
		0x7f, 0x2a, 0x1e, 0x9b, 0xf0, 0x2a, 0x0a, 0x0e, 0x6c,
		0x6f, 0xd1, 0x64, 0xf1, 0xd1,
	}
	
	// Stats
	start := time.Now()
	count := 0
	headers := 0
	bodies := 0
	receipts := 0
	state := 0
	canonical := 0
	maxBlock := uint64(0)
	var tipHash []byte
	
	// Migrate ALL data
	fmt.Println("Migrating all data...")
	batch := ethdb.NewBatch()
	batchSize := 0
	
	iter, _ := source.NewIter(nil)
	defer iter.Close()
	
	for iter.First(); iter.Valid(); iter.Next() {
		key := iter.Key()
		value, err := iter.ValueAndErr()
		if err != nil {
			continue
		}
		
		// Strip namespace if present
		actualKey := key
		if bytes.HasPrefix(key, namespace) && len(key) > 32 {
			actualKey = key[32:]
		}
		
		// Skip empty keys
		if len(actualKey) == 0 {
			continue
		}
		
		// Write to destination
		batch.Put(actualKey, value)
		count++
		batchSize++
		
		// Track what we're migrating
		if len(actualKey) == 41 && actualKey[0] == 'h' {
			headers++
			num := binary.BigEndian.Uint64(actualKey[1:9])
			if num > maxBlock {
				maxBlock = num
				tipHash = actualKey[9:41]
			}
		} else if len(actualKey) == 41 && actualKey[0] == 'b' {
			bodies++
		} else if len(actualKey) == 41 && actualKey[0] == 'r' {
			receipts++
		} else if len(actualKey) == 10 && actualKey[0] == 'h' && actualKey[9] == 'n' {
			canonical++
			num := binary.BigEndian.Uint64(actualKey[1:9])
			if num == 1082780 {
				tipHash = value
			}
		} else if len(actualKey) == 32 {
			state++
		}
		
		// Write batch periodically
		if batchSize >= 10000 {
			batch.Write()
			batch.Reset()
			batchSize = 0
			
			if count%100000 == 0 {
				elapsed := time.Since(start)
				rate := float64(count) / elapsed.Seconds()
				fmt.Printf("Progress: %d keys (%.0f/sec) - %d headers, %d bodies, %d state\n",
					count, rate, headers, bodies, state)
			}
		}
	}
	
	// Write final batch
	if batchSize > 0 {
		batch.Write()
	}
	
	fmt.Printf("\nMigrated %d total keys\n", count)
	fmt.Printf("  Headers:   %d\n", headers)
	fmt.Printf("  Bodies:    %d\n", bodies)
	fmt.Printf("  Receipts:  %d\n", receipts)
	fmt.Printf("  Canonical: %d\n", canonical)
	fmt.Printf("  State:     %d\n", state)
	fmt.Printf("  Max block: %d\n", maxBlock)
	
	// Add Total Difficulty for all blocks
	fmt.Println("\nAdding Total Difficulty...")
	for i := uint64(0); i <= 1082780; i++ {
		// Get canonical hash
		canonKey := make([]byte, 10)
		canonKey[0] = 'h'
		binary.BigEndian.PutUint64(canonKey[1:9], i)
		canonKey[9] = 'n'
		
		hash, err := ethdb.Get(canonKey)
		if err != nil || len(hash) != 32 {
			continue
		}
		
		// Write TD
		td := big.NewInt(int64(i + 1))
		tdKey := make([]byte, 42)
		tdKey[0] = 'h'
		binary.BigEndian.PutUint64(tdKey[1:9], i)
		copy(tdKey[9:41], hash)
		tdKey[41] = 't'
		
		ethdb.Put(tdKey, td.Bytes())
		
		if i%100000 == 0 || i == 1082780 {
			fmt.Printf("  TD at %d: %s\n", i, td.String())
		}
	}
	
	// Set heads
	fmt.Println("\nSetting head pointers...")
	if len(tipHash) == 32 {
		ethdb.Put([]byte("LastHeader"), tipHash)
		ethdb.Put([]byte("LastBlock"), tipHash)
		ethdb.Put([]byte("LastFast"), tipHash)
		fmt.Printf("Heads set to: %x\n", tipHash)
	}
	
	ethdb.Close()
	
	// Write VM metadata
	fmt.Println("\nWriting VM metadata...")
	vmdb, err := badgerdb.New(vmdbPath, nil, "", prometheus.DefaultRegisterer)
	if err != nil {
		log.Fatal(err)
	}
	
	vmdb.Put([]byte("lastAccepted"), tipHash)
	heightBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(heightBytes, 1082780)
	vmdb.Put([]byte("lastAcceptedHeight"), heightBytes)
	vmdb.Put([]byte("initialized"), []byte{1})
	vmdb.Close()
	
	elapsed := time.Since(start)
	fmt.Printf("\n✅ Migration complete in %v\n", elapsed)
	fmt.Printf("Database size: %s\n", ethdbPath)
}