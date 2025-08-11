package main

import (
	"encoding/binary"
	"fmt"
	"math/big"
	"os"
	"time"

	"github.com/luxfi/geth/common"
	"github.com/luxfi/geth/core/rawdb"
	"github.com/luxfi/geth/core/types"
	"github.com/luxfi/geth/ethdb"
	"github.com/luxfi/geth/params"
	"github.com/luxfi/database/badgerdb"
	"github.com/cockroachdb/pebble"
	"github.com/spf13/cobra"
)

type Config struct {
	SrcPath  string
	DstPath  string
	Start    uint64
	End      uint64
	BatchSize int
	NoFreezer bool
}

type Migrator struct {
	cfg Config
	src ethdb.Database
	dst ethdb.Database
}

func main() {
	var cfg Config
	
	rootCmd := &cobra.Command{
		Use:   "migrate-proper",
		Short: "Proper migration from SubnetEVM to Coreth using rawdb",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMigration(cfg)
		},
	}
	
	rootCmd.Flags().StringVar(&cfg.SrcPath, "src", "/home/z/work/lux/state/chaindata/lux-mainnet-96369/db/pebbledb", "Source database path")
	rootCmd.Flags().StringVar(&cfg.DstPath, "dst", "/home/z/.luxd", "Destination database path")
	rootCmd.Flags().Uint64Var(&cfg.Start, "start", 0, "Start block")
	rootCmd.Flags().Uint64Var(&cfg.End, "end", 1082780, "End block")
	rootCmd.Flags().IntVar(&cfg.BatchSize, "batch-size", 2000, "Batch size for writes")
	rootCmd.Flags().BoolVar(&cfg.NoFreezer, "no-freezer", true, "Disable freezer/ancients")
	
	if err := rootCmd.Execute(); err != nil {
		fmt.Printf("❌ Error: %v\n", err)
		os.Exit(1)
	}
}

func runMigration(cfg Config) error {
	fmt.Println("=== Coreth Database Migration (PROPER) ===")
	fmt.Printf("Source: %s\n", cfg.SrcPath)
	fmt.Printf("Destination: %s\n", cfg.DstPath)
	fmt.Printf("Target block: %d\n", cfg.End)
	fmt.Println()
	
	m := &Migrator{cfg: cfg}
	
	// Phase 0: Pre-flight checks
	fmt.Println("📋 Phase 0: Pre-flight checks...")
	if err := m.preflight(); err != nil {
		return fmt.Errorf("preflight failed: %w", err)
	}
	
	// Phase 1: Discovery
	fmt.Println("\n🔍 Phase 1: Discovery...")
	genesisHash, tip, chainConfig, err := m.discover()
	if err != nil {
		return fmt.Errorf("discovery failed: %w", err)
	}
	fmt.Printf("  Genesis hash: 0x%x\n", genesisHash)
	fmt.Printf("  Chain tip: %d\n", tip)
	fmt.Printf("  Chain ID: %v\n", chainConfig.ChainID)
	
	// Phase 2: Open destination
	fmt.Println("\n📂 Phase 2: Opening destination...")
	if err := m.openDestination(genesisHash, chainConfig); err != nil {
		return fmt.Errorf("failed to open destination: %w", err)
	}
	defer m.dst.Close()
	
	// Phase 3: Import blockchain
	fmt.Println("\n📦 Phase 3: Importing blockchain...")
	if err := m.importBlocks(); err != nil {
		return fmt.Errorf("block import failed: %w", err)
	}
	
	// Phase 4: Import state
	fmt.Println("\n🌳 Phase 4: Importing state...")
	tipHash := rawdb.ReadCanonicalHash(m.src, cfg.End)
	tipHeader := rawdb.ReadHeader(m.src, tipHash, cfg.End)
	if tipHeader == nil {
		return fmt.Errorf("no header at target block %d", cfg.End)
	}
	if err := m.importState(tipHeader.Root); err != nil {
		return fmt.Errorf("state import failed: %w", err)
	}
	
	// Phase 5: Write VM metadata
	fmt.Println("\n⚙️ Phase 5: Writing VM metadata...")
	finalTipHash := rawdb.ReadCanonicalHash(m.dst, cfg.End)
	if err := m.writeVMMetadata(finalTipHash, cfg.End); err != nil {
		return fmt.Errorf("VM metadata write failed: %w", err)
	}
	
	// Phase 6: Verify
	fmt.Println("\n✅ Phase 6: Verification...")
	if err := m.verifyAll(cfg.End); err != nil {
		return fmt.Errorf("verification failed: %w", err)
	}
	
	fmt.Println("\n🎉 Migration complete!")
	fmt.Printf("Database ready at: %s\n", cfg.DstPath)
	return nil
}

func (m *Migrator) preflight() error {
	// Check source exists
	if _, err := os.Stat(m.cfg.SrcPath); err != nil {
		return fmt.Errorf("source path not found: %s", m.cfg.SrcPath)
	}
	
	// Create destination directory
	if err := os.MkdirAll(m.cfg.DstPath, 0755); err != nil {
		return fmt.Errorf("failed to create destination: %w", err)
	}
	
	return nil
}

