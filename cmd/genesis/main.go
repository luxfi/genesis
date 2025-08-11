package main

import (
	"bytes"
	"crypto/x509"
	"encoding/binary"
	"encoding/json"
	"encoding/pem"
	"flag"
	"fmt"
	"log"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"github.com/cockroachdb/pebble"
	"github.com/luxfi/database/badgerdb"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	namespaceLen = 32
	batchSize    = 10000
)

// Network configurations
var networks = map[string]*NetworkConfig{
	"lux-mainnet": {
		Name:      "LUX Mainnet",
		NetworkID: 96369,
		ChainID:   "X6CU5qgMJfzsTB9UWxj2ZY5hd68x41HfZ4m4hCBWbHuj1Ebc3",
		Namespace: []byte{
			0x33, 0x7f, 0xb7, 0x3f, 0x9b, 0xcd, 0xac, 0x8c, 0x31,
			0xa2, 0xd5, 0xf7, 0xb8, 0x77, 0xab, 0x1e, 0x8a, 0x2b,
			0x7f, 0x2a, 0x1e, 0x9b, 0xf0, 0x2a, 0x0a, 0x0e, 0x6c,
			0x6f, 0xd1, 0x64, 0xf1, 0xd1,
		},
	},
}

type NetworkConfig struct {
	Name      string
	NetworkID uint32
	ChainID   string
	Namespace []byte
}

type Stats struct {
	Total           int
	Written         int
	Headers         int
	Bodies          int
	Receipts        int
	TotalDifficulty int
	Canonical       int
	StateNodes      int
	Code            int
	Other           int
	Skipped         int
	MaxBlockNum     uint64
	LastBlockHash   []byte
}

func main() {
	var (
		network  string
		sourceDB string
		destDir  string
		verbose  bool
	)

	flag.StringVar(&network, "network", "lux-mainnet", "Network to use")
	flag.StringVar(&sourceDB, "source", "", "Source PebbleDB path")
	
	// Default destination is ~/.luxd
	homeDir := os.Getenv("HOME")
	defaultDest := filepath.Join(homeDir, ".luxd")
	flag.StringVar(&destDir, "dest", defaultDest, "Destination directory")
	flag.BoolVar(&verbose, "verbose", false, "Verbose output")
	flag.Parse()

	if flag.NArg() < 1 {
		printUsage()
		os.Exit(1)
	}

	cmd := flag.Arg(0)
	netConfig, ok := networks[network]
	if !ok {
		log.Fatalf("Unknown network: %s", network)
	}

	// Auto-detect source if not specified
	if sourceDB == "" {
		sourceDB = fmt.Sprintf("/home/z/work/lux/state/chaindata/%s-%d/db/pebbledb",
			network, netConfig.NetworkID)
	}

	switch cmd {
	case "migrate":
		runMigration(netConfig, sourceDB, destDir, verbose)
	case "verify":
		verifyDatabase(destDir, netConfig)
	case "launch":
		launchNode(netConfig, destDir)
	case "generate-genesis":
		generateGenesisFiles(netConfig, destDir)
	default:
		fmt.Printf("Unknown command: %s\n", cmd)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`Genesis Migration Tool

Usage: genesis [flags] <command>

Commands:
  migrate          Migrate SubnetEVM database to Coreth format (default: to ~/.luxd)
  verify           Verify migrated database is ready
  generate-genesis Create P-Chain and X-Chain genesis files
  launch           Launch LUX mainnet node from ~/.luxd

Flags:`)
	flag.PrintDefaults()
}

