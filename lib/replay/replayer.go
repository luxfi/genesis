// Package replay handles replaying blockchain data from one database to another
package replay

import (
	"encoding/binary"
	"fmt"
	"log"
	"time"

	"github.com/luxfi/database"
)

// Config holds replay configuration
type Config struct {
	ChainID    uint64
	StartBlock uint64
	EndBlock   uint64
	BatchSize  int
}

// Results contains replay results
type Results struct {
	BlocksProcessed int
	BlocksWritten   int
	StateEntries    int
	Duration        time.Duration
}

// Replayer handles chain replay operations
type Replayer struct {
	srcDB  database.Database
	dstDB  database.Database
	config *Config
}

// NewReplayer creates a new replayer instance
func NewReplayer(src, dst database.Database, config *Config) *Replayer {
	return &Replayer{
		srcDB:  src,
		dstDB:  dst,
		config: config,
	}
}

// Execute performs the chain replay
func (r *Replayer) Execute() (*Results, error) {
	start := time.Now()
	results := &Results{}

	// Determine block range
	endBlock := r.config.EndBlock
	if endBlock == 0 {
		latest, err := r.findLatestBlock()
		if err != nil {
			return nil, fmt.Errorf("failed to find latest block: %w", err)
		}
		endBlock = latest
	}

	log.Printf("Replaying blocks %d to %d", r.config.StartBlock, endBlock)

	// Process in batches
	for blockNum := r.config.StartBlock; blockNum <= endBlock; {
		batchEnd := blockNum + uint64(r.config.BatchSize)
		if batchEnd > endBlock {
			batchEnd = endBlock
		}

		if err := r.replayBatch(blockNum, batchEnd, results); err != nil {
			return nil, fmt.Errorf("failed to replay batch %d-%d: %w", blockNum, batchEnd, err)
		}

		blockNum = batchEnd + 1
		
		// Progress update
		if results.BlocksProcessed%10000 == 0 {
			log.Printf("Progress: %d blocks processed", results.BlocksProcessed)
		}
	}

	// Copy state data
	log.Printf("Copying state data...")
	if err := r.copyStateData(results); err != nil {
		return nil, fmt.Errorf("failed to copy state data: %w", err)
	}

	// Copy additional chain data
	if err := r.copyChainMetadata(); err != nil {
		return nil, fmt.Errorf("failed to copy chain metadata: %w", err)
	}

	results.Duration = time.Since(start)
	return results, nil
}

// replayBatch processes a batch of blocks
func (r *Replayer) replayBatch(start, end uint64, results *Results) error {
	batch := r.dstDB.NewBatch()
	defer batch.Close()

	for blockNum := start; blockNum <= end; blockNum++ {
		// Copy canonical hash mapping
		hashKey := append(headerHashSuffix, encodeBlockNumber(blockNum)...)
		hash, err := r.srcDB.Get(hashKey)
		if err != nil {
			continue // Skip missing blocks
		}

		if err := batch.Put(hashKey, hash); err != nil {
			return err
		}

		// Copy header
		headerKey := makeHeaderKey(blockNum, hash)
		header, err := r.srcDB.Get(headerKey)
		if err != nil {
			continue
		}

		if err := batch.Put(headerKey, header); err != nil {
			return err
		}

		// Copy block body
		bodyKey := makeBodyKey(blockNum, hash)
		if body, err := r.srcDB.Get(bodyKey); err == nil {
			if err := batch.Put(bodyKey, body); err != nil {
				return err
			}
		}

		// Copy receipts
		receiptsKey := makeReceiptsKey(blockNum, hash)
		if receipts, err := r.srcDB.Get(receiptsKey); err == nil {
			if err := batch.Put(receiptsKey, receipts); err != nil {
				return err
			}
		}

		// Copy TD (total difficulty)
		tdKey := makeTDKey(blockNum, hash)
		if td, err := r.srcDB.Get(tdKey); err == nil {
			if err := batch.Put(tdKey, td); err != nil {
				return err
			}
		}

		results.BlocksProcessed++
		results.BlocksWritten++
	}

	return batch.Write()
}

