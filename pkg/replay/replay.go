package replay

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"sort"

	"github.com/luxfi/database"
	"github.com/luxfi/database/manager"
	"github.com/luxfi/genesis/pkg/application"
	"github.com/luxfi/geth/common"
	"github.com/luxfi/geth/core/types"
	"github.com/luxfi/geth/rlp"
	"github.com/prometheus/client_golang/prometheus"
)

// Replayer handles blockchain replay operations
type Replayer struct {
	app *application.Genesis
}

// Options for replay operations
type Options struct {
	RPC      string
	Start    uint64
	End      uint64
	DirectDB bool
	Output   string
}

// New creates a new Replayer instance
func New(app *application.Genesis) *Replayer {
	return &Replayer{app: app}
}

// openDatabase opens a database for replay operations
func (r *Replayer) openDatabase(dbPath string, readOnly bool) (database.Database, error) {
	// Auto-detect database type
	dbType := r.detectDatabaseType(dbPath)

	// Create database manager
	dbManager := manager.NewManager(filepath.Dir(dbPath), prometheus.NewRegistry())

	// Configure database
	config := &manager.Config{
		Type:      dbType,
		Path:      filepath.Base(dbPath),
		Namespace: "replay",
		CacheSize: 512, // MB
		HandleCap: 1024,
		ReadOnly:  readOnly,
	}

	return dbManager.New(config)
}

// detectDatabaseType tries to determine the database type
func (r *Replayer) detectDatabaseType(dbPath string) string {
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

// ReplayBlocks replays blockchain blocks from source to destination
func (r *Replayer) ReplayBlocks(sourceDB string, opts Options) error {
	r.app.Log.Info("Replaying blocks", "source", sourceDB, "rpc", opts.RPC)

	// Open source database
	db, err := r.openDatabase(sourceDB, true)
	if err != nil {
		return fmt.Errorf("failed to open source database: %w", err)
	}
	defer db.Close()

	if opts.DirectDB {
		return r.replayDirectToDB(db, opts)
	}

	return r.replayViaRPC(db, opts)
}

func (r *Replayer) replayDirectToDB(db database.Database, opts Options) error {
	if opts.Output == "" {
		return fmt.Errorf("output path required for direct DB mode")
	}

	r.app.Log.Info("Direct database replay", "output", opts.Output)

	// Create output database
	outDB, err := r.openDatabase(opts.Output, false)
	if err != nil {
		return fmt.Errorf("failed to create output database: %w", err)
	}
	defer outDB.Close()

	// Get all canonical block hashes
	canonicalBlocks := make(map[uint64]common.Hash)
	blockNumbers := []uint64{}

	iter := db.NewIterator()
	defer iter.Release()

	// First pass: collect all canonical blocks
	for iter.Next() {
		key := iter.Key()
		if len(key) == 10 && key[0] == 'H' { // Canonical hash prefix
			num := binary.BigEndian.Uint64(key[1:9])
			if (opts.Start == 0 || num >= opts.Start) && (opts.End == 0 || num <= opts.End) {
				hash := common.BytesToHash(iter.Value())
				canonicalBlocks[num] = hash
				blockNumbers = append(blockNumbers, num)
			}
		}
	}

	sort.Slice(blockNumbers, func(i, j int) bool { return blockNumbers[i] < blockNumbers[j] })
	r.app.Log.Info("Found blocks to replay", "count", len(blockNumbers))

	// Second pass: copy block data
	batch := outDB.NewBatch()
	count := 0

	for _, num := range blockNumbers {
		hash := canonicalBlocks[num]

		// Copy canonical hash entry
		canonicalKey := append([]byte("H"), encodeBlockNumber(num)...)
		if err := batch.Put(canonicalKey, hash.Bytes()); err != nil {
			return fmt.Errorf("failed to set canonical hash: %w", err)
		}

		// Copy header
		headerKey := append([]byte("h"), append(hash.Bytes(), encodeBlockNumber(num)...)...)
		if headerData, err := db.Get(headerKey); err == nil {
			if err := batch.Put(headerKey, headerData); err != nil {
				return fmt.Errorf("failed to set header: %w", err)
			}
		}

		// Copy body
		bodyKey := append([]byte("b"), append(hash.Bytes(), encodeBlockNumber(num)...)...)
		if bodyData, err := db.Get(bodyKey); err == nil {
			if err := batch.Put(bodyKey, bodyData); err != nil {
				return fmt.Errorf("failed to set body: %w", err)
			}
		}

		// Copy receipts
		receiptKey := append([]byte("r"), append(hash.Bytes(), encodeBlockNumber(num)...)...)
		if receiptData, err := db.Get(receiptKey); err == nil {
			if err := batch.Put(receiptKey, receiptData); err != nil {
				return fmt.Errorf("failed to set receipt: %w", err)
			}
		}

		count++
		if count%1000 == 0 {
			if err := batch.Write(); err != nil {
				return fmt.Errorf("failed to commit batch: %w", err)
			}
			batch.Reset()
			r.app.Log.Info("Replay progress", "blocks", count)
		}
	}

	if err := batch.Write(); err != nil {
		return fmt.Errorf("failed to commit final batch: %w", err)
	}

	r.app.Log.Info("Replay complete", "blocks", count)
	return nil
}

func (r *Replayer) replayViaRPC(db database.Database, opts Options) error {
	// Check RPC connection
	if err := r.checkRPCConnection(opts.RPC); err != nil {
		return fmt.Errorf("RPC check failed: %w", err)
	}

	// Get current chain head
	chainHead, err := r.getChainHead(opts.RPC)
	if err != nil {
		return fmt.Errorf("failed to get chain head: %w", err)
	}

	r.app.Log.Info("Current chain head", "block", chainHead)

	// Find blocks to replay
	canonicalBlocks := make(map[uint64]common.Hash)
	blockNumbers := []uint64{}

	iter := db.NewIterator()
	defer iter.Release()

	for iter.Next() {
		key := iter.Key()
		if len(key) == 10 && key[0] == 'H' { // Canonical hash prefix
			num := binary.BigEndian.Uint64(key[1:9])
			if num > chainHead && (opts.Start == 0 || num >= opts.Start) && (opts.End == 0 || num <= opts.End) {
				hash := common.BytesToHash(iter.Value())
				canonicalBlocks[num] = hash
				blockNumbers = append(blockNumbers, num)
			}
		}
	}

	sort.Slice(blockNumbers, func(i, j int) bool { return blockNumbers[i] < blockNumbers[j] })
	r.app.Log.Info("Blocks to replay", "count", len(blockNumbers))

	// Replay blocks
	for i, num := range blockNumbers {
		hash := canonicalBlocks[num]

		// Get block data
		block, err := r.getBlock(db, num, hash)
		if err != nil {
			r.app.Log.Error("Failed to get block", "number", num, "error", err)
			continue
		}

		// Submit block via RPC
		if err := r.submitBlock(opts.RPC, block); err != nil {
			r.app.Log.Error("Failed to submit block", "number", num, "error", err)
			return err
		}

		if i%100 == 0 {
			r.app.Log.Info("Replay progress", "block", num, "progress", fmt.Sprintf("%d/%d", i+1, len(blockNumbers)))
		}
	}

	r.app.Log.Info("Replay complete")
	return nil
}

func (r *Replayer) checkRPCConnection(rpcURL string) error {
	// Simple eth_blockNumber call to check connection
	payload := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "eth_blockNumber",
		"params":  []interface{}{},
		"id":      1,
	}

	resp, err := r.rpcCall(rpcURL, payload)
	if err != nil {
		return err
	}

	if resp["error"] != nil {
		return fmt.Errorf("RPC error: %v", resp["error"])
	}

	return nil
}

