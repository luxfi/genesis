package main

import (
	"encoding/hex"
	"fmt"
	"path/filepath"
	
	"github.com/luxfi/database/badgerdb"
	"github.com/luxfi/geth/common"
	"golang.org/x/crypto/sha3"
)

func keccak256(data []byte) []byte {
	hasher := sha3.NewLegacyKeccak256()
	hasher.Write(data)
	return hasher.Sum(nil)
}

func main() {
	dbPath := "/Users/z/.luxd/network-96369/chains/EWi9aPkGe6EfJ3SobCAmSUXRPLa4brF3cThwPwmHTrD1y13jy/ethdb"
	db, _ := badgerdb.New(filepath.Clean(dbPath), nil, "", nil)
	defer db.Close()
	
	// Treasury
	addr := common.HexToAddress("0x9011E888251AB053B7bD1cdB598Db4f9DEd94714")
	addrHash := keccak256(addr.Bytes())
	
	fmt.Printf("Address: %s\n", addr.Hex())
	fmt.Printf("Address hash: %s\n\n", hex.EncodeToString(addrHash))
	
	// Check different key patterns
	prefixes := []string{"", "a", "s", "S"}
	for _, prefix := range prefixes {
		var key []byte
		if prefix == "" {
			key = addrHash
		} else {
			key = append([]byte(prefix), addrHash...)
		}
		
		if val, err := db.Get(key); err == nil {
			fmt.Printf("Found with prefix '%s': %s\n", prefix, hex.EncodeToString(val))
			
			// If it's exactly 20 bytes, it might be an address
			if len(val) == 20 {
				fmt.Printf("  -> Looks like address: %s\n", common.BytesToAddress(val).Hex())
			}
		}
	}
	
	// Try to find any key containing the treasury address bytes
	fmt.Println("\nScanning for treasury address in database...")
	iter := db.NewIterator()
	defer iter.Release()
	
	count := 0
	for iter.Next() && count < 1000000 {
		val := iter.Value()
		// Look for the address bytes in values
		addrBytes := addr.Bytes()
		if len(val) >= 20 {
			for i := 0; i <= len(val)-20; i++ {
				if string(val[i:i+20]) == string(addrBytes) {
					key := iter.Key()
					fmt.Printf("Found address in value at key: %s\n", hex.EncodeToString(key))
					fmt.Printf("  Value: %s\n", hex.EncodeToString(val))
					if len(key) > 0 {
						fmt.Printf("  Key prefix: '%c' (0x%02x)\n", key[0], key[0])
					}
					break
				}
			}
		}
		count++
	}
	fmt.Printf("Scanned %d entries\n", count)
}
