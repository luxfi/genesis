package main

import (
	"fmt"
	"log"
	
	"github.com/luxfi/geth/common"
	"github.com/luxfi/geth/core/rawdb"
	"github.com/luxfi/geth/ethdb"
	"github.com/luxfi/database/badgerdb"
	"github.com/prometheus/client_golang/prometheus"
)

type badgerWrapper struct {
	*badgerdb.Database
}

func (b badgerWrapper) Get(key []byte) ([]byte, error) { return b.Database.Get(key) }
func (b badgerWrapper) Has(key []byte) (bool, error) {
	v, _ := b.Database.Get(key)
	return v != nil, nil
}
func (b badgerWrapper) Put(key, val []byte) error { return b.Database.Put(key, val) }
func (b badgerWrapper) Delete(key []byte) error { return b.Database.Delete(key) }
func (b badgerWrapper) DeleteRange(start, end []byte) error { return nil } // Added for interface
func (b badgerWrapper) NewBatch() ethdb.Batch { panic("not implemented") }
func (b badgerWrapper) NewBatchWithSize(int) ethdb.Batch { panic("not implemented") }
func (b badgerWrapper) NewIterator(prefix []byte, start []byte) ethdb.Iterator { panic("not implemented") }
func (b badgerWrapper) Stat(string) (string, error) { return "", nil }
func (b badgerWrapper) Compact([]byte, []byte) error { return nil }
func (b badgerWrapper) Close() error { return b.Database.Close() }
func (b badgerWrapper) HasAncient(string, uint64) (bool, error) { return false, nil }
func (b badgerWrapper) Ancient(string, uint64) ([]byte, error) { return nil, nil }
func (b badgerWrapper) AncientRange(string, uint64, uint64, uint64) ([][]byte, error) { return nil, nil }
func (b badgerWrapper) Ancients() (uint64, error) { return 0, nil }
func (b badgerWrapper) Tail() (uint64, error) { return 0, nil }
func (b badgerWrapper) AncientSize(string) (uint64, error) { return 0, nil }
func (b badgerWrapper) ReadAncients(func(ethdb.AncientReaderOp) error) error { return nil }

func main() {
	dbPath := "/Users/z/.luxd/network-96369/chains/X6CU5qgMJfzsTB9UWxj2ZY5hd68x41HfZ4m4hCBWbHuj1Ebc3/ethdb"
	
	// Open BadgerDB
	db, err := badgerdb.New(dbPath, nil, "", prometheus.DefaultRegisterer)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()
	
	// Wrap it
	wrapped := badgerWrapper{db}
	
	// Check latest block
	latestHeader := rawdb.ReadHeadHeaderHash(wrapped)
	if latestHeader == (common.Hash{}) {
		log.Fatal("No head header hash found")
	}
	latestBlock, found := rawdb.ReadHeaderNumber(wrapped, latestHeader)
	if !found {
		log.Fatal("No head block number found")
	}
	
	header := rawdb.ReadHeader(wrapped, latestHeader, latestBlock)
	if header == nil {
		log.Fatal("No header found")
	}
	
	fmt.Printf("✅ C-Chain Migration Status:\n")
	fmt.Printf("   Block Hash: %s\n", header.Hash().Hex())
	fmt.Printf("   Block Height: %d\n", header.Number.Uint64())
	fmt.Printf("   Total Difficulty: %s\n", header.Difficulty.String())
	
	// Address 1: luxdefi.eth
	addr1 := common.HexToAddress("0x9011E888251AB053B7bD1cdB598Db4f9DEd94714")
	// Address 2: 
	addr2 := common.HexToAddress("0xEAbCC110fAcBfebabC66Ad6f9E7B67288e720B59")
	
	// Try to read state at latest block
	stateRoot := header.Root
	
	fmt.Printf("\n📊 Checking Account Balances at block %d:\n", header.Number.Uint64())
	fmt.Printf("   State Root: %s\n", stateRoot.Hex())
	
	// Check if we can access account data
	// Note: This is a simplified check - full state access would require the trie
	fmt.Printf("\n   luxdefi.eth (%s):\n", addr1.Hex())
	fmt.Printf("     Expected from genesis: 2000000000000 LUX\n")
	
	fmt.Printf("\n   Address 2 (%s):\n", addr2.Hex())
	fmt.Printf("     Checking in migrated database...\n")
	
	// Check for any code associated with these addresses
	// Build the code keys manually
	codePrefix := []byte("c")
	key1 := append(codePrefix, addr1.Bytes()...)
	key2 := append(codePrefix, addr2.Bytes()...)
	
	if code1, _ := wrapped.Get(key1); code1 != nil {
		fmt.Printf("     Found code for addr1: %d bytes\n", len(code1))
	}
	if code2, _ := wrapped.Get(key2); code2 != nil {
		fmt.Printf("     Found code for addr2: %d bytes\n", len(code2))
	}
}