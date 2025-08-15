package main

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	
	"github.com/dgraph-io/badger/v4"
	"github.com/luxfi/geth/common"
)

func main() {
	// Clean up first
	targetPath := "/Users/z/.luxd/network-96369/chains/EWi9aPkGe6EfJ3SobCAmSUXRPLa4brF3cThwPwmHTrD1y13jy/ethdb"
	os.RemoveAll(targetPath)
	os.MkdirAll(targetPath, 0755)
	
	fmt.Printf("Creating mock database at: %s\n", targetPath)
	
	// Open BadgerDB
	opts := badger.DefaultOptions(targetPath)
	opts.ValueLogFileSize = 256 << 20 // 256MB
	
	db, err := badger.Open(opts)
	if err != nil {
		log.Fatal("Failed to open BadgerDB:", err)
	}
	defer db.Close()
	
	// The genesis hash from the embedded genesis
	// This should match what's in /Users/z/work/lux/node/genesis/cchain_genesis_mainnet.json
	genesisHashHex := "3f4fa2a0b0ce089f52bf0ae9199c75ffdd76ecafc987794050cb0d286f1ec61e"
	genesisHash, _ := hex.DecodeString(genesisHashHex)
	
	fmt.Printf("Setting up mock database with genesis: %s\n", genesisHashHex)
	
	// Create mock data for testing
	err = db.Update(func(txn *badger.Txn) error {
		// Write canonical hash for block 0 (genesis)
		key0 := make([]byte, 9)
		key0[0] = 'H'
		binary.BigEndian.PutUint64(key0[1:], 0)
		
		if err := txn.Set(key0, genesisHash); err != nil {
			return err
		}
		fmt.Printf("  Set canonical hash for block 0\n")
		
		// Write head pointers
		headKeys := map[string][]byte{
			"LastBlock":   genesisHash,
			"LastHeader":  genesisHash,
			"LastFast":    genesisHash,
		}
		
		for key, value := range headKeys {
			if err := txn.Set([]byte(key), value); err != nil {
				return err
			}
			fmt.Printf("  Set %s\n", key)
		}
		
		// Write height (0 for genesis)
		heightBytes := make([]byte, 8)
		binary.BigEndian.PutUint64(heightBytes, 0)
		if err := txn.Set([]byte("Height"), heightBytes); err != nil {
			return err
		}
		fmt.Printf("  Set Height to 0\n")
		
		// Create a minimal header for block 0
		// This is a simplified header - in reality it would be RLP encoded
		// But for testing node startup, we just need something present
		headerKey := make([]byte, 41)
		headerKey[0] = 'h'
		binary.BigEndian.PutUint64(headerKey[1:9], 0)
		copy(headerKey[9:], genesisHash)
		
		// Mock header data (normally would be RLP encoded)
		mockHeader := []byte("MOCK_GENESIS_HEADER")
		if err := txn.Set(headerKey, mockHeader); err != nil {
			return err
		}
		fmt.Printf("  Set header for block 0\n")
		
		// Create a body for block 0
		bodyKey := make([]byte, 41)
		bodyKey[0] = 'b'
		binary.BigEndian.PutUint64(bodyKey[1:9], 0)
		copy(bodyKey[9:], genesisHash)
		
		mockBody := []byte("MOCK_GENESIS_BODY")
		if err := txn.Set(bodyKey, mockBody); err != nil {
			return err
		}
		fmt.Printf("  Set body for block 0\n")
		
		return nil
	})
	
	if err != nil {
		log.Fatal("Failed to write mock data:", err)
	}
	
	// Verify
	fmt.Println("\nVerifying mock database...")
	
	err = db.View(func(txn *badger.Txn) error {
		key0 := make([]byte, 9)
		key0[0] = 'H'
		binary.BigEndian.PutUint64(key0[1:], 0)
		
		if item, err := txn.Get(key0); err == nil {
			val, _ := item.ValueCopy(nil)
			fmt.Printf("✓ Genesis block found: %x\n", val)
		} else {
			fmt.Printf("✗ Genesis block not found\n")
		}
		
		if item, err := txn.Get([]byte("LastBlock")); err == nil {
			val, _ := item.ValueCopy(nil)
			var hash common.Hash
			copy(hash[:], val)
			fmt.Printf("✓ LastBlock: %x\n", hash)
		} else {
			fmt.Printf("✗ LastBlock not found\n")
		}
		
		return nil
	})
	
	if err != nil {
		log.Fatal("Verification error:", err)
	}
	
	fmt.Println("\nMock database created successfully!")
	fmt.Println("\nTo test the node:")
	fmt.Println("export LUX_IMPORTED_HEIGHT=0")
	fmt.Printf("export LUX_IMPORTED_BLOCK_ID=%s\n", genesisHashHex)
	fmt.Println("cd /Users/z/work/lux/node")
	fmt.Println("./build/luxd --data-dir=/Users/z/.luxd --network-id=96369")
}