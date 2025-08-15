package main

import (
	"encoding/binary"
	"fmt"
	"log"
	"math/big"
	"os"
	
	"github.com/cockroachdb/pebble"
	
	// Use luxfi/geth consistently for all ethereum types
	"github.com/luxfi/database/badgerdb"
	"github.com/luxfi/geth/common"
	"github.com/luxfi/geth/core/rawdb"
	"github.com/luxfi/geth/core/types"
	"github.com/luxfi/geth/params"
	"github.com/luxfi/geth/rlp"
)

// SubnetEVM namespace prefix (32 bytes)
var subnetNamespace = []byte{
	0x33, 0x7f, 0xb7, 0x3f, 0x9b, 0xcd, 0xac, 0x8c,
	0x31, 0xa2, 0xd5, 0xf7, 0xb8, 0x77, 0xab, 0x1e,
	0x8a, 0x2b, 0x7f, 0x2a, 0x1e, 0x9b, 0xf0, 0x2a,
	0x0a, 0x0e, 0x6c, 0x6f, 0xd1, 0x64, 0xf1, 0xd1,
}

func main() {
	// Paths
	sourcePath := "/Users/z/work/lux/state/chaindata/lux-mainnet-96369/db/pebbledb"
	targetPath := "/Users/z/.luxd/network-96369/chains/EWi9aPkGe6EfJ3SobCAmSUXRPLa4brF3cThwPwmHTrD1y13jy/ethdb"
	vmPath := "/Users/z/.luxd/network-96369/chains/EWi9aPkGe6EfJ3SobCAmSUXRPLa4brF3cThwPwmHTrD1y13jy/vm"
	
	fmt.Println("=== Lux C-Chain Migration with Coreth Adapters ===")
	fmt.Printf("Source: %s\n", sourcePath)
	fmt.Printf("Target ethdb: %s\n", targetPath)
	fmt.Printf("Target vm: %s\n", vmPath)
	fmt.Println()
	
	// Open source PebbleDB
	sourceDB, err := pebble.Open(sourcePath, &pebble.Options{
		ReadOnly: true,
	})
	if err != nil {
		log.Fatal("Failed to open source database:", err)
	}
	defer sourceDB.Close()
	
	// Clean target directory
	os.RemoveAll(targetPath)
	os.MkdirAll(targetPath, 0755)
	
	// Open target BadgerDB using Coreth adapter
	targetDB, err := badgerdb.New(targetPath, nil, "", nil)
	if err != nil {
		log.Fatal("Failed to open target database:", err)
	}
	defer targetDB.Close()
	
	// Statistics
	var (
		headers   = 0
		bodies    = 0
		receipts  = 0
		canonical = 0
		hashToNum = 0
		
		highestBlock uint64
		highestHash  common.Hash
		genesisHash  common.Hash
	)
	
	// Create iterator
	iter, err := sourceDB.NewIter(nil)
	if err != nil {
		log.Fatal("Failed to create iterator:", err)
	}
	defer iter.Close()
	
	fmt.Println("Starting migration...")
	
	// Iterate through source database
	for iter.First(); iter.Valid(); iter.Next() {
		key := iter.Key()
		val := iter.Value()
		
		// Skip non-namespaced keys
		if len(key) < 33 {
			continue
		}
		
		// Check if it's a namespaced key
		if len(key) >= 64 && len(key) <= 73 {
			// Extract namespace and actual key
			ns := key[:32]
			if !equalBytes(ns, subnetNamespace) {
				continue
			}
			
			actualKey := key[32:]
			tag := actualKey[0]
			
			switch tag {
			case 'h': // Header
				if len(actualKey) == 41 {
					// h + num(8) + hash(32)
					num := binary.BigEndian.Uint64(actualKey[1:9])
					hash := common.BytesToHash(actualKey[9:41])
					
					// Write header using rawdb
					var header types.Header
					if err := rlp.DecodeBytes(val, &header); err == nil {
						rawdb.WriteHeader(targetDB, &header)
						headers++
						
						if num > highestBlock {
							highestBlock = num
							highestHash = hash
						}
						
						if num == 0 {
							genesisHash = hash
						}
						
						if headers%10000 == 0 {
							fmt.Printf("  Headers: %d\n", headers)
						}
					}
				}
				
			case 'b': // Body
				if len(actualKey) == 41 {
					// b + num(8) + hash(32)
					num := binary.BigEndian.Uint64(actualKey[1:9])
					hash := common.BytesToHash(actualKey[9:41])
					
					// Decode and write body
					var body types.Body
					if err := rlp.DecodeBytes(val, &body); err == nil {
						rawdb.WriteBody(targetDB, hash, num, &body)
						bodies++
						
						if bodies%10000 == 0 {
							fmt.Printf("  Bodies: %d\n", bodies)
						}
					}
				}
				
			case 'r': // Receipts
				if len(actualKey) == 41 {
					// r + num(8) + hash(32)
					num := binary.BigEndian.Uint64(actualKey[1:9])
					hash := common.BytesToHash(actualKey[9:41])
					
					// Decode and write receipts
					var receiptsList types.Receipts
					if err := rlp.DecodeBytes(val, &receiptsList); err == nil {
						rawdb.WriteReceipts(targetDB, hash, num, receiptsList)
						receipts++
						
						if receipts%10000 == 0 {
							fmt.Printf("  Receipts: %d\n", receipts)
						}
					}
				}
				
			case 'H': // Hash to number mapping
				if len(actualKey) == 33 {
					// H + hash(32)
					hash := common.BytesToHash(actualKey[1:33])
					
					// Value is 8-byte big-endian number
					if len(val) == 8 {
						num := binary.BigEndian.Uint64(val)
						
						// Write canonical hash
						rawdb.WriteCanonicalHash(targetDB, hash, num)
						canonical++
						
						// Also write hash->number mapping
						headerNumberKey := append([]byte("H"), hash.Bytes()...)
						targetDB.Put(headerNumberKey, val)
						hashToNum++
						
						if canonical%10000 == 0 {
							fmt.Printf("  Canonical: %d\n", canonical)
						}
					}
				}
			}
		}
	}
	
	fmt.Printf("\nMigration complete!\n")
	fmt.Printf("Headers:   %d\n", headers)
	fmt.Printf("Bodies:    %d\n", bodies)
	fmt.Printf("Receipts:  %d\n", receipts)
	fmt.Printf("Canonical: %d\n", canonical)
	fmt.Printf("H->num:    %d\n", hashToNum)
	fmt.Printf("Highest:   Block %d, Hash %s\n", highestBlock, highestHash.Hex())
	fmt.Printf("Genesis:   %s\n", genesisHash.Hex())
	
	// Set head pointers
	fmt.Println("\nSetting head pointers...")
	rawdb.WriteHeadHeaderHash(targetDB, highestHash)
	rawdb.WriteHeadBlockHash(targetDB, highestHash)
	rawdb.WriteHeadFastBlockHash(targetDB, highestHash)
	
	// Write chain config
	fmt.Println("Writing chain config...")
	chainConfig := &params.ChainConfig{
		ChainID:             big.NewInt(96369),
		HomesteadBlock:      big.NewInt(0),
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
		ShanghaiTime:        uint64Ptr(1607144400),
		CancunTime:          uint64Ptr(253399622400),
	}
	rawdb.WriteChainConfig(targetDB, genesisHash, chainConfig)
	
	// Close ethdb
	targetDB.Close()
	
	// Write VM metadata
	fmt.Println("Writing VM metadata...")
	os.MkdirAll(vmPath, 0755)
	
	vmDB, err := badgerdb.New(vmPath, nil, "", nil)
	if err != nil {
		log.Fatal("Failed to open VM database:", err)
	}
	defer vmDB.Close()
	
	// Write lastAccepted (32 bytes hash)
	vmDB.Put([]byte("lastAccepted"), highestHash.Bytes())
	
	// Write lastAcceptedHeight (8 bytes BE)
	heightBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(heightBytes, highestBlock)
	vmDB.Put([]byte("lastAcceptedHeight"), heightBytes)
	
	// Write initialized flag
	vmDB.Put([]byte("initialized"), []byte{0x01})
	
	fmt.Println("\n✅ Migration completed successfully!")
	fmt.Printf("Database ready at: %s\n", targetPath)
	fmt.Printf("VM metadata at: %s\n", vmPath)
	fmt.Printf("Tip: Block %d, Hash %s\n", highestBlock, highestHash.Hex())
}

func equalBytes(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func uint64Ptr(v uint64) *uint64 {
	return &v
}