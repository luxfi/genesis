// derive60: derive C-Chain EVM addr+privkey on the Ethereum path
// m/44'/60'/0'/0/<i> via luxfi/go-bip32, for the LightMnemonic devnet seed.
// This is the path that funds the C-Chain 500M dev accounts (0x35D64Ff3…).
// No EWOQ key.
package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"strings"

	"github.com/luxfi/crypto"
	bip32 "github.com/luxfi/go-bip32"
	bip39 "github.com/luxfi/go-bip39"
)

func main() {
	mnemonic := flag.String("mnemonic", "light light light light light light light light light light light energy", "BIP-39 mnemonic")
	count := flag.Int("n", 10, "derive indices 0..n-1")
	flag.Parse()

	m := strings.TrimSpace(*mnemonic)
	if !bip39.IsMnemonicValid(m) {
		log.Fatal("invalid mnemonic")
	}
	seed := bip39.NewSeed(m, "")
	master, err := bip32.NewMasterKey(seed)
	if err != nil {
		log.Fatalf("master: %v", err)
	}
	purpose, _ := master.NewChildKey(bip32.FirstHardenedChild + 44)
	coin, _ := purpose.NewChildKey(bip32.FirstHardenedChild + 60)
	acct, _ := coin.NewChildKey(bip32.FirstHardenedChild + 0)
	change, _ := acct.NewChildKey(0)

	for i := 0; i < *count; i++ {
		child, err := change.NewChildKey(uint32(i))
		if err != nil {
			log.Fatalf("derive %d: %v", i, err)
		}
		ecdsa, err := crypto.ToECDSA(child.Key)
		if err != nil {
			log.Fatalf("ToECDSA %d: %v", i, err)
		}
		addr := crypto.PubkeyToAddress(ecdsa.PublicKey)
		fmt.Printf("idx=%d addr=%s privkey=%s\n", i, addr.Hex(), hex.EncodeToString(child.Key))
	}
}
