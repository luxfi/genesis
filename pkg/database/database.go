package database

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/luxfi/database"
	"github.com/luxfi/database/manager"
	"github.com/luxfi/genesis/pkg/application"
	"github.com/prometheus/client_golang/prometheus"
)

// Manager handles database operations
type Manager struct {
	app *application.Genesis
}

// New creates a new database Manager
func New(app *application.Genesis) *Manager {
	return &Manager{app: app}
}

// openDatabase opens a database with the appropriate backend
func (m *Manager) openDatabase(dbPath string, readOnly bool) (database.Database, error) {
	// Try to detect database type by looking at the directory structure
	dbType := m.detectDatabaseType(dbPath)
	
	// Create database manager
	dbManager := manager.NewManager(filepath.Dir(dbPath), prometheus.NewRegistry())
	
	// Configure database
	config := &manager.Config{
		Type:      dbType,
		Path:      filepath.Base(dbPath),
		Namespace: "genesis",
		CacheSize: 512, // MB
		HandleCap: 1024,
		ReadOnly:  readOnly,
	}
	
	db, err := dbManager.New(config)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	
	return db, nil
}

// detectDatabaseType tries to determine the database type
func (m *Manager) detectDatabaseType(dbPath string) string {
	// Check if directory exists
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		// Default to PebbleDB for new databases
		return "pebbledb"
	}
	
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
	
	// Check for MANIFEST files (could be either)
	matches, _ = filepath.Glob(filepath.Join(dbPath, "MANIFEST-*"))
	if len(matches) > 0 {
		// Try to read OPTIONS file to determine type
		if data, err := os.ReadFile(filepath.Join(dbPath, "OPTIONS-000001")); err == nil {
			if len(data) > 0 {
				// PebbleDB typically has different OPTIONS format
				return "pebbledb"
			}
		}
		return "leveldb"
	}
	
	// Default to PebbleDB
	return "pebbledb"
}

// WriteHeight writes a height key to the database
func (m *Manager) WriteHeight(dbPath string, height uint64) error {
	db, err := m.openDatabase(dbPath, false)
	if err != nil {
		return err
	}
	defer db.Close()

	// Create the height key: 0x01 + Height
	key := append([]byte{0x01}, []byte("Height")...)
	
	// Encode height as big-endian
	value := make([]byte, 8)
	binary.BigEndian.PutUint64(value, height)

	// Write to database
	if err := db.Put(key, value); err != nil {
		return fmt.Errorf("failed to write height: %w", err)
	}

	m.app.Log.Info("Height written successfully", "height", height, "key", hex.EncodeToString(key))
	return nil
}

// GetCanonicalHash retrieves the canonical block hash at a specific height
func (m *Manager) GetCanonicalHash(dbPath string, height uint64) (string, error) {
	db, err := m.openDatabase(dbPath, true)
	if err != nil {
		return "", err
	}
	defer db.Close()

	// Create canonical hash key: 'H' + number
	key := make([]byte, 9)
	key[0] = 'H'
	binary.BigEndian.PutUint64(key[1:], height)

	value, err := db.Get(key)
	if err != nil {
		return "", fmt.Errorf("canonical hash not found for height %d: %w", height, err)
	}

	return "0x" + hex.EncodeToString(value), nil
}

// CheckStatus displays database statistics and information
func (m *Manager) CheckStatus(dbPath string) error {
	db, err := m.openDatabase(dbPath, true)
	if err != nil {
		return err
	}
	defer db.Close()

	fmt.Printf("Database Status: %s\n", dbPath)
	fmt.Printf("=================================\n")
	fmt.Printf("Type: %s\n", m.detectDatabaseType(dbPath))
	
	// Count different key types
	keyTypes := make(map[byte]int)
	var totalKeys int
	var highestBlock uint64
	
	iter := db.NewIterator()
	defer iter.Release()

	for iter.Next() {
		key := iter.Key()
		if len(key) > 0 {
			keyTypes[key[0]]++
			totalKeys++
			
			// Track highest block
			if key[0] == 'H' && len(key) == 9 {
				num := binary.BigEndian.Uint64(key[1:])
				if num > highestBlock {
					highestBlock = num
				}
			}
		}
	}

	if err := iter.Error(); err != nil {
		return fmt.Errorf("iterator error: %w", err)
	}

	fmt.Printf("\nKey Distribution:\n")
	for prefix, count := range keyTypes {
		fmt.Printf("  %c (%s): %d\n", prefix, getKeyTypeName(prefix), count)
	}
	
	fmt.Printf("\nTotal Keys: %d\n", totalKeys)
	fmt.Printf("Highest Block: %d\n", highestBlock)

	return nil
}

// PrepareMigration prepares database for LUX_GENESIS migration
func (m *Manager) PrepareMigration(dbPath string, height uint64) error {
	m.app.Log.Info("Preparing database for migration", "path", dbPath, "height", height)

	db, err := m.openDatabase(dbPath, false)
	if err != nil {
		return err
	}
	defer db.Close()

	// Write the height marker
	heightKey := append([]byte{0x01}, []byte("Height")...)
	heightValue := make([]byte, 8)
	binary.BigEndian.PutUint64(heightValue, height)
	
	if err := db.Put(heightKey, heightValue); err != nil {
		return fmt.Errorf("failed to write height: %w", err)
	}

	// Write migration marker
	migrationKey := []byte("LUX_GENESIS_READY")
	migrationValue := []byte(fmt.Sprintf("%d", time.Now().Unix()))
	
	if err := db.Put(migrationKey, migrationValue); err != nil {
		return fmt.Errorf("failed to write migration marker: %w", err)
	}

	m.app.Log.Info("Database prepared for migration", 
		"height", height,
		"heightKey", hex.EncodeToString(heightKey),
		"migrationKey", string(migrationKey))

	return nil
}

// CompactAncient compacts ancient data in the database
func (m *Manager) CompactAncient(dbPath string, blockNum uint64) error {
	// For SubnetEVM databases, this might involve:
	// 1. Removing old receipts
	// 2. Compacting state tries
	// 3. Removing transaction lookup indices

	m.app.Log.Info("Compacting ancient data", "path", dbPath, "before", blockNum)

	// TODO: Implement actual compaction logic
	return fmt.Errorf("ancient data compaction not yet implemented")
}

func getKeyTypeName(prefix byte) string {
	switch prefix {
	case 'H':
		return "canonical hash"
	case 'h':
		return "header"
	case 'b':
		return "body"
	case 'r':
		return "receipt"
	case 'n':
		return "number"
	case 't':
		return "transaction"
	case 'R':
		return "block receipts"
	case 'l':
		return "lookup"
	case 0x01:
		return "metadata"
	default:
		return "unknown"
	}
}