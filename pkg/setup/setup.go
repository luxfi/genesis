package setup

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"path/filepath"

	"github.com/luxfi/database"
	"github.com/luxfi/database/manager"
	"github.com/luxfi/genesis/pkg/application"
	"github.com/luxfi/geth/common"
	"github.com/prometheus/client_golang/prometheus"
)

// ChainStateManager handles chain state setup operations
type ChainStateManager struct {
	app *application.Genesis
}

// New creates a new ChainStateManager
func New(app *application.Genesis) *ChainStateManager {
	return &ChainStateManager{app: app}
}

// Key prefixes for chain state
var (
	// Chain index prefixes
	lastHeaderKey    = []byte("LastHeader")
	lastBlockKey     = []byte("LastBlock")
	lastFastBlockKey = []byte("LastFast")


	// Chain state keys
	acceptedKey = []byte("snowman_lastAccepted")
	heightKey   = []byte("height")
)

// openDatabase opens a database for setup operations
func (c *ChainStateManager) openDatabase(dbPath string) (database.Database, error) {
	// Auto-detect database type
	dbType := c.detectDatabaseType(dbPath)

	// Create database manager
	dbManager := manager.NewManager(filepath.Dir(dbPath), prometheus.NewRegistry())

	// Configure database
	config := &manager.Config{
		Type:      dbType,
		Path:      filepath.Base(dbPath),
		Namespace: "setup",
		CacheSize: 512, // MB
		HandleCap: 1024,
		ReadOnly:  false,
	}

	return dbManager.New(config)
}

// detectDatabaseType tries to determine the database type
func (c *ChainStateManager) detectDatabaseType(dbPath string) string {
	// Check for PebbleDB markers (SST files)
	matches, _ := filepath.Glob(filepath.Join(dbPath, "*.sst"))
	if len(matches) > 0 {
		return "pebbledb"
	}

	// Check for LevelDB markers (LDB files)
	matches, _ = filepath.Glob(filepath.Join(dbPath, "*.ldb"))
	if len(matches) > 0 {
		return "leveldb"
	}

	// Default to PebbleDB
	return "pebbledb"
}

// SetupChainState sets up C-Chain state with imported blockchain data
func (c *ChainStateManager) SetupChainState(dbPath string, targetHeight uint64) error {
	c.app.Log.Info("Setting up chain state", "path", dbPath, "targetHeight", targetHeight)

	// Open database
	db, err := c.openDatabase(dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	// Find the highest block if target not specified
	if targetHeight == 0 {
		c.app.Log.Info("Finding highest block...")
		highestBlock, highestHash, err := c.findHighestBlock(db)
		if err != nil {
			return fmt.Errorf("failed to find highest block: %w", err)
		}
		targetHeight = highestBlock
		c.app.Log.Info("Found highest block", "height", highestBlock, "hash", highestHash.Hex())
	}

	// Find the block hash for target height
	blockHash, err := c.getBlockHash(db, targetHeight)
	if err != nil {
		return fmt.Errorf("failed to get block hash for height %d: %w", targetHeight, err)
	}

	c.app.Log.Info("Target block", "height", targetHeight, "hash", blockHash.Hex())

	// Create a batch for atomic updates
	batch := db.NewBatch()

	// Set head block references
	c.app.Log.Info("Setting head block references...")

	// Set LastHeader
	if err := batch.Put(lastHeaderKey, blockHash[:]); err != nil {
		return fmt.Errorf("failed to set LastHeader: %w", err)
	}

	// Set LastBlock
	if err := batch.Put(lastBlockKey, blockHash[:]); err != nil {
		return fmt.Errorf("failed to set LastBlock: %w", err)
	}

	// Set LastFast
	if err := batch.Put(lastFastBlockKey, blockHash[:]); err != nil {
		return fmt.Errorf("failed to set LastFast: %w", err)
	}

	// Set accepted block for consensus
	if err := batch.Put(acceptedKey, blockHash[:]); err != nil {
		return fmt.Errorf("failed to set lastAccepted: %w", err)
	}

	// Set height
	heightBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(heightBytes, targetHeight)
	if err := batch.Put(heightKey, heightBytes); err != nil {
		return fmt.Errorf("failed to set height: %w", err)
	}

	// Commit all changes
	c.app.Log.Info("Committing chain state...")
	if err := batch.Write(); err != nil {
		return fmt.Errorf("failed to commit batch: %w", err)
	}

	fmt.Printf("\n✅ Chain state setup complete!\n")
	fmt.Printf("   Height: %d\n", targetHeight)
	fmt.Printf("   Hash: %s\n", blockHash.Hex())
	fmt.Printf("\nThe C-Chain should now recognize block %d as the current head.\n", targetHeight)

	return nil
}

func (c *ChainStateManager) findHighestBlock(db database.Database) (uint64, common.Hash, error) {
	var highestNum uint64
	var highestHash common.Hash

	// Iterate through canonical hash mappings
	iter := db.NewIteratorWithPrefix([]byte("H"))
	defer iter.Release()

	for iter.Next() {
		key := iter.Key()
		value := iter.Value()

		// Canonical hash keys are "H" + hash -> number
		if len(key) == 33 && key[0] == 'H' && len(value) == 8 {
			blockNum := binary.BigEndian.Uint64(value)
			if blockNum > highestNum {
				highestNum = blockNum
				copy(highestHash[:], key[1:33])
			}
		}
	}

	if err := iter.Error(); err != nil {
		return 0, common.Hash{}, err
	}

	return highestNum, highestHash, nil
}

func (c *ChainStateManager) getBlockHash(db database.Database, blockNum uint64) (common.Hash, error) {
	// Look for canonical hash at this height
	// The key format for canonical hash is: "h" + num (8 bytes) + "n"
	numBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(numBytes, blockNum)

	canonicalKey := append([]byte("h"), numBytes...)
	canonicalKey = append(canonicalKey, []byte("n")...)

	value, err := db.Get(canonicalKey)
	if err != nil {
		// Try alternative format - iterate through headers at this height
		headerPrefix := append([]byte("h"), numBytes...)

		iter := db.NewIteratorWithPrefix(headerPrefix)
		defer iter.Release()

		for iter.Next() {
			key := iter.Key()
			if len(key) == 41 && bytes.HasPrefix(key, headerPrefix) {
				var hash common.Hash
				copy(hash[:], key[9:41])
				return hash, nil
			}
		}

		return common.Hash{}, fmt.Errorf("no canonical hash found for block %d", blockNum)
	}

	if len(value) != 32 {
		return common.Hash{}, fmt.Errorf("invalid hash length: %d", len(value))
	}

	var hash common.Hash
	copy(hash[:], value)
	return hash, nil
}
