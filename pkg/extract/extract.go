package extract

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"

	"github.com/luxfi/genesis/pkg/application"
	"github.com/luxfi/geth/common"
	"github.com/luxfi/geth/core/types"
	"github.com/luxfi/geth/ethdb"
	"github.com/luxfi/geth/rlp"
)

// Options holds configuration for extraction operations
type Options struct {
	Format     string
	StartBlock uint64
	EndBlock   uint64
	Network    string
}

// Extractor handles blockchain data extraction from a generic database.
type Extractor struct {
	app *application.Genesis
	db  ethdb.Database
}

// New creates a new Extractor instance with a given database.
func New(app *application.Genesis, db ethdb.Database) *Extractor {
	return &Extractor{app: app, db: db}
}

// ExtractBlockchain extracts blockchain data from the database.
func (e *Extractor) ExtractBlockchain(outputPath string, opts Options) error {
	e.app.Log.Info("Extracting blockchain data", "output", outputPath, "format", opts.Format)

	switch opts.Format {
	case "json":
		return e.extractToJSON(outputPath, opts)
	default:
		return fmt.Errorf("unsupported format: %s", opts.Format)
	}
}

// ExtractGenesis extracts the genesis data from the database.
func (e *Extractor) ExtractGenesis(outputPath string) error {
	e.app.Log.Info("Extracting genesis data", "output", outputPath)

	genesisBlock, err := e.readBlock(0)
	if err != nil {
		return fmt.Errorf("failed to read genesis block: %w", err)
	}

	// TODO: Extract state from genesis block
	data, err := json.MarshalIndent(genesisBlock, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal genesis: %w", err)
	}

	return os.WriteFile(outputPath, data, 0644)
}

func (e *Extractor) extractToJSON(outputPath string, opts Options) error {
	outFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer outFile.Close()

	encoder := json.NewEncoder(outFile)
	encoder.SetIndent("", "  ")

	var blocks []*types.Block
	blockNum := opts.StartBlock
	for {
		if opts.EndBlock > 0 && blockNum > opts.EndBlock {
			break
		}

		block, err := e.readBlock(blockNum)
		if err != nil {
			if blockNum == opts.StartBlock {
				return fmt.Errorf("failed to read first block %d: %w", blockNum, err)
			}
			break // No more blocks
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

func (e *Extractor) readBlock(number uint64) (*types.Block, error) {
	hashKey := append([]byte("H"), encodeBlockNumber(number)...)
	hashData, err := e.db.Get(hashKey)
	if err != nil {
		return nil, err
	}
	hash := common.BytesToHash(hashData)

	headerKey := append([]byte("h"), hash.Bytes()...)
	headerKey = append(headerKey, encodeBlockNumber(number)...)
	headerData, err := e.db.Get(headerKey)
	if err != nil {
		return nil, err
	}

	var header types.Header
	if err := rlp.DecodeBytes(headerData, &header); err != nil {
		return nil, err
	}

	bodyKey := append([]byte("b"), hash.Bytes()...)
	bodyKey = append(bodyKey, encodeBlockNumber(number)...)
	bodyData, err := e.db.Get(bodyKey)
	if err != nil {
		return nil, err
	}

	var body types.Body
	if err := rlp.DecodeBytes(bodyData, &body); err != nil {
		return nil, err
	}

	return types.NewBlockWithHeader(&header).WithBody(&body), nil
}

func encodeBlockNumber(number uint64) []byte {
	enc := make([]byte, 8)
	binary.BigEndian.PutUint64(enc, number)
	return enc
}