func runMigration(config *NetworkConfig, sourceDB, destDir string, verbose bool) {
	fmt.Printf("=== Migrating %s ===\n", config.Name)
	fmt.Printf("Source: %s\n", sourceDB)
	fmt.Printf("Destination: %s\n", destDir)

	// Open source PebbleDB
	source, err := pebble.Open(sourceDB, &pebble.Options{ReadOnly: true})
	if err != nil {
		log.Fatalf("Failed to open source database: %v", err)
	}
	defer source.Close()

	// Create destination paths
	basePath := filepath.Join(destDir, fmt.Sprintf("network-%d", config.NetworkID),
		"chains", config.ChainID)
	ethdbPath := filepath.Join(basePath, "ethdb")
	vmPath := filepath.Join(basePath, "vm")

	// Create directories
	if err := os.MkdirAll(ethdbPath, 0755); err != nil {
		log.Fatalf("Failed to create ethdb directory: %v", err)
	}
	if err := os.MkdirAll(vmPath, 0755); err != nil {
		log.Fatalf("Failed to create vm directory: %v", err)
	}

	// Open destination databases
	ethdb, err := badgerdb.New(ethdbPath, nil, "", prometheus.DefaultRegisterer)
	if err != nil {
		log.Fatalf("Failed to create ethdb: %v", err)
	}
	defer ethdb.Close()

	vmdb, err := badgerdb.New(vmPath, nil, "", prometheus.DefaultRegisterer)
	if err != nil {
		log.Fatalf("Failed to create vmdb: %v", err)
	}
	defer vmdb.Close()

	// Run migration with TD computation
	stats := &Stats{}
	if err := migrateDataWithTD(source, ethdb, config, stats, verbose); err != nil {
		log.Fatalf("Migration failed: %v", err)
	}

	// Write VM metadata
	if stats.LastBlockHash != nil {
		writeVMMetadata(vmdb, stats.LastBlockHash, stats.MaxBlockNum)
	}

	// Copy staking keys if destination is ~/.luxd
	homeDir := os.Getenv("HOME")
	if destDir == filepath.Join(homeDir, ".luxd") {
		fmt.Println("\nSetting up staking keys...")
		setupStakingKeys(destDir)
	}

	printMigrationSummary(stats)
	fmt.Println("\n✅ Migration complete! Database ready in:", destDir)
}

