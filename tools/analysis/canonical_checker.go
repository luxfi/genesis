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

	fmt.Println("Looking for canonical blocks...")
	
	// Check specific block 1082780
	targetNum := uint64(1082780)
	targetBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(targetBytes, targetNum)
	
	iter, _ := source.NewIter(nil)
	defer iter.Close()
	
	found := 0
	for iter.First(); iter.Valid(); iter.Next() {
		key := iter.Key()
		
		// Check if this key contains our target block number
		if bytes.Contains(key, targetBytes) {
			value, _ := iter.ValueAndErr()
			fmt.Printf("\nFound key with block %d:\n", targetNum)
			fmt.Printf("  Key len: %d\n", len(key))
			fmt.Printf("  Key: %x\n", key)
			if bytes.HasPrefix(key, namespace) {
				actualKey := key[32:]
				fmt.Printf("  After namespace (%d bytes): %x\n", len(actualKey), actualKey)
				if len(actualKey) > 0 {
					fmt.Printf("  First byte: '%c' (0x%02x)\n", actualKey[0], actualKey[0])
				}
			}
			fmt.Printf("  Value len: %d\n", len(value))
			if len(value) == 32 {
				fmt.Printf("  Value (hash?): %x\n", value)
			}
			found++
			if found > 10 {
				break
			}
		}
	}
	
	// Now check for patterns like H<num> without full iteration
	fmt.Printf("\n\nChecking for H<num> pattern (41 bytes)...\n")
	
	// Build key: namespace + 'H' + targetNum
	testKey := make([]byte, 41)
	copy(testKey[:32], namespace)
	testKey[32] = 'H'
	binary.BigEndian.PutUint64(testKey[33:41], targetNum)
	
	if value, closer, err := source.Get(testKey); err == nil {
		defer closer.Close()
		fmt.Printf("Found H<%d> -> %x\n", targetNum, value)
	} else {
		fmt.Printf("No H<%d> found\n", targetNum)
	}
	
	// Try lowercase h
	testKey[32] = 'h'
	if value, closer, err := source.Get(testKey); err == nil {
		defer closer.Close()
		fmt.Printf("Found h<%d> -> %x\n", targetNum, value)
	} else {
		fmt.Printf("No h<%d> found\n", targetNum)
	}
	
	// Count 41-byte and 42-byte keys
	fmt.Printf("\n\nCounting key patterns...\n")
	iter2, _ := source.NewIter(nil)
	defer iter2.Close()
	
	count41 := 0
	count42 := 0
	samples41 := [][]byte{}
	samples42 := [][]byte{}
	
	for iter2.First(); iter2.Valid(); iter2.Next() {
		key := iter2.Key()
		keyLen := len(key)
		
		if keyLen == 41 && bytes.HasPrefix(key, namespace) {
			count41++
			if len(samples41) < 5 {
				samples41 = append(samples41, append([]byte(nil), key...))
			}
		}
		if keyLen == 42 && bytes.HasPrefix(key, namespace) {
			count42++
			if len(samples42) < 5 {
				samples42 = append(samples42, append([]byte(nil), key...))
			}
		}
	}
	
	fmt.Printf("\n41-byte keys: %d\n", count41)
	for i, key := range samples41 {
		actualKey := key[32:]
		fmt.Printf("  Sample %d: %c%x\n", i+1, actualKey[0], actualKey[1:])
	}
	
	fmt.Printf("\n42-byte keys: %d\n", count42)
	for i, key := range samples42 {
		actualKey := key[32:]
		fmt.Printf("  Sample %d: %c%x\n", i+1, actualKey[0], actualKey[1:])
	}
}