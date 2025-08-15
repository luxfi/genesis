package main

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"time"
	
	"github.com/cockroachdb/pebble"
	"github.com/luxfi/database/badgerdb"
	"github.com/luxfi/geth/common"
)

const subnetNamespace = "\x33\x7f\xb7\x3f\x9b\xcd\xac\x8c\x31\xa2\xd5\xf7\xb8\x77\xab\x1e\x8a\x2b\x7f\x2a\x1e\x9b\xf0\x2a\x0a\x0e\x6c\x6f\xd1\x64\xf1\xd1"

func main() {
	srcPath := os.Args[1]
	dstPath := os.Args[2]
	
	fmt.Printf("Source: %s\n", srcPath)
	fmt.Printf("Dest: %s\n", dstPath)
	
	// Open source
	srcDB, err := pebble.Open(srcPath, &pebble.Options{ReadOnly: true})
	if err != nil {
		panic(err)
	}
	defer srcDB.Close()
	
	// Create dest
	dstDB, err := badgerdb.New(filepath.Clean(dstPath), nil, "", nil)
	if err != nil {
		panic(err)
	}
	defer dstDB.Close()
	
	// Stats
	var count, bytes int64
	start := time.Now()
	
	// Iterate
	iter, err := srcDB.NewIter(nil)
	if err != nil {
		panic(err)
	}
	defer iter.Close()
	
	// Small batches for BadgerDB
	batch := dstDB.NewBatch()
	batchSize := 0
	maxBatch := 100 * 1024 // 100KB batches
	
	for iter.First(); iter.Valid(); iter.Next() {
		key := iter.Key()
		value := iter.Value()
		
		if len(key) == 0 || len(value) == 0 {
			continue
		}
		
		// Remove namespace if present
		destKey := key
		if len(key) > 32 && string(key[:32]) == subnetNamespace {
			destKey = key[32:]
		}
		
		// Copy to avoid reuse
		k := make([]byte, len(destKey))
		v := make([]byte, len(value))
		copy(k, destKey)
		copy(v, value)
		
		batch.Put(k, v)
		batchSize += len(k) + len(v)
		bytes += int64(len(k) + len(v))
		count++
		
		// Write batch
		if batchSize >= maxBatch {
			if err := batch.Write(); err != nil {
				fmt.Printf("Batch error at key %d: %v\n", count, err)
				// Try single write
				if err := dstDB.Put(k, v); err != nil {
					panic(err)
				}
			}
			batch = dstDB.NewBatch()
			batchSize = 0
		}
		
		// Progress
		if count%10000 == 0 {
			elapsed := time.Since(start)
			rate := float64(count) / elapsed.Seconds()
			fmt.Printf("\r%d keys, %.2f MB, %.0f keys/sec", count, float64(bytes)/(1024*1024), rate)
		}
	}
	
	// Final batch
	if batchSize > 0 {
		batch.Write()
	}
	
	fmt.Printf("\n✓ Migrated %d keys, %.2f GB in %s\n", count, float64(bytes)/(1024*1024*1024), time.Since(start))
	
	// Set metadata
	dstDB.Put([]byte("LastHeader"), common.HexToHash("0x32dede1fc8e0f11ecde12fb42aef7933fc6c5fcf863bc277b5eac08ae4d461f0").Bytes())
	dstDB.Put([]byte("LastBlock"), common.HexToHash("0x32dede1fc8e0f11ecde12fb42aef7933fc6c5fcf863bc277b5eac08ae4d461f0").Bytes())
	dstDB.Put([]byte("LastFast"), common.HexToHash("0x32dede1fc8e0f11ecde12fb42aef7933fc6c5fcf863bc277b5eac08ae4d461f0").Bytes())
	
	// Check genesis
	canonKey := make([]byte, 10)
	canonKey[0] = 'h'
	binary.BigEndian.PutUint64(canonKey[1:9], 0)
	canonKey[9] = 'n'
	
	if val, err := dstDB.Get(canonKey); err == nil {
		var hash common.Hash
		copy(hash[:], val)
		fmt.Printf("Genesis: %s\n", hash.Hex())
	}
}
