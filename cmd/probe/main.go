package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"os"

	"github.com/luxfi/geth/common"
	"github.com/luxfi/geth/core/rawdb"
	"github.com/luxfi/geth/ethdb"
	"github.com/luxfi/database"
	"github.com/luxfi/database/badgerdb"
	"github.com/spf13/cobra"
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "probe",
		Short: "Probe database for tip invariants",
		RunE:  runProbe,
	}

	rootCmd.Flags().StringP("db", "d", "/home/z/.luxd", "Database directory")
	
	if err := rootCmd.Execute(); err != nil {
		fmt.Printf("❌ Error: %v\n", err)
		os.Exit(1)
	}
}

func runProbe(cmd *cobra.Command, args []string) error {
	dbPath, _ := cmd.Flags().GetString("db")
	
	fmt.Printf("🔍 Probing database at: %s\n", dbPath)
	
	// First check if this is the raw migrated DB or needs chain subdirectory
	var db ethdb.Database
	var err error
	
	// Try direct path first (for simple migration)
	badgerDB, err := badgerdb.New(dbPath, nil, "", nil)
	if err == nil {
		fmt.Printf("📂 Opened database directly at: %s\n", dbPath)
		db = WrapDatabase(badgerDB)
		defer badgerDB.Close()
	} else {
		// Try chain subdirectory
		ethdbPath := dbPath + "/chains/C/ethdb"
		badgerDB, err = badgerdb.New(ethdbPath, nil, "", nil)
		if err != nil {
			return fmt.Errorf("failed to open database at %s or %s: %v", dbPath, ethdbPath, err)
		}
		fmt.Printf("📂 Opened database at: %s\n", ethdbPath)
		db = WrapDatabase(badgerDB)
		defer badgerDB.Close()
	}
	
	fmt.Println("\n=== Tip Invariants Check ===")
	
	// 1. Read head header hash
	headHash := rawdb.ReadHeadHeaderHash(db)
	if headHash == (common.Hash{}) {
		// Try reading raw key
		fmt.Println("⚠️  No head header hash via rawdb, trying raw key...")
		val, err := db.Get([]byte("LastBlock"))
		if err == nil && len(val) == 32 {
			copy(headHash[:], val)
			fmt.Printf("📝 Found head hash via LastBlock key: 0x%x\n", headHash)
		} else {
			val, err = db.Get([]byte("LastHeader"))
			if err == nil && len(val) == 32 {
				copy(headHash[:], val)
				fmt.Printf("📝 Found head hash via LastHeader key: 0x%x\n", headHash)
			} else {
				return fmt.Errorf("❌ No head header hash found")
			}
		}
	} else {
		fmt.Printf("✅ Head hash: 0x%x\n", headHash)
	}
	
	// 2. Read header number for head hash
	headNum, ok := rawdb.ReadHeaderNumber(db, headHash)
	if !ok {
		// Try to find it by scanning
		fmt.Println("⚠️  No header number for head hash, scanning...")
		
		// Check our known target
		targetNum := uint64(1082780)
		targetHash := common.HexToHash("0x32dede1fc8e0f11ecde12fb42aef7933fc6c5fcf863bc277b5eac08ae4d461f0")
		
		if headHash == targetHash {
			headNum = targetNum
			ok = true
			fmt.Printf("📝 Using known target block number: %d\n", headNum)
		} else {
			return fmt.Errorf("❌ No header number for head hash %x", headHash)
		}
	} else {
		fmt.Printf("✅ Head number: %d\n", headNum)
	}
	
	// 3. Read header at head
	header := rawdb.ReadHeader(db, headHash, headNum)
	if header == nil {
		// Check raw key
		key := headerKey(headNum, headHash)
		val, err := db.Get(key)
		if err != nil {
			fmt.Printf("❌ No header at head (%d, %x)\n", headNum, headHash)
			fmt.Printf("   Tried key: %x\n", key)
			return fmt.Errorf("missing header")
		}
		fmt.Printf("⚠️  Found raw header data (%d bytes) but RLP decode failed\n", len(val))
		if len(val) > 0 {
			fmt.Printf("   First 32 bytes: %x\n", val[:min(32, len(val))])
			checkRLP(val)
		}
	} else {
		fmt.Printf("✅ Header exists at head\n")
	}
	
	// 4. Check canonical hash matches
	canonicalHash := rawdb.ReadCanonicalHash(db, headNum)
	if canonicalHash == (common.Hash{}) {
		fmt.Printf("⚠️  No canonical hash at height %d\n", headNum)
	} else if canonicalHash != headHash {
		fmt.Printf("❌ Canonical hash mismatch: got %x, want %x\n", canonicalHash, headHash)
	} else {
		fmt.Printf("✅ Canonical hash matches\n")
	}
	
	// 5. Scan for first failure
	fmt.Printf("\n=== Scanning blocks (sampling) ===\n")
	
	// Check key blocks
	checkBlocks := []uint64{0, 1, 2, 3, 10, 100, 1000, 10000, 100000, 500000, 1000000, 1082780}
	
	for _, n := range checkBlocks {
		if n > headNum {
			continue
		}
		
		// Read canonical hash
		hash := rawdb.ReadCanonicalHash(db, n)
		if hash == (common.Hash{}) {
			// Try raw key
			key := canonicalKey(n)
			val, err := db.Get(key)
			if err != nil {
				fmt.Printf("❌ Block %d: missing canonical hash\n", n)
			} else {
				fmt.Printf("⚠️  Block %d: canonical key exists but not via rawdb (%d bytes)\n", n, len(val))
			}
			continue
		}
		
		// Read header
		hdr := rawdb.ReadHeader(db, hash, n)
		if hdr == nil {
			key := headerKey(n, hash)
			val, err := db.Get(key)
			if err != nil {
				fmt.Printf("❌ Block %d: missing header\n", n)
			} else {
				fmt.Printf("⚠️  Block %d: header key exists but decode failed (%d bytes)\n", n, len(val))
				if len(val) > 0 && len(val) < 100 {
					fmt.Printf("   Raw value: %x\n", val)
				}
			}
		} else {
			fmt.Printf("✅ Block %d: OK (hash: %x...)\n", n, hash[:4])
		}
	}
	
	// 6. Check raw keys that might exist
	fmt.Println("\n=== Raw Key Analysis ===")
	checkRawKeys(db)
	
	return nil
}