func (r *Replayer) getChainHead(rpcURL string) (uint64, error) {
	payload := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "eth_blockNumber",
		"params":  []interface{}{},
		"id":      1,
	}

	resp, err := r.rpcCall(rpcURL, payload)
	if err != nil {
		return 0, err
	}

	if resp["error"] != nil {
		return 0, fmt.Errorf("RPC error: %v", resp["error"])
	}

	result, ok := resp["result"].(string)
	if !ok {
		return 0, fmt.Errorf("unexpected result type")
	}

	// Convert hex string to uint64
	var blockNum uint64
	_, _ = fmt.Sscanf(result, "0x%x", &blockNum)
	return blockNum, nil
}

func (r *Replayer) getBlock(db database.Database, num uint64, hash common.Hash) (*types.Block, error) {
	// Get header
	headerKey := append([]byte("h"), append(hash.Bytes(), encodeBlockNumber(num)...)...)
	headerData, err := db.Get(headerKey)
	if err != nil {
		return nil, fmt.Errorf("header not found: %w", err)
	}

	var header types.Header
	if err := rlp.DecodeBytes(headerData, &header); err != nil {
		return nil, fmt.Errorf("failed to decode header: %w", err)
	}

	// Get body
	bodyKey := append([]byte("b"), append(hash.Bytes(), encodeBlockNumber(num)...)...)
	bodyData, err := db.Get(bodyKey)
	if err != nil {
		return nil, fmt.Errorf("body not found: %w", err)
	}

	var body types.Body
	if err := rlp.DecodeBytes(bodyData, &body); err != nil {
		return nil, fmt.Errorf("failed to decode body: %w", err)
	}

	return types.NewBlockWithHeader(&header).WithBody(body), nil
}

func (r *Replayer) submitBlock(rpcURL string, block *types.Block) error {
	// Encode block to RLP
	var buf bytes.Buffer
	if err := block.EncodeRLP(&buf); err != nil {
		return fmt.Errorf("failed to encode block: %w", err)
	}

	// Submit via debug_setHead or similar API
	payload := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "debug_importBlock",
		"params":  []interface{}{"0x" + hex.EncodeToString(buf.Bytes())},
		"id":      1,
	}

	resp, err := r.rpcCall(rpcURL, payload)
	if err != nil {
		return err
	}

	if resp["error"] != nil {
		return fmt.Errorf("RPC error: %v", resp["error"])
	}

	return nil
}

func (r *Replayer) rpcCall(url string, payload interface{}) (map[string]interface{}, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}

	resp, err := http.Post(url, "application/json", bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return result, nil
}

func encodeBlockNumber(number uint64) []byte {
	enc := make([]byte, 8)
	binary.BigEndian.PutUint64(enc, number)
	return enc
}
