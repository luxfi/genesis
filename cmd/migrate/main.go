package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"log"
	"math/big"
	"os"
	"os/signal"
	"path/filepath"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/cockroachdb/pebble"
	"github.com/luxfi/database"
	"github.com/luxfi/database/badgerdb"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	namespaceLen = 32
	batchSize    = 1000 // Smaller batches for stability
)

type Stats struct {
	Total           uint64
	Written         uint64
	Headers         uint64
	Bodies          uint64
	Receipts        uint64
	TotalDifficulty uint64
	Canonical       uint64
	StateNodes      uint64
	Code            uint64
	Other           uint64
	Skipped         uint64
	MaxBlockNum     uint64
	LastBlockHash   []byte
}

var (
	running = int32(1)
	stats   = &Stats{}
)

func main() {
	// Set up signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("\nGracefully shutting down...")
		atomic.StoreInt32(&running, 0)
	}()

	// Paths
	sourceDB := "/home/z/work/lux/state/chaindata/lux-mainnet-96369/db/pebbledb"
	homeDir := os.Getenv("HOME")
	destDir := filepath.Join(homeDir, ".luxd")
	
	// Network config
	networkID := uint32(96369)
	chainID := "X6CU5qgMJfzsTB9UWxj2ZY5hd68x41HfZ4m4hCBWbHuj1Ebc3"
	namespace := []byte{
		0x33, 0x7f, 0xb7, 0x3f, 0x9b, 0xcd, 0xac, 0x8c, 0x31,
		0xa2, 0xd5, 0xf7, 0xb8, 0x77, 0xab, 0x1e, 0x8a, 0x2b,
		0x7f, 0x2a, 0x1e, 0x9b, 0xf0, 0x2a, 0x0a, 0x0e, 0x6c,
		0x6f, 0xd1, 0x64, 0xf1, 0xd1,
	}

	fmt.Println("=== LUX Mainnet Migration ===")
	fmt.Printf("Source: %s\n", sourceDB)
	fmt.Printf("Destination: %s\n", destDir)
	
	// Open source PebbleDB
	source, err := pebble.Open(sourceDB, &pebble.Options{ReadOnly: true})
	if err != nil {
		log.Fatalf("Failed to open source: %v", err)
	}
	defer source.Close()

	// Create destination paths
	basePath := filepath.Join(destDir, fmt.Sprintf("network-%d", networkID), "chains", chainID)
	ethdbPath := filepath.Join(basePath, "ethdb")
	vmPath := filepath.Join(basePath, "vm")
	
	os.MkdirAll(ethdbPath, 0755)
	os.MkdirAll(vmPath, 0755)

	// Open destination databases with optimized settings
	ethdb, err := badgerdb.New(ethdbPath, nil, "", prometheus.DefaultRegisterer)
	if err != nil {
		log.Fatalf("Failed to create ethdb: %v", err)
	}
	defer ethdb.Close()
	
	vmdb, err := badgerdb.New(vmPath, nil, "", prometheus.DefaultRegisterer)
	if err != nil {
		log.Fatalf("Failed to create vmdb: %v", err)
	}
	defer vmdb.Close()

	// Start monitoring goroutine
	go monitor()

	// Run migration
	if err := migrate(source, ethdb, namespace); err != nil {
		log.Fatalf("Migration failed: %v", err)
	}

	// Write VM metadata
	writeVMMetadata(vmdb)
	
	// Copy staking keys
	setupStakingKeys(destDir)
	
	printSummary()
	fmt.Println("\n✅ Migration complete!")
}

