package main

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"log"
	
	"github.com/cockroachdb/pebble"
	"golang.org/x/crypto/sha3"
)

func main() {
	dbPath := "/Users/z/work/lux/state/chaindata/lux-mainnet-96369/db/pebbledb"
	
	db, err := pebble.Open(dbPath, &pebble.Options{ReadOnly: true})
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	
	// Known namespace from previous scan
	namespace, _ := hex.DecodeString("337fb73f9bcdac8c31a2d5f7b877ab1e8a2b7f2a1e9bf02a0a0e6c6fd164f1d1")
	
	fmt.Println("Scanning for actual tip block...")
	
	// Try to find canonical hash entries
	// These would be namespace + keccak256('H' + blockNumber)
	maxHeight := uint64(0)
	var maxHash []byte
	
	// Check a range of heights around the expected
	for height := uint64(1082700); height <= 1082800; height++ {
		// Try canonical hash key: namespace + keccak256('H' + number)
		suffix := append([]byte("H"), be8(height)...)
		h := sha3.NewLegacyKeccak256()
		h.Write(suffix)
		key := append(namespace, h.Sum(nil)...)
		
		val, closer, err := db.Get(key)
		if err == nil {
			fmt.Printf("Found canonical hash at height %d: %x\n", height, val)
			if height > maxHeight {
				maxHeight = height
				maxHash = append([]byte{}, val...)
			}
			closer.Close()
		}
		
		// Also try 'n' prefix (number to hash)
		suffix2 := append([]byte("n"), be8(height)...)
		h2 := sha3.NewLegacyKeccak256()
		h2.Write(suffix2)
		key2 := append(namespace, h2.Sum(nil)...)
		
		val2, closer2, err2 := db.Get(key2)
		if err2 == nil {
			fmt.Printf("Found 'n' entry at height %d: %x\n", height, val2)
			closer2.Close()
		}
	}
	
	if maxHeight > 0 {
		fmt.Printf("\nMax canonical height found: %d\n", maxHeight)
		fmt.Printf("Hash: %x\n", maxHash)
		
		// Now try to read that header
		suffix := append([]byte("h"), be8(maxHeight)...)
		suffix = append(suffix, maxHash...)
		h := sha3.NewLegacyKeccak256()
		h.Write(suffix)
		key := append(namespace, h.Sum(nil)...)
		
		val, closer, err := db.Get(key)
		if err == nil {
			fmt.Printf("Header exists! Length: %d bytes\n", len(val))
			if len(val) > 0 {
				fmt.Printf("RLP starts with: %x\n", val[:min(32, len(val))])
			}
			closer.Close()
		} else {
			fmt.Printf("Header not found for this hash\n")
		}
	}
	
	// Also scan for any header keys in the database
	fmt.Println("\nScanning for any header keys...")
	it, _ := db.NewIter(&pebble.IterOptions{})
	defer it.Close()
	
	count := 0
	headerCount := 0
	for it.First(); it.Valid(); it.Next() {
		k := it.Key()
		v := it.Value()
		
		// Check if value looks like an RLP header (starts with f9)
		if len(v) > 200 && len(v) < 600 && v[0] == 0xf9 {
			// This looks like a header
			fmt.Printf("Found potential header key: %x\n", k)
			fmt.Printf("  Value length: %d, starts with: %x\n", len(v), v[:4])
			headerCount++
			
			// Try to extract block number from the value
			// In RLP headers, the block number is usually around offset 0x100-0x120
			if headerCount <= 5 {
				fmt.Printf("  Full key: %x\n", k)
			}
		}
		
		count++
		if count > 100000 || headerCount >= 10 {
			break
		}
	}
	
	fmt.Printf("\nScanned %d keys, found %d potential headers\n", count, headerCount)
}

func be8(n uint64) []byte {
	var b [8]byte
	binary.BigEndian.PutUint64(b[:], n)
	return b[:]
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}