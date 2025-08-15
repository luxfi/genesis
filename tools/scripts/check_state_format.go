package main

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"path/filepath"
	
	"github.com/luxfi/database/badgerdb"
	"github.com/luxfi/geth/common"
	"golang.org/x/crypto/sha3"
)

func keccak256(data []byte) []byte {
	hasher := sha3.NewLegacyKeccak256()
	hasher.Write(data)
	return hasher.Sum(nil)
}

func main() {
	dbPath := "/Users/z/.luxd/network-96369/chains/EWi9aPkGe6EfJ3SobCAmSUXRPLa4brF3cThwPwmHTrD1y13jy/ethdb"
	db, _ := badgerdb.New(filepath.Clean(dbPath), nil, "", nil)
	defer db.Close()
	
	// Get state root
	canonKey := make([]byte, 10)
	canonKey[0] = 'h'
	binary.BigEndian.PutUint64(canonKey[1:9], 1082780)
	canonKey[9] = 'n'
	hashBytes, _ := db.Get(canonKey)
	var blockHash common.Hash
	copy(blockHash[:], hashBytes)
	
	headerKey := append([]byte{'h'}, append(make([]byte, 8), blockHash[:]...)...)
	binary.BigEndian.PutUint64(headerKey[1:9], 1082780)
	headerRLP, _ := db.Get(headerKey)
	
	// Extract state root (skip proper RLP decoding, just find the hash)
	stateRoot := headerRLP[123:155] // Approximate location in header
	fmt.Printf("State root: %s\n", hex.EncodeToString(stateRoot))
	
	// Check if state root exists as a key
	if val, err := db.Get(stateRoot); err == nil {
		fmt.Printf("✓ State root found as hash key: %d bytes\n", len(val))
	} else {
		fmt.Printf("✗ State root NOT found as hash key\n")
	}
	
	// Check treasury account hash
	treasury := common.HexToAddress("0x9011E888251AB053B7bD1cdB598Db4f9DEd94714")
	treasuryHash := keccak256(treasury.Bytes())
	fmt.Printf("\nTreasury address hash: %s\n", hex.EncodeToString(treasuryHash))
	
	// Try different key formats
	tests := []struct{name string; key []byte}{
		{"Direct hash", treasuryHash},
		{"With 's' prefix", append([]byte{'s'}, treasuryHash...)},
		{"With 'a' prefix", append([]byte{'a'}, treasuryHash...)},
		{"With 0x00 prefix", append([]byte{0x00}, treasuryHash...)},
	}
	
	for _, test := range tests {
		if val, err := db.Get(test.key); err == nil {
			fmt.Printf("  ✓ Found with %s: %d bytes\n", test.name, len(val))
		}
	}
}