func migrate(source *pebble.DB, dest *badgerdb.Database, namespace []byte) error {
	blockHashes := make(map[uint64][]byte)
	canonicalBlocks := make(map[uint64][]byte)
	
	it, err := source.NewIter(nil)
	if err != nil {
		return err
	}
	defer it.Close()

	batch := dest.NewBatch()
	batchCount := 0
	
	fmt.Println("\nPhase 1: Migrating data...")
	
	for it.First(); it.Valid() && atomic.LoadInt32(&running) == 1; it.Next() {
		atomic.AddUint64(&stats.Total, 1)
		
		key := it.Key()
		value, err := it.ValueAndErr()
		if err != nil {
			continue
		}

		// Strip namespace
		actualKey := key
		if len(namespace) > 0 && len(key) >= namespaceLen {
			if bytes.Equal(key[:namespaceLen], namespace) {
				actualKey = key[namespaceLen:]
			}
		}

		if len(actualKey) == 0 {
			atomic.AddUint64(&stats.Skipped, 1)
			continue
		}

		// Process different key types
		processKey(actualKey, value, batch, blockHashes, canonicalBlocks)
		
		batchCount++
		if batchCount >= batchSize {
			if err := batch.Write(); err != nil {
				return fmt.Errorf("batch write failed: %v", err)
			}
			batch.Reset()
			batchCount = 0
		}
	}

	// Write final batch
	if batchCount > 0 {
		if err := batch.Write(); err != nil {
			return fmt.Errorf("final batch write failed: %v", err)
		}
	}

	if atomic.LoadInt32(&running) == 0 {
		return fmt.Errorf("migration interrupted")
	}

	// Merge maps
	for num, hash := range blockHashes {
		if _, exists := canonicalBlocks[num]; !exists {
			canonicalBlocks[num] = hash
		}
	}

	fmt.Printf("\nPhase 2: Computing Total Difficulty for %d blocks...\n", len(canonicalBlocks))
	computeTotalDifficulty(dest, canonicalBlocks)
	
	// Set head pointers
	if stats.LastBlockHash != nil {
		fmt.Printf("\nPhase 3: Setting head pointers to block %d\n", stats.MaxBlockNum)
		dest.Put([]byte("LastHeader"), stats.LastBlockHash)
		dest.Put([]byte("LastBlock"), stats.LastBlockHash)
		dest.Put([]byte("LastFast"), stats.LastBlockHash)
	}

	return nil
}

func processKey(key []byte, value []byte, batch database.Batch, blockHashes, canonicalBlocks map[uint64][]byte) {
	if len(key) == 0 {
		return
	}

	atomic.AddUint64(&stats.Written, 1)
	
	switch {
	// Canonical format 1: H<num> -> hash
	case key[0] == 'H' && len(key) == 9 && len(value) == 32:
		num := binary.BigEndian.Uint64(key[1:9])
		canonicalBlocks[num] = value
		
		// Convert to Coreth format
		newKey := make([]byte, 10)
		newKey[0] = 'h'
		copy(newKey[1:9], key[1:9])
		newKey[9] = 'n'
		batch.Put(newKey, value)
		
		// Hash to number mapping
		hashKey := make([]byte, 33)
		hashKey[0] = 'H'
		copy(hashKey[1:], value)
		batch.Put(hashKey, key[1:9])
		
		atomic.AddUint64(&stats.Written, 1)
		atomic.AddUint64(&stats.Canonical, 1)
		
		if num > stats.MaxBlockNum {
			stats.MaxBlockNum = num
			stats.LastBlockHash = value
		}
		
	// Canonical format 2: h<num>n -> hash
	case key[0] == 'h' && len(key) == 10 && key[9] == 'n' && len(value) == 32:
		num := binary.BigEndian.Uint64(key[1:9])
		canonicalBlocks[num] = value
		batch.Put(key, value)
		
		// Hash to number mapping
		hashKey := make([]byte, 33)
		hashKey[0] = 'H'
		copy(hashKey[1:], value)
		batch.Put(hashKey, key[1:9])
		
		atomic.AddUint64(&stats.Written, 1)
		atomic.AddUint64(&stats.Canonical, 1)
		
		if num > stats.MaxBlockNum {
			stats.MaxBlockNum = num
			stats.LastBlockHash = value
		}
		
	// Headers: h<num><hash> -> header
	case key[0] == 'h' && len(key) == 41:
		num := binary.BigEndian.Uint64(key[1:9])
		hash := key[9:41]
		blockHashes[num] = hash
		batch.Put(key, value)
		atomic.AddUint64(&stats.Headers, 1)
		
		if num > stats.MaxBlockNum {
			stats.MaxBlockNum = num
			stats.LastBlockHash = hash
		}
		
	// Bodies: b<num><hash> -> body
	case key[0] == 'b' && len(key) == 41:
		batch.Put(key, value)
		atomic.AddUint64(&stats.Bodies, 1)
		
	// Receipts: r<num><hash> -> receipts
	case key[0] == 'r' && len(key) == 41:
		batch.Put(key, value)
		atomic.AddUint64(&stats.Receipts, 1)
		
	// Code: c<hash> -> code
	case key[0] == 'c' && len(key) == 33:
		batch.Put(key, value)
		atomic.AddUint64(&stats.Code, 1)
		
	// State nodes (32-byte keys)
	case len(key) == 32:
		batch.Put(key, value)
		atomic.AddUint64(&stats.StateNodes, 1)
		
	// Everything else
	default:
		batch.Put(key, value)
		atomic.AddUint64(&stats.Other, 1)
	}
}

