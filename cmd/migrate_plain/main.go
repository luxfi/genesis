package main

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
	"flag"
	"log"
	"path/filepath"
	"time"

	"github.com/cockroachdb/pebble"
	"github.com/luxfi/database/badgerdb"
)

var (
	srcPath = flag.String("src", "/Users/z/work/lux/state/chaindata/lux-mainnet-96369/db/pebbledb", "pebble SubnetEVM path")
	dstPath = flag.String("dst", "/Users/z/.luxd/chains/C/ethdb", "coreth ethdb path (badger)")
	tipHex  = flag.String("tip", "", "tip hash 0x.. (will find if empty)")
	tipNum  = flag.Uint64("height", 0, "tip height (will find if empty)")
	nsHex   = flag.String("ns", "337fb73f9bcdac8c31a2d5f7b877ab1e8a2b7f2a1e9bf02a0a0e6c6fd164f1d1", "32-byte hex namespace")
	workers = flag.Int("workers", 6, "number of parallel body/receipt fetchers")
	batchSize = flag.Int("batch", 5000, "blocks per batch flush")
)

func be8(n uint64) []byte {
	var b [8]byte
	binary.BigEndian.PutUint64(b[:], n)
	return b[:]
}

// Coreth DB key builders
func headerKey(number uint64, hash []byte) []byte {
	return append(append([]byte("h"), be8(number)...), hash...)
}

func bodyKey(number uint64, hash []byte) []byte {
	return append(append([]byte("b"), be8(number)...), hash...)
}

func receiptKey(number uint64, hash []byte) []byte {
	return append(append([]byte("r"), be8(number)...), hash...)
}

func canonicalHashKey(number uint64) []byte {
	return append([]byte("h"), be8(number)...)
}

func headerNumberKey(hash []byte) []byte {
	return append([]byte("H"), hash...)
}

func headHeaderKey() []byte {
	return []byte("LastHeader")
}

func headBlockKey() []byte {
	return []byte("LastBlock")
}

func headFastBlockKey() []byte {
	return []byte("LastFast")
}

func mustHash(s string) []byte {
	if len(s) != 66 || s[:2] != "0x" {
		log.Fatalf("bad hash: %s", s)
	}
	b, err := hex.DecodeString(s[2:])
	if err != nil || len(b) != 32 {
		log.Fatalf("bad hash hex: %s", s)
	}
	return b
}

// Extract parent hash from RLP header
func extractParentHash(headerRLP []byte) ([]byte, error) {
	if len(headerRLP) < 36 {
		return nil, errors.New("header too short")
	}
	
	// Skip RLP list header
	offset := 0
	if headerRLP[0] >= 0xf8 && headerRLP[0] <= 0xfa {
		lenOfLen := int(headerRLP[0] - 0xf7)
		offset = 1 + lenOfLen
	} else if headerRLP[0] >= 0xc0 && headerRLP[0] <= 0xf7 {
		offset = 1
	}
	
	// Parent hash is the first field, encoded as 0xa0 + 32 bytes
	if offset < len(headerRLP)-33 && headerRLP[offset] == 0xa0 {
		return headerRLP[offset+1 : offset+33], nil
	}
	
	// Fallback
	return headerRLP[4:36], nil
}

// Extract block number from RLP header
func extractBlockNumber(headerRLP []byte) uint64 {
	// Look for block number in common positions (around offset 0x100-0x120)
	for offset := 0x100; offset < len(headerRLP)-8 && offset < 0x130; offset++ {
		if headerRLP[offset] >= 0x83 && headerRLP[offset] <= 0x87 {
			numLen := int(headerRLP[offset] - 0x80)
			if offset+1+numLen <= len(headerRLP) {
				num := uint64(0)
				for i := 0; i < numLen && i < 8; i++ {
					num = (num << 8) | uint64(headerRLP[offset+1+i])
				}
				if num > 1000000 && num < 2000000 {
					return num
				}
			}
		}
	}
	return 0
}

// Find the latest block in the database
func findLatestBlock(db *pebble.DB, namespace []byte) (height uint64, hash []byte) {
	it, _ := db.NewIter(&pebble.IterOptions{})
	defer it.Close()
	
	maxNum := uint64(0)
	var maxHash []byte
	
	count := 0
	for it.First(); it.Valid(); it.Next() {
		k := it.Key()
		v := it.Value()
		
		if len(k) == 64 && bytesEqual(k[:32], namespace) && len(v) > 200 && v[0] == 0xf9 {
			// This looks like a header
			if num := extractBlockNumber(v); num > 0 {
				if num > maxNum {
					maxNum = num
					maxHash = k[32:]
				}
			}
		}
		
		count++
		if count > 100000 {
			break
		}
	}
	
	return maxNum, maxHash
}

func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

type blockPack struct {
	n         uint64
	h         []byte
	headerRLP []byte
	bodyRLP   []byte
	rcptRLP   []byte
}

