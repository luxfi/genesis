package main

import (
	"fmt"
	"os"

	"github.com/luxfi/genesis/pkg/genesis"
)

func main() {
	mnemonic := ""
	for _, env := range []string{"MNEMONIC", "LIGHT_MNEMONIC"} {
		if v := os.Getenv(env); v != "" {
			mnemonic = v
			break
		}
	}
	if mnemonic == "" {
		fmt.Println("mnemonic not set (set MNEMONIC or LIGHT_MNEMONIC)")
		os.Exit(1)
	}

	keys, err := genesis.LoadKeysFromMnemonic(mnemonic, 10)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	genesisETH := map[string]string{
		"9011e888251ab053b7bd1cdb598db4f9ded94714": "treasury (100PQ locktime=0)",
		"17f7c3c6d56c41f32a7daf65db0743a7d6cb74a6": "validator 0 (100yr vesting)",
		"775438812d358fd1e090cf04962f1f8f34dafbac": "validator 1 (100yr vesting)",
		"8078fc2bd0df7a5943ba1a81d0ae9572089a24d1": "validator 2 (100yr vesting)",
		"e3c9c6b304b875f0be94481ce1f1f634ceee0338": "validator 3 (100yr vesting)",
		"c18f7344b235903e796f52e8c0e5b21247d3faa4": "validator 4 (100yr vesting)",
	}

	for i, key := range keys {
		ethHex := fmt.Sprintf("%x", key.ETHAddr[:])
		match := ""
		for addr, label := range genesisETH {
			if ethHex == addr {
				match = fmt.Sprintf(" <-- GENESIS MATCH: %s", label)
				break
			}
		}
		fmt.Printf("BIP44 index %d: ETH=0x%x NodeID=%s%s\n", i, key.ETHAddr[:], key.NodeID, match)
	}
}
