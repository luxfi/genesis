package main

import (
	"bytes"
	"fmt"
	"log"

	"github.com/cockroachdb/pebble"
	"github.com/ethereum/go-ethereum/rlp"
)

// SubnetEVM namespace prefix (32 bytes)
var subnetNamespace = []byte{
	0x33, 0x7f, 0xb7, 0x3f, 0x9b, 0xcd, 0xac, 0x8c,
	0x31, 0xa2, 0xd5, 0xf7, 0xb8, 0x77, 0xab, 0x1e,
	0x8a, 0x2b, 0x7f, 0x2a, 0x1e, 0x9b, 0xf0, 0x2a,
	0x0a, 0x0e, 0x6c, 0x6f, 0xd1, 0x64, 0xf1, 0xd1,
}

type Header struct {
	ParentHash  []byte
	UncleHash   []byte
	Coinbase    []byte
	Root        []byte
	TxHash      []byte
	ReceiptHash []byte
	Bloom       []byte
	Difficulty  []byte
	Number      []byte
	GasLimit    []byte
	GasUsed     []byte
	Time        []byte
	Extra       []byte
	MixDigest   []byte
	Nonce       []byte
}

func main() {
	sourcePath := "/Users/z/work/lux/state/chaindata/lux-mainnet-96369/db/pebbledb"
	
	fmt.Println("Checking Block Heights in Database")
	fmt.Println("===================================")
	fmt.Printf("Database: %s\n\n", sourcePath)
	
	// Open source PebbleDB
	db, err := pebble.Open(sourcePath, &pebble.Options{
		ReadOnly: true,
	})
	if err != nil {
		log.Fatal("Failed to open database:", err)
	}
	defer db.Close()
	
	// Find some blocks
	iter, err := db.NewIter(nil)
	if err != nil {
		log.Fatal("Failed to create iterator:", err)
	}
	defer iter.Close()
	
	blockHeights := make(map[uint64]bool)
	samplesShown := 0
	
	for iter.First(); iter.Valid(); iter.Next() {
		key := iter.Key()
		val := iter.Value()
		
		// Check if this is a namespaced key
		if len(key) == 64 && bytes.HasPrefix(key, subnetNamespace) {
			actualKey := key[32:]
			
			// Check if this looks like a block header (RLP encoded)
			if len(val) > 500 && (val[0] == 0xf8 || val[0] == 0xf9 || val[0] == 0xfa) {
				// Try to decode as header
				var header Header
				if err := rlp.DecodeBytes(val, &header); err == nil {
					// The block number is in the Number field
					if len(header.Number) > 0 && len(header.Number) <= 8 {
						blockNum := uint64(0)
						for _, b := range header.Number {
							blockNum = (blockNum << 8) | uint64(b)
						}
						
						if blockNum < 10000000 { // Reasonable block number
							blockHeights[blockNum] = true
							
							if samplesShown < 10 {
								fmt.Printf("Block %d: hash=0x%x\n", blockNum, actualKey[:8])
								samplesShown++
							}
						}
					}
				}
			}
		}
	}
	
	fmt.Printf("\nTotal unique block heights found: %d\n", len(blockHeights))
	
	if len(blockHeights) > 0 {
		// Find min and max
		minBlock := uint64(^uint64(0))
		maxBlock := uint64(0)
		
		for height := range blockHeights {
			if height < minBlock {
				minBlock = height
			}
			if height > maxBlock {
				maxBlock = height
			}
		}
		
		fmt.Printf("Block range: %d to %d\n", minBlock, maxBlock)
		
		// Check for specific blocks
		checkBlocks := []uint64{0, 1, 100, 1000, 10000, 100000, 500000, 1000000, 1082780}
		fmt.Println("\nChecking specific blocks:")
		for _, block := range checkBlocks {
			if blockHeights[block] {
				fmt.Printf("  ✓ Block %d found\n", block)
			} else if block <= maxBlock {
				fmt.Printf("  ✗ Block %d missing (within range)\n", block)
			} else {
				fmt.Printf("  - Block %d beyond range\n", block)
			}
		}
	}
}