// copyStateData copies state trie data
func (r *Replayer) copyStateData(results *Results) error {
	iter := r.srcDB.NewIterator()
	defer iter.Release()

	batch := r.dstDB.NewBatch()
	defer batch.Close()

	count := 0
	
	// State data typically uses specific prefixes
	statePrefix := []byte{0x00} // Account trie nodes
	codePrefix := []byte{0x63}  // Contract code
	
	// Copy state trie nodes
	iter.Seek(statePrefix)
	for iter.Next() {
		key := iter.Key()
		
		// Check if we're still in state data range
		if len(key) == 0 || key[0] != 0x00 {
			break
		}

		if err := batch.Put(key, iter.Value()); err != nil {
			return err
		}

		count++
		if count%10000 == 0 {
			// Write batch periodically
			if err := batch.Write(); err != nil {
				return err
			}
			batch.Reset()
			log.Printf("Copied %d state entries", count)
		}
	}

	// Copy contract code
	iter.Seek(codePrefix)
	for iter.Next() {
		key := iter.Key()
		
		if len(key) == 0 || key[0] != 0x63 {
			break
		}

		if err := batch.Put(key, iter.Value()); err != nil {
			return err
		}
		count++
	}

	if err := batch.Write(); err != nil {
		return err
	}

	results.StateEntries = count
	return iter.Error()
}

// copyChainMetadata copies chain configuration and metadata
func (r *Replayer) copyChainMetadata() error {
	// Important metadata keys
	metadataKeys := [][]byte{
		[]byte("LastBlock"),
		[]byte("LastFast"),
		[]byte("LastPivot"),
		[]byte("lastFinalized"),
		[]byte("eth-config-"), // Chain config prefix
	}

	batch := r.dstDB.NewBatch()
	defer batch.Close()

	for _, key := range metadataKeys {
		if value, err := r.srcDB.Get(key); err == nil {
			if err := batch.Put(key, value); err != nil {
				return err
			}
		}
	}

	// Copy chain config
	iter := r.srcDB.NewIterator()
	defer iter.Release()
	
	configPrefix := []byte("eth-config-")
	iter.Seek(configPrefix)
	
	for iter.Next() {
		key := iter.Key()
		if !startsWith(key, configPrefix) {
			break
		}
		
		if err := batch.Put(key, iter.Value()); err != nil {
			return err
		}
	}

	return batch.Write()
}

// findLatestBlock finds the highest block number in source DB
func (r *Replayer) findLatestBlock() (uint64, error) {
	iter := r.srcDB.NewIterator()
	defer iter.Release()
	
	var maxBlock uint64
	
	// Look for canonical hash entries
	iter.Seek(headerHashSuffix)
	for iter.Next() {
		key := iter.Key()
		
		if !startsWith(key, headerHashSuffix) {
			break
		}
		
		if len(key) == len(headerHashSuffix)+8 {
			blockNum := binary.BigEndian.Uint64(key[len(headerHashSuffix):])
			if blockNum > maxBlock {
				maxBlock = blockNum
			}
		}
	}
	
	return maxBlock, iter.Error()
}

// Database key prefixes (matching go-ethereum)
var (
	headerPrefix       = []byte{0x30} // headerPrefix + num + hash -> header
	headerHashSuffix   = []byte{0x32} // headerHashSuffix + num -> hash
	headerNumberPrefix = []byte{0x33} // headerNumberPrefix + hash -> num
	blockBodyPrefix    = []byte{0x31} // blockBodyPrefix + num + hash -> body
	blockReceiptsPrefix = []byte{0x34} // receiptsPrefix + num + hash -> receipts
	headerTDSuffix     = []byte{0x35} // headerTDSuffix + num + hash -> TD
)

// Helper functions
func encodeBlockNumber(number uint64) []byte {
	enc := make([]byte, 8)
	binary.BigEndian.PutUint64(enc, number)
	return enc
}

func makeHeaderKey(number uint64, hash []byte) []byte {
	return append(append(headerPrefix, encodeBlockNumber(number)...), hash...)
}

func makeBodyKey(number uint64, hash []byte) []byte {
	return append(append(blockBodyPrefix, encodeBlockNumber(number)...), hash...)
}

func makeReceiptsKey(number uint64, hash []byte) []byte {
	return append(append(blockReceiptsPrefix, encodeBlockNumber(number)...), hash...)
}

func makeTDKey(number uint64, hash []byte) []byte {
	return append(append(headerTDSuffix, encodeBlockNumber(number)...), hash...)
}

func startsWith(data, prefix []byte) bool {
	return len(data) >= len(prefix) && string(data[:len(prefix)]) == string(prefix)
}