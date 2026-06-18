// derivekey: derive the C-Chain EVM private key + address for a given index from
// a BIP-39 mnemonic using the CANONICAL Lux genesis derivation path
// m/44'/9000'/0'/0/<i> (the SAME path pkg/genesis.LoadKeysFromMnemonic funds in
// the genesis alloc — coin type 9000, NOT the coin-type-60 ETH path). For
// localnet (network 3/1337) the well-known dev seed is genesis.LightMnemonic.
// This lets a test driver sign C-Chain txs from a genesis-FUNDED account without
// any EWOQ key.
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
	mnemonic := flag.String("mnemonic", "light light light light light light light light light light light energy", "BIP-39 mnemonic (default: LightMnemonic dev seed)")
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
		child, err := change.NewChildKey(uint32(i)) //nolint:gosec
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
