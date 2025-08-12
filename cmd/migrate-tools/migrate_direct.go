package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"log"
	"math/big"
	"path/filepath"
	"time"

	"github.com/cockroachdb/pebble"
	"github.com/luxfi/database/badgerdb"
	"github.com/prometheus/client_golang/prometheus"
)

func main() {
	fmt.Println("=== Direct Migration Tool ===")
	fmt.Println("This will migrate all data from SubnetEVM to Coreth format")
	
	sourceDB := "/home/z/work/lux/state/chaindata/lux-mainnet-96369/db/pebbledb"
	destDir := "/home/z/.luxd"
	chainID := "X6CU5qgMJfzsTB9UWxj2ZY5hd68x41HfZ4m4hCBWbHuj1Ebc3"
	
	ethdbPath := filepath.Join(destDir, "network-96369", "chains", chainID, "ethdb")
	vmdbPath := filepath.Join(destDir, "network-96369", "chains", chainID, "vm")
	
	// Open source
	source, err := pebble.Open(sourceDB, &pebble.Options{ReadOnly: true})
	if err != nil {
		log.Fatal(err)
	}
	defer source.Close()
	
	// Clean destination for fresh migration
	fmt.Println("Cleaning destination for fresh migration...")
	ethdb, err := badgerdb.New(ethdbPath, nil, "", prometheus.DefaultRegisterer)
	if err != nil {
		log.Fatal(err)
	}
	defer ethdb.Close()
	
	namespace := []byte{
		0x33, 0x7f, 0xb7, 0x3f, 0x9b, 0xcd, 0xac, 0x8c, 0x31,
		0xa2, 0xd5, 0xf7, 0xb8, 0x77, 0xab, 0x1e, 0x8a, 0x2b,
		0x7f, 0x2a, 0x1e, 0x9b, 0xf0, 0x2a, 0x0a, 0x0e, 0x6c,
		0x6f, 0xd1, 0x64, 0xf1, 0xd1,
	}
	
	// Stats
	start := time.Now()
	count := 0
	stateNodes := 0
	headers := 0
	bodies := 0
	receipts := 0
	canonical := 0
	hashToNum := 0
	other := 0
	maxBlock := uint64(0)
	var tipHash []byte
	
	fmt.Println("Starting full migration...")
	batch := ethdb.NewBatch()
	batchSize := 0
	
	iter, _ := source.NewIter(nil)
	defer iter.Close()
	
	// Track canonical blocks for TD computation
	canonicalBlocks := make(map[uint64][]byte)
	
	for iter.First(); iter.Valid(); iter.Next() {
		key := iter.Key()
		value, err := iter.ValueAndErr()
		if err != nil {
			continue
		}
		
		// Skip non-namespaced keys
		if !bytes.HasPrefix(key, namespace) {
			continue
		}
		
		actualKey := key[32:]
		if len(actualKey) == 0 {
			continue
		}
		
		// Process all keys
		batch.Put(actualKey, value)
		count++
		batchSize++
		
		// Categorize keys
		keyLen := len(key)
		switch keyLen {
		case 64: // State nodes (32 namespace + 32 hash)
			stateNodes++
			
		case 73: // Block data (32 namespace + 41 key)
			prefix := actualKey[0]
			num := binary.BigEndian.Uint64(actualKey[1:9])
			hash := actualKey[9:41]
			
			switch prefix {
			case 'h': // header
				headers++
				if num > maxBlock {
					maxBlock = num
				}
				// Track for canonical chain
				if num <= 1082780 {
					canonicalBlocks[num] = hash
					if num == 1082780 {
						tipHash = hash
					}
				}
			case 'b': // body
				bodies++
			case 'r': // receipt
				receipts++
			}
			
		case 42: // Canonical mappings (32 namespace + 10 key)
			if actualKey[0] == 'h' && actualKey[9] == 'n' {
				canonical++
				num := binary.BigEndian.Uint64(actualKey[1:9])
				if num == 1082780 && len(value) == 32 {
					tipHash = value
				}
			}
			
		case 65: // Hash to number mappings (32 namespace + 33 key)
			if actualKey[0] == 'H' {
				hashToNum++
			}
			
		default:
			other++
		}
		
		// Write batch periodically
		if batchSize >= 10000 {
			batch.Write()
			batch.Reset()
			batchSize = 0
			
			if count%100000 == 0 {
				elapsed := time.Since(start)
				rate := float64(count) / elapsed.Seconds()
				fmt.Printf("Progress: %d keys (%.0f/sec)\n", count, rate)
				fmt.Printf("  State: %d, Headers: %d, Bodies: %d, Receipts: %d\n",
					stateNodes, headers, bodies, receipts)
			}
		}
	}
	
	// Write final batch
	if batchSize > 0 {
		batch.Write()
	}
	
	fmt.Printf("\n=== Migration Statistics ===\n")
	fmt.Printf("Total keys:     %d\n", count)
	fmt.Printf("State nodes:    %d\n", stateNodes)
	fmt.Printf("Headers:        %d\n", headers)
	fmt.Printf("Bodies:         %d\n", bodies)
	fmt.Printf("Receipts:       %d\n", receipts)
	fmt.Printf("Canonical:      %d\n", canonical)
	fmt.Printf("Hash->Num:      %d\n", hashToNum)
	fmt.Printf("Other:          %d\n", other)
	fmt.Printf("Max block:      %d\n", maxBlock)
	
	// Now add Total Difficulty for all canonical blocks
	fmt.Println("\nAdding Total Difficulty...")
	tdCount := 0
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
		
		// Write TD with correct format: h + num(8) + hash(32) + 't'
		tdKey := make([]byte, 42)
		tdKey[0] = 'h'
		binary.BigEndian.PutUint64(tdKey[1:9], i)
		copy(tdKey[9:41], hash)
		tdKey[41] = 't'
		
		// TD = height + 1 for C-Chain
		tdValue := big.NewInt(int64(i + 1))
		ethdb.Put(tdKey, tdValue.Bytes())
		tdCount++
		
		if i%100000 == 0 || i == 1082780 {
			fmt.Printf("  TD at block %d\n", i)
		}
	}
	fmt.Printf("Added %d TD entries\n", tdCount)
	
	// Set head pointers
	fmt.Println("\nSetting head pointers...")
	if len(tipHash) == 32 {
		ethdb.Put([]byte("LastHeader"), tipHash)
		ethdb.Put([]byte("LastBlock"), tipHash)
		ethdb.Put([]byte("LastFast"), tipHash)
		ethdb.Put([]byte("LastFinalized"), tipHash)
		fmt.Printf("Heads set to: %x\n", tipHash)
	} else {
		fmt.Println("WARNING: Could not determine tip hash!")
	}
	
	// Write chain config with correct chain ID
	fmt.Println("\nWriting chain config...")
	genesisCanonKey := make([]byte, 10)
	genesisCanonKey[0] = 'h'
	binary.BigEndian.PutUint64(genesisCanonKey[1:9], 0)
	genesisCanonKey[9] = 'n'
	
	if genesisHash, err := ethdb.Get(genesisCanonKey); err == nil && len(genesisHash) == 32 {
		configKey := append([]byte("ethereum-config-"), genesisHash...)
		config := []byte(`{"chainId":96369}`)
		ethdb.Put(configKey, config)
		fmt.Printf("Chain config written for genesis: %x\n", genesisHash)
	}
	
	// Update VM metadata
	fmt.Println("\nUpdating VM metadata...")
	vmdb, err := badgerdb.New(vmdbPath, nil, "", prometheus.DefaultRegisterer)
	if err == nil {
		vmdb.Put([]byte("lastAccepted"), tipHash)
		heightBytes := make([]byte, 8)
		binary.BigEndian.PutUint64(heightBytes, 1082780)
		vmdb.Put([]byte("lastAcceptedHeight"), heightBytes)
		vmdb.Put([]byte("initialized"), []byte{1})
		vmdb.Close()
		fmt.Println("VM metadata updated")
	}
	
	elapsed := time.Since(start)
	fmt.Printf("\n✅ Migration completed in %v\n", elapsed)
	fmt.Printf("Processing rate: %.0f keys/sec\n", float64(count)/elapsed.Seconds())
	fmt.Println("\nDatabase ready at:", ethdbPath)
}