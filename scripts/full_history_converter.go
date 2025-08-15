package main

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"time"

	"github.com/cockroachdb/pebble"
	"github.com/luxfi/database/badgerdb"
	"github.com/luxfi/geth/common"
)

const (
	// SubnetEVM namespace - 32 bytes
	subnetNamespace = "\x33\x7f\xb7\x3f\x9b\xcd\xac\x8c\x31\xa2\xd5\xf7\xb8\x77\xab\x1e\x8a\x2b\x7f\x2a\x1e\x9b\xf0\x2a\x0a\x0e\x6c\x6f\xd1\x64\xf1\xd1"
)

// ChainConfig for Lux mainnet
type ChainConfig struct {
	ChainID             *big.Int               `json:"chainId"`
	HomesteadBlock      *big.Int               `json:"homesteadBlock"`
	DAOForkBlock        *big.Int               `json:"daoForkBlock"`
	DAOForkSupport      bool                   `json:"daoForkSupport"`
	EIP150Block         *big.Int               `json:"eip150Block"`
	EIP155Block         *big.Int               `json:"eip155Block"`
	EIP158Block         *big.Int               `json:"eip158Block"`
	ByzantiumBlock      *big.Int               `json:"byzantiumBlock"`
	ConstantinopleBlock *big.Int               `json:"constantinopleBlock"`
	PetersburgBlock     *big.Int               `json:"petersburgBlock"`
	IstanbulBlock       *big.Int               `json:"istanbulBlock"`
	MuirGlacierBlock    *big.Int               `json:"muirGlacierBlock"`
	BerlinBlock         *big.Int               `json:"berlinBlock"`
	LondonBlock         *big.Int               `json:"londonBlock"`
	ArrowGlacierBlock   *big.Int               `json:"arrowGlacierBlock"`
	GrayGlacierBlock    *big.Int               `json:"grayGlacierBlock"`
	MergeNetsplitBlock  *big.Int               `json:"mergeNetsplitBlock"`
	ShanghaiTime        *uint64                `json:"shanghaiTime"`
	CancunTime          *uint64                `json:"cancunTime"`
}

