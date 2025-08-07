package migration

import (
	"encoding/binary"
	"fmt"
	"sort"

	"github.com/cockroachdb/pebble"
	"github.com/luxfi/geth/common"
	"github.com/luxfi/geth/rlp"
)

// SubnetToCChain migrates SubnetEVM database to C-chain format
type SubnetToCChain struct {
	sourceDB *pebble.DB
	targetDB *pebble.DB
}

// NewSubnetToCChain creates a new migrator
func NewSubnetToCChain(sourcePath, targetPath string) (*SubnetToCChain, error) {
	// Open source database
	sourceOpts := &pebble.Options{
		ReadOnly: true,
	}
	sourceDB, err := pebble.Open(sourcePath, sourceOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to open source database: %w", err)
	}

	// Create target database directory
	targetOpts := &pebble.Options{}
	targetDB, err := pebble.Open(targetPath, targetOpts)
	if err != nil {
		sourceDB.Close()
		return nil, fmt.Errorf("failed to create target database: %w", err)
	}

	return &SubnetToCChain{
		sourceDB: sourceDB,
		targetDB: targetDB,
	}, nil
}

// Close closes both databases
func (m *SubnetToCChain) Close() {
	if m.sourceDB != nil {
		m.sourceDB.Close()
	}
	if m.targetDB != nil {
		m.targetDB.Close()
	}
}

// Migrate performs the migration
func (m *SubnetToCChain) Migrate() error {
	fmt.Println("Starting SubnetEVM to C-chain migration...")

	// Step 1: Find all blocks
	blocks, err := m.findAllBlocks()
	if err != nil {
		return fmt.Errorf("failed to find blocks: %w", err)
	}
	fmt.Printf("Found %d blocks to migrate\n", len(blocks))

	// Step 2: Migrate blocks in order
	batch := m.targetDB.NewBatch()
	for i, blockInfo := range blocks {
		if err := m.migrateBlock(blockInfo, batch); err != nil {
			return fmt.Errorf("failed to migrate block %d: %w", blockInfo.Number, err)
		}

		// Commit batch every 1000 blocks
		if (i+1)%1000 == 0 {
			if err := batch.Commit(pebble.Sync); err != nil {
				return fmt.Errorf("failed to commit batch at block %d: %w", blockInfo.Number, err)
			}
			batch = m.targetDB.NewBatch()
			fmt.Printf("Migrated %d blocks...\n", i+1)
		}
	}

	// Final commit
	if err := batch.Commit(pebble.Sync); err != nil {
		return fmt.Errorf("failed to commit final batch: %w", err)
	}

	// Step 3: Migrate state data
	if err := m.migrateState(); err != nil {
		return fmt.Errorf("failed to migrate state: %w", err)
	}

	fmt.Println("Migration complete!")
	return nil
}

type blockInfo struct {
	Number uint64
	Hash   common.Hash
}

// findAllBlocks discovers all blocks in the SubnetEVM database
func (m *SubnetToCChain) findAllBlocks() ([]blockInfo, error) {
	blocks := []blockInfo{}
	
	// Look for canonical mappings (number->hash)
	// Pattern: 0x33...68<8-byte-number>6e -> 32-byte hash
	iter, err := m.sourceDB.NewIter(&pebble.IterOptions{
		LowerBound: []byte{0x33}, // SubnetEVM prefix
		UpperBound: []byte{0x34}, // Next prefix
	})
	if err != nil {
		return nil, err
	}
	defer iter.Close()

	for iter.First(); iter.Valid(); iter.Next() {
		key := iter.Key()
		value := iter.Value()

		// Check if this looks like a canonical mapping
		// Pattern: 0x33[31 bytes]68[8-byte-number]6e -> 32-byte hash
		if len(key) == 42 && key[0] == 0x33 && key[32] == 0x68 && key[41] == 0x6e && len(value) == 32 {
			// Extract block number (8 bytes starting at position 33)
			number := binary.BigEndian.Uint64(key[33:41])
			hash := common.BytesToHash(value)
			
			blocks = append(blocks, blockInfo{
				Number: number,
				Hash:   hash,
			})
		}
	}

	// Sort by block number
	sort.Slice(blocks, func(i, j int) bool {
		return blocks[i].Number < blocks[j].Number
	})

	return blocks, nil
}