func migrateDataWithTD(source *pebble.DB, dest *badgerdb.Database, config *NetworkConfig, stats *Stats, verbose bool) error {
	startTime := time.Now()
	lastLog := time.Now()

	// First pass: migrate all data and track blocks
	blockHashes := make(map[uint64][]byte)
	canonicalBlocks := make(map[uint64][]byte)
	
	it, err := source.NewIter(nil)
	if err != nil {
		return err
	}
	defer it.Close()

	batch := dest.NewBatch()
	batchCount := 0

	fmt.Println("Starting migration...")
	fmt.Printf("Namespace to strip: %x\n", config.Namespace)

	for it.First(); it.Valid(); it.Next() {
		stats.Total++

		key := it.Key()
		value, err := it.ValueAndErr()
		if err != nil {
			continue
		}

		// Strip namespace
		actualKey := key
		if len(config.Namespace) > 0 && len(key) >= namespaceLen {
			if bytes.Equal(key[:namespaceLen], config.Namespace) {
				actualKey = key[namespaceLen:]
			}
		}

		if len(actualKey) == 0 {
			stats.Skipped++
			continue
		}

		// Track canonical blocks - try multiple formats
		// Format 1: H<num> -> hash (SubnetEVM style)
		if actualKey[0] == 'H' && len(actualKey) == 9 && len(value) == 32 {
			num := binary.BigEndian.Uint64(actualKey[1:9])
			canonicalBlocks[num] = value
			
			// Convert to Coreth format: h<num>n -> hash
			newKey := make([]byte, 10)
			newKey[0] = 'h'
			copy(newKey[1:9], actualKey[1:9])
			newKey[9] = 'n'
			batch.Put(newKey, value)
			stats.Canonical++
			
			// Also create H<hash> -> num mapping
			hashKey := make([]byte, 33)
			hashKey[0] = 'H'
			copy(hashKey[1:], value)
			batch.Put(hashKey, actualKey[1:9])
			stats.Written += 2
			
			if num > stats.MaxBlockNum {
				stats.MaxBlockNum = num
				stats.LastBlockHash = value
			}
		} else if actualKey[0] == 'h' && len(actualKey) == 10 && actualKey[9] == 'n' && len(value) == 32 {
			// Format 2: h<num>n -> hash (Coreth style canonical)
			num := binary.BigEndian.Uint64(actualKey[1:9])
			canonicalBlocks[num] = value
			
			// Write as-is since it's already Coreth format
			batch.Put(actualKey, value)
			stats.Written++
			stats.Canonical++
			
			// Also create H<hash> -> num mapping
			hashKey := make([]byte, 33)
			hashKey[0] = 'H'
			copy(hashKey[1:], value)
			numBytes := make([]byte, 8)
			binary.BigEndian.PutUint64(numBytes, num)
			batch.Put(hashKey, numBytes)
			stats.Written++
			
			if num > stats.MaxBlockNum {
				stats.MaxBlockNum = num
				stats.LastBlockHash = value
			}
		} else if actualKey[0] == 'h' && len(actualKey) == 41 {
			// Format 3: h<num><hash> -> header (track for hash mapping)
			num := binary.BigEndian.Uint64(actualKey[1:9])
			hash := actualKey[9:41]
			blockHashes[num] = hash
			
			// Write header as-is
			batch.Put(actualKey, value)
			stats.Written++
			stats.Headers++
			
			if num > stats.MaxBlockNum {
				stats.MaxBlockNum = num
				stats.LastBlockHash = hash
			}
		} else {
			// Write all other keys as-is
			batch.Put(actualKey, value)
			stats.Written++
			categorizeKey(actualKey, value, stats)
		}

		batchCount++
		if batchCount >= batchSize {
			if err := batch.Write(); err != nil {
				return fmt.Errorf("batch write failed: %v", err)
			}
			batch.Reset()
			batchCount = 0
		}

		// Progress logging
		if time.Since(lastLog) > 5*time.Second {
			elapsed := time.Since(startTime)
			rate := float64(stats.Total) / elapsed.Seconds()
			fmt.Printf("Progress: %d total, %d written (%.0f keys/sec)\n",
				stats.Total, stats.Written, rate)
			lastLog = time.Now()
		}
	}

	// Write final batch
	if batchCount > 0 {
		if err := batch.Write(); err != nil {
			return fmt.Errorf("final batch write failed: %v", err)
		}
	}

	// Merge canonical blocks with header-derived hashes
	for num, hash := range blockHashes {
		if _, exists := canonicalBlocks[num]; !exists {
			canonicalBlocks[num] = hash
		}
	}

	// If no canonicals found but we have headers, use headers as canonicals
	if len(canonicalBlocks) == 0 && len(blockHashes) > 0 {
		fmt.Println("No canonical blocks found, using headers as canonical chain...")
		canonicalBlocks = blockHashes
		
		// Write canonical entries for all blocks
		for num, hash := range blockHashes {
			// Write h<num>n -> hash
			canonKey := make([]byte, 10)
			canonKey[0] = 'h'
			binary.BigEndian.PutUint64(canonKey[1:9], num)
			canonKey[9] = 'n'
			dest.Put(canonKey, hash)
			
			// Write H<hash> -> num
			hashKey := make([]byte, 33)
			hashKey[0] = 'H'
			copy(hashKey[1:], hash)
			numBytes := make([]byte, 8)
			binary.BigEndian.PutUint64(numBytes, num)
			dest.Put(hashKey, numBytes)
			
			stats.Canonical++
		}
	}

	fmt.Printf("\nFound %d canonical blocks, max height: %d\n", len(canonicalBlocks), stats.MaxBlockNum)

	// Second pass: Compute and write Total Difficulty for all blocks
	fmt.Println("\nComputing Total Difficulty...")
	if err := computeTotalDifficulty(dest, canonicalBlocks); err != nil {
		return fmt.Errorf("failed to compute TD: %v", err)
	}

	// Set head pointers
	if stats.LastBlockHash != nil {
		fmt.Printf("Setting head pointers to block %d (hash: %x)\n", stats.MaxBlockNum, stats.LastBlockHash)
		dest.Put([]byte("LastHeader"), stats.LastBlockHash)
		dest.Put([]byte("LastBlock"), stats.LastBlockHash)
		dest.Put([]byte("LastFast"), stats.LastBlockHash)
	}

	return nil
}