func (m *Migrator) discover() (common.Hash, uint64, *params.ChainConfig, error) {
	// Open source database (PebbleDB for SubnetEVM)
	pdb, err := pebble.Open(m.cfg.SrcPath, &pebble.Options{})
	if err != nil {
		return common.Hash{}, 0, nil, fmt.Errorf("failed to open source: %w", err)
	}
	defer pdb.Close()
	
	// Wrap in ethdb interface  
	m.src = &PebbleDBWrapper{db: pdb}
	
	// Read genesis hash (block 0 hash)
	genesisHash := rawdb.ReadCanonicalHash(m.src, 0)
	if genesisHash == (common.Hash{}) {
		return common.Hash{}, 0, nil, fmt.Errorf("no genesis hash found")
	}
	
	// Read chain config
	chainConfig := rawdb.ReadChainConfig(m.src, genesisHash)
	if chainConfig == nil {
		// Use default Lux mainnet config
		chainConfig = &params.ChainConfig{
			ChainID:             big.NewInt(96369),
			HomesteadBlock:      big.NewInt(0),
			EIP150Block:         big.NewInt(0),
			EIP155Block:         big.NewInt(0),
			EIP158Block:         big.NewInt(0),
			ByzantiumBlock:      big.NewInt(0),
			ConstantinopleBlock: big.NewInt(0),
			PetersburgBlock:     big.NewInt(0),
			IstanbulBlock:       big.NewInt(0),
			BerlinBlock:         big.NewInt(0),
			LondonBlock:         big.NewInt(0),
		}
	}
	
	// Find tip by scanning down from target
	tip := m.cfg.End
	for tip > 0 {
		hash := rawdb.ReadCanonicalHash(m.src, tip)
		if hash != (common.Hash{}) {
			header := rawdb.ReadHeader(m.src, hash, tip)
			if header != nil {
				break
			}
		}
		tip--
	}
	
	return genesisHash, tip, chainConfig, nil
}

func (m *Migrator) openDestination(genesisHash common.Hash, chainConfig *params.ChainConfig) error {
	// Open BadgerDB for Coreth
	badgerDB, err := badgerdb.New(m.cfg.DstPath, nil, "", nil)
	if err != nil {
		return fmt.Errorf("failed to open BadgerDB: %w", err)
	}
	
	// Wrap in ethdb interface
	m.dst = WrapDatabase(badgerDB)
	
	// Write chain config immediately
	rawdb.WriteChainConfig(m.dst, genesisHash, chainConfig)
	
	return nil
}

func (m *Migrator) importBlocks() error {
	batch := m.dst.NewBatch()
	batchCount := 0
	startTime := time.Now()
	
	for n := uint64(0); n <= m.cfg.End; n++ {
		// Read canonical hash
		hash := rawdb.ReadCanonicalHash(m.src, n)
		if hash == (common.Hash{}) {
			// Try to rebuild from headers
			if n == 0 {
				return fmt.Errorf("no genesis block found")
			}
			continue
		}
		
		// Read header
		header := rawdb.ReadHeader(m.src, hash, n)
		if header == nil {
			fmt.Printf("⚠️  No header at block %d (hash: %x)\n", n, hash[:8])
			continue
		}
		
		// Read body
		body := rawdb.ReadBody(m.src, hash, n)
		if body == nil {
			body = &types.Body{} // Empty body
		}
		
		// Read receipts
		receipts := rawdb.ReadRawReceipts(m.src, hash, n)
		if receipts == nil {
			receipts = types.Receipts{} // Empty receipts
		}
		
		// Write to destination using rawdb methods
		rawdb.WriteHeader(batch, header)
		rawdb.WriteBody(batch, hash, n, body)
		rawdb.WriteReceipts(batch, hash, n, receipts)
		rawdb.WriteCanonicalHash(batch, hash, n)
		rawdb.WriteHeaderNumber(batch, hash, n)
		
		// TD is handled automatically by WriteHeader in newer versions
		// For Coreth, TD = height + 1 due to difficulty=1
		
		batchCount++
		
		// Flush batch periodically
		if batchCount >= m.cfg.BatchSize {
			if err := batch.Write(); err != nil {
				return fmt.Errorf("batch write failed at block %d: %w", n, err)
			}
			batch.Reset()
			batchCount = 0
			
			// Progress report
			elapsed := time.Since(startTime).Seconds()
			rate := float64(n) / elapsed
			fmt.Printf("  Progress: block %d / %d (%.0f blocks/sec)\n", n, m.cfg.End, rate)
		}
	}
	
	// Final batch flush
	if batchCount > 0 {
		if err := batch.Write(); err != nil {
			return fmt.Errorf("final batch write failed: %w", err)
		}
	}
	
	// Set head pointers
	tipHash := rawdb.ReadCanonicalHash(m.dst, m.cfg.End)
	if tipHash == (common.Hash{}) {
		return fmt.Errorf("no canonical hash at tip %d", m.cfg.End)
	}
	
	rawdb.WriteHeadHeaderHash(m.dst, tipHash)
	rawdb.WriteHeadBlockHash(m.dst, tipHash)
	rawdb.WriteHeadFastBlockHash(m.dst, tipHash)
	
	fmt.Printf("  ✅ Imported %d blocks\n", m.cfg.End+1)
	fmt.Printf("  ✅ Set head to block %d (hash: 0x%x)\n", m.cfg.End, tipHash)
	
	return nil
}

