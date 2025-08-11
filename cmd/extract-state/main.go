package main

import (
	"encoding/hex"
	"fmt"
	"log"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/trie"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/opt"
)

func main() {
	fmt.Println("=== Extracting State from Original SubnetEVM Database ===")
	
	// Target account
	targetAddr := common.HexToAddress("0x9011E888251AB053B7bD1cdB598Db4f9DEd94714")
	fmt.Printf("Looking for account: %s\n", targetAddr.Hex())
	
	// Open the ORIGINAL database (before migration)
	dbPath := "/home/z/work/lux/state/chaindata/lux-mainnet-96369"
	fmt.Printf("Opening original database at: %s\n", dbPath)
	
	opts := &opt.Options{
		ReadOnly: true,
	}
	ldb, err := leveldb.OpenFile(dbPath, opts)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer ldb.Close()
	
	// Create a wrapper for ethereum database interface
	db := &levelDBWrapper{db: ldb}
	
	// Target block
	blockHeight := uint64(1082780)
	blockHash := common.HexToHash("0x32dede1fc8e0f11ecde12fb42aef7933fc6c5fcf863bc277b5eac08ae4d461f0")
	
	fmt.Printf("\nLooking for block %d (hash: %x)\n", blockHeight, blockHash[:8])
	
	// Try to read the header
	header := rawdb.ReadHeader(db, blockHash, blockHeight)
	if header == nil {
		fmt.Printf("Header not found using standard method, trying direct read...\n")
		
		// Try direct key access
		headerKey := headerKey(blockHash, blockHeight)
		if data, err := ldb.Get(headerKey, nil); err == nil {
			header = new(types.Header)
			if err := header.UnmarshalJSON(data); err != nil {
				fmt.Printf("Failed to unmarshal header: %v\n", err)
			}
		}
	}
	
	if header != nil {
		fmt.Printf("✅ Found header!\n")
		fmt.Printf("  Block: %d\n", header.Number.Uint64())
		fmt.Printf("  StateRoot: %x\n", header.Root)
		fmt.Printf("  Time: %d\n", header.Time)
		
		// Now try to access the state
		stateDB, err := state.New(header.Root, state.NewDatabase(db), nil)
		if err != nil {
			fmt.Printf("Failed to open state: %v\n", err)
			
			// Try manual trie access
			fmt.Printf("\nTrying manual trie access...\n")
			tr, err := trie.New(trie.TrieID(header.Root), trie.NewDatabase(db, nil))
			if err != nil {
				fmt.Printf("Failed to open trie: %v\n", err)
			} else {
				// Get account data from trie
				accountHash := crypto.Keccak256(targetAddr[:])
				data, err := tr.Get(accountHash)
				if err != nil {
					fmt.Printf("Failed to get account from trie: %v\n", err)
				} else if data != nil {
					fmt.Printf("✅ Found account data in trie!\n")
					var acc types.StateAccount
					if err := acc.UnmarshalJSON(data); err == nil {
						fmt.Printf("  Balance: %s\n", acc.Balance)
					}
				}
			}
		} else {
			// Check the balance
			balance := stateDB.GetBalance(targetAddr)
			fmt.Printf("\n✅✅ FOUND BALANCE!\n")
			fmt.Printf("Account: %s\n", targetAddr.Hex())
			fmt.Printf("Balance: %s wei\n", balance.String())
			
			// Convert to LUX (18 decimals)
			luxBalance := new(big.Float).Quo(
				new(big.Float).SetInt(balance),
				new(big.Float).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)),
			)
			fmt.Printf("Balance: %s LUX\n", luxBalance.String())
			
			// Check if it's 1.9T
			expectedBalance := new(big.Int)
			expectedBalance.SetString("1900000000000000000000000000000", 10) // 1.9T with 18 decimals
			
			if balance.Cmp(expectedBalance) == 0 {
				fmt.Printf("\n🎉 SUCCESS! The account has exactly 1.9T LUX!\n")
			} else {
				fmt.Printf("\nBalance doesn't match expected 1.9T LUX\n")
			}
		}
	} else {
		fmt.Printf("❌ Header not found at block %d\n", blockHeight)
		
		// Try to find the latest block
		fmt.Printf("\nSearching for latest block...\n")
		
		// Check canonical chain
		for h := blockHeight; h > blockHeight-100 && h > 0; h-- {
			hash := rawdb.ReadCanonicalHash(db, h)
			if hash != (common.Hash{}) {
				fmt.Printf("Found canonical block at height %d: %x\n", h, hash[:8])
				
				header := rawdb.ReadHeader(db, hash, h)
				if header != nil {
					fmt.Printf("  StateRoot: %x\n", header.Root)
					
					// Try to get balance at this block
					stateDB, err := state.New(header.Root, state.NewDatabase(db), nil)
					if err == nil {
						balance := stateDB.GetBalance(targetAddr)
						if balance.Cmp(big.NewInt(0)) > 0 {
							fmt.Printf("  Account balance at block %d: %s\n", h, balance)
						}
					}
				}
				break
			}
		}
	}
}

// levelDBWrapper wraps LevelDB to implement ethdb.Database
type levelDBWrapper struct {
	db *leveldb.DB
}

func (l *levelDBWrapper) Has(key []byte) (bool, error) {
	return l.db.Has(key, nil)
}

func (l *levelDBWrapper) Get(key []byte) ([]byte, error) {
	return l.db.Get(key, nil)
}

func (l *levelDBWrapper) Put(key []byte, value []byte) error {
	return fmt.Errorf("read-only database")
}

func (l *levelDBWrapper) Delete(key []byte) error {
	return fmt.Errorf("read-only database")
}

func (l *levelDBWrapper) NewBatch() ethdb.Batch {
	return nil
}

func (l *levelDBWrapper) NewBatchWithSize(size int) ethdb.Batch {
	return nil
}

func (l *levelDBWrapper) NewIterator(prefix []byte, start []byte) ethdb.Iterator {
	return l.db.NewIterator(nil, nil)
}

func (l *levelDBWrapper) Stat() (string, error) {
	return "leveldb", nil
}

func (l *levelDBWrapper) Compact(start []byte, limit []byte) error {
	return nil
}

func (l *levelDBWrapper) NewSnapshot() (ethdb.Snapshot, error) {
	return nil, fmt.Errorf("snapshots not supported")
}

func (l *levelDBWrapper) Close() error {
	return l.db.Close()
}

func headerKey(hash common.Hash, number uint64) []byte {
	return append(append([]byte("h"), hash[:]...), encodeBlockNumber(number)...)
}

func encodeBlockNumber(number uint64) []byte {
	enc := make([]byte, 8)
	for i := 0; i < 8; i++ {
		enc[i] = byte(number >> uint(56-i*8))
	}
	return enc
}