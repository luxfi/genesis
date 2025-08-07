package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"os"

	"github.com/cockroachdb/pebble"
	"github.com/luxfi/geth/core/types"
	"github.com/luxfi/geth/rlp"
)

func main() {
	var dbPath string
	var blockNum uint64

	flag.StringVar(&dbPath, "db", "", "Path to database")
	flag.Uint64Var(&blockNum, "block", 0, "Block number")
	flag.Parse()

	if dbPath == "" {
		fmt.Println("Usage: check-header -db /path/to/db -block 0")
		os.Exit(1)
	}

	// Open database
	opts := &pebble.Options{
		ReadOnly: true,
	}
	
	db, err := pebble.Open(dbPath, opts)
	if err != nil {
		fmt.Printf("Failed to open database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	// Find canonical hash first
	canonicalKey := make([]byte, 42)
	canonicalKey[0] = 0x33
	// Padding
	copy(canonicalKey[1:32], []byte{
		0x7f, 0xb7, 0x3f, 0x9b, 0xcd, 0xac, 0x8c, 0x31,
		0xa2, 0xd5, 0xf7, 0xb8, 0x77, 0xab, 0x1e, 0x8a,
		0x2b, 0x7f, 0x2a, 0x1e, 0x9b, 0xf0, 0x2a, 0x0a,
		0x0e, 0x6c, 0x6f, 0xd1, 0x64, 0xf1, 0xd1,
	})
	canonicalKey[32] = 0x68
	// Block number
	canonicalKey[33] = byte(blockNum >> 56)
	canonicalKey[34] = byte(blockNum >> 48)
	canonicalKey[35] = byte(blockNum >> 40)
	canonicalKey[36] = byte(blockNum >> 32)
	canonicalKey[37] = byte(blockNum >> 24)
	canonicalKey[38] = byte(blockNum >> 16)
	canonicalKey[39] = byte(blockNum >> 8)
	canonicalKey[40] = byte(blockNum)
	canonicalKey[41] = 0x6e

	hashData, closer, err := db.Get(canonicalKey)
	if err != nil {
		fmt.Printf("Failed to get canonical hash for block %d: %v\n", blockNum, err)
		os.Exit(1)
	}
	closer.Close()

	fmt.Printf("Block %d hash: %s\n", blockNum, hex.EncodeToString(hashData))

	// Now get the header
	headerKey := make([]byte, 73)
	copy(headerKey[:41], canonicalKey[:41])
	headerKey[32] = 0x68 // 'h'
	copy(headerKey[41:], hashData[:32])

	headerData, closer, err := db.Get(headerKey)
	if err != nil {
		fmt.Printf("Failed to get header: %v\n", err)
		os.Exit(1)
	}
	defer closer.Close()

	fmt.Printf("Header data (%d bytes): %s\n", len(headerData), hex.EncodeToString(headerData[:100]))

	// Try to decode as legacy header
	type LegacyHeader struct {
		ParentHash  [32]byte
		UncleHash   [32]byte
		Coinbase    [20]byte
		Root        [32]byte
		TxHash      [32]byte
		ReceiptHash [32]byte
		Bloom       [256]byte
		Difficulty  *rlp.RawValue
		Number      *rlp.RawValue
		GasLimit    uint64
		GasUsed     uint64
		Time        uint64
		Extra       []byte
		MixDigest   [32]byte
		Nonce       [8]byte
		BaseFee     *rlp.RawValue `rlp:"optional"`
	}

	var legacy LegacyHeader
	if err := rlp.DecodeBytes(headerData, &legacy); err != nil {
		fmt.Printf("Failed to decode as legacy: %v\n", err)
	} else {
		fmt.Println("Successfully decoded as legacy header!")
	}

	// Also try standard decode
	var header types.Header
	if err := rlp.DecodeBytes(headerData, &header); err != nil {
		fmt.Printf("Failed to decode as standard header: %v\n", err)
		fmt.Printf("This is likely a pre-Shanghai header without WithdrawalsHash\n")
	}
}