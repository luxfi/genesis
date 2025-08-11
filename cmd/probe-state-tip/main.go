package main

import (
	"fmt"
	"log"
	"math/big"
	"os"
	"path/filepath"

	"github.com/luxfi/geth/common"
	"github.com/luxfi/geth/core/rawdb"
	"github.com/luxfi/geth/core/state"
	"github.com/luxfi/geth/triedb"
	"github.com/holiman/uint256"
)

func main() {
	// The actual path where the VM opens the database
	chainDir := filepath.Join(
		"/home/z/.luxd", "network-96369", "chains",
		"X6CU5qgMJfzsTB9UWxj2ZY5hd68x41HfZ4m4hCBWbHuj1Ebc3", "ethdb",
	)

	fmt.Printf("=== Probing State at Tip ===\n")
	fmt.Printf("Opening database at: %s\n", chainDir)

	// Check if the directory exists
	if _, err := os.Stat(chainDir); os.IsNotExist(err) {
		log.Fatalf("Database directory does not exist: %s", chainDir)
	}

	// Open the database with proper wrapper
	db, err := rawdb.NewPebbleDBDatabase(chainDir, 0, 0, "", false)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Read the head header
	head := rawdb.ReadHeadHeader(db)
	if head == nil {
		log.Fatal("❌ No HeadHeader – fix headers/heads first")
	}

	fmt.Printf("✅ Found head at block %d\n", head.Number.Uint64())
	fmt.Printf("   Hash: %s\n", head.Hash().Hex())
	fmt.Printf("   Root: %s\n", head.Root.Hex())

	// Try to open the state at the tip
	fmt.Printf("\nAttempting to open state at root %s...\n", head.Root.Hex())
	
	// Create trie database with hash scheme
	trieDb := triedb.NewDatabase(db, triedb.HashDefaults)
	
	// Create state database
	sdb := state.NewDatabase(trieDb, nil)
	
	// Try to open the state
	st, err := state.New(head.Root, sdb)
	if err != nil {
		log.Fatalf("❌ state.New(root) failed: %v\nThis means trie nodes are missing!", err)
	}

	fmt.Printf("✅ State opened successfully!\n")

	// Sanity check - read the known address balance
	addr := common.HexToAddress("0x9011E888251AB053B7bD1cdB598Db4f9DEd94714")
	bal := st.GetBalance(addr)
	
	fmt.Printf("\n=== Result ===\n")
	fmt.Printf("✅ Tip: block %d\n", head.Number.Uint64())
	fmt.Printf("✅ Root: %s\n", head.Root.Hex())
	fmt.Printf("✅ Balance of %s: %s LUX\n", addr.Hex(), formatBalance(bal))
}

func formatBalance(wei *uint256.Int) string {
	// Convert wei to LUX (18 decimals)
	bigWei := wei.ToBig()
	lux := new(big.Float).SetInt(bigWei)
	divisor := new(big.Float).SetFloat64(1e18)
	lux.Quo(lux, divisor)
	return fmt.Sprintf("%.2f", lux)
}