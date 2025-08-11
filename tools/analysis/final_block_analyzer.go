package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"log"
	
	"github.com/cockroachdb/pebble"
)

func main() {
	sourceDB := "/home/z/work/lux/state/chaindata/lux-mainnet-96369/db/pebbledb"
	source, err := pebble.Open(sourceDB, &pebble.Options{ReadOnly: true})
	if err != nil {
		log.Fatal(err)
	}
	defer source.Close()

	namespace := []byte{
		0x33, 0x7f, 0xb7, 0x3f, 0x9b, 0xcd, 0xac, 0x8c, 0x31,
		0xa2, 0xd5, 0xf7, 0xb8, 0x77, 0xab, 0x1e, 0x8a, 0x2b,
		0x7f, 0x2a, 0x1e, 0x9b, 0xf0, 0x2a, 0x0a, 0x0e, 0x6c,
		0x6f, 0xd1, 0x64, 0xf1, 0xd1,
	}

	fmt.Println("=== Analyzing Block Data in SubnetEVM Database ===")
	fmt.Printf("Namespace: %x\n\n", namespace)
	
	// Track what we find
	headers := 0
	bodies := 0
	receipts := 0
	canonicals := 0
	maxBlock := uint64(0)
	
	// Key patterns we've discovered:
	// - 73 bytes: namespace(32) + 'b'(1) + num(8) + hash(32) = bodies
	// - 41 bytes: namespace(32) + 'H'(1) + num(8) = canonical mappings
	// - 65 bytes: namespace(32) + 'H'(1) + hash(32) = hash->num mappings
	
	iter, err := source.NewIter(nil)
	if err != nil {
		log.Fatal(err)
	}
	defer iter.Close()
	
	// Count all key types
	keySizes := make(map[int]int)
	samples := make(map[string][]byte)
	
	for iter.First(); iter.Valid(); iter.Next() {
		key := iter.Key()
		keyLen := len(key)
		keySizes[keyLen]++
		
		// Sample specific blocks we care about
		if bytes.Contains(key, []byte{0x00, 0x10, 0x85, 0x9c}) { // 1082780
			samples["block_1082780"] = key
		}
		
		// Check 73-byte keys for block data
		if keyLen == 73 && bytes.HasPrefix(key, namespace) {
			actualKey := key[32:]
			prefix := actualKey[0]
			
			if len(actualKey) >= 41 {
				num := binary.BigEndian.Uint64(actualKey[1:9])
				hash := actualKey[9:41]
				
				if num > maxBlock && num < 2000000 {
					maxBlock = num
				}
				
				switch prefix {
				case 'b': // 0x62 - body
					bodies++
					if num == 0 || num == 1 || num == 1082780 {
						fmt.Printf("Found BODY: block %d, hash=%x\n", num, hash)
					}
				case 'h': // 0x68 - header
					headers++
				case 'r': // 0x72 - receipt
					receipts++
				}
			}
		}
		
		// Check 41-byte keys for canonical mappings (H + num)
		if keyLen == 41 && bytes.HasPrefix(key, namespace) {
			actualKey := key[32:]
			if actualKey[0] == 'H' && len(actualKey) == 9 {
				num := binary.BigEndian.Uint64(actualKey[1:9])
				value, _ := iter.ValueAndErr()
				if len(value) == 32 {
					canonicals++
					if num == 0 || num == 1 || num == 1082780 {
						fmt.Printf("Found CANONICAL: H<%d> -> hash=%x\n", num, value)
					}
					if num > maxBlock && num < 2000000 {
						maxBlock = num
					}
				}
			}
		}
	}
	
	fmt.Printf("\n=== Summary ===\n")
	fmt.Printf("Headers:    %d\n", headers)
	fmt.Printf("Bodies:     %d\n", bodies)
	fmt.Printf("Receipts:   %d\n", receipts)
	fmt.Printf("Canonicals: %d\n", canonicals)
	fmt.Printf("Max block:  %d\n", maxBlock)
	
	fmt.Printf("\n=== Key Size Distribution ===\n")
	for size, count := range keySizes {
		if count > 1000 {
			fmt.Printf("Size %d: %d keys\n", size, count)
		}
	}
	
	fmt.Printf("\n=== Sample Keys ===\n")
	for name, key := range samples {
		fmt.Printf("%s: %x\n", name, key)
		if bytes.HasPrefix(key, namespace) {
			fmt.Printf("  After namespace: %x\n", key[32:])
		}
	}
	
	// Check specific blocks
	fmt.Printf("\n=== Checking Key Blocks ===\n")
	checkBlocks := []uint64{0, 1, 100, 1000, 10000, 100000, 1000000, 1082780}
	
	for _, blockNum := range checkBlocks {
		// Search for any key containing this block number
		targetNum := make([]byte, 8)
		binary.BigEndian.PutUint64(targetNum, blockNum)
		
		iter2, _ := source.NewIter(nil)
		found := false
		for iter2.First(); iter2.Valid() && !found; iter2.Next() {
			key := iter2.Key()
			if len(key) >= 41 && bytes.HasPrefix(key, namespace) {
				actualKey := key[32:]
				if len(actualKey) >= 9 && bytes.Equal(actualKey[1:9], targetNum) {
					prefix := actualKey[0]
					fmt.Printf("Block %d: Found with prefix '%c' (0x%02x)\n", blockNum, prefix, prefix)
					found = true
				}
			}
		}
		iter2.Close()
		
		if !found {
			fmt.Printf("Block %d: NOT FOUND\n", blockNum)
		}
	}
}