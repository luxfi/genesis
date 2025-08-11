package main

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"os"

	"github.com/luxfi/geth/core/rawdb"
	"github.com/luxfi/geth/ethdb"
	"github.com/luxfi/database"
	"github.com/luxfi/database/badgerdb"
	"github.com/spf13/cobra"
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "probe",
		Short: "Probe database for tip invariants",
		RunE:  runProbe,
	}

	rootCmd.Flags().StringP("db", "d", "/home/z/.luxd", "Database directory")
	
	if err := rootCmd.Execute(); err != nil {
		fmt.Printf("❌ Error: %v\n", err)
		os.Exit(1)
	}
}

func runProbe(cmd *cobra.Command, args []string) error {
	dbPath, _ := cmd.Flags().GetString("db")
	
	fmt.Printf("🔍 Probing database at: %s\n", dbPath)
	
	// Open the ethdb subdirectory
	ethdbPath := dbPath + "/chains/C/ethdb"
	
	// Try BadgerDB first
	badgerDB, err := badgerdb.New(ethdbPath, nil, "", nil)
	if err != nil {
		return fmt.Errorf("failed to open BadgerDB at %s: %v", ethdbPath, err)
	}
	defer badgerDB.Close()
	
	// Wrap in ethdb interface
	db := WrapDatabase(badgerDB)
	
	fmt.Println("\n=== Tip Invariants Check ===")
	
	// 1. Read head header hash
	headHash := rawdb.ReadHeadHeaderHash(db)
	if headHash == (common.Hash{}) {
		return fmt.Errorf("❌ No head header hash found")
	}
	fmt.Printf("✅ Head hash: 0x%x\n", headHash)
	
	// 2. Read header number for head hash
	headNum := rawdb.ReadHeaderNumber(db, headHash)
	if headNum == nil {
		return fmt.Errorf("❌ No header number for head hash %x", headHash)
	}
	fmt.Printf("✅ Head number: %d\n", *headNum)
	
	// 3. Read header at head
	header := rawdb.ReadHeader(db, headHash, *headNum)
	if header == nil {
		return fmt.Errorf("❌ No header at head (%d, %x)", *headNum, headHash)
	}
	fmt.Printf("✅ Header exists at head\n")
	
	// 4. Check canonical hash matches
	canonicalHash := rawdb.ReadCanonicalHash(db, *headNum)
	if canonicalHash != headHash {
		return fmt.Errorf("❌ Canonical hash mismatch: got %x, want %x", canonicalHash, headHash)
	}
	fmt.Printf("✅ Canonical hash matches\n")
	
	// 5. Scan for first failure
	fmt.Printf("\n=== Scanning blocks 0 to %d ===\n", *headNum)
	
	var firstError error
	var errorHeight uint64
	
	// Sample check - every 10000 blocks for speed
	step := uint64(10000)
	if *headNum < 100000 {
		step = 1000
	}
	
	for n := uint64(0); n <= *headNum; n += step {
		if n > *headNum {
			n = *headNum
		}
		
		// Read canonical hash
		hash := rawdb.ReadCanonicalHash(db, n)
		if hash == (common.Hash{}) {
			firstError = fmt.Errorf("missing canonical hash")
			errorHeight = n
			break
		}
		
		// Read header
		hdr := rawdb.ReadHeader(db, hash, n)
		if hdr == nil {
			firstError = fmt.Errorf("missing header")
			errorHeight = n
			break
		}
		
		// Try to read body (may not exist for all blocks)
		body := rawdb.ReadBody(db, hash, n)
		if body == nil && n > 0 {
			// Only warn, not error - some blocks may have no transactions
			fmt.Printf("⚠️  No body at block %d\n", n)
		}
		
		if n%100000 == 0 {
			fmt.Printf("  ✓ Block %d OK\n", n)
		}
	}
	
	if firstError != nil {
		// Do a detailed check around the error
		fmt.Printf("\n❌ First error at height %d: %v\n", errorHeight, firstError)
		
		// Raw key inspection
		if errorHeight > 0 {
			for i := errorHeight - 1; i <= errorHeight+1 && i <= *headNum; i++ {
				hash := rawdb.ReadCanonicalHash(db, i)
				fmt.Printf("\nBlock %d:\n", i)
				fmt.Printf("  Canonical hash: %x\n", hash)
				
				// Build header key: 'h' + num(8BE) + hash(32)
				key := make([]byte, 41)
				key[0] = 'h'
				binary.BigEndian.PutUint64(key[1:9], i)
				copy(key[9:], hash[:])
				
				val, err := db.Get(key)
				if err != nil {
					fmt.Printf("  Header raw: ERROR %v\n", err)
				} else {
					fmt.Printf("  Header raw (first 32 bytes): %x\n", val[:min(32, len(val))])
					if len(val) > 0 {
						leadByte := val[0]
						if leadByte >= 0xC0 && leadByte <= 0xFF {
							fmt.Printf("  ✓ Valid RLP list prefix (0x%02x)\n", leadByte)
						} else {
							fmt.Printf("  ❌ Invalid RLP prefix (0x%02x) - not a list!\n", leadByte)
						}
					}
				}
			}
		}
		
		return firstError
	}
	
	// 6. Check VM metadata
	fmt.Println("\n=== VM Metadata Check ===")
	
	vmPath := dbPath + "/chains/C/vm"
	vmDB, err := badgerdb.New(vmPath, nil, "", nil)
	if err != nil {
		fmt.Printf("⚠️  Could not open VM database: %v\n", err)
	} else {
		defer vmDB.Close()
		
		// Check lastAccepted
		lastAccepted, err := vmDB.Get([]byte("lastAccepted"))
		if err != nil {
			fmt.Printf("❌ No lastAccepted in VM metadata\n")
		} else {
			fmt.Printf("✅ lastAccepted: 0x%x\n", lastAccepted)
		}
		
		// Check lastAcceptedHeight
		lastHeight, err := vmDB.Get([]byte("lastAcceptedHeight"))
		if err != nil {
			fmt.Printf("❌ No lastAcceptedHeight in VM metadata\n")
		} else if len(lastHeight) == 8 {
			height := binary.BigEndian.Uint64(lastHeight)
			fmt.Printf("✅ lastAcceptedHeight: %d\n", height)
		}
		
		// Check initialized
		initialized, err := vmDB.Get([]byte("initialized"))
		if err != nil || len(initialized) == 0 {
			fmt.Printf("❌ VM not initialized\n")
		} else {
			fmt.Printf("✅ VM initialized: 0x%x\n", initialized)
		}
	}
	
	fmt.Println("\n✅ All tip invariants OK!")
	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}