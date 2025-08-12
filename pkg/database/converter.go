package database

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/cockroachdb/pebble"
	"github.com/dgraph-io/badger/v4"
	"github.com/ethereum/go-ethereum/common"
)

// ConversionType represents the type of database conversion
type ConversionType string

const (
	SubnetToCoreth   ConversionType = "subnet-to-coreth"
	CorethToSubnet   ConversionType = "coreth-to-subnet"
	PebbleToBadger   ConversionType = "pebble-to-badger"
	BadgerToPebble   ConversionType = "badger-to-pebble"
	DenamespaceDB    ConversionType = "denamespace"
	AddNamespaceDB   ConversionType = "add-namespace"
)

// DatabaseType represents the database backend type
type DatabaseType string

const (
	PebbleDB DatabaseType = "pebbledb"
	BadgerDB DatabaseType = "badgerdb"
	LevelDB  DatabaseType = "leveldb"
)

// ConversionConfig holds configuration for database conversion
type ConversionConfig struct {
	SourcePath      string
	DestPath        string
	SourceType      DatabaseType
	DestType        DatabaseType
	ConversionType  ConversionType
	Namespace       []byte
	BatchSize       int
	Verbose         bool
	VerifyData      bool
	PreserveHeaders bool
	FixCanonical    bool
}

// ConversionStats tracks conversion statistics
type ConversionStats struct {
	TotalKeys       uint64
	Headers         uint64
	Bodies          uint64
	Receipts        uint64
	Canonical       uint64
	HashToNumber    uint64
	StateNodes      uint64
	Code            uint64
	Other           uint64
	LastBlockNum    uint64
	LastBlockHash   common.Hash
	StartTime       time.Time
}

// DatabaseConverter handles database conversions
type DatabaseConverter struct {
	config *ConversionConfig
	stats  *ConversionStats
}

// NewDatabaseConverter creates a new database converter
func NewDatabaseConverter(config *ConversionConfig) *DatabaseConverter {
	if config.BatchSize == 0 {
		config.BatchSize = 10000
	}
	return &DatabaseConverter{
		config: config,
		stats:  &ConversionStats{StartTime: time.Now()},
	}
}

// Convert performs the database conversion
func (c *DatabaseConverter) Convert() error {
	fmt.Printf("Starting database conversion:\n")
	fmt.Printf("  Source: %s (%s)\n", c.config.SourcePath, c.config.SourceType)
	fmt.Printf("  Destination: %s (%s)\n", c.config.DestPath, c.config.DestType)
	fmt.Printf("  Conversion Type: %s\n\n", c.config.ConversionType)

	switch c.config.ConversionType {
	case SubnetToCoreth:
		return c.convertSubnetToCoreth()
	case CorethToSubnet:
		return c.convertCorethToSubnet()
	case PebbleToBadger:
		return c.convertPebbleToBadger()
	case BadgerToPebble:
		return c.convertBadgerToPebble()
	case DenamespaceDB:
		return c.denamespaceDatabase()
	case AddNamespaceDB:
		return c.addNamespaceToDatabase()
	default:
		return fmt.Errorf("unsupported conversion type: %s", c.config.ConversionType)
	}
}

