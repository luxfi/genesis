package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/dgraph-io/badger/v4"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"
)

func be(n uint64) []byte {
	var b [8]byte
	binary.BigEndian.PutUint64(b[:], n)
	return b[:]
}

// Raw database keys matching go-ethereum/core/rawdb
var (
	headerPrefix       = []byte("h")  // headerPrefix + num (uint64 big endian) + hash -> header
	headerHashSuffix   = []byte("n")  // headerPrefix + num (uint64 big endian) + headerHashSuffix -> hash
	headerNumberPrefix = []byte("H")  // headerNumberPrefix + hash -> num (uint64 big endian)
	
	headHeaderKey = []byte("LastHeader")
	headBlockKey  = []byte("LastBlock")
	headFastKey   = []byte("LastFast")
)

func writeCanonicalHash(db *badger.DB, hash common.Hash, number uint64) error {
	// h + num + 'n' -> hash
	key := append(headerPrefix, be(number)...)
	key = append(key, headerHashSuffix...)
	
	return db.Update(func(txn *badger.Txn) error {
		return txn.Set(key, hash.Bytes())
	})
}

func writeHeaderNumber(db *badger.DB, hash common.Hash, number uint64) error {
	// H + hash -> num
	key := append(headerNumberPrefix, hash.Bytes()...)
	
	return db.Update(func(txn *badger.Txn) error {
		return txn.Set(key, be(number))
	})
}

func writeHeadHashes(db *badger.DB, hash common.Hash) error {
	return db.Update(func(txn *badger.Txn) error {
		if err := txn.Set(headHeaderKey, hash.Bytes()); err != nil {
			return err
		}
		if err := txn.Set(headBlockKey, hash.Bytes()); err != nil {
			return err
		}
		return txn.Set(headFastKey, hash.Bytes())
	})
}

func getHeader(db *badger.DB, hash common.Hash, number uint64) (*types.Header, error) {
	// h + num + hash -> header
	key := append(headerPrefix, be(number)...)
	key = append(key, hash.Bytes()...)
	
	var headerData []byte
	err := db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(key)
		if err != nil {
			return err
		}
		headerData, err = item.ValueCopy(nil)
		return err
	})
	
	if err != nil {
		return nil, err
	}
	
	var header types.Header
	if err := rlp.DecodeBytes(headerData, &header); err != nil {
		return nil, err
	}
	
	return &header, nil
}

func findTip(db *badger.DB) (common.Hash, uint64, error) {
	// First try to read head header
	var headHash []byte
	err := db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(headHeaderKey)
		if err != nil {
			return err
		}
		headHash, err = item.ValueCopy(nil)
		return err
	})
	
	if err == nil && len(headHash) == 32 {
		hash := common.BytesToHash(headHash)
		
		// Get its number
		var numBytes []byte
		err = db.View(func(txn *badger.Txn) error {
			key := append(headerNumberPrefix, headHash...)
			item, err := txn.Get(key)
			if err != nil {
				return err
			}
			numBytes, err = item.ValueCopy(nil)
			return err
		})
		
		if err == nil && len(numBytes) == 8 {
			num := binary.BigEndian.Uint64(numBytes)
			return hash, num, nil
		}
	}
	
	// Fallback: scan for highest block
	fmt.Println("Head pointers not found, scanning for highest block...")
	
	// Try expected range
	for n := uint64(1082781); n >= 1082770; n-- {
		// Check canonical hash
		key := append(headerPrefix, be(n)...)
		key = append(key, headerHashSuffix...)
		
		var hash []byte
		err := db.View(func(txn *badger.Txn) error {
			item, err := txn.Get(key)
			if err != nil {
				return err
			}
			hash, err = item.ValueCopy(nil)
			return err
		})
		
		if err == nil && len(hash) == 32 {
			return common.BytesToHash(hash), n, nil
		}
	}
	
	// Last resort: scan headers directly
	fmt.Println("Scanning raw headers...")
	var highestNum uint64
	var highestHash common.Hash
	
	err = db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = false
		it := txn.NewIterator(opts)
		defer it.Close()
		
		prefix := headerPrefix
		for it.Seek(prefix); it.Valid(); it.Next() {
			key := it.Item().Key()
			if len(key) == 41 && key[0] == 'h' {
				// This is a header key: h + 8 bytes num + 32 bytes hash
				num := binary.BigEndian.Uint64(key[1:9])
				if num > highestNum {
					highestNum = num
					highestHash = common.BytesToHash(key[9:41])
				}
			}
		}
		return nil
	})
	
	if highestNum > 0 {
		return highestHash, highestNum, nil
	}
	
	return common.Hash{}, 0, fmt.Errorf("no blocks found in database")
}

