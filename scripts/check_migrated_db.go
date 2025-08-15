package main

import (
	"fmt"
	"log"
	
	"github.com/dgraph-io/badger/v4"
	"github.com/luxfi/geth/common"
	"github.com/luxfi/geth/core/rawdb"
	"github.com/luxfi/geth/ethdb"
)

// BadgerDatabase wraps a Badger database
type BadgerDatabase struct {
	db *badger.DB
}

func (b *BadgerDatabase) Has(key []byte) (bool, error) {
	var exists bool
	err := b.db.View(func(txn *badger.Txn) error {
		_, err := txn.Get(key)
		if err == badger.ErrKeyNotFound {
			exists = false
			return nil
		}
		if err != nil {
			return err
		}
		exists = true
		return nil
	})
	return exists, err
}

func (b *BadgerDatabase) Get(key []byte) ([]byte, error) {
	var value []byte
	err := b.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(key)
		if err != nil {
			return err
		}
		value, err = item.ValueCopy(nil)
		return err
	})
	return value, err
}

func (b *BadgerDatabase) Put(key []byte, value []byte) error {
	return b.db.Update(func(txn *badger.Txn) error {
		return txn.Set(key, value)
	})
}

func (b *BadgerDatabase) Delete(key []byte) error {
	return b.db.Update(func(txn *badger.Txn) error {
		return txn.Delete(key)
	})
}

func (b *BadgerDatabase) NewBatch() ethdb.Batch {
	return &badgerBatch{db: b.db, ops: make([]batchOp, 0)}
}

func (b *BadgerDatabase) NewBatchWithSize(size int) ethdb.Batch {
	return &badgerBatch{db: b.db, ops: make([]batchOp, 0, size/100)}
}

func (b *BadgerDatabase) NewIterator(prefix []byte, start []byte) ethdb.Iterator {
	return &badgerIterator{
		db:     b.db,
		prefix: prefix,
		start:  start,
	}
}

func (b *BadgerDatabase) NewSnapshot() (ethdb.Snapshot, error) {
	return b, nil
}

func (b *BadgerDatabase) Stat(property string) (string, error) {
	return "", nil
}

func (b *BadgerDatabase) Compact(start []byte, limit []byte) error {
	return nil
}

func (b *BadgerDatabase) Close() error {
	return b.db.Close()
}

type batchOp struct {
	key    []byte
	value  []byte
	delete bool
}

type badgerBatch struct {
	db   *badger.DB
	ops  []batchOp
	size int
}

func (b *badgerBatch) Put(key, value []byte) error {
	b.ops = append(b.ops, batchOp{key: key, value: value})
	b.size += len(key) + len(value)
	return nil
}

func (b *badgerBatch) Delete(key []byte) error {
	b.ops = append(b.ops, batchOp{key: key, delete: true})
	b.size += len(key)
	return nil
}

func (b *badgerBatch) ValueSize() int {
	return b.size
}

func (b *badgerBatch) Write() error {
	return b.db.Update(func(txn *badger.Txn) error {
		for _, op := range b.ops {
			if op.delete {
				if err := txn.Delete(op.key); err != nil {
					return err
				}
			} else {
				if err := txn.Set(op.key, op.value); err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func (b *badgerBatch) Reset() {
	b.ops = b.ops[:0]
	b.size = 0
}

func (b *badgerBatch) Replay(w ethdb.KeyValueWriter) error {
	for _, op := range b.ops {
		if op.delete {
			if err := w.Delete(op.key); err != nil {
				return err
			}
		} else {
			if err := w.Put(op.key, op.value); err != nil {
				return err
			}
		}
	}
	return nil
}

type badgerIterator struct {
	db     *badger.DB
	prefix []byte
	start  []byte
	it     *badger.Iterator
	txn    *badger.Txn
	key    []byte
	value  []byte
	err    error
}

func (it *badgerIterator) Next() bool {
	if it.it == nil {
		it.txn = it.db.NewTransaction(false)
		opts := badger.DefaultIteratorOptions
		opts.PrefetchSize = 10
		it.it = it.txn.NewIterator(opts)
		
		if it.start != nil {
			it.it.Seek(it.start)
		} else if it.prefix != nil {
			it.it.Seek(it.prefix)
		} else {
			it.it.Rewind()
		}
	} else {
		it.it.Next()
	}
	
	if !it.it.Valid() {
		return false
	}
	
	item := it.it.Item()
	it.key = item.KeyCopy(nil)
	
	if it.prefix != nil && len(it.key) >= len(it.prefix) {
		for i := 0; i < len(it.prefix); i++ {
			if it.key[i] != it.prefix[i] {
				return false
			}
		}
	}
	
	it.value, it.err = item.ValueCopy(nil)
	return it.err == nil
}

func (it *badgerIterator) Error() error {
	return it.err
}

func (it *badgerIterator) Key() []byte {
	return it.key
}

func (it *badgerIterator) Value() []byte {
	return it.value
}

func (it *badgerIterator) Release() {
	if it.it != nil {
		it.it.Close()
		it.it = nil
	}
	if it.txn != nil {
		it.txn.Discard()
		it.txn = nil
	}
}

func main() {
	// Open the ethdb database directly
	ethdbPath := "/Users/z/.luxd/network-96369/chains/EWi9aPkGe6EfJ3SobCAmSUXRPLa4brF3cThwPwmHTrD1y13jy/ethdb"
	
	db, err := badger.Open(badger.DefaultOptions(ethdbPath))
	if err != nil {
		log.Fatal("Failed to open ethdb:", err)
	}
	defer db.Close()
	
	ethDB := &BadgerDatabase{db: db}
	
	// Check for head block
	headHash := rawdb.ReadHeadBlockHash(ethDB)
	fmt.Printf("Head block hash: %s\n", headHash.Hex())
	
	// Check for head header
	headHeaderHash := rawdb.ReadHeadHeaderHash(ethDB)
	fmt.Printf("Head header hash: %s\n", headHeaderHash.Hex())
	
	// Try to read block 0
	block0Hash := rawdb.ReadCanonicalHash(ethDB, 0)
	fmt.Printf("Block 0 hash: %s\n", block0Hash.Hex())
	
	if block0Hash != (common.Hash{}) {
		header := rawdb.ReadHeader(ethDB, block0Hash, 0)
		if header != nil {
			fmt.Printf("Block 0 found! Root: %s, Time: %d\n", header.Root.Hex(), header.Time)
		}
	}
	
	// Try to read block 1082780
	targetHeight := uint64(1082780)
	targetHash := rawdb.ReadCanonicalHash(ethDB, targetHeight)
	fmt.Printf("Block %d hash: %s\n", targetHeight, targetHash.Hex())
	
	if targetHash != (common.Hash{}) {
		header := rawdb.ReadHeader(ethDB, targetHash, targetHeight)
		if header != nil {
			fmt.Printf("Block %d found! Root: %s, Time: %d\n", targetHeight, header.Root.Hex(), header.Time)
		}
	}
	
	// Count total blocks
	count := 0
	for i := uint64(0); i <= targetHeight; i++ {
		hash := rawdb.ReadCanonicalHash(ethDB, i)
		if hash != (common.Hash{}) {
			count++
			if count <= 5 || count%100000 == 0 || i == targetHeight {
				fmt.Printf("Block %d: %s\n", i, hash.Hex())
			}
		}
	}
	
	fmt.Printf("\nTotal blocks found: %d\n", count)
}