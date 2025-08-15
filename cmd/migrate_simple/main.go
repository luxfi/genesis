package main

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
	"flag"
	"log"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cockroachdb/pebble"
	"github.com/luxfi/database/badgerdb"
	"golang.org/x/crypto/sha3"
)

var (
	srcPath = flag.String("src", "/Users/z/work/lux/state/chaindata/lux-mainnet-96369/db/pebbledb", "pebble SubnetEVM path")
	dstPath = flag.String("dst", "/Users/z/.luxd/chains/C/ethdb", "coreth ethdb path (badger)")
	tipHex  = flag.String("tip", "0x32dede1fc8e0f11ecde12fb42aef7933fc6c5fcf863bc277b5eac08ae4d461f0", "tip hash 0x..")
	tipNum  = flag.Uint64("height", 1082780, "tip height")
	nsHex   = flag.String("ns", "337fb73f9bcdac8c31a2d5f7b877ab1e8a2b7f2a1e9bf02a0a0e6c6fd164f1d1", "32-byte hex namespace")
	workers = flag.Int("workers", 6, "number of parallel body/receipt fetchers")
	batchSize = flag.Int("batch", 5000, "blocks per batch flush")
)

func be8(n uint64) []byte {
	var b [8]byte
	binary.BigEndian.PutUint64(b[:], n)
	return b[:]
}

// SubnetEVM suffix makers
func suffixHeader(num uint64, hash []byte) []byte {
	s := make([]byte, 1+8+32)
	s[0] = 'h'
	copy(s[1:9], be8(num))
	copy(s[9:], hash)
	return s
}

func suffixBody(num uint64, hash []byte) []byte {
	s := suffixHeader(num, hash)
	s[0] = 'b'
	return s
}

func suffixRcpt(num uint64, hash []byte) []byte {
	s := suffixHeader(num, hash)
	s[0] = 'r'
	return s
}