// convertSubnetToCoreth converts SubnetEVM database to Coreth format
func (c *DatabaseConverter) convertSubnetToCoreth() error {
	// Open source PebbleDB
	pdb, err := pebble.Open(c.config.SourcePath, &pebble.Options{})
	if err != nil {
		return fmt.Errorf("failed to open source database: %w", err)
	}
	defer pdb.Close()

	// Open destination BadgerDB
	opts := badger.DefaultOptions(c.config.DestPath)
	opts.Logger = nil
	bdb, err := badger.Open(opts)
	if err != nil {
		return fmt.Errorf("failed to open destination database: %w", err)
	}
	defer bdb.Close()

	// Track blocks for canonical chain
	blockMap := make(map[uint64]common.Hash)
	var maxBlockNum uint64

	// First pass: Scan for block information
	fmt.Println("Phase 1: Scanning for blocks...")
	iter, _ := pdb.NewIter(nil)
	for iter.First(); iter.Valid(); iter.Next() {
		key := iter.Key()
		
		// Remove namespace if present
		if len(c.config.Namespace) > 0 && len(key) > len(c.config.Namespace) {
			if bytes.Equal(key[:len(c.config.Namespace)], c.config.Namespace) {
				key = key[len(c.config.Namespace):]
			}
		}

		// Track headers for canonical chain
		if c.isHeaderKey(key) {
			blockNum, hash := c.parseHeaderKey(key)
			if blockNum > 0 {
				blockMap[blockNum] = hash
				if blockNum > maxBlockNum {
					maxBlockNum = blockNum
					c.stats.LastBlockNum = blockNum
					c.stats.LastBlockHash = hash
				}
			}
		}
	}
	iter.Close()

	fmt.Printf("Found blocks up to height %d\n\n", maxBlockNum)

	// Second pass: Migrate all data
	fmt.Println("Phase 2: Migrating data...")
	iter, _ = pdb.NewIter(nil)
	defer iter.Close()

	batch := bdb.NewWriteBatch()
	batchCount := 0

	for iter.First(); iter.Valid(); iter.Next() {
		origKey := iter.Key()
		value := iter.Value()

		// Strip namespace if present
		key := origKey
		if len(c.config.Namespace) > 0 && len(key) > len(c.config.Namespace) {
			if bytes.Equal(key[:len(c.config.Namespace)], c.config.Namespace) {
				key = key[len(c.config.Namespace):]
			}
		}

		// Make copies
		keyCopy := make([]byte, len(key))
		copy(keyCopy, key)
		valueCopy := make([]byte, len(value))
		copy(valueCopy, value)

		// Write to destination
		if err := batch.Set(keyCopy, valueCopy); err != nil {
			return fmt.Errorf("failed to set key: %w", err)
		}

		// Update stats
		c.updateStats(keyCopy)
		c.stats.TotalKeys++
		batchCount++

		// Flush batch periodically
		if batchCount >= c.config.BatchSize {
			if err := batch.Flush(); err != nil {
				return fmt.Errorf("failed to flush batch: %w", err)
			}
			batch = bdb.NewWriteBatch()
			batchCount = 0

			if c.config.Verbose && c.stats.TotalKeys%100000 == 0 {
				c.printProgress()
			}
		}
	}

	// Flush final batch
	if batchCount > 0 {
		if err := batch.Flush(); err != nil {
			return fmt.Errorf("failed to flush final batch: %w", err)
		}
	}

	// Phase 3: Fix canonical mappings if needed
	if c.config.FixCanonical && c.stats.Canonical == 0 && len(blockMap) > 0 {
		fmt.Println("\nPhase 3: Creating canonical mappings...")
		batch = bdb.NewWriteBatch()
		for blockNum, hash := range blockMap {
			canonKey := c.canonicalKey(blockNum)
			if err := batch.Set(canonKey, hash.Bytes()); err != nil {
				return fmt.Errorf("failed to write canonical key: %w", err)
			}
			c.stats.Canonical++

			if c.stats.Canonical%10000 == 0 {
				if err := batch.Flush(); err != nil {
					return fmt.Errorf("failed to flush canonical batch: %w", err)
				}
				batch = bdb.NewWriteBatch()
				fmt.Printf("  Written %d canonical mappings...\n", c.stats.Canonical)
			}
		}
		if err := batch.Flush(); err != nil {
			return fmt.Errorf("failed to flush final canonical batch: %w", err)
		}
	}

	// Write metadata keys
	c.writeMetadataKeys(bdb)

	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("CONVERSION COMPLETED SUCCESSFULLY")
	fmt.Println(strings.Repeat("=", 60))
	c.printFinalStats()

	return nil
}

// convertPebbleToBadger converts PebbleDB to BadgerDB
func (c *DatabaseConverter) convertPebbleToBadger() error {
	// Open source PebbleDB
	pdb, err := pebble.Open(c.config.SourcePath, &pebble.Options{})
	if err != nil {
		return fmt.Errorf("failed to open PebbleDB: %w", err)
	}
	defer pdb.Close()

	// Open destination BadgerDB
	opts := badger.DefaultOptions(c.config.DestPath)
	opts.Logger = nil
	bdb, err := badger.Open(opts)
	if err != nil {
		return fmt.Errorf("failed to open BadgerDB: %w", err)
	}
	defer bdb.Close()

	// Copy all keys
	iter, _ := pdb.NewIter(nil)
	defer iter.Close()

	batch := bdb.NewWriteBatch()
	count := 0

	for iter.First(); iter.Valid(); iter.Next() {
		key := make([]byte, len(iter.Key()))
		copy(key, iter.Key())
		
		value := make([]byte, len(iter.Value()))
		copy(value, iter.Value())
		
		if err := batch.Set(key, value); err != nil {
			return fmt.Errorf("failed to set key: %w", err)
		}
		
		count++
		
		if count%c.config.BatchSize == 0 {
			if err := batch.Flush(); err != nil {
				return fmt.Errorf("failed to flush batch: %w", err)
			}
			batch = bdb.NewWriteBatch()
			if c.config.Verbose {
				fmt.Printf("Converted %d entries...\n", count)
			}
		}
	}

	// Flush final batch
	if err := batch.Flush(); err != nil {
		return fmt.Errorf("failed to flush final batch: %w", err)
	}

	fmt.Printf("\n✅ Successfully converted %d entries from PebbleDB to BadgerDB!\n", count)
	return nil
}