func computeTotalDifficulty(db *badgerdb.Database, blockHashes map[uint64][]byte) {
	td := big.NewInt(0)
	maxBlock := uint64(0)
	
	for num := range blockHashes {
		if num > maxBlock {
			maxBlock = num
		}
	}
	
	for height := uint64(0); height <= maxBlock; height++ {
		td.Add(td, big.NewInt(1))
		
		hash, exists := blockHashes[height]
		if !exists {
			continue
		}
		
		// Write TD: t + num(8) + hash(32) -> TD
		tdKey := make([]byte, 41)
		tdKey[0] = 't'
		binary.BigEndian.PutUint64(tdKey[1:9], height)
		copy(tdKey[9:41], hash)
		
		db.Put(tdKey, td.Bytes())
		atomic.AddUint64(&stats.TotalDifficulty, 1)
		
		if height%10000 == 0 || height == maxBlock {
			fmt.Printf("  TD at block %d: %s\n", height, td.String())
		}
	}
	
	fmt.Printf("✅ TD chain complete. Final TD: %s\n", td.String())
}

func writeVMMetadata(vmdb *badgerdb.Database) {
	fmt.Println("\nWriting VM metadata...")
	
	if stats.LastBlockHash != nil {
		vmdb.Put([]byte("lastAccepted"), stats.LastBlockHash)
		
		heightBytes := make([]byte, 8)
		binary.BigEndian.PutUint64(heightBytes, stats.MaxBlockNum)
		vmdb.Put([]byte("lastAcceptedHeight"), heightBytes)
		
		vmdb.Put([]byte("initialized"), []byte{1})
		
		fmt.Printf("✅ VM metadata written (height: %d)\n", stats.MaxBlockNum)
	}
}

func setupStakingKeys(destDir string) {
	stakingDir := filepath.Join(destDir, "staking")
	os.MkdirAll(stakingDir, 0700)
	
	srcStaking := "/home/z/work/lux/staking-keys/node1/staking"
	files := []string{"staker.key", "staker.crt", "signer.key"}
	
	for _, file := range files {
		src := filepath.Join(srcStaking, file)
		dst := filepath.Join(stakingDir, file)
		if input, err := os.ReadFile(src); err == nil {
			os.WriteFile(dst, input, 0600)
		}
	}
	
	fmt.Printf("✅ Staking keys copied to %s\n", stakingDir)
}

func monitor() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	
	startTime := time.Now()
	
	for atomic.LoadInt32(&running) == 1 {
		select {
		case <-ticker.C:
			total := atomic.LoadUint64(&stats.Total)
			written := atomic.LoadUint64(&stats.Written)
			elapsed := time.Since(startTime)
			rate := float64(total) / elapsed.Seconds()
			
			fmt.Printf("Progress: %d total, %d written (%.0f keys/sec)\n", total, written, rate)
		}
	}
}

func printSummary() {
	fmt.Println("\n=== Migration Summary ===")
	fmt.Printf("Total keys: %d\n", stats.Total)
	fmt.Printf("Written: %d\n", stats.Written)
	fmt.Printf("Skipped: %d\n", stats.Skipped)
	fmt.Printf("\nData categories:\n")
	fmt.Printf("  Headers: %d\n", stats.Headers)
	fmt.Printf("  Bodies: %d\n", stats.Bodies)
	fmt.Printf("  Receipts: %d\n", stats.Receipts)
	fmt.Printf("  TD entries: %d\n", stats.TotalDifficulty)
	fmt.Printf("  Canonical: %d\n", stats.Canonical)
	fmt.Printf("  State: %d\n", stats.StateNodes)
	fmt.Printf("  Code: %d\n", stats.Code)
	fmt.Printf("  Other: %d\n", stats.Other)
	fmt.Printf("\nMax block: %d\n", stats.MaxBlockNum)
	if stats.LastBlockHash != nil {
		fmt.Printf("Tip hash: 0x%x\n", stats.LastBlockHash)
	}
}