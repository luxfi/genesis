
package balance

import (
	"fmt"
	"math/big"

	"github.com/luxfi/database/badgerdb"
	"github.com/luxfi/geth/common"
	"github.com/luxfi/geth/core/rawdb"
	"github.com/luxfi/geth/ethdb"
	"github.com/luxfi/geth/rlp"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/crypto/sha3"
)

// Account represents the core data of an Ethereum account.
type Account struct {
	Nonce    uint64
	Balance  *big.Int
	Root     common.Hash
	CodeHash []byte
}

// Config holds the configuration for the balance checker.
type Config struct {
	DBPath string
}

// Info holds the retrieved balance and nonce for an account.
type Info struct {
	Address common.Address
	Balance *big.Int
	Nonce   uint64
}

// Checker provides functionality to check account balances from a database.
type Checker struct {
	db ethdb.Database
}

// NewChecker creates a new balance checker instance connected to the specified database.
func NewChecker(cfg Config) (*Checker, error) {
	// The scripts show a mix of PebbleDB and BadgerDB. The project default is BadgerDB.
	// This checker will standardize on BadgerDB and wrap it for the Geth interfaces.
	db, err := badgerdb.New(cfg.DBPath, nil, "", prometheus.DefaultRegisterer)
	if err != nil {
		return nil, fmt.Errorf("failed to open badger database at %s: %w", cfg.DBPath, err)
	}
	return &Checker{db: &badgerWrapper{db}}, nil
}

// GetBalance retrieves the balance and nonce for a given address from the database.
func (c *Checker) GetBalance(addr common.Address) (*Info, error) {
	// This synthesizes the logic from `direct_balance_check.go` and other scripts.
	// It uses a direct lookup of the hashed address key.
	addrHash := keccak256(addr.Bytes())
	accountKey := append([]byte{'a'}, addrHash...) // 'a' is a common prefix for account data

	data, err := c.db.Get(accountKey)
	if err != nil {
		// Could be a different prefix or not found
		return nil, fmt.Errorf("could not find account data for address %s with key %x: %w", addr.Hex(), accountKey, err)
	}

	var acc Account
	if err := rlp.DecodeBytes(data, &acc); err != nil {
		return nil, fmt.Errorf("failed to RLP-decode account data for address %s: %w", addr.Hex(), err)
	}

	return &Info{
		Address: addr,
		Balance: acc.Balance,
		Nonce:   acc.Nonce,
	}, nil
}

// GetChainStatus retrieves high-level information about the chain head.
func (c *Checker) GetChainStatus() (string, error) {
	headHash := rawdb.ReadHeadBlockHash(c.db)
	if headHash == (common.Hash{}) {
		return "", fmt.Errorf("could not read head block hash")
	}
	headNum := rawdb.ReadHeaderNumber(c.db, headHash)
	if headNum == nil {
		return "", fmt.Errorf("could not read head block number for hash %s", headHash.Hex())
	}
	return fmt.Sprintf("Head Block: %d, Hash: %s", *headNum, headHash.Hex()), nil
}


// Close closes the underlying database connection.
func (c *Checker) Close() {
	c.db.Close()
}

// keccak256 is a utility function to compute the Keccak256 hash.
func keccak256(data []byte) []byte {
	hasher := sha3.NewLegacyKeccak256()
	hasher.Write(data)
	return hasher.Sum(nil)
}

// badgerWrapper makes our badgerdb instance compatible with the ethdb.Database interface.
type badgerWrapper struct {
	*badgerdb.Database
}

func (b *badgerWrapper) Get(key []byte) ([]byte, error) { return b.Database.Get(key) }
func (b *badgerWrapper) Has(key []byte) (bool, error) {
	_, err := b.Database.Get(key)
	if err != nil {
		return false, err
	}
	return true, nil
}
func (b *badgerWrapper) Put(key, val []byte) error      { return b.Database.Put(key, val) }
func (b *badgerWrapper) Delete(key []byte) error         { return b.Database.Delete(key) }
func (b *badgerWrapper) NewBatch() ethdb.Batch           { panic("not implemented") }
func (b *badgerWrapper) NewBatchWithSize(int) ethdb.Batch { panic("not implemented") }
func (b *badgerWrapper) NewIterator(prefix []byte, start []byte) ethdb.Iterator {
	panic("not implemented")
}
func (b *badgerWrapper) Stat(string) (string, error)                            { return "", nil }
func (b *badgerWrapper) Compact([]byte, []byte) error                           { return nil }
func (b *badgerWrapper) HasAncient(string, uint64) (bool, error)                { return false, nil }
func (b *badgerWrapper) Ancient(string, uint64) ([]byte, error)                 { return nil, nil }
func (b *badgerWrapper) AncientRange(string, uint64, uint64, uint64) ([][]byte, error) { return nil, nil }
func (b *badgerWrapper) Ancients() (uint64, error)                              { return 0, nil }
func (b *badgerWrapper) Tail() (uint64, error)                                  { return 0, nil }
func (b *badgerWrapper) AncientSize(string) (uint64, error)                     { return 0, nil }
func (b *badgerWrapper) ReadAncients(func(ethdb.AncientReaderOp) error) error   { return nil }
