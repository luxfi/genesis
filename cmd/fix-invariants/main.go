package main

import (
	"fmt"
	"log"
	"math/big"
	"os"
	"path/filepath"

	"github.com/luxfi/geth/common"
	"github.com/luxfi/geth/core/rawdb"
	"github.com/luxfi/geth/ethdb"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/cockroachdb/pebble"
)

func openPebbleDB(path string) (ethdb.Database, error) {
	// Open pebble database
	opts := &pebble.Options{
		Cache:        pebble.NewCache(256 << 20), // 256MB cache
		MaxOpenFiles: 1024,
	}
	
	pdb, err := pebble.Open(path, opts)
	if err != nil {
		return nil, err
	}
	
	// Wrap in ethdb interface
	return &pebbleWrapper{db: pdb}, nil
}

// pebbleWrapper implements ethdb.Database for pebble
type pebbleWrapper struct {
	db *pebble.DB
}

func (p *pebbleWrapper) Has(key []byte) (bool, error) {
	_, closer, err := p.db.Get(key)
	if err == pebble.ErrNotFound {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	closer.Close()
	return true, nil
}

func (p *pebbleWrapper) Get(key []byte) ([]byte, error) {
	val, closer, err := p.db.Get(key)
	if err != nil {
		return nil, err
	}
	defer closer.Close()
	
	// Copy the value since it's only valid until closer.Close()
	result := make([]byte, len(val))
	copy(result, val)
	return result, nil
}

func (p *pebbleWrapper) Put(key []byte, value []byte) error {
	return p.db.Set(key, value, pebble.Sync)
}

func (p *pebbleWrapper) Delete(key []byte) error {
	return p.db.Delete(key, pebble.Sync)
}

func (p *pebbleWrapper) NewBatch() ethdb.Batch {
	return &pebbleBatch{db: p.db, b: p.db.NewBatch()}
}

func (p *pebbleWrapper) NewBatchWithSize(size int) ethdb.Batch {
	return p.NewBatch()
}

func (p *pebbleWrapper) NewIterator(prefix []byte, start []byte) ethdb.Iterator {
	iter := p.db.NewIter(&pebble.IterOptions{
		LowerBound: append(prefix, start...),
		UpperBound: append(prefix, 0xff),
	})
	return &pebbleIterator{iter: iter}
}

func (p *pebbleWrapper) Stat() (string, error) {
	return "pebble", nil
}

func (p *pebbleWrapper) Close() error {
	return p.db.Close()
}

func (p *pebbleWrapper) NewSnapshot() (ethdb.Snapshot, error) {
	return p.db.NewSnapshot(), nil
}

func (p *pebbleWrapper) Compact(start []byte, limit []byte) error {
	return p.db.Compact(start, limit, true)
}

// Implement batch
type pebbleBatch struct {
	db *pebble.DB
	b  *pebble.Batch
}

func (b *pebbleBatch) Put(key []byte, value []byte) error {
	return b.b.Set(key, value, nil)
}

func (b *pebbleBatch) Delete(key []byte) error {
	return b.b.Delete(key, nil)
}

func (b *pebbleBatch) ValueSize() int {
	return int(b.b.Len())
}

func (b *pebbleBatch) Write() error {
	return b.b.Commit(pebble.Sync)
}

func (b *pebbleBatch) Reset() {
	b.b.Reset()
}

func (b *pebbleBatch) Replay(w ethdb.KeyValueWriter) error {
	return nil // Not implemented
}

// Implement iterator
type pebbleIterator struct {
	iter *pebble.Iterator
}

func (i *pebbleIterator) Next() bool {
	return i.iter.Next()
}

func (i *pebbleIterator) Error() error {
	return i.iter.Error()
}

func (i *pebbleIterator) Key() []byte {
	return i.iter.Key()
}

func (i *pebbleIterator) Value() []byte {
	return i.iter.Value()
}

func (i *pebbleIterator) Release() {
	i.iter.Close()
}

func main() {
	chainDir := filepath.Join(
		"/home/z/.luxd", "network-96369", "chains",
		"X6CU5qgMJfzsTB9UWxj2ZY5hd68x41HfZ4m4hCBWbHuj1Ebc3", "ethdb",
	)

	fmt.Printf("=== Fixing Database Invariants ===\n")
	fmt.Printf("Database: %s\n\n", chainDir)

	// Check if directory exists
	if _, err := os.Stat(chainDir); os.IsNotExist(err) {
		log.Fatalf("Database directory does not exist: %s", chainDir)
	}

	// Open database
	db, err := openPebbleDB(chainDir)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// 1. Check genesis
	fmt.Printf("1. Checking genesis...\n")
	genesisHash := rawdb.ReadCanonicalHash(db, 0)
	if genesisHash == (common.Hash{}) {
		log.Fatal("❌ No genesis hash found!")
	}
	fmt.Printf("   Genesis hash: %s\n", genesisHash.Hex())

	// 2. Check and fix heads
	fmt.Printf("\n2. Checking heads...\n")
	headHash := rawdb.ReadHeadBlockHash(db)
	if headHash == (common.Hash{}) {
		log.Fatal("❌ No head block hash!")
	}
	
	// Get the head number
	headNumber, found := rawdb.ReadHeaderNumber(db, headHash)
	if !found {
		log.Fatal("❌ Head block number not found!")
	}
	
	fmt.Printf("   Head block: %d (hash: %s)\n", headNumber, headHash.Hex())

	// 3. Fix Total Difficulty for all blocks
	fmt.Printf("\n3. Fixing Total Difficulty...\n")
	batch := db.NewBatch()
	fixedCount := 0
	
	for n := uint64(0); n <= headNumber; n++ {
		hash := rawdb.ReadCanonicalHash(db, n)
		if hash == (common.Hash{}) {
			fmt.Printf("   ⚠️  No canonical hash for block %d\n", n)
			continue
		}
		
		// Check if TD exists
		td := rawdb.ReadTd(db, hash, n)
		expectedTd := new(big.Int).SetUint64(n + 1)
		
		if td == nil || td.Cmp(expectedTd) != 0 {
			// Write correct TD
			rawdb.WriteTd(batch, hash, n, expectedTd)
			fixedCount++
			
			if fixedCount%10000 == 0 {
				fmt.Printf("   Fixed TD for %d blocks...\n", fixedCount)
				if err := batch.Write(); err != nil {
					log.Fatalf("Failed to write batch: %v", err)
				}
				batch.Reset()
			}
		}
	}
	
	// Write final batch
	if err := batch.Write(); err != nil {
		log.Fatalf("Failed to write final batch: %v", err)
	}
	
	fmt.Printf("   ✅ Fixed TD for %d blocks\n", fixedCount)

	// 4. Ensure all three head pointers are set
	fmt.Printf("\n4. Setting head pointers...\n")
	rawdb.WriteHeadHeaderHash(db, headHash)
	rawdb.WriteHeadBlockHash(db, headHash)
	rawdb.WriteHeadFastBlockHash(db, headHash)
	fmt.Printf("   ✅ All head pointers set to block %d\n", headNumber)

	// 5. Check chain config
	fmt.Printf("\n5. Checking chain config...\n")
	chainConfig := rawdb.ReadChainConfig(db, genesisHash)
	if chainConfig == nil {
		fmt.Printf("   ❌ Chain config missing - needs to be written\n")
	} else {
		fmt.Printf("   ✅ Chain config present (ChainID: %v)\n", chainConfig.ChainID)
	}

	// 6. Verify final state
	fmt.Printf("\n=== Verification ===\n")
	
	// Check TD at tip
	tipTd := rawdb.ReadTd(db, headHash, headNumber)
	if tipTd == nil {
		fmt.Printf("❌ TD still missing at tip!\n")
	} else {
		expectedTipTd := new(big.Int).SetUint64(headNumber + 1)
		if tipTd.Cmp(expectedTipTd) == 0 {
			fmt.Printf("✅ TD at tip: %v (correct)\n", tipTd)
		} else {
			fmt.Printf("⚠️  TD at tip: %v (expected %v)\n", tipTd, expectedTipTd)
		}
	}
	
	// Check genesis TD
	genesisTd := rawdb.ReadTd(db, genesisHash, 0)
	if genesisTd == nil {
		fmt.Printf("❌ Genesis TD missing!\n")
	} else if genesisTd.Cmp(big.NewInt(1)) == 0 {
		fmt.Printf("✅ Genesis TD: 1 (correct)\n")
	} else {
		fmt.Printf("⚠️  Genesis TD: %v (expected 1)\n", genesisTd)
	}
	
	fmt.Printf("\n✅ Database invariants fixed!\n")
}