// denamespaceDatabase removes namespace prefix from all keys
func (c *DatabaseConverter) denamespaceDatabase() error {
	if len(c.config.Namespace) == 0 {
		return fmt.Errorf("namespace is required for denamespace operation")
	}

	fmt.Printf("Removing namespace: %s\n", hex.EncodeToString(c.config.Namespace))
	
	// For now, delegate to convertSubnetToCoreth with namespace stripping
	return c.convertSubnetToCoreth()
}

// Helper methods

func (c *DatabaseConverter) isHeaderKey(key []byte) bool {
	return len(key) == 41 && key[0] == 'h' && key[9] != 'n'
}

func (c *DatabaseConverter) parseHeaderKey(key []byte) (uint64, common.Hash) {
	if !c.isHeaderKey(key) {
		return 0, common.Hash{}
	}
	blockNum := binary.BigEndian.Uint64(key[1:9])
	hash := common.BytesToHash(key[9:41])
	return blockNum, hash
}

func (c *DatabaseConverter) canonicalKey(number uint64) []byte {
	key := make([]byte, 10)
	key[0] = 'h'
	binary.BigEndian.PutUint64(key[1:9], number)
	key[9] = 'n'
	return key
}

func (c *DatabaseConverter) updateStats(key []byte) {
	switch {
	case len(key) == 41 && key[0] == 'h' && key[9] != 'n':
		c.stats.Headers++
	case len(key) == 10 && key[0] == 'h' && key[9] == 'n':
		c.stats.Canonical++
	case len(key) == 41 && key[0] == 'b':
		c.stats.Bodies++
	case len(key) == 41 && key[0] == 'r':
		c.stats.Receipts++
	case len(key) == 33 && key[0] == 'H':
		c.stats.HashToNumber++
	case len(key) == 33 && key[0] == 'n':
		c.stats.StateNodes++
	case len(key) >= 2 && key[0] == 'c' && key[1] == 'o':
		c.stats.Code++
	default:
		c.stats.Other++
	}
}

func (c *DatabaseConverter) writeMetadataKeys(bdb *badger.DB) {
	batch := bdb.NewWriteBatch()

	// Write LastBlock key
	if c.stats.LastBlockNum > 0 {
		lastBlockKey := []byte("LastBlock")
		batch.Set(lastBlockKey, c.stats.LastBlockHash.Bytes())
	}

	// Write head block hash
	headBlockHashKey := []byte("LastHeader")
	batch.Set(headBlockHashKey, c.stats.LastBlockHash.Bytes())

	// Write head header hash
	headHeaderHashKey := []byte("LastFinalized")
	batch.Set(headHeaderHashKey, c.stats.LastBlockHash.Bytes())

	if err := batch.Flush(); err != nil {
		log.Printf("Failed to write metadata keys: %v", err)
	}
}

func (c *DatabaseConverter) printProgress() {
	fmt.Printf("\rProgress: %d keys | H:%d B:%d R:%d C:%d HN:%d S:%d | Block: %d",
		c.stats.TotalKeys, c.stats.Headers, c.stats.Bodies, c.stats.Receipts,
		c.stats.Canonical, c.stats.HashToNumber, c.stats.StateNodes, c.stats.LastBlockNum)
}

func (c *DatabaseConverter) printFinalStats() {
	fmt.Printf("Total Keys: %d\n", c.stats.TotalKeys)
	fmt.Printf("Headers: %d\n", c.stats.Headers)
	fmt.Printf("Bodies: %d\n", c.stats.Bodies)
	fmt.Printf("Receipts: %d\n", c.stats.Receipts)
	fmt.Printf("Canonical: %d\n", c.stats.Canonical)
	fmt.Printf("Hash-to-Number: %d\n", c.stats.HashToNumber)
	fmt.Printf("State Nodes: %d\n", c.stats.StateNodes)
	fmt.Printf("Code: %d\n", c.stats.Code)
	fmt.Printf("Other: %d\n", c.stats.Other)
	fmt.Printf("\nLast Block: #%d (%s)\n", c.stats.LastBlockNum, c.stats.LastBlockHash.Hex())
	fmt.Printf("Time taken: %v\n", time.Since(c.stats.StartTime))
}

// convertCorethToSubnet converts Coreth database to SubnetEVM format
func (c *DatabaseConverter) convertCorethToSubnet() error {
	// TODO: Implement reverse conversion
	return fmt.Errorf("coreth to subnet conversion not yet implemented")
}

// convertBadgerToPebble converts BadgerDB to PebbleDB
func (c *DatabaseConverter) convertBadgerToPebble() error {
	// TODO: Implement BadgerDB to PebbleDB conversion
	return fmt.Errorf("badger to pebble conversion not yet implemented")
}

// addNamespaceToDatabase adds namespace prefix to all keys
func (c *DatabaseConverter) addNamespaceToDatabase() error {
	// TODO: Implement adding namespace
	return fmt.Errorf("add namespace operation not yet implemented")
}