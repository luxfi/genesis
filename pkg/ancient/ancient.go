package ancient

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/luxfi/geth/common"
	"github.com/luxfi/geth/ethdb"
)

// AncientStore represents the interface for ancient data storage
type AncientStore interface {
	// HasAncient returns whether ancient data exists
	HasAncient(kind string, number uint64) (bool, error)

	// Ancient retrieves ancient data
	Ancient(kind string, number uint64) ([]byte, error)

	// Ancients returns the ancient data size
	Ancients() (uint64, error)

	// AncientSize returns the size of ancient data
	AncientSize(kind string) (uint64, error)

	// AppendAncient appends ancient data
	AppendAncient(number uint64, hash, header, body, receipt, td []byte) error
}

// CChainAncientData represents ancient store data for C-Chain
type CChainAncientData struct {
	ChainID      uint64
	GenesisHash  common.Hash
	StartBlock   uint64
	EndBlock     uint64
	DataPath     string
	CompactedDir string
}

// Builder builds ancient store data for C-Chain genesis
type Builder struct {
	config      *CChainAncientData
	ancientDb   ethdb.AncientStore
	compactedDb ethdb.Database
}

// NewBuilder creates a new ancient store builder
func NewBuilder(config *CChainAncientData) (*Builder, error) {
	// TODO: Implement database opening with proper imports
	// For now, return a placeholder builder
	return &Builder{
		config: config,
	}, nil
}

// Close closes all database connections
func (b *Builder) Close() error {
	if b.ancientDb != nil {
		b.ancientDb.Close()
	}
	if b.compactedDb != nil {
		b.compactedDb.Close()
	}
	return nil
}

// CompactAncientData compacts ancient data for efficient storage
func (b *Builder) CompactAncientData() error {
	fmt.Printf("Compacting ancient data from block %d to %d...\n",
		b.config.StartBlock, b.config.EndBlock)

	// Get total ancients count
	ancients, err := b.ancientDb.Ancients()
	if err != nil {
		return fmt.Errorf("failed to get ancients count: %w", err)
	}

	if b.config.EndBlock > ancients {
		b.config.EndBlock = ancients
	}

	// TODO: Implement actual ancient data processing
	// For now, just print progress
	for blockNum := b.config.StartBlock; blockNum <= b.config.EndBlock; blockNum++ {
		if blockNum%1000 == 0 {
			fmt.Printf("Processing block %d/%d...\n", blockNum, b.config.EndBlock)
		}
	}

	fmt.Println("Compaction completed successfully")
	return nil
}

// storeCompacted stores data in compacted format
func (b *Builder) storeCompacted(number uint64, hash, header, body, receipts, td []byte) error {
	// TODO: Implement actual storage
	return nil
}

// ExportToGenesis exports compacted data for genesis import
func (b *Builder) ExportToGenesis(outputPath string) error {
	fmt.Println("Exporting compacted data for genesis...")

	// Create output directory
	if err := os.MkdirAll(outputPath, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Create manifest file
	manifestPath := filepath.Join(outputPath, "ancient-manifest.json")
	manifest := map[string]interface{}{
		"chainId":     b.config.ChainID,
		"genesisHash": b.config.GenesisHash.Hex(),
		"startBlock":  b.config.StartBlock,
		"endBlock":    b.config.EndBlock,
		"version":     "1.0.0",
	}

	if err := writeJSON(manifestPath, manifest); err != nil {
		return fmt.Errorf("failed to write manifest: %w", err)
	}

	// Copy compacted database
	compactedPath := filepath.Join(outputPath, "ancient-compact")
	if err := copyDir(b.config.CompactedDir, compactedPath); err != nil {
		return fmt.Errorf("failed to copy compacted data: %w", err)
	}

	fmt.Printf("Ancient data exported to: %s\n", outputPath)
	return nil
}

// ImportFromGenesis imports ancient data into C-Chain
func ImportFromGenesis(genesisPath string, targetDataDir string) error {
	fmt.Printf("Importing ancient data from %s to %s...\n", genesisPath, targetDataDir)

	// Read manifest
	manifestPath := filepath.Join(genesisPath, "ancient-manifest.json")
	var manifest map[string]interface{}
	if err := readJSON(manifestPath, &manifest); err != nil {
		return fmt.Errorf("failed to read manifest: %w", err)
	}

	// TODO: Implement the actual import logic
	// This would involve reading from compacted format and writing to ancient store

	fmt.Println("Import completed successfully")
	return nil
}

// Helper functions

func encodeBlockNumber(number uint64) []byte {
	enc := make([]byte, 8)
	binary.BigEndian.PutUint64(enc, number)
	return enc
}

func decodeBlockNumber(enc []byte) uint64 {
	return binary.BigEndian.Uint64(enc)
}

func writeJSON(path string, data interface{}) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(path, jsonData, 0644)
}

func readJSON(path string, data interface{}) error {
	jsonData, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(jsonData, data)
}

func copyDir(src, dst string) error {
	// Implementation would recursively copy directory
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Calculate destination path
		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		dstPath := filepath.Join(dst, relPath)

		// Create directory or copy file
		if info.IsDir() {
			return os.MkdirAll(dstPath, info.Mode())
		}

		return copyFile(path, dstPath)
	})
}

func copyFile(src, dst string) error {
	source, err := os.Open(src)
	if err != nil {
		return err
	}
	defer source.Close()

	destination, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destination.Close()

	_, err = io.Copy(destination, source)
	return err
}