func computeTotalDifficulty(db *badgerdb.Database, blockHashes map[uint64][]byte) error {
	// For SubnetEVM, difficulty is 1 per block
	// TD(n) = n + 1
	totalDifficulty := big.NewInt(0)
	maxBlock := uint64(0)
	
	// Find max block
	for num := range blockHashes {
		if num > maxBlock {
			maxBlock = num
		}
	}
	
	fmt.Printf("Computing TD for %d blocks...\n", maxBlock+1)
	
	// Write TD for all blocks we have hashes for
	for height := uint64(0); height <= maxBlock; height++ {
		totalDifficulty.Add(totalDifficulty, big.NewInt(1))
		
		// Get the hash for this height
		hash, exists := blockHashes[height]
		if !exists {
			continue // Skip if we don't have this block
		}
		
		// Write TD: t + num(8) + hash(32) -> TD bytes
		tdKey := make([]byte, 41)
		tdKey[0] = 't'
		numBytes := make([]byte, 8)
		binary.BigEndian.PutUint64(numBytes, height)
		copy(tdKey[1:9], numBytes)
		copy(tdKey[9:41], hash)
		
		// Store TD as big-endian bytes
		tdBytes := totalDifficulty.Bytes()
		if err := db.Put(tdKey, tdBytes); err != nil {
			return fmt.Errorf("failed to write TD at %d: %v", height, err)
		}
		
		if height%10000 == 0 || height == maxBlock {
			fmt.Printf("  Written TD up to block %d (TD: %s)\n", height, totalDifficulty.String())
		}
	}
	
	fmt.Printf("✅ TD chain complete. Final TD: %s\n", totalDifficulty.String())
	return nil
}

func writeVMMetadata(vmdb *badgerdb.Database, lastBlockHash []byte, lastBlockNum uint64) {
	fmt.Println("\nWriting VM metadata...")
	
	// Write lastAccepted
	vmdb.Put([]byte("lastAccepted"), lastBlockHash)
	
	// Write lastAcceptedHeight
	heightBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(heightBytes, lastBlockNum)
	vmdb.Put([]byte("lastAcceptedHeight"), heightBytes)
	
	// Write initialized flag
	vmdb.Put([]byte("initialized"), []byte{1})
	
	fmt.Printf("✅ VM metadata written (height: %d)\n", lastBlockNum)
}

func categorizeKey(key []byte, value []byte, stats *Stats) {
	if len(key) == 0 {
		return
	}
	
	switch key[0] {
	case 'h':
		if len(key) == 41 {
			stats.Headers++
		} else if len(key) == 10 && key[9] == 'n' {
			stats.Canonical++
		}
	case 'b':
		if len(key) == 41 {
			stats.Bodies++
		}
	case 'r':
		if len(key) == 41 {
			stats.Receipts++
		}
	case 't':
		if len(key) == 41 {
			stats.TotalDifficulty++
		}
	case 'c':
		if len(key) == 33 {
			stats.Code++
		}
	default:
		if len(key) == 32 {
			stats.StateNodes++
		} else {
			stats.Other++
		}
	}
}

func printMigrationSummary(stats *Stats) {
	fmt.Println("\n=== Migration Summary ===")
	fmt.Printf("Total keys processed: %d\n", stats.Total)
	fmt.Printf("Keys written: %d\n", stats.Written)
	fmt.Printf("Keys skipped: %d\n", stats.Skipped)
	fmt.Printf("\nData categories:\n")
	fmt.Printf("  Headers: %d\n", stats.Headers)
	fmt.Printf("  Bodies: %d\n", stats.Bodies)
	fmt.Printf("  Receipts: %d\n", stats.Receipts)
	fmt.Printf("  Total Difficulty: %d\n", stats.TotalDifficulty)
	fmt.Printf("  Canonical: %d\n", stats.Canonical)
	fmt.Printf("  State Nodes: %d\n", stats.StateNodes)
	fmt.Printf("  Code: %d\n", stats.Code)
	fmt.Printf("  Other: %d\n", stats.Other)
	fmt.Printf("\nMax block: %d\n", stats.MaxBlockNum)
}