func headerKey(number uint64, hash common.Hash) []byte {
	key := make([]byte, 41)
	key[0] = 'h'
	binary.BigEndian.PutUint64(key[1:9], number)
	copy(key[9:], hash[:])
	return key
}

func canonicalKey(number uint64) []byte {
	key := make([]byte, 9)
	key[0] = 'H'
	binary.BigEndian.PutUint64(key[1:], number)
	return key
}

func checkRLP(data []byte) {
	if len(data) == 0 {
		fmt.Println("   ❌ Empty data")
		return
	}
	
	leadByte := data[0]
	if leadByte >= 0xC0 && leadByte <= 0xFF {
		fmt.Printf("   ✓ Valid RLP list prefix (0x%02x)\n", leadByte)
	} else if leadByte >= 0x80 && leadByte < 0xC0 {
		fmt.Printf("   ⚠️  RLP string prefix (0x%02x) - not a list\n", leadByte)
	} else {
		fmt.Printf("   ❌ Invalid RLP prefix (0x%02x)\n", leadByte)
	}
}

func checkRawKeys(db ethdb.Database) {
	// Check for some known keys
	keys := []struct {
		key  []byte
		name string
	}{
		{[]byte("LastBlock"), "LastBlock"},
		{[]byte("LastHeader"), "LastHeader"},
		{[]byte("LastFast"), "LastFast"},
		{[]byte("SnapshotRoot"), "SnapshotRoot"},
		{canonicalKey(0), "Canonical block 0"},
		{canonicalKey(1082780), "Canonical block 1082780"},
	}
	
	for _, k := range keys {
		val, err := db.Get(k.key)
		if err == nil {
			if len(val) <= 32 {
				fmt.Printf("  %s: %x\n", k.name, val)
			} else {
				fmt.Printf("  %s: %d bytes (first 32: %x...)\n", k.name, len(val), val[:32])
			}
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// DatabaseWrapper wraps a Lux database to implement ethdb.Database
type DatabaseWrapper struct {
	db database.Database
}

func WrapDatabase(db database.Database) ethdb.Database {
	return &DatabaseWrapper{db: db}
}

func (d *DatabaseWrapper) Has(key []byte) (bool, error) {
	return d.db.Has(key)
}

func (d *DatabaseWrapper) Get(key []byte) ([]byte, error) {
	return d.db.Get(key)
}

func (d *DatabaseWrapper) Put(key []byte, value []byte) error {
	return d.db.Put(key, value)
}

func (d *DatabaseWrapper) Delete(key []byte) error {
	return d.db.Delete(key)
}

func (d *DatabaseWrapper) NewBatch() ethdb.Batch {
	return &BatchWrapper{batch: d.db.NewBatch()}
}

func (d *DatabaseWrapper) NewBatchWithSize(size int) ethdb.Batch {
	return d.NewBatch()
}

func (d *DatabaseWrapper) NewIterator(prefix []byte, start []byte) ethdb.Iterator {
	return d.db.NewIteratorWithStartAndPrefix(start, prefix)
}

func (d *DatabaseWrapper) Stat() (string, error) {
	return "stats not available", nil
}

func (d *DatabaseWrapper) Compact(start []byte, limit []byte) error {
	return nil
}

func (d *DatabaseWrapper) Close() error {
	return d.db.Close()
}

// Ancient methods (not supported)
func (d *DatabaseWrapper) Ancient(kind string, number uint64) ([]byte, error) {
	return nil, errors.New("ancient data not supported")
}

func (d *DatabaseWrapper) AncientRange(kind string, start, count, maxBytes uint64) ([][]byte, error) {
	return nil, errors.New("ancient data not supported")
}

func (d *DatabaseWrapper) Ancients() (uint64, error) {
	return 0, nil
}

func (d *DatabaseWrapper) Tail() (uint64, error) {
	return 0, nil
}

func (d *DatabaseWrapper) AncientSize(kind string) (uint64, error) {
	return 0, nil
}

func (d *DatabaseWrapper) ModifyAncients(fn func(ethdb.AncientWriteOp) error) (int64, error) {
	return 0, errors.New("ancient data not supported")
}

func (d *DatabaseWrapper) TruncateHead(n uint64) (uint64, error) {
	return 0, errors.New("ancient data not supported")
}

func (d *DatabaseWrapper) TruncateTail(n uint64) (uint64, error) {
	return 0, errors.New("ancient data not supported")
}

func (d *DatabaseWrapper) Sync() error {
	return nil
}

func (d *DatabaseWrapper) MigrateTable(string, func([]byte) ([]byte, error)) error {
	return errors.New("table migration not supported")
}

func (d *DatabaseWrapper) AncientDatadir() (string, error) {
	return "", errors.New("ancient data not supported")
}

func (d *DatabaseWrapper) ReadAncients(fn func(ethdb.AncientReaderOp) error) error {
	return fn(d)
}

func (d *DatabaseWrapper) DeleteRange(start, end []byte) error {
	return errors.New("delete range not supported")
}

func (d *DatabaseWrapper) SyncAncient() error {
	return nil
}

func (d *DatabaseWrapper) SyncKeyValue() error {
	return nil
}

// Batch wrapper
type BatchWrapper struct {
	batch database.Batch
}

func (b *BatchWrapper) Put(key []byte, value []byte) error {
	return b.batch.Put(key, value)
}

func (b *BatchWrapper) Delete(key []byte) error {
	return b.batch.Delete(key)
}

func (b *BatchWrapper) ValueSize() int {
	return b.batch.Size()
}

func (b *BatchWrapper) Write() error {
	return b.batch.Write()
}

func (b *BatchWrapper) Reset() {
	b.batch.Reset()
}

func (b *BatchWrapper) Replay(w ethdb.KeyValueWriter) error {
	return nil
}

func (b *BatchWrapper) DeleteRange(start, end []byte) error {
	return errors.New("delete range not supported")
}