// Source key = ns || keccak(suffix) for hashed mode
func srcKey(ns, suffix []byte) []byte {
	h := sha3.NewLegacyKeccak256()
	h.Write(suffix)
	sum := h.Sum(nil)
	
	k := make([]byte, 0, 64)
	k = append(k, ns...)
	k = append(k, sum...)
	return k
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

func readRaw(pdb *pebble.DB, key []byte) []byte {
	val, closer, err := pdb.Get(key)
	if err != nil {
		return nil
	}
	defer closer.Close()
	return append([]byte(nil), val...)
}

// Extract parent hash from RLP header (first field after list header)
func extractParentHash(headerRLP []byte) ([]byte, error) {
	if len(headerRLP) < 36 {
		return nil, errors.New("header too short")
	}
	
	// Skip RLP list header (usually 3 bytes for f90211)
	offset := 0
	if headerRLP[0] >= 0xf7 && headerRLP[0] <= 0xfa {
		lenOfLen := int(headerRLP[0] - 0xf7)
		offset = 1 + lenOfLen
	} else if headerRLP[0] >= 0xc0 && headerRLP[0] <= 0xf7 {
		offset = 1
	}
	
	// Parent hash is the first 32 bytes after list header + hash encoding byte
	if headerRLP[offset] == 0xa0 { // 32-byte string
		return headerRLP[offset+1 : offset+33], nil
	}
	
	// Try alternate offset
	return headerRLP[4:36], nil
}

// Return header RLP and parent hash
func readHeaderRLP(pdb *pebble.DB, ns []byte, num uint64, hash []byte) (raw []byte, parent []byte) {
	k := srcKey(ns, suffixHeader(num, hash))
	val, closer, err := pdb.Get(k)
	if err != nil {
		log.Fatalf("missing header at %d %x", num, hash[:8])
	}
	defer closer.Close()

	raw = append([]byte(nil), val...) // copy
	parent, err = extractParentHash(val)
	if err != nil {
		log.Fatalf("failed to extract parent hash: %v", err)
	}
	return raw, parent
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

	log.Printf("Starting SubnetEVM → Coreth migration (simplified)")
	log.Printf("Source: %s", *srcPath)
	log.Printf("Destination: %s", *dstPath)
	log.Printf("Tip: %s at height %d", *tipHex, *tipNum)

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

	// Open destination BadgerDB
	ddb, err := badgerdb.New(filepath.Clean(*dstPath), nil, "", nil)
	if err != nil {
		log.Fatalf("open badger: %v", err)
	}
	defer ddb.Close()

	tip := mustHash(*tipHex)
	curN := *tipNum
	curH := tip

	// Pipeline channels
	packs := make(chan blockPack, 4096)
	var processed uint64
	startTime := time.Now()

	// 1) Walker goroutine
	walkerDone := make(chan struct{})
	go func() {
		defer close(walkerDone)
		log.Printf("Walking chain backwards from %d", curN)
		for {
			hdrRLP, parent := readHeaderRLP(sdb, ns, curN, curH)
			packs <- blockPack{n: curN, h: curH, headerRLP: hdrRLP}

			if curN%10000 == 0 {
				log.Printf("Walker at block %d", curN)
			}

			if curN == 0 {
				log.Printf("Reached genesis")
				return
			}
			curN--
			curH = parent
		}
	}()

	// 2) Body/receipt fetchers
	enriched := make(chan blockPack, 4096)
	var wgFetch sync.WaitGroup
	for i := 0; i < *workers; i++ {
		wgFetch.Add(1)
		go func(id int) {
			defer wgFetch.Done()
			fetched := 0
			for p := range packs {
				p.bodyRLP = readRaw(sdb, srcKey(ns, suffixBody(p.n, p.h)))
				p.rcptRLP = readRaw(sdb, srcKey(ns, suffixRcpt(p.n, p.h)))
				enriched <- p
				fetched++
				if fetched%10000 == 0 {
					log.Printf("Fetcher %d processed %d blocks", id, fetched)
				}
			}
			log.Printf("Fetcher %d done (%d blocks)", id, fetched)
		}(i)
	}

	go func() {
		<-walkerDone
		close(packs)
		wgFetch.Wait()
		close(enriched)
	}()

	// 3) Writer goroutine
	batch := ddb.NewBatch()
	defer batch.Reset()

	lastFlush := time.Now()
	batchBlocks := 0

	flush := func() {
		if batchBlocks == 0 {
			return
		}
		if err := batch.Write(); err != nil {
			log.Fatalf("batch write failed: %v", err)
		}
		batch.Reset()
		lastFlush = time.Now()
		batchBlocks = 0
	}

	for p := range enriched {
		// Write header
		if err := batch.Put(headerKey(p.n, p.h), p.headerRLP); err != nil {
			log.Fatalf("Failed to write header: %v", err)
		}

		// Write body if present
		if len(p.bodyRLP) > 0 {
			if err := batch.Put(bodyKey(p.n, p.h), p.bodyRLP); err != nil {
				log.Fatalf("Failed to write body: %v", err)
			}
		}

		// Write receipts if present
		if len(p.rcptRLP) > 0 {
			if err := batch.Put(receiptKey(p.n, p.h), p.rcptRLP); err != nil {
				log.Fatalf("Failed to write receipts: %v", err)
			}
		}

		// Write canonical hash
		if err := batch.Put(canonicalHashKey(p.n), p.h); err != nil {
			log.Fatalf("Failed to write canonical hash: %v", err)
		}

		// Write header number
		numBytes := be8(p.n)
		if err := batch.Put(headerNumberKey(p.h), numBytes); err != nil {
			log.Fatalf("Failed to write header number: %v", err)
		}

		batchBlocks++
		n := atomic.AddUint64(&processed, 1)

		// Flush periodically
		if batchBlocks >= *batchSize || time.Since(lastFlush) > time.Second {
			flush()
			elapsed := time.Since(startTime)
			rate := float64(n) / elapsed.Seconds()
			eta := time.Duration(float64(*tipNum+1-n) / rate * float64(time.Second))
			log.Printf("Written %d/%d blocks (%.1f blocks/s, ETA: %v)",
				n, *tipNum+1, rate, eta)
		}
	}
	flush()

	// Set heads to tip
	log.Printf("Setting head to %x at %d", tip[:8], *tipNum)
	if err := ddb.Put(headHeaderKey(), tip); err != nil {
		log.Printf("Warning: Failed to write head header: %v", err)
	}
	if err := ddb.Put(headBlockKey(), tip); err != nil {
		log.Printf("Warning: Failed to write head block: %v", err)
	}
	if err := ddb.Put(headFastBlockKey(), tip); err != nil {
		log.Printf("Warning: Failed to write head fast block: %v", err)
	}

	// Write VM metadata
	vmPath := filepath.Join(filepath.Dir(*dstPath), "vm")
	log.Printf("Writing VM metadata to %s", vmPath)

	vmDB, err := badgerdb.New(vmPath, nil, "", nil)
	if err != nil {
		log.Printf("Warning: Could not create VM database: %v", err)
	} else {
		defer vmDB.Close()

		// Write lastAccepted
		if err := vmDB.Put([]byte("lastAccepted"), tip); err != nil {
			log.Printf("Warning: Failed to write lastAccepted: %v", err)
		}

		// Write lastAcceptedHeight
		heightBytes := be8(*tipNum)
		if err := vmDB.Put([]byte("lastAcceptedHeight"), heightBytes); err != nil {
			log.Printf("Warning: Failed to write lastAcceptedHeight: %v", err)
		}

		// Write initialized flag
		if err := vmDB.Put([]byte("initialized"), []byte{1}); err != nil {
			log.Printf("Warning: Failed to write initialized: %v", err)
		}

		log.Printf("VM metadata written successfully")
	}

	elapsed := time.Since(startTime)
	log.Printf("✅ Migration complete!")
	log.Printf("   Copied %d blocks (0..%d)", processed, *tipNum)
	log.Printf("   Time: %v", elapsed)
	log.Printf("   Rate: %.1f blocks/s", float64(processed)/elapsed.Seconds())
	log.Printf("   Database: %s", *dstPath)
}