func main() {
	dbPath := flag.String("db", filepath.Join(os.Getenv("HOME"), ".luxd", "chainData", "2q9e4r6Mu3U68nU1fYjgbR6JvwrRx36CohpAX5UQxse55x1Q5", "ethdb"), "ethdb path")
	tipHex := flag.String("tip", "", "lastAccepted hash (0x...)")
	tipNum := flag.Uint64("num", 0, "lastAccepted height")
	findOnly := flag.Bool("find", false, "Only find tip, don't repair")
	flag.Parse()

	// Open database
	opts := badger.DefaultOptions(*dbPath)
	opts.Logger = nil
	db, err := badger.Open(opts)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Find or use tip
	var tip common.Hash
	var height uint64
	
	if *findOnly || (*tipHex == "" && *tipNum == 0) {
		fmt.Println("Finding tip block...")
		tip, height, err = findTip(db)
		if err != nil {
			log.Fatalf("Failed to find tip: %v", err)
		}
		
		fmt.Printf("\nFound tip:\n")
		fmt.Printf("  Height: %d (0x%x)\n", height, height)
		fmt.Printf("  Hash: %s\n", tip.Hex())
		
		if *findOnly {
			fmt.Printf("\nTo repair, run:\n")
			fmt.Printf("./bin/repair-canonical --tip %s --num %d\n", tip.Hex(), height)
			return
		}
	} else if *tipHex != "" && *tipNum != 0 {
		tip = common.HexToHash(*tipHex)
		height = *tipNum
	} else {
		log.Fatal("need both --tip and --num, or use --find")
	}

	fmt.Printf("\nRepairing canonical chain from tip...\n")
	fmt.Printf("  Tip hash: %s\n", tip.Hex())
	fmt.Printf("  Tip height: %d\n", height)
	fmt.Println()

	// Repair canonical chain
	h := tip
	n := height
	repaired := 0
	batch := db.NewWriteBatch()
	
	for {
		// Write canonical hash
		canonicalKey := append(append(headerPrefix, be(n)...), headerHashSuffix...)
		batch.Set(canonicalKey, h.Bytes())
		
		// Write hash->number mapping
		hashNumKey := append(headerNumberPrefix, h.Bytes()...)
		batch.Set(hashNumKey, be(n))
		
		repaired++
		
		// Flush batch periodically
		if repaired%1000 == 0 {
			if err := batch.Flush(); err != nil {
				log.Fatalf("Failed to flush batch: %v", err)
			}
			batch = db.NewWriteBatch()
			fmt.Printf("  Repaired %d blocks (at height %d)...\n", repaired, n)
		}
		
		if n == 0 {
			break
		}
		
		// Get parent from header
		header, err := getHeader(db, h, n)
		if err != nil {
			log.Fatalf("Failed to get header at %d %s: %v", n, h, err)
		}
		
		h = header.ParentHash
		n--
	}
	
	// Flush final batch
	if err := batch.Flush(); err != nil {
		log.Fatalf("Failed to flush final batch: %v", err)
	}
	
	// Write head pointers
	if err := writeHeadHashes(db, tip); err != nil {
		log.Fatalf("Failed to write head hashes: %v", err)
	}
	
	fmt.Printf("\n✅ Successfully repaired canonical chain!\n")
	fmt.Printf("  Total blocks: %d\n", repaired)
	fmt.Printf("  Genesis: %s\n", h.Hex())
	fmt.Printf("  Tip: %s\n", tip.Hex())
	fmt.Printf("\nDatabase is ready for use.\n")
}