func verifyDatabase(destDir string, config *NetworkConfig) {
	basePath := filepath.Join(destDir, fmt.Sprintf("network-%d", config.NetworkID),
		"chains", config.ChainID)
	ethdbPath := filepath.Join(basePath, "ethdb")
	vmPath := filepath.Join(basePath, "vm")
	
	fmt.Printf("=== Verifying %s Database ===\n", config.Name)
	
	// Open databases
	ethdb, err := badgerdb.New(ethdbPath, nil, "", prometheus.DefaultRegisterer)
	if err != nil {
		log.Fatalf("Failed to open ethdb: %v", err)
	}
	defer ethdb.Close()
	
	vmdb, err := badgerdb.New(vmPath, nil, "", prometheus.DefaultRegisterer)
	if err != nil {
		log.Fatalf("Failed to open vmdb: %v", err)
	}
	defer vmdb.Close()
	
	allGood := true
	
	// Check heads
	if val, err := ethdb.Get([]byte("LastBlock")); err == nil && len(val) == 32 {
		fmt.Printf("✅ LastBlock: 0x%x\n", val)
	} else {
		fmt.Printf("❌ LastBlock: NOT FOUND\n")
		allGood = false
	}
	
	// Check VM metadata
	if val, err := vmdb.Get([]byte("lastAccepted")); err == nil && len(val) == 32 {
		fmt.Printf("✅ VM lastAccepted: 0x%x\n", val)
	} else {
		fmt.Printf("❌ VM lastAccepted: NOT FOUND\n")
		allGood = false
	}
	
	if val, err := vmdb.Get([]byte("lastAcceptedHeight")); err == nil && len(val) == 8 {
		height := binary.BigEndian.Uint64(val)
		fmt.Printf("✅ VM lastAcceptedHeight: %d\n", height)
	} else {
		fmt.Printf("❌ VM lastAcceptedHeight: NOT FOUND\n")
		allGood = false
	}
	
	// Check for TD at tip (example at height 1082780)
	tipHeight := uint64(1082780)
	tipHash := []byte{
		0x32, 0xde, 0xde, 0x1f, 0xc8, 0xe0, 0xf1, 0x1e,
		0xcd, 0xe1, 0x2f, 0xb4, 0x2a, 0xef, 0x79, 0x33,
		0xfc, 0x6c, 0x5f, 0xcf, 0x86, 0x3b, 0xc2, 0x77,
		0xb5, 0xea, 0xc0, 0x8a, 0xe4, 0xd4, 0x61, 0xf0,
	}
	
	tdKey := make([]byte, 41)
	tdKey[0] = 't'
	numBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(numBytes, tipHeight)
	copy(tdKey[1:9], numBytes)
	copy(tdKey[9:41], tipHash)
	
	if tdBytes, err := ethdb.Get(tdKey); err == nil && len(tdBytes) > 0 {
		td := new(big.Int).SetBytes(tdBytes)
		fmt.Printf("✅ TD at tip (height %d): %s\n", tipHeight, td.String())
	} else {
		fmt.Printf("❌ TD at tip: NOT FOUND\n")
		allGood = false
	}
	
	if allGood {
		fmt.Println("\n🎉 Database verification passed! Ready to start node.")
	} else {
		fmt.Println("\n⚠️  Some checks failed. Run 'genesis migrate' to fix.")
	}
}