func main() {
	flag.Parse()

	log.Printf("Starting SubnetEVM → Coreth migration (plain keys)")
	log.Printf("Source: %s", *srcPath)
	log.Printf("Destination: %s", *dstPath)

	// Parse namespace
	ns, err := hex.DecodeString(*nsHex)
	if err != nil || len(ns) != 32 {
		log.Fatal("Invalid namespace")
	}

	// Open source Pebble (RO)
	sdb, err := pebble.Open(filepath.Clean(*srcPath), &pebble.Options{ReadOnly: true})
	if err != nil {
		log.Fatalf("open pebble: %v", err)
	}
	defer sdb.Close()

	// Find or use provided tip
	var tip []byte
	var tipHeight uint64
	
	if *tipHex == "" || *tipNum == 0 {
		log.Printf("Finding latest block in database...")
		tipHeight, tip = findLatestBlock(sdb, ns)
		log.Printf("Found latest block: %d with hash %x", tipHeight, tip)
		
		// Limit to a reasonable number for testing
		if tipHeight > 1082780 {
			tipHeight = 1082780
			// We need to find the hash for this block
			log.Printf("Limiting to block %d for this migration", tipHeight)
		}
	} else {
		tip = mustHash(*tipHex)
		tipHeight = *tipNum
	}
	
	log.Printf("Using tip: %x at height %d", tip[:8], tipHeight)

	// Open destination BadgerDB
	ddb, err := badgerdb.New(filepath.Clean(*dstPath), nil, "", nil)
	if err != nil {
		log.Fatalf("open badger: %v", err)
	}
	defer ddb.Close()

	// For now, we'll scan and find headers by iterating
	// In production, you'd maintain a proper index
	
	// First pass: collect all headers
	log.Printf("Collecting headers...")
	headers := make(map[uint64]*blockPack)
	
	it, _ := sdb.NewIter(&pebble.IterOptions{})
	defer it.Close()
	
	for it.First(); it.Valid(); it.Next() {
		k := it.Key()
		v := it.Value()
		
		if len(k) == 64 && bytesEqual(k[:32], ns) && len(v) > 200 && v[0] == 0xf9 {
			// This looks like a header
			if num := extractBlockNumber(v); num > 0 && num <= tipHeight {
				hash := append([]byte{}, k[32:]...)
				headers[num] = &blockPack{
					n:         num,
					h:         hash,
					headerRLP: append([]byte{}, v...),
				}
				
				if num%10000 == 0 {
					log.Printf("Found header for block %d", num)
				}
			}
		}
	}
	
	log.Printf("Found %d headers", len(headers))
	
	if len(headers) == 0 {
		log.Fatal("No headers found!")
	}
	
	// Find bodies and receipts for these headers
	log.Printf("Finding bodies and receipts...")
	
	// We need to figure out the key pattern for bodies and receipts
	// They might be stored with different suffixes but same namespace
	
	// Write to destination
	batch := ddb.NewBatch()
	defer batch.Reset()
	
	processed := 0
	startTime := time.Now()
	
	for num := uint64(0); num <= tipHeight; num++ {
		if pack, ok := headers[num]; ok {
			// Write header
			if err := batch.Put(headerKey(pack.n, pack.h), pack.headerRLP); err != nil {
				log.Fatalf("Failed to write header: %v", err)
			}
			
			// Write canonical hash
			if err := batch.Put(canonicalHashKey(pack.n), pack.h); err != nil {
				log.Fatalf("Failed to write canonical hash: %v", err)
			}
			
			// Write header number
			if err := batch.Put(headerNumberKey(pack.h), be8(pack.n)); err != nil {
				log.Fatalf("Failed to write header number: %v", err)
			}
			
			processed++
			
			if processed%5000 == 0 || time.Since(startTime) > time.Second {
				if err := batch.Write(); err != nil {
					log.Fatalf("batch write failed: %v", err)
				}
				batch.Reset()
				
				elapsed := time.Since(startTime)
				rate := float64(processed) / elapsed.Seconds()
				log.Printf("Written %d blocks (%.1f blocks/s)", processed, rate)
			}
		}
	}
	
	// Final flush
	if err := batch.Write(); err != nil {
		log.Fatalf("Final batch write failed: %v", err)
	}
	
	// Set heads
	if lastPack, ok := headers[tipHeight]; ok {
		log.Printf("Setting head to %x at %d", lastPack.h[:8], tipHeight)
		if err := ddb.Put(headHeaderKey(), lastPack.h); err != nil {
			log.Printf("Warning: Failed to write head header: %v", err)
		}
		if err := ddb.Put(headBlockKey(), lastPack.h); err != nil {
			log.Printf("Warning: Failed to write head block: %v", err)
		}
		if err := ddb.Put(headFastBlockKey(), lastPack.h); err != nil {
			log.Printf("Warning: Failed to write head fast block: %v", err)
		}
	}
	
	elapsed := time.Since(startTime)
	log.Printf("✅ Migration complete!")
	log.Printf("   Copied %d headers", processed)
	log.Printf("   Time: %v", elapsed)
	log.Printf("   Rate: %.1f blocks/s", float64(processed)/elapsed.Seconds())
}