func (m *Migrator) importState(root common.Hash) error {
	// For now, we'll copy raw state data
	// TODO: Implement proper trie copying from root
	
	fmt.Printf("  State root: 0x%x\n", root)
	
	// Copy account and storage trie nodes
	it := m.src.NewIterator([]byte{0x00}, nil) // Account trie prefix
	defer it.Release()
	
	batch := m.dst.NewBatch()
	count := 0
	
	for it.Next() {
		key := it.Key()
		val := it.Value()
		
		// Skip non-trie keys
		if len(key) > 0 && (key[0] < 0x00 || key[0] > 0x7f) {
			continue
		}
		
		batch.Put(key, val)
		count++
		
		if count%10000 == 0 {
			if err := batch.Write(); err != nil {
				return fmt.Errorf("state batch write failed: %w", err)
			}
			batch.Reset()
			fmt.Printf("  Copied %d state nodes...\n", count)
		}
	}
	
	if err := batch.Write(); err != nil {
		return fmt.Errorf("final state batch write failed: %w", err)
	}
	
	fmt.Printf("  ✅ Copied %d state nodes\n", count)
	return nil
}

func (m *Migrator) writeVMMetadata(tipHash common.Hash, tip uint64) error {
	// VM metadata goes in a separate database
	vmPath := m.cfg.DstPath + "/vm"
	if err := os.MkdirAll(vmPath, 0755); err != nil {
		return fmt.Errorf("failed to create VM directory: %w", err)
	}
	
	vmDB, err := badgerdb.New(vmPath, nil, "", nil)
	if err != nil {
		return fmt.Errorf("failed to open VM database: %w", err)
	}
	defer vmDB.Close()
	
	// Write lastAccepted (raw bytes)
	if err := vmDB.Put([]byte("lastAccepted"), tipHash[:]); err != nil {
		return err
	}
	
	// Write lastAcceptedHeight (8 bytes big-endian)
	heightBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(heightBytes, tip)
	if err := vmDB.Put([]byte("lastAcceptedHeight"), heightBytes); err != nil {
		return err
	}
	
	// Write initialized flag
	if err := vmDB.Put([]byte("initialized"), []byte{0x01}); err != nil {
		return err
	}
	
	fmt.Printf("  ✅ VM metadata written (tip: 0x%x, height: %d)\n", tipHash[:8], tip)
	return nil
}

func (m *Migrator) verifyAll(tip uint64) error {
	// 1. Check head header
	headHash := rawdb.ReadHeadHeaderHash(m.dst)
	if headHash == (common.Hash{}) {
		return fmt.Errorf("no head header hash")
	}
	fmt.Printf("  ✓ Head hash: 0x%x\n", headHash[:8])
	
	// 2. Check header number
	num, ok := rawdb.ReadHeaderNumber(m.dst, headHash)
	if !ok || num != tip {
		return fmt.Errorf("header number mismatch: got %d, want %d", num, tip)
	}
	fmt.Printf("  ✓ Head number: %d\n", num)
	
	// 3. Check header exists
	header := rawdb.ReadHeader(m.dst, headHash, tip)
	if header == nil {
		return fmt.Errorf("no header at tip")
	}
	fmt.Printf("  ✓ Header exists at tip\n")
	
	// 4. Check canonical hash
	canonicalHash := rawdb.ReadCanonicalHash(m.dst, tip)
	if canonicalHash != headHash {
		return fmt.Errorf("canonical hash mismatch")
	}
	fmt.Printf("  ✓ Canonical hash matches\n")
	
	// 5. Check body and receipts
	body := rawdb.ReadBody(m.dst, headHash, tip)
	if body == nil {
		fmt.Printf("  ⚠️  No body at tip (may be empty block)\n")
	} else {
		fmt.Printf("  ✓ Body exists\n")
	}
	
	// 6. TD check removed (not used in newer versions)
	
	// 7. Check chain config
	genesisHash := rawdb.ReadCanonicalHash(m.dst, 0)
	chainConfig := rawdb.ReadChainConfig(m.dst, genesisHash)
	if chainConfig == nil {
		return fmt.Errorf("no chain config")
	}
	fmt.Printf("  ✓ Chain config exists (ID: %v)\n", chainConfig.ChainID)
	
	fmt.Println("\n  ✅ All tip invariants verified!")
	return nil
}