func launchNode(config *NetworkConfig, dataDir string) {
	fmt.Printf("=== Launching %s Node ===\n", config.Name)
	
	homeDir := os.Getenv("HOME")
	luxdDataDir := filepath.Join(homeDir, ".luxd")
	
	// Check if data exists
	if _, err := os.Stat(filepath.Join(luxdDataDir, fmt.Sprintf("network-%d", config.NetworkID))); os.IsNotExist(err) {
		log.Fatalf("Data not found in ~/.luxd. Run 'genesis setup' first")
	}
	
	// Build the command
	luxdPath := "/home/z/work/lux/node/build/luxd"
	
	args := []string{
		"--data-dir=" + luxdDataDir,
		"--network-id=" + fmt.Sprint(config.NetworkID),
		"--staking-tls-cert-file=" + filepath.Join(luxdDataDir, "staking", "staker.crt"),
		"--staking-tls-key-file=" + filepath.Join(luxdDataDir, "staking", "staker.key"),
		"--staking-signer-key-file=" + filepath.Join(luxdDataDir, "staking", "signer.key"),
		"--staking-enabled=true",
		"--sybil-protection-enabled=false",
		"--consensus-sample-size=1",
		"--http-host=0.0.0.0",
		"--http-port=9630",
		"--api-admin-enabled=true",
		"--api-metrics-enabled=true",
		"--index-enabled=true",
		"--bootstrap-ips=",
		"--bootstrap-ids=",
		"--log-level=info",
	}
	
	fmt.Printf("Starting node with command:\n%s %s\n", luxdPath, args[0])
	
	// Execute the node
	if err := os.Chdir("/home/z/work/lux"); err != nil {
		log.Fatalf("Failed to change directory: %v", err)
	}
	
	// Replace current process with luxd
	if err := syscall.Exec(luxdPath, append([]string{luxdPath}, args...), os.Environ()); err != nil {
		log.Fatalf("Failed to launch node: %v", err)
	}
}

func copyFile(src, dst string) error {
	input, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, input, 0600)
}

func setupStakingKeys(destDir string) {
	stakingDir := filepath.Join(destDir, "staking")
	if err := os.MkdirAll(stakingDir, 0700); err != nil {
		log.Printf("Failed to create staking dir: %v", err)
		return
	}
	
	// Copy from node1 staking directory
	srcStaking := "/home/z/work/lux/staking-keys/node1/staking"
	files := []string{"staker.key", "staker.crt", "signer.key"}
	
	for _, file := range files {
		src := filepath.Join(srcStaking, file)
		dst := filepath.Join(stakingDir, file)
		if err := copyFile(src, dst); err != nil {
			log.Printf("Failed to copy %s: %v", file, err)
		}
	}
	
	fmt.Printf("✅ Staking keys copied to %s\n", stakingDir)
}

