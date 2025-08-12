package main

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"log"
	"math/big"
	
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
	
	ethdb := badgerWrapper{db}
	
	// Check block height
	lastHash := rawdb.ReadHeadBlockHash(ethdb)
	if lastHash == (common.Hash{}) {
		log.Fatal("No head block found")
	}
	
	fmt.Printf("✅ C-Chain Migration Status:\n")
	fmt.Printf("   Block Hash: 0x%s\n", hex.EncodeToString(lastHash[:]))
	
	// Read the block number from the hash
	numBytes, err := db.Get(append([]byte("H"), lastHash[:]...))
	if err == nil && len(numBytes) == 8 {
		blockNum := binary.BigEndian.Uint64(numBytes)
		fmt.Printf("   Block Height: %d\n", blockNum)
		
		// Read header at this height
		lastHeader := rawdb.ReadHeader(ethdb, lastHash, blockNum)
		if lastHeader != nil {
			fmt.Printf("   State Root: 0x%s\n", hex.EncodeToString(lastHeader.Root[:]))
		}
		
		// Read TD manually
		tdKey := make([]byte, 41)
		tdKey[0] = 't'
		binary.BigEndian.PutUint64(tdKey[1:9], blockNum)
		copy(tdKey[9:41], lastHash[:])
		
		if tdBytes, err := db.Get(tdKey); err == nil && len(tdBytes) > 0 {
			td := new(big.Int).SetBytes(tdBytes)
			fmt.Printf("   Total Difficulty: %s\n", td.String())
		}
	}
	
	// Check specific account - luxdefi.eth
	addresses := map[string]common.Address{
		"luxdefi.eth": common.HexToAddress("0x9011E888251AB053B7bD1cdB598Db4f9DEd94714"),
	}
	
	fmt.Printf("\n📊 Expected Account Balances:\n")
	
	for name, addr := range addresses {
		fmt.Printf("   %s (%s):\n", name, addr.Hex())
		
		// The genesis allocation shows this address should have balance
		// From the genesis: "0x193e5939a08ce9dbd480000000" = 500000000000000000000000000000 wei = 500M LUX
		expectedBalance := new(big.Int)
		expectedBalance.SetString("193e5939a08ce9dbd480000000", 16)
		fmt.Printf("     Balance from genesis: %s LUX\n", new(big.Int).Div(expectedBalance, big.NewInt(1e18)))
	}
}