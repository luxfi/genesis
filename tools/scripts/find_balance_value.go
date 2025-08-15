package main

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"path/filepath"
	"strings"
	
	"github.com/luxfi/database/badgerdb"
	"github.com/luxfi/geth/common"
	"github.com/luxfi/geth/rlp"
)

func main() {
	dbPath := "/Users/z/.luxd/network-96369/chains/EWi9aPkGe6EfJ3SobCAmSUXRPLa4brF3cThwPwmHTrD1y13jy/ethdb"
	db, _ := badgerdb.New(filepath.Clean(dbPath), nil, "", nil)
	defer db.Close()
	
	treasury := common.HexToAddress("0x9011E888251AB053B7bD1cdB598Db4f9DEd94714")
	treasuryBytes := hex.EncodeToString(treasury.Bytes())
	
	fmt.Printf("Looking for treasury balance data...\n")
	fmt.Printf("Treasury: %s\n", treasury.Hex())
	fmt.Printf("Hex bytes: %s\n\n", treasuryBytes)
	
	// Scan for values containing the treasury address
	iter := db.NewIterator()
	defer iter.Release()
	
	found := 0
	for iter.Next() && found < 10 {
		val := iter.Value()
		valHex := hex.EncodeToString(val)
		
		// Look for treasury address in the value
		if strings.Contains(valHex, treasuryBytes) {
			key := iter.Key()
			fmt.Printf("Found in key: %s\n", hex.EncodeToString(key))
			
			// Try to parse as RLP
			var decoded []interface{}
			if err := rlp.DecodeBytes(val, &decoded); err == nil {
				fmt.Printf("  RLP decoded: %d elements\n", len(decoded))
				
				// Look for balance-like values
				for i, elem := range decoded {
					switch v := elem.(type) {
					case *big.Int:
						if v.Sign() > 0 {
							balanceEth := new(big.Float).Quo(new(big.Float).SetInt(v), big.NewFloat(1e18))
							fmt.Printf("    Element %d (big.Int): %.2f LUX\n", i, balanceEth)
						}
					case []byte:
						if len(v) == 20 && hex.EncodeToString(v) == treasuryBytes {
							fmt.Printf("    Element %d: Treasury address\n", i)
							// Check next element for balance
							if i+1 < len(decoded) {
								if nextVal, ok := decoded[i+1].(*big.Int); ok {
									balanceEth := new(big.Float).Quo(new(big.Float).SetInt(nextVal), big.NewFloat(1e18))
									fmt.Printf("    Next element might be balance: %.2f LUX\n", balanceEth)
								}
							}
						}
					}
				}
			}
			fmt.Println()
			found++
		}
	}
}
