package main

import (
	"errors"
	
	"github.com/luxfi/geth/ethdb"
	"github.com/luxfi/database"
	"github.com/cockroachdb/pebble"
)

// PebbleDBWrapper wraps PebbleDB to implement ethdb.Database
type PebbleDBWrapper struct {
	db *pebble.DB
}

func (p *PebbleDBWrapper) Has(key []byte) (bool, error) {
	val, closer, err := p.db.Get(key)
	if err == pebble.ErrNotFound {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	closer.Close()
	return len(val) > 0, nil
}

func (p *PebbleDBWrapper) Get(key []byte) ([]byte, error) {
	val, closer, err := p.db.Get(key)
	if err == pebble.ErrNotFound {
		return nil, errors.New("not found")
	}
	if err != nil {
		return nil, err
	}
	defer closer.Close()
	
	// Copy the value since it's only valid until closer.Close()
	result := make([]byte, len(val))
	copy(result, val)
	return result, nil
}

func (p *PebbleDBWrapper) Put(key []byte, value []byte) error {
	return p.db.Set(key, value, pebble.Sync)
}

func (p *PebbleDBWrapper) Delete(key []byte) error {
	return p.db.Delete(key, pebble.Sync)
}

func (p *PebbleDBWrapper) NewBatch() ethdb.Batch {
	return &PebbleBatch{
		db:    p.db,
		batch: p.db.NewBatch(),
	}
}

func (p *PebbleDBWrapper) NewBatchWithSize(size int) ethdb.Batch {
	return p.NewBatch()
}

func (p *PebbleDBWrapper) NewIterator(prefix []byte, start []byte) ethdb.Iterator {
	opts := &pebble.IterOptions{}
	if len(prefix) > 0 {
		opts.LowerBound = prefix
		opts.UpperBound = append(prefix, 0xff)
	}
	if len(start) > 0 {
		opts.LowerBound = start
	}
	
	it, err := p.db.NewIter(opts)
	if err != nil {
		return &PebbleIterator{it: nil}
	}
	if len(start) > 0 {
		it.SeekGE(start)
	} else if len(prefix) > 0 {
		it.SeekGE(prefix)
	} else {
		it.First()
	}
	
	return &PebbleIterator{it: it}
}

func (p *PebbleDBWrapper) Close() error {
	return p.db.Close()
}

// Implement remaining required methods
func (p *PebbleDBWrapper) Stat() (string, error) { return "", nil }
func (p *PebbleDBWrapper) Compact(start []byte, limit []byte) error { return nil }
func (p *PebbleDBWrapper) Ancient(kind string, number uint64) ([]byte, error) {
	return nil, errors.New("not supported")
}
func (p *PebbleDBWrapper) AncientRange(kind string, start, count, maxBytes uint64) ([][]byte, error) {
	return nil, errors.New("not supported")
}
func (p *PebbleDBWrapper) Ancients() (uint64, error) { return 0, nil }
func (p *PebbleDBWrapper) Tail() (uint64, error) { return 0, nil }
func (p *PebbleDBWrapper) AncientSize(kind string) (uint64, error) { return 0, nil }
func (p *PebbleDBWrapper) ModifyAncients(fn func(ethdb.AncientWriteOp) error) (int64, error) {
	return 0, errors.New("not supported")
}
func (p *PebbleDBWrapper) TruncateHead(n uint64) (uint64, error) { return 0, errors.New("not supported") }
func (p *PebbleDBWrapper) TruncateTail(n uint64) (uint64, error) { return 0, errors.New("not supported") }
func (p *PebbleDBWrapper) Sync() error { return nil }
func (p *PebbleDBWrapper) MigrateTable(string, func([]byte) ([]byte, error)) error {
	return errors.New("not supported")
}
func (p *PebbleDBWrapper) AncientDatadir() (string, error) { return "", errors.New("not supported") }
func (p *PebbleDBWrapper) ReadAncients(fn func(ethdb.AncientReaderOp) error) error { return fn(p) }
func (p *PebbleDBWrapper) DeleteRange(start, end []byte) error { return errors.New("not supported") }
func (p *PebbleDBWrapper) SyncAncient() error { return nil }
func (p *PebbleDBWrapper) SyncKeyValue() error { return nil }

// PebbleBatch implements ethdb.Batch
type PebbleBatch struct {
	db    *pebble.DB
	batch *pebble.Batch
	size  int
}

func (b *PebbleBatch) Put(key []byte, value []byte) error {
	b.size += len(key) + len(value)
	return b.batch.Set(key, value, nil)
}

func (b *PebbleBatch) Delete(key []byte) error {
	b.size += len(key)
	return b.batch.Delete(key, nil)
}

func (b *PebbleBatch) ValueSize() int {
	return b.size
}

func (b *PebbleBatch) Write() error {
	return b.batch.Commit(pebble.Sync)
}

func (b *PebbleBatch) Reset() {
	b.batch.Close()
	b.batch = b.db.NewBatch()
	b.size = 0
}

func (b *PebbleBatch) Replay(w ethdb.KeyValueWriter) error {
	return errors.New("not supported")
}

func (b *PebbleBatch) DeleteRange(start, end []byte) error {
	return errors.New("not supported")
}

// PebbleIterator implements ethdb.Iterator
type PebbleIterator struct {
	it *pebble.Iterator
}

func (i *PebbleIterator) Next() bool {
	return i.it.Next()
}

func (i *PebbleIterator) Error() error {
	return i.it.Error()
}

func (i *PebbleIterator) Key() []byte {
	return i.it.Key()
}

func (i *PebbleIterator) Value() []byte {
	return i.it.Value()
}

func (i *PebbleIterator) Release() {
	i.it.Close()
}

// DatabaseWrapper wraps Lux database to implement ethdb.Database
type DatabaseWrapper struct {
	db database.Database
}

func WrapDatabase(db database.Database) ethdb.Database {
	return &DatabaseWrapper{db: db}
}

func (d *DatabaseWrapper) Has(key []byte) (bool, error) {
	return d.db.Has(key)
}

func (d *DatabaseWrapper) Get(key []byte) ([]byte, error) {
	return d.db.Get(key)
}

func (d *DatabaseWrapper) Put(key []byte, value []byte) error {
	return d.db.Put(key, value)
}

func (d *DatabaseWrapper) Delete(key []byte) error {
	return d.db.Delete(key)
}

func (d *DatabaseWrapper) NewBatch() ethdb.Batch {
	return &BatchWrapper{batch: d.db.NewBatch()}
}

func (d *DatabaseWrapper) NewBatchWithSize(size int) ethdb.Batch {
	return d.NewBatch()
}

func (d *DatabaseWrapper) NewIterator(prefix []byte, start []byte) ethdb.Iterator {
	return d.db.NewIteratorWithStartAndPrefix(start, prefix)
}

func (d *DatabaseWrapper) Close() error {
	return d.db.Close()
}

// Implement remaining required methods
func (d *DatabaseWrapper) Stat() (string, error) { return "", nil }
func (d *DatabaseWrapper) Compact(start []byte, limit []byte) error { return nil }
func (d *DatabaseWrapper) Ancient(kind string, number uint64) ([]byte, error) {
	return nil, errors.New("not supported")
}
func (d *DatabaseWrapper) AncientRange(kind string, start, count, maxBytes uint64) ([][]byte, error) {
	return nil, errors.New("not supported")
}
func (d *DatabaseWrapper) Ancients() (uint64, error) { return 0, nil }
func (d *DatabaseWrapper) Tail() (uint64, error) { return 0, nil }
func (d *DatabaseWrapper) AncientSize(kind string) (uint64, error) { return 0, nil }
func (d *DatabaseWrapper) ModifyAncients(fn func(ethdb.AncientWriteOp) error) (int64, error) {
	return 0, errors.New("not supported")
}
func (d *DatabaseWrapper) TruncateHead(n uint64) (uint64, error) { return 0, errors.New("not supported") }
func (d *DatabaseWrapper) TruncateTail(n uint64) (uint64, error) { return 0, errors.New("not supported") }
func (d *DatabaseWrapper) Sync() error { return nil }
func (d *DatabaseWrapper) MigrateTable(string, func([]byte) ([]byte, error)) error {
	return errors.New("not supported")
}
func (d *DatabaseWrapper) AncientDatadir() (string, error) { return "", errors.New("not supported") }
func (d *DatabaseWrapper) ReadAncients(fn func(ethdb.AncientReaderOp) error) error { return fn(d) }
func (d *DatabaseWrapper) DeleteRange(start, end []byte) error { return errors.New("not supported") }
func (d *DatabaseWrapper) SyncAncient() error { return nil }
func (d *DatabaseWrapper) SyncKeyValue() error { return nil }

// BatchWrapper wraps database.Batch
type BatchWrapper struct {
	batch database.Batch
}

func (b *BatchWrapper) Put(key []byte, value []byte) error {
	return b.batch.Put(key, value)
}

func (b *BatchWrapper) Delete(key []byte) error {
	return b.batch.Delete(key)
}

func (b *BatchWrapper) ValueSize() int {
	return b.batch.Size()
}

func (b *BatchWrapper) Write() error {
	return b.batch.Write()
}

func (b *BatchWrapper) Reset() {
	b.batch.Reset()
}

func (b *BatchWrapper) Replay(w ethdb.KeyValueWriter) error {
	return nil
}

func (b *BatchWrapper) DeleteRange(start, end []byte) error {
	return errors.New("not supported")
}