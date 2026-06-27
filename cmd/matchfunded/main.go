package main

import (
	"bufio"
	"encoding/hex"
	"fmt"
	"os"
	"strings"

	"github.com/luxfi/crypto"
	bip32 "github.com/luxfi/go-bip32"
	bip39 "github.com/luxfi/go-bip39"
)

func deriveEVM(mnemonic string, coin, i uint32) (string, string) {
	seed := bip39.NewSeed(mnemonic, "")
	m, err := bip32.NewMasterKey(seed)
	if err != nil { return "", "" }
	p, _ := m.NewChildKey(bip32.FirstHardenedChild + 44)
	c, _ := p.NewChildKey(bip32.FirstHardenedChild + coin)
	a, _ := c.NewChildKey(bip32.FirstHardenedChild + 0)
	ch, _ := a.NewChildKey(0)
	child, err := ch.NewChildKey(i)
	if err != nil { return "", "" }
	ek, err := crypto.ToECDSA(child.Key)
	if err != nil { return "", "" }
	return strings.ToLower(crypto.PubkeyToAddress(ek.PublicKey).Hex()), hex.EncodeToString(child.Key)
}

func main() {
	funded := map[string]bool{}
	f, _ := os.Open("/tmp/funded.txt")
	sc := bufio.NewScanner(f)
	for sc.Scan() { funded[strings.ToLower(strings.TrimSpace(sc.Text()))] = true }
	for _, mn := range []string{"light light light light light light light light light light light energy"} {
		for _, coin := range []uint32{60, 9000} {
			hits := 0; var first string
			for i := uint32(0); i < 200; i++ {
				addr, priv := deriveEVM(mn, coin, i)
				if funded[addr] { hits++; if first=="" { first=fmt.Sprintf("idx=%d addr=%s priv=%s", i, addr, priv) } }
			}
			fmt.Printf("coin=%d hits=%d/%d  first: %s\n", coin, hits, len(funded), first)
		}
	}
}