// migrateBlock migrates a single block and its associated data
func (m *SubnetToCChain) migrateBlock(info blockInfo, batch *pebble.Batch) error {
	// Construct SubnetEVM header key
	// Pattern: 0x33[31 bytes padding]68<8-byte-number><32-byte-hash>
	// The padding is a specific pattern used by SubnetEVM
	padding := []byte{
		0x7f, 0xb7, 0x3f, 0x9b, 0xcd, 0xac, 0x8c, 0x31,
		0xa2, 0xd5, 0xf7, 0xb8, 0x77, 0xab, 0x1e, 0x8a,
		0x2b, 0x7f, 0x2a, 0x1e, 0x9b, 0xf0, 0x2a, 0x0a,
		0x0e, 0x6c, 0x6f, 0xd1, 0x64, 0xf1, 0xd1,
	}
	
	subnetHeaderKey := make([]byte, 73)
	subnetHeaderKey[0] = 0x33
	copy(subnetHeaderKey[1:32], padding)
	subnetHeaderKey[32] = 0x68 // 'h' for header
	
	numBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(numBytes, info.Number)
	copy(subnetHeaderKey[33:41], numBytes)
	copy(subnetHeaderKey[41:73], info.Hash.Bytes())

	// Get header data
	headerData, closer, err := m.sourceDB.Get(subnetHeaderKey)
	if err != nil {
		return fmt.Errorf("header not found for block %d: %w", info.Number, err)
	}
	defer closer.Close()

	// Decode header using SubnetEVM format
	var subnetHeader SubnetEVMHeader
	if err := rlp.DecodeBytes(headerData, &subnetHeader); err != nil {
		return fmt.Errorf("failed to decode header: %w", err)
	}
	
	// Convert to standard header for verification
	header := subnetHeader.ToStandardHeader()
	
	// Re-encode as standard header for C-chain
	standardHeaderData, err := rlp.EncodeToBytes(header)
	if err != nil {
		return fmt.Errorf("failed to re-encode header: %w", err)
	}

	// Create C-chain canonical hash key
	canonicalKey := append([]byte("H"), numBytes...)
	if err := batch.Set(canonicalKey, info.Hash.Bytes(), nil); err != nil {
		return err
	}

	// Create C-chain header key with standard header data
	headerKey := append([]byte("h"), append(info.Hash.Bytes(), numBytes...)...)
	if err := batch.Set(headerKey, standardHeaderData, nil); err != nil {
		return err
	}

	// Try to get body (if exists)
	subnetBodyKey := make([]byte, len(subnetHeaderKey))
	copy(subnetBodyKey, subnetHeaderKey)
	subnetBodyKey[32] = 0x62 // 'b' for body

	if bodyData, closer, err := m.sourceDB.Get(subnetBodyKey); err == nil {
		defer closer.Close()
		bodyKey := append([]byte("b"), append(info.Hash.Bytes(), numBytes...)...)
		if err := batch.Set(bodyKey, bodyData, nil); err != nil {
			return err
		}
	}

	// Try to get receipts (if exists)
	subnetReceiptKey := make([]byte, len(subnetHeaderKey))
	copy(subnetReceiptKey, subnetHeaderKey)
	subnetReceiptKey[32] = 0x72 // 'r' for receipts

	if receiptData, closer, err := m.sourceDB.Get(subnetReceiptKey); err == nil {
		defer closer.Close()
		receiptKey := append([]byte("r"), append(info.Hash.Bytes(), numBytes...)...)
		if err := batch.Set(receiptKey, receiptData, nil); err != nil {
			return err
		}
	}

	return nil
}

// migrateState migrates state trie data
func (m *SubnetToCChain) migrateState() error {
	fmt.Println("Migrating state data...")
	
	// SubnetEVM state data uses prefix 0x337f
	iter, err := m.sourceDB.NewIter(&pebble.IterOptions{
		LowerBound: []byte{0x33, 0x7f},
		UpperBound: []byte{0x33, 0x80},
	})
	if err != nil {
		return err
	}
	defer iter.Close()

	batch := m.targetDB.NewBatch()
	count := 0

	for iter.First(); iter.Valid(); iter.Next() {
		key := iter.Key()
		value := iter.Value()

		// Convert SubnetEVM state key to C-chain format
		// Remove the 0x33 prefix and use 's' prefix instead
		if len(key) > 2 {
			cchainKey := append([]byte("s"), key[2:]...)
			if err := batch.Set(cchainKey, value, nil); err != nil {
				return err
			}
		}

		count++
		if count%10000 == 0 {
			if err := batch.Commit(pebble.Sync); err != nil {
				return fmt.Errorf("failed to commit state batch: %w", err)
			}
			batch = m.targetDB.NewBatch()
			fmt.Printf("Migrated %d state entries...\n", count)
		}
	}

	if err := batch.Commit(pebble.Sync); err != nil {
		return fmt.Errorf("failed to commit final state batch: %w", err)
	}

	fmt.Printf("Migrated %d state entries\n", count)
	return nil
}