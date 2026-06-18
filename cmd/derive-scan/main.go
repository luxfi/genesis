// derive-scan: take BIP-39 mnemonic, derive the canonical Avalanche/Lux
// P-Chain (UTXO, coinType=9000) and C-Chain (EVM, coinType=60) addresses
// at index 0..N, print both. Also queries P-Chain balance to confirm
// which index is the funded one.
//
// Run:
//   LUX_MNEMONIC="..." derive-scan --env mainnet --uri https://api.lux.network --scan 10
package main

import (
	"context"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	luxbip32 "github.com/luxfi/go-bip32"
	luxbip39 "github.com/luxfi/go-bip39"

	"github.com/luxfi/crypto/secp256k1"
	"github.com/luxfi/ids"
	"github.com/luxfi/node/utils/formatting/address"
	"github.com/luxfi/sdk/platformvm"
)

func hardened(n uint32) uint32 { return luxbip32.FirstHardenedChild + n }

// Avalanche/Lux standard BIP-44 paths
//   P/X-Chain (UTXO): m/44'/9000'/0'/0/<idx>   — coin type 9000
//   C-Chain  (EVM):  m/44'/60'/0'/0/<idx>      — coin type 60 (ETH)
func pchainSegs(idx uint32) []uint32 {
	return []uint32{hardened(44), hardened(9000), hardened(0), 0, idx}
}
func cchainSegs(idx uint32) []uint32 {
	return []uint32{hardened(44), hardened(60), hardened(0), 0, idx}
}

func deriveKey(seed []byte, segs []uint32) (*secp256k1.PrivateKey, error) {
	master, err := luxbip32.NewMasterKey(seed)
	if err != nil {
		return nil, fmt.Errorf("master: %w", err)
	}
	cur := master
	for _, s := range segs {
		cur, err = cur.NewChildKey(s)
		if err != nil {
			return nil, fmt.Errorf("derive 0x%x: %w", s, err)
		}
	}
	return secp256k1.ToPrivateKey(cur.Key)
}

func hrpForEnv(env string) string {
	switch env {
	case "mainnet":
		return "lux"
	case "testnet":
		return "test"
	case "devnet":
		return "dev"
	case "localnet", "local":
		return "local"
	}
	return "lux"
}

func formatPAddr(hrp string, addr ids.ShortID) string {
	s, err := address.Format("P", hrp, addr.Bytes())
	if err != nil {
		return fmt.Sprintf("[err: %v]", err)
	}
	return s
}

func pathStr(coinType, idx uint32) string {
	return fmt.Sprintf("m/44'/%d'/0'/0/%d", coinType, idx)
}

func main() {
	env := flag.String("env", "mainnet", "mainnet|testnet|devnet|localnet")
	uri := flag.String("uri", "https://api.lux.network", "luxd API URI for balance check (empty to skip)")
	scan := flag.Int("scan", 5, "how many indices [0,N) to derive per chain")
	flag.Parse()

	mnemonic := strings.TrimSpace(os.Getenv("LUX_MNEMONIC"))
	if mnemonic == "" {
		log.Fatal("LUX_MNEMONIC env not set")
	}
	if !luxbip39.IsMnemonicValid(mnemonic) {
		log.Fatal("invalid mnemonic")
	}
	hrp := hrpForEnv(*env)
	seed := luxbip39.NewSeed(mnemonic, "")

	fmt.Printf("env=%s hrp=%s scan=0..%d\n", *env, hrp, *scan-1)
	fmt.Printf("mnemonic words=%d sha256(seed)=%s\n\n", len(strings.Fields(mnemonic)), hex.EncodeToString(seed[:8]))

	var pAddrs []ids.ShortID
	for idx := uint32(0); idx < uint32(*scan); idx++ {
		psk, err := deriveKey(seed, pchainSegs(idx))
		if err != nil {
			log.Fatalf("P idx=%d: %v", idx, err)
		}
		pAddr := psk.PublicKey().Address()
		pAddrs = append(pAddrs, pAddr)
		pBech := formatPAddr(hrp, pAddr)

		csk, err := deriveKey(seed, cchainSegs(idx))
		if err != nil {
			log.Fatalf("C idx=%d: %v", idx, err)
		}
		evm := csk.PublicKey().EVMAddress()

		fmt.Printf("idx=%d\n", idx)
		fmt.Printf("  P-Chain  %s -> P-%s\n", pathStr(9000, idx), pBech)
		fmt.Printf("  C-Chain  %s -> 0x%s\n", pathStr(60, idx), hex.EncodeToString(evm[:]))
	}

	if *uri == "" {
		return
	}

	fmt.Printf("\n--- P-Chain balances @ %s ---\n", *uri)
	pClient := platformvm.NewClient(*uri)
	ctx := context.Background()
	for idx, addr := range pAddrs {
		bal, err := pClient.GetBalance(ctx, []ids.ShortID{addr})
		if err != nil {
			fmt.Printf("idx=%d (P-%s): error %v\n", idx, formatPAddr(hrp, addr), err)
			continue
		}
		funded := ""
		if bal.Balance > 0 {
			funded = "  <-- FUNDED"
		}
		fmt.Printf("idx=%d (P-%s): balance=%d nLUX, unlocked=%d, utxos=%d%s\n",
			idx, formatPAddr(hrp, addr), bal.Balance, bal.Unlocked, len(bal.UTXOIDs), funded)
	}
}