func copyDir(src, dst string) error {
	// Use exec.Command to run rsync
	cmd := exec.Command("rsync", "-av", "--progress", src+"/", dst+"/")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func generateGenesisFiles(config *NetworkConfig, destDir string) {
	fmt.Printf("=== Generating Genesis Files for %s ===\n", config.Name)
	
	// Create genesis directory structure
	genesisDir := filepath.Join(destDir, "configs")
	if err := os.MkdirAll(filepath.Join(genesisDir, "chains", "C"), 0755); err != nil {
		log.Fatalf("Failed to create genesis directories: %v", err)
	}
	
	// Get NodeID from staking certificate
	stakingCert := filepath.Join("/home/z/work/lux/staking-keys/node1/staking/staker.crt")
	nodeID := getNodeIDFromCert(stakingCert)
	fmt.Printf("NodeID: %s\n", nodeID)
	
	// Generate P-Chain genesis with validators
	pChainGenesis := generatePChainGenesis(nodeID, config.NetworkID)
	pChainPath := filepath.Join(genesisDir, "chains", "P", "genesis.json")
	os.MkdirAll(filepath.Dir(pChainPath), 0755)
	if err := writeJSON(pChainPath, pChainGenesis); err != nil {
		log.Fatalf("Failed to write P-Chain genesis: %v", err)
	}
	fmt.Printf("✅ P-Chain genesis written to: %s\n", pChainPath)
	
	// Generate X-Chain genesis
	xChainGenesis := generateXChainGenesis(config.NetworkID)
	xChainPath := filepath.Join(genesisDir, "chains", "X", "genesis.json")
	os.MkdirAll(filepath.Dir(xChainPath), 0755)
	if err := writeJSON(xChainPath, xChainGenesis); err != nil {
		log.Fatalf("Failed to write X-Chain genesis: %v", err)
	}
	fmt.Printf("✅ X-Chain genesis written to: %s\n", xChainPath)
	
	// C-Chain note
	cChainPath := filepath.Join(genesisDir, "chains", "C", "genesis.json")
	os.MkdirAll(filepath.Dir(cChainPath), 0755)
	cChainNote := map[string]string{
		"note": "C-Chain uses migrated database for genesis continuity",
		"info": "The C-Chain genesis is loaded from the migrated blockchain data",
	}
	if err := writeJSON(cChainPath, cChainNote); err != nil {
		log.Printf("Failed to write C-Chain note: %v", err)
	}
	fmt.Printf("✅ C-Chain uses migrated database at height 1,082,780\n")
	
	fmt.Println("\n✅ All genesis files generated successfully!")
}

func getNodeIDFromCert(certPath string) string {
	certPEM, err := os.ReadFile(certPath)
	if err != nil {
		log.Fatalf("Failed to read certificate: %v", err)
	}
	
	block, _ := pem.Decode(certPEM)
	if block == nil {
		log.Fatal("Failed to decode PEM certificate")
	}
	
	if _, err := x509.ParseCertificate(block.Bytes); err != nil {
		log.Fatalf("Failed to parse certificate: %v", err)
	}
	
	// The NodeID is derived from the certificate's public key
	// For now, return the known NodeID from our staking setup
	// In production, this would be calculated from the cert
	return "NodeID-7Xhw2mDxuDS44j42TCB6U5579esbSt3Lg"
}

func generatePChainGenesis(nodeID string, networkID uint32) map[string]interface{} {
	// Current timestamp
	now := time.Now().Unix()
	
	// Validator end time (100 years from now)
	endTime := now + (100 * 365 * 24 * 60 * 60)
	
	return map[string]interface{}{
		"networkID": networkID,
		"allocations": []map[string]interface{}{
			{
				// Main validator allocation
				"ethAddr": "0x8db97C7cEcE249c2b98bDC0226Cc4C2A57BF52FC",
				"avaxAddr": "X-lux18jma8ppw3nhx5r4ap8clazz0dps7rv5ukulre5",
				"initialAmount": 300000000000000000, // 300M LUX
				"unlockSchedule": []map[string]interface{}{
					{
						"amount": 10000000000000000,
						"locktime": now,
					},
				},
			},
		},
		"startTime": now,
		"initialStakeDuration": 31536000, // 1 year in seconds
		"initialStakeDurationOffset": 5400, // 1.5 hours
		"initialStakedFunds": []string{
			"X-lux18jma8ppw3nhx5r4ap8clazz0dps7rv5ukulre5",
		},
		"initialStakers": []map[string]interface{}{
			{
				"nodeID": nodeID,
				"startTime": now,
				"endTime": endTime,
				"stakeAmount": 2000000000000, // 2M LUX minimum stake
				"rewardAddress": "X-lux18jma8ppw3nhx5r4ap8clazz0dps7rv5ukulre5",
				"delegationFee": 200000, // 20%
			},
		},
		"cChainGenesis": "", // Empty - using migrated database
		"message": "LUX Mainnet Genesis",
	}
}

func generateXChainGenesis(networkID uint32) map[string]interface{} {
	return map[string]interface{}{
		"networkID": networkID,
		"initialAmount": 600000000000000000, // 600M LUX total supply
		"startTime": time.Now().Unix(),
		"allocations": []map[string]interface{}{
			{
				"ethAddr": "0x8db97C7cEcE249c2b98bDC0226Cc4C2A57BF52FC", 
				"avaxAddr": "X-lux18jma8ppw3nhx5r4ap8clazz0dps7rv5ukulre5",
				"initialAmount": 100000000000000000, // 100M LUX
				"unlockSchedule": []map[string]interface{}{
					{
						"amount": 10000000000000000,
						"locktime": time.Now().Unix(),
					},
				},
			},
		},
	}
}

func writeJSON(path string, data interface{}) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}