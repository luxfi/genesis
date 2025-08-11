package main

import (
	"encoding/binary"
	"fmt"
	"log"
	"os"

	"github.com/cockroachdb/pebble"
	"github.com/dgraph-io/badger/v4"
)

func migrate(sourceDB, destDB string) error {
	// Open source PebbleDB
	src, err := pebble.Open(sourceDB, &pebble.Options{ReadOnly: true})
	if err != nil {
		return fmt.Errorf("open source: %w", err)
	}
	defer src.Close()

	// Open destination BadgerDB
	opts := badger.DefaultOptions(destDB)
	dst, err := badger.Open(opts)
	if err != nil {
		return fmt.Errorf("open dest: %w", err)
	}
	defer dst.Close()

	// Copy all data
	iter := src.NewIter(&pebble.IterOptions{})
	defer iter.Close()

	batch := dst.NewWriteBatch()
	count := 0

	for iter.First(); iter.Valid(); iter.Next() {
		key := append([]byte(nil), iter.Key()...)
		val := append([]byte(nil), iter.Value()...)

		if err := batch.Set(key, val); err != nil {
			return fmt.Errorf("set key: %w", err)
		}

		count++
		if count%10000 == 0 {
			if err := batch.Flush(); err != nil {
				return fmt.Errorf("flush batch: %w", err)
			}
			batch = dst.NewWriteBatch()
			fmt.Printf("Migrated %d keys...\n", count)
		}
	}

	if err := batch.Flush(); err != nil {
		return fmt.Errorf("flush final: %w", err)
	}

	fmt.Printf("Migration complete: %d keys\n", count)
	return nil
}

func verify(dbPath string) error {
	opts := badger.DefaultOptions(dbPath)
	opts.ReadOnly = true
	db, err := badger.Open(opts)
	if err != nil {
		return err
	}
	defer db.Close()

	var genesisHash string
	blockCount := 0

	err = db.View(func(txn *badger.Txn) error {
		prefix := []byte("h")
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()

		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			key := item.Key()

			if len(key) == 41 {
				blockNum := binary.BigEndian.Uint64(key[1:9])
				if blockNum == 0 {
					genesisHash = fmt.Sprintf("0x%x", key[9:41])
				}
				blockCount++
			}
		}
		return nil
	})

	if err != nil {
		return err
	}

	expectedGenesis := "0x3f4fa2a0b0ce089f52bf0ae9199c75ffdd76ecafc987794050cb0d286f1ec61e"
	if genesisHash != expectedGenesis {
		return fmt.Errorf("genesis mismatch: got %s, want %s", genesisHash, expectedGenesis)
	}

	if blockCount != 1082781 {
		return fmt.Errorf("block count: got %d, want 1082781", blockCount)
	}

	fmt.Println("Verification PASS")
	return nil
}

func convertFormat(sourceDB, destDB string) error {
	// Open source (migrated format)
	srcOpts := badger.DefaultOptions(sourceDB)
	srcOpts.ReadOnly = true
	src, err := badger.Open(srcOpts)
	if err != nil {
		return err
	}
	defer src.Close()

	// Open destination
	dstOpts := badger.DefaultOptions(destDB)
	dst, err := badger.Open(dstOpts)
	if err != nil {
		return err
	}
	defer dst.Close()

	// Convert keys
	err = src.View(func(srcTxn *badger.Txn) error {
		return dst.Update(func(dstTxn *badger.Txn) error {
			it := srcTxn.NewIterator(badger.DefaultIteratorOptions)
			defer it.Close()

			for it.Rewind(); it.Valid(); it.Next() {
				item := it.Item()
				key := item.Key()

				// Convert 41-byte format to standard
				if len(key) == 41 && key[0] == 'h' {
					blockNum := key[1:9]
					hash := key[9:41]
					
					// Write canonical: 'H' + blockNum -> hash
					canonicalKey := append([]byte{'H'}, blockNum...)
					if err := dstTxn.Set(canonicalKey, hash); err != nil {
						return err
					}
				} else {
					// Copy as-is
					val, _ := item.ValueCopy(nil)
					if err := dstTxn.Set(key, val); err != nil {
						return err
					}
				}
			}
			return nil
		})
	})

	return err
}