package extract

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"github.com/cockroachdb/pebble"
	"github.com/luxfi/genesis/pkg/application"
	"github.com/luxfi/geth/common"
	"github.com/luxfi/geth/core/types"
	"github.com/luxfi/geth/rlp"
)

// Options holds configuration for extraction operations
type Options struct {
	Format     string
	StartBlock uint64
	EndBlock   uint64
	Network    string
}

// Extractor handles blockchain data extraction
type Extractor struct {
	app *application.Genesis
}

// New creates a new Extractor instance
func New(app *application.Genesis) *Extractor {
	return &Extractor{app: app}
}

// ExtractBlockchain extracts blockchain data from SubnetEVM format
func (e *Extractor) ExtractBlockchain(dbPath, outputPath string, opts Options) error {
	e.app.Log.Info("Extracting blockchain data", "db", dbPath, "output", outputPath, "format", opts.Format)

	// Open the database
	db, err := pebble.Open(dbPath, &pebble.Options{})
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	switch opts.Format {
	case "bytes":
		return e.extractToBytes(db, outputPath, opts)
	case "json":
		return e.extractToJSON(db, outputPath, opts)
	case "coreth":
		return e.extractToCoreth(db, outputPath, opts)
	default:
		return fmt.Errorf("unsupported format: %s", opts.Format)
	}
}

// ExtractGenesis extracts genesis data from a blockchain database
func (e *Extractor) ExtractGenesis(dbPath, outputPath string) error {
	e.app.Log.Info("Extracting genesis data", "db", dbPath, "output", outputPath)

	// Open the database
	db, err := pebble.Open(dbPath, &pebble.Options{})
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	// Extract genesis block
	genesisBlock, err := e.readBlock(db, 0)
	if err != nil {
		return fmt.Errorf("failed to read genesis block: %w", err)
	}

	// TODO: Extract state from genesis block
	// For now, just save the block
	data, err := json.MarshalIndent(genesisBlock, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal genesis: %w", err)
	}

	return os.WriteFile(outputPath, data, 0644)
}

// ExtractState extracts state data from a blockchain database
func (e *Extractor) ExtractState(dbPath, outputPath string, blockNumber uint64, includeCode bool) error {
	e.app.Log.Info("Extracting state data", "db", dbPath, "output", outputPath, "block", blockNumber)

	// Open the database
	db, err := pebble.Open(dbPath, &pebble.Options{})
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	// TODO: Implement state extraction
	// This requires iterating through the state trie at the given block
	return fmt.Errorf("state extraction not yet implemented")
}

func (e *Extractor) extractToBytes(db *pebble.DB, outputPath string, opts Options) error {
	// Create output file
	outFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer outFile.Close()

	// Extract blocks in byte format
	blockNum := opts.StartBlock
	for {
		if opts.EndBlock > 0 && blockNum > opts.EndBlock {
			break
		}

		block, err := e.readBlock(db, blockNum)
		if err != nil {
			if blockNum == opts.StartBlock {
				return fmt.Errorf("failed to read first block: %w", err)
			}
			// No more blocks
			break
		}

		// Encode block to RLP
		data, err := rlp.EncodeToBytes(block)
		if err != nil {
			return fmt.Errorf("failed to encode block %d: %w", blockNum, err)
		}

		// Write length prefix and data
		if err := binary.Write(outFile, binary.BigEndian, uint32(len(data))); err != nil {
			return fmt.Errorf("failed to write length: %w", err)
		}
		if _, err := outFile.Write(data); err != nil {
			return fmt.Errorf("failed to write block data: %w", err)
		}

		blockNum++
	}

	e.app.Log.Info("Extraction complete", "blocks", blockNum-opts.StartBlock)
	return nil
}

func (e *Extractor) extractToJSON(db *pebble.DB, outputPath string, opts Options) error {
	// Create output file
	outFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer outFile.Close()

	encoder := json.NewEncoder(outFile)
	encoder.SetIndent("", "  ")

	// Extract blocks in JSON format
	var blocks []*types.Block
	blockNum := opts.StartBlock
	for {
		if opts.EndBlock > 0 && blockNum > opts.EndBlock {
			break
		}

		block, err := e.readBlock(db, blockNum)
		if err != nil {
			if blockNum == opts.StartBlock {
				return fmt.Errorf("failed to read first block: %w", err)
			}
			// No more blocks
			break
		}

		blocks = append(blocks, block)
		blockNum++
	}

	if err := encoder.Encode(blocks); err != nil {
		return fmt.Errorf("failed to encode blocks: %w", err)
	}

	e.app.Log.Info("Extraction complete", "blocks", len(blocks))
	return nil
}

func (e *Extractor) extractToCoreth(db *pebble.DB, outputPath string, opts Options) error {
	// TODO: Implement C-Chain compatible format extraction
	return fmt.Errorf("coreth format extraction not yet implemented")
}

func (e *Extractor) readBlock(db *pebble.DB, number uint64) (*types.Block, error) {
	// Read block hash
	hashKey := append([]byte("H"), encodeBlockNumber(number)...)
	hashData, closer, err := db.Get(hashKey)
	if err != nil {
		return nil, err
	}
	hash := common.BytesToHash(hashData)
	closer.Close()

	// Read block header
	headerKey := append([]byte("h"), hash.Bytes()...)
	headerKey = append(headerKey, encodeBlockNumber(number)...)
	headerData, closer, err := db.Get(headerKey)
	if err != nil {
		return nil, err
	}
	defer closer.Close()

	var header types.Header
	if err := rlp.DecodeBytes(headerData, &header); err != nil {
		return nil, err
	}

	// Read block body
	bodyKey := append([]byte("b"), hash.Bytes()...)
	bodyKey = append(bodyKey, encodeBlockNumber(number)...)
	bodyData, closer, err := db.Get(bodyKey)
	if err != nil {
		return nil, err
	}
	defer closer.Close()

	var body types.Body
	if err := rlp.DecodeBytes(bodyData, &body); err != nil {
		return nil, err
	}

	return types.NewBlockWithHeader(&header).WithBody(body), nil
}

func encodeBlockNumber(number uint64) []byte {
	enc := make([]byte, 8)
	binary.BigEndian.PutUint64(enc, number)
	return enc
}

// PrintKeys prints all keys in the database (for debugging)
func (e *Extractor) PrintKeys(dbPath string, limit int) error {
	db, err := pebble.Open(dbPath, &pebble.Options{})
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	iter, err := db.NewIter(nil)
	if err != nil {
		return fmt.Errorf("failed to create iterator: %w", err)
	}
	defer iter.Close()

	type keyInfo struct {
		key    string
		prefix string
		size   int
	}

	var keys []keyInfo
	for iter.First(); iter.Valid(); iter.Next() {
		key := iter.Key()
		if len(key) > 0 {
			keys = append(keys, keyInfo{
				key:    hex.EncodeToString(key),
				prefix: string(key[0]),
				size:   len(iter.Value()),
			})
		}
		if limit > 0 && len(keys) >= limit {
			break
		}
	}

	// Sort by prefix for better readability
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].prefix != keys[j].prefix {
			return keys[i].prefix < keys[j].prefix
		}
		return keys[i].key < keys[j].key
	})

	fmt.Printf("Found %d keys:\n", len(keys))
	for _, k := range keys {
		fmt.Printf("  %s: %s (size: %d)\n", k.prefix, k.key, k.size)
	}

	return nil
}
