package main

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"log"
	"math/big"
	"os"
	
	"github.com/cockroachdb/pebble"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"
)

const (
	SOURCE_DB = "/home/z/work/lux/state/chaindata/lux-mainnet-96369/db/pebbledb"
	TARGET_DB = "/tmp/lux-c-chain-replay"
)

var namespace = []byte{
	0x33, 0x7f, 0xb7, 0x3f, 0x9b, 0xcd, 0xac, 0x8c,
	0x31, 0xa2, 0xd5, 0xf7, 0xb8, 0x77, 0xab, 0x1e,
	0x8a, 0x2b, 0x7f, 0x2a, 0x1e, 0x9b, 0xf0, 0x2a,
	0x0a, 0x0e, 0x6c, 0x6f, 0xd1, 0x64, 0xf1, 0xd1,
}

type BlockData struct {
	Number uint64
	Hash   common.Hash
	Header []byte
}

func main() {
	fmt.Println("=============================================================")
	fmt.Println("    Direct Block Injection for LUX C-Chain")
	fmt.Println("=============================================================")
	fmt.Println()
	fmt.Println("Source:", SOURCE_DB)
	fmt.Println("Target:", TARGET_DB)
	fmt.Println()
	
	// Remove target if exists
	os.RemoveAll(TARGET_DB)
	
	// Open source database
	srcDB, err := pebble.Open(SOURCE_DB, &pebble.Options{ReadOnly: true})
	if err != nil {
		log.Fatal("Failed to open source database:", err)
	}
	defer srcDB.Close()
	
	// Create target database
	targetDB, err := pebble.Open(TARGET_DB, &pebble.Options{})
	if err != nil {
		log.Fatal("Failed to create target database:", err)
	}
	defer targetDB.Close()
	
	fmt.Println("Phase 1: Extracting blocks from subnet-EVM database...")
	fmt.Println("========================================================")
	
	// Collect all blocks
	blocks := make(map[uint64]*BlockData)
	iter, _ := srcDB.NewIter(&pebble.IterOptions{})
	
	for iter.First(); iter.Valid(); iter.Next() {
		key := make([]byte, len(iter.Key()))
		copy(key, iter.Key())
		value := make([]byte, len(iter.Value()))
		copy(value, iter.Value())
		
		if len(key) == 64 && bytes.Equal(key[:32], namespace) {
			hash := key[32:]
			
			// Check if it's a header (RLP data)
			if len(value) > 100 && (value[0] == 0xf8 || value[0] == 0xf9) {
				// Decode block number from first 3 bytes of hash
				blockNum := uint64(hash[0])<<16 | uint64(hash[1])<<8 | uint64(hash[2])
				
				if blockNum < 1500000 { // Limit to reasonable range
					var h common.Hash
					copy(h[:], hash)
					
					blocks[blockNum] = &BlockData{
						Number: blockNum,
						Hash:   h,
						Header: value,
					}
					
					if len(blocks)%100000 == 0 {
						fmt.Printf("  Extracted %d blocks...\n", len(blocks))
					}
				}
			}
		}
	}
	iter.Close()
	
	fmt.Printf("\n✅ Extracted %d blocks\n\n", len(blocks))
	
	fmt.Println("Phase 2: Creating C-Chain database with proper structure...")
	fmt.Println("===========================================================")
	
	batch := targetDB.NewBatch()
	written := 0
	
	// Write genesis configuration
	genesisConfig := map[string]interface{}{
		"chainId": 96369,
		"homesteadBlock": 0,
		"eip150Block": 0,
		"eip155Block": 0,
		"eip158Block": 0,
		"byzantiumBlock": 0,
		"constantinopleBlock": 0,
		"petersburgBlock": 0,
		"istanbulBlock": 0,
		"berlinBlock": 0,
		"londonBlock": 0,
	}
	
	configBytes, _ := rlp.EncodeToBytes(genesisConfig)
	batch.Set([]byte("chain-config"), configBytes, nil)
	
	// Process blocks in order
	for blockNum := uint64(0); blockNum <= 1082781; blockNum++ {
		block, exists := blocks[blockNum]
		if !exists {
			continue
		}
		
		// Decode the header to get proper structure
		var header types.Header
		if err := rlp.DecodeBytes(block.Header, &header); err != nil {
			// If we can't decode it, at least write the raw data
			// Create canonical mapping: H + blockNum(8) -> hash
			hKey := make([]byte, 9)
			hKey[0] = 'H'
			binary.BigEndian.PutUint64(hKey[1:], blockNum)
			batch.Set(hKey, block.Hash[:], nil)
			
			// Write header: h + hash -> header
			headerKey := append([]byte{'h'}, block.Hash[:]...)
			batch.Set(headerKey, block.Header, nil)
			
			written++
		} else {
			// Properly structured write
			// Canonical hash mapping
			hKey := make([]byte, 9)
			hKey[0] = 'H'
			binary.BigEndian.PutUint64(hKey[1:], blockNum)
			batch.Set(hKey, block.Hash[:], nil)
			
			// Header with block number and hash
			headerKey := make([]byte, 41)
			headerKey[0] = 'h'
			binary.BigEndian.PutUint64(headerKey[1:9], blockNum)
			copy(headerKey[9:], block.Hash[:])
			batch.Set(headerKey, block.Header, nil)
			
			// Block number to hash mapping
			nKey := make([]byte, 9)
			nKey[0] = 'n'
			binary.BigEndian.PutUint64(nKey[1:], blockNum)
			batch.Set(nKey, block.Hash[:], nil)
			
			// TD (total difficulty) - set to 0 for PoA
			tdKey := append([]byte("td"), block.Hash[:]...)
			tdBytes, _ := rlp.EncodeToBytes(big.NewInt(0))
			batch.Set(tdKey, tdBytes, nil)
			
			written++
		}
		
		// Commit batch every 10000 blocks
		if written%10000 == 0 {
			if err := batch.Commit(nil); err != nil {
				log.Fatal("Failed to commit batch:", err)
			}
			batch = targetDB.NewBatch()
			fmt.Printf("  Injected %d blocks...\n", written)
		}
	}
	
	// Final batch commit
	if err := batch.Commit(nil); err != nil {
		log.Fatal("Failed to commit final batch:", err)
	}
	
	// Write head pointers
	if lastBlock, exists := blocks[1082781]; exists {
		batch = targetDB.NewBatch()
		batch.Set([]byte("LastBlock"), lastBlock.Hash[:], nil)
		batch.Set([]byte("head-block"), lastBlock.Hash[:], nil)
		batch.Set([]byte("head-header"), lastBlock.Hash[:], nil)
		batch.Set([]byte("head-fast"), lastBlock.Hash[:], nil)
		
		heightBytes := make([]byte, 8)
		binary.BigEndian.PutUint64(heightBytes, 1082781)
		batch.Set([]byte("Height"), heightBytes, nil)
		
		batch.Commit(nil)
	}
	
	fmt.Printf("\n✅ Successfully injected %d blocks\n", written)
	
	// Verify the injection
	fmt.Println("\nPhase 3: Verifying injection...")
	fmt.Println("================================")
	
	// Check if we can read block 0
	hKey := make([]byte, 9)
	hKey[0] = 'H'
	binary.BigEndian.PutUint64(hKey[1:], 0)
	
	if hash, closer, err := targetDB.Get(hKey); err == nil {
		closer.Close()
		fmt.Printf("✅ Block 0 canonical hash: %s\n", hex.EncodeToString(hash))
		
		// Try to get the header
		headerKey := append([]byte{'h'}, hash...)
		if header, closer, err := targetDB.Get(headerKey); err == nil {
			closer.Close()
			fmt.Printf("✅ Block 0 header found: %d bytes\n", len(header))
		}
	}
	
	// Check highest block
	hKey = make([]byte, 9)
	hKey[0] = 'H'
	binary.BigEndian.PutUint64(hKey[1:], 1082781)
	
	if hash, closer, err := targetDB.Get(hKey); err == nil {
		closer.Close()
		fmt.Printf("✅ Block 1082781 canonical hash: %s\n", hex.EncodeToString(hash))
	}
	
	fmt.Println("\n=============================================================")
	fmt.Println("Injection complete!")
	fmt.Println()
	fmt.Println("To use with luxd:")
	fmt.Printf("  ./build/luxd --db-dir=%s --network-id=96369\n", TARGET_DB)
	fmt.Println()
	fmt.Println("Or start with POA automining:")
	fmt.Println("  ./run-lux-mainnet-automining.sh")
	fmt.Println("=============================================================")
}