func main() {
	fmt.Println("╔════════════════════════════════════════════════════════╗")
	fmt.Println("║   FULL-HISTORY CHAIN BUILD - SubnetEVM → Coreth       ║")
	fmt.Println("╚════════════════════════════════════════════════════════╝")
	fmt.Println()

	// Setup paths
	NET := "96369"
	CID := "EWi9aPkGe6EfJ3SobCAmSUXRPLa4brF3cThwPwmHTrD1y13jy"
	
	// Source paths
	SRC_BLOCKS := "/Users/z/work/lux/state/chaindata/lux-mainnet-96369/db/pebbledb"
	SRC_STATE := "/Users/z/work/lux/state/chaindata/lux-mainnet-96369/db/pebbledb" // Same DB has both
	
	// Destination paths
	DST_BASE := fmt.Sprintf("/Users/z/.luxd/network-%s/chains/%s", NET, CID)
	DST_ETHDB := filepath.Join(DST_BASE, "ethdb")
	DST_VM := filepath.Join(DST_BASE, "vm")
	
	fmt.Printf("Source blocks: %s\n", SRC_BLOCKS)
	fmt.Printf("Source state:  %s\n", SRC_STATE)
	fmt.Printf("Dest ethdb:    %s\n", DST_ETHDB)
	fmt.Printf("Dest vm:       %s\n", DST_VM)
	fmt.Println()

	// Backup existing
	if _, err := os.Stat(DST_ETHDB); err == nil {
		backup := fmt.Sprintf("%s.backup.%d", DST_ETHDB, time.Now().Unix())
		fmt.Printf("Backing up existing to %s\n", backup)
		os.Rename(DST_ETHDB, backup)
	}

	// Open source database
	fmt.Println("Opening source database...")
	srcDB, err := pebble.Open(SRC_BLOCKS, &pebble.Options{ReadOnly: true})
	if err != nil {
		panic(fmt.Sprintf("Failed to open source: %v", err))
	}
	defer srcDB.Close()

	// Create destination database  
	fmt.Println("Creating destination database...")
	os.MkdirAll(DST_BASE, 0755)
	dstDB, err := badgerdb.New(filepath.Clean(DST_ETHDB), nil, "", nil)
	if err != nil {
		panic(fmt.Sprintf("Failed to create destination: %v", err))
	}
	defer dstDB.Close()

	// Statistics
	var (
		totalKeys    int64
		headers      int64
		bodies       int64
		receipts     int64
		canonical    int64
		hashToNum    int64
		stateNodes   int64
		accounts     int64
		storage      int64
		code         int64
		other        int64
		totalBytes   int64
		startTime    = time.Now()
	)

	fmt.Println("\n🔄 Phase 1: Converting blocks...")
	fmt.Println("Converting: headers, bodies, receipts, canonical chain, hash→number")
	
	// Create iterator
	iter, err := srcDB.NewIter(nil)
	if err != nil {
		panic(err)
	}
	defer iter.Close()

	// Use smaller batches for BadgerDB
	batch := dstDB.NewBatch()
	batchSize := 0
	maxBatchSize := 100 * 1024 // 100KB batches

	// Helper to flush batch
	flushBatch := func() error {
		if batchSize > 0 {
			if err := batch.Write(); err != nil {
				// Try single writes on batch failure
				fmt.Printf("\nBatch write failed, trying single writes...\n")
				return err
			}
			batch = dstDB.NewBatch()
			batchSize = 0
		}
		return nil
	}

	// Process all keys
	for iter.First(); iter.Valid(); iter.Next() {
		key := iter.Key()
		value := iter.Value()
		
		if len(key) == 0 || len(value) == 0 {
			continue
		}

		// Strip namespace if present
		destKey := key
		if len(key) > 32 && string(key[:32]) == subnetNamespace {
			destKey = key[32:]
		}

		// Make copies to avoid reuse
		k := make([]byte, len(destKey))
		v := make([]byte, len(value))
		copy(k, destKey)
		copy(v, value)

		// Track what we're converting
		if len(k) > 0 {
			switch k[0] {
			case 'h':
				if len(k) == 41 {
					headers++ // header: h + num(8) + hash(32)
				} else if len(k) == 10 && k[9] == 'n' {
					canonical++ // canonical: h + num(8) + n
				}
			case 'H':
				if len(k) == 33 {
					hashToNum++ // hash to number: H + hash(32)
				}
			case 'b':
				if len(k) == 41 {
					bodies++ // body: b + num(8) + hash(32)
				}
			case 'r':
				if len(k) == 41 {
					receipts++ // receipt: r + num(8) + hash(32)
				}
			case 's', 'S': // state nodes and storage
				stateNodes++
			case 'a': // accounts
				accounts++
			case 'c': // code
				code++
			default:
				// Check for storage keys or other state data
				if len(k) >= 32 {
					storage++
				} else {
					other++
				}
			}
		}

		// Add to batch
		batch.Put(k, v)
		batchSize += len(k) + len(v)
		totalKeys++
		totalBytes += int64(len(k) + len(v))

		// Flush if batch is full
		if batchSize >= maxBatchSize {
			if err := flushBatch(); err != nil {
				// Fall back to single write
				if err := dstDB.Put(k, v); err != nil {
					fmt.Printf("Failed to write key: %v\n", err)
				}
				batch = dstDB.NewBatch()
				batchSize = 0
			}
		}

		// Progress report
		if totalKeys%10000 == 0 {
			elapsed := time.Since(startTime)
			rate := float64(totalKeys) / elapsed.Seconds()
			fmt.Printf("\r  Processed %d keys (%.0f keys/sec) - H:%d B:%d R:%d C:%d State:%d",
				totalKeys, rate, headers, bodies, receipts, canonical, stateNodes)
		}
	}

	// Final batch
	flushBatch()
	
	fmt.Printf("\n✅ Converted %d block-related keys\n", headers+bodies+receipts+canonical+hashToNum)
	fmt.Printf("   Headers:   %d\n", headers)
	fmt.Printf("   Bodies:    %d\n", bodies)  
	fmt.Printf("   Receipts:  %d\n", receipts)
	fmt.Printf("   Canonical: %d\n", canonical)
	fmt.Printf("   Hash→Num:  %d\n", hashToNum)

	if stateNodes > 0 || accounts > 0 || storage > 0 || code > 0 {
		fmt.Printf("\n✅ Converted %d state-related keys\n", stateNodes+accounts+storage+code)
		fmt.Printf("   State Nodes: %d\n", stateNodes)
		fmt.Printf("   Accounts:    %d\n", accounts)
		fmt.Printf("   Storage:     %d\n", storage)
		fmt.Printf("   Code:        %d\n", code)
	}

	// Phase 2: Write chain config
	fmt.Println("\n🔧 Phase 2: Writing chain configuration...")
	
	chainConfig := &ChainConfig{
		ChainID:             big.NewInt(96369),
		HomesteadBlock:      big.NewInt(0),
		DAOForkBlock:        nil,
		DAOForkSupport:      false,
		EIP150Block:         big.NewInt(0),
		EIP155Block:         big.NewInt(0),
		EIP158Block:         big.NewInt(0),
		ByzantiumBlock:      big.NewInt(0),
		ConstantinopleBlock: big.NewInt(0),
		PetersburgBlock:     big.NewInt(0),
		IstanbulBlock:       big.NewInt(0),
		MuirGlacierBlock:    big.NewInt(0),
		BerlinBlock:         big.NewInt(0),
		LondonBlock:         big.NewInt(0),
	}

	configData, err := json.Marshal(chainConfig)
	if err != nil {
		panic(fmt.Sprintf("Failed to marshal config: %v", err))
	}

	// Write chain config
	configKey := []byte("ethereum-chain-config")
	if err := dstDB.Put(configKey, configData); err != nil {
		panic(fmt.Sprintf("Failed to write chain config: %v", err))
	}
	fmt.Println("  ✓ Chain config written")

	// Phase 3: Stamp metadata
	fmt.Println("\n📝 Phase 3: Stamping metadata...")
	
	// Expected head hash
	headHash := common.HexToHash("0x32dede1fc8e0f11ecde12fb42aef7933fc6c5fcf863bc277b5eac08ae4d461f0")
	headNumber := uint64(1082780)

	// Write database markers
	markers := map[string][]byte{
		"LastHeader":     headHash.Bytes(),
		"LastBlock":      headHash.Bytes(), 
		"LastFast":       headHash.Bytes(),
		"LastFinalized":  headHash.Bytes(),
		"LastPivot":      headHash.Bytes(),
	}

	for key, value := range markers {
		if err := dstDB.Put([]byte(key), value); err != nil {
			fmt.Printf("  ⚠️  Failed to write %s: %v\n", key, err)
		} else {
			fmt.Printf("  ✓ %s stamped\n", key)
		}
	}

	// Write head number
	headNumKey := append([]byte("H"), headHash.Bytes()...)
	headNumVal := make([]byte, 8)
	binary.BigEndian.PutUint64(headNumVal, headNumber)
	dstDB.Put(headNumKey, headNumVal)

	// Phase 4: Create VM database
	fmt.Println("\n🗄️  Phase 4: Creating VM database...")
	os.MkdirAll(DST_VM, 0755)
	
	vmDB, err := pebble.Open(DST_VM, &pebble.Options{})
	if err != nil {
		panic(fmt.Sprintf("Failed to create VM database: %v", err))
	}
	defer vmDB.Close()

	// Write VM metadata
	vmDB.Set([]byte("initialized"), []byte{1}, pebble.Sync)
	vmDB.Set([]byte("lastAccepted"), headHash.Bytes(), pebble.Sync)
	
	heightBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(heightBytes, headNumber)
	vmDB.Set([]byte("lastAcceptedHeight"), heightBytes, pebble.Sync)
	
	fmt.Println("  ✓ VM database initialized")

	// Final summary
	elapsed := time.Since(startTime)
	fmt.Println("\n╔════════════════════════════════════════════════════════╗")
	fmt.Println("║                  ✅ BUILD COMPLETE!                     ║")
	fmt.Println("╚════════════════════════════════════════════════════════╝")
	fmt.Printf("\n📊 Summary:\n")
	fmt.Printf("  Total Keys:    %d\n", totalKeys)
	fmt.Printf("  Total Data:    %.2f GB\n", float64(totalBytes)/(1024*1024*1024))
	fmt.Printf("  Time Taken:    %s\n", elapsed.Round(time.Second))
	fmt.Printf("  Rate:          %.0f keys/sec\n", float64(totalKeys)/elapsed.Seconds())
	fmt.Printf("\n  Block Data:\n")
	fmt.Printf("    Headers:     %d\n", headers)
	fmt.Printf("    Bodies:      %d\n", bodies)
	fmt.Printf("    Receipts:    %d\n", receipts)
	fmt.Printf("    Canonical:   %d\n", canonical)
	fmt.Printf("\n  State Data:\n")
	fmt.Printf("    State Nodes: %d\n", stateNodes)
	fmt.Printf("    Accounts:    %d\n", accounts)
	fmt.Printf("    Storage:     %d\n", storage)
	fmt.Printf("    Code:        %d\n", code)
	
	fmt.Printf("\n🚀 Next Steps:\n")
	fmt.Printf("  1. Verify with: ./verify_migration\n")
	fmt.Printf("  2. Boot node with: luxd --network-id=%s\n", NET)
	fmt.Printf("  3. Test RPC at: http://localhost:9630\n")
}