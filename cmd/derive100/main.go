// Copyright (C) 2019-2025, Lux Industries Inc. All rights reserved.
// derive100 derives 100 P-chain addresses from a mnemonic across testnet/mainnet/devnet
// HRPs, then probes platform.getBalance over JSON-RPC for each to find which idx is funded.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	luxcrypto "github.com/luxfi/crypto/secp256k1"
	"github.com/luxfi/go-bip32"
	"github.com/luxfi/go-bip39"
	"github.com/luxfi/ids"
	"github.com/luxfi/node/utils/formatting/address"
)

const numAccounts = 100

type rpcReq struct {
	JSONRPC string                 `json:"jsonrpc"`
	ID      int                    `json:"id"`
	Method  string                 `json:"method"`
	Params  map[string]interface{} `json:"params"`
}

type rpcResp struct {
	Result struct {
		Balance            string   `json:"balance"`
		Unlocked           string   `json:"unlocked"`
		LockedStakeable    string   `json:"lockedStakeable"`
		LockedNotStakeable string   `json:"lockedNotStakeable"`
		UTXOIDs            []string `json:"utxoIDs"`
	} `json:"result"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func formatPAddr(hrp string, addr ids.ShortID) string {
	b32, err := address.FormatBech32(hrp, addr[:])
	if err != nil {
		return ""
	}
	return "P-" + b32
}

func deriveAddrs(mnemonic, hrp string) []string {
	if !bip39.IsMnemonicValid(mnemonic) {
		fmt.Fprintln(os.Stderr, "invalid mnemonic")
		os.Exit(1)
	}
	seed := bip39.NewSeed(mnemonic, "")
	master, err := bip32.NewMasterKey(seed)
	if err != nil {
		panic(err)
	}
	purpose, _ := master.NewChildKey(bip32.FirstHardenedChild + 44)
	coinType, _ := purpose.NewChildKey(bip32.FirstHardenedChild + 9000)
	account, _ := coinType.NewChildKey(bip32.FirstHardenedChild + 0)
	change, _ := account.NewChildKey(0)

	out := make([]string, numAccounts)
	for i := 0; i < numAccounts; i++ {
		child, err := change.NewChildKey(uint32(i))
		if err != nil {
			panic(err)
		}
		priv, err := luxcrypto.ToPrivateKey(child.Key)
		if err != nil {
			panic(err)
		}
		out[i] = formatPAddr(hrp, priv.Address())
	}
	return out
}

func getBalance(rpc, paddr string) (uint64, string, error) {
	req := rpcReq{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "platform.getBalance",
		Params: map[string]interface{}{
			"addresses": []string{paddr},
		},
	}
	body, _ := json.Marshal(req)
	httpReq, err := http.NewRequest("POST", rpc+"/ext/bc/P", bytes.NewReader(body))
	if err != nil {
		return 0, "", err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	cli := &http.Client{Timeout: 10 * time.Second}
	resp, err := cli.Do(httpReq)
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var r rpcResp
	if err := json.Unmarshal(raw, &r); err != nil {
		return 0, string(raw), err
	}
	if r.Error != nil {
		return 0, r.Error.Message, fmt.Errorf("rpc error")
	}
	bal, _ := parseUint64(r.Result.Balance)
	return bal, r.Result.Balance, nil
}

func parseUint64(s string) (uint64, error) {
	var x uint64
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("non-digit")
		}
		x = x*10 + uint64(c-'0')
	}
	return x, nil
}

func main() {
	if len(os.Args) < 4 {
		fmt.Fprintln(os.Stderr, "usage: derive100 <hrp> <rpc> <mnemonic-env>")
		fmt.Fprintln(os.Stderr, "  hrp: test|lux|dev|local")
		fmt.Fprintln(os.Stderr, "  rpc: http://24.199.71.151:9650")
		fmt.Fprintln(os.Stderr, "  mnemonic-env: MNEMONIC|LIGHT_MNEMONIC")
		os.Exit(1)
	}
	hrp := os.Args[1]
	rpc := os.Args[2]
	envName := os.Args[3]
	mnemonic := strings.TrimSpace(os.Getenv(envName))
	if mnemonic == "" {
		fmt.Fprintf(os.Stderr, "%s not set\n", envName)
		os.Exit(1)
	}

	addrs := deriveAddrs(mnemonic, hrp)

	fmt.Printf("hrp=%s rpc=%s mnemonic=%s\n", hrp, rpc, envName)
	fmt.Printf("idx0 = %s\n", addrs[0])
	fmt.Printf("idx5 = %s\n", addrs[5])

	type hit struct {
		idx int
		bal uint64
		raw string
	}
	hits := []hit{}
	for i, a := range addrs {
		bal, raw, err := getBalance(rpc, a)
		if err != nil {
			fmt.Printf("idx %3d %s ERR: %v %s\n", i, a, err, raw)
			continue
		}
		if bal > 0 {
			hits = append(hits, hit{i, bal, raw})
			fmt.Printf("idx %3d %s balance=%d nLUX (%s LUX)\n", i, a, bal, formatLUX(bal))
		}
	}
	fmt.Printf("\nFUNDED IDXES (%d): ", len(hits))
	for _, h := range hits {
		fmt.Printf("%d ", h.idx)
	}
	fmt.Println()
}

func formatLUX(nLUX uint64) string {
	// On P-chain platform.getBalance returns nLUX (1 LUX = 1e9 nLUX)
	whole := nLUX / 1_000_000_000
	frac := nLUX % 1_000_000_000
	if frac == 0 {
		return fmt.Sprintf("%d", whole)
	}
	return fmt.Sprintf("%d.%09d", whole, frac)
}
