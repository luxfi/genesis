package builder_test

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/luxfi/crypto/secp256k1"
	"github.com/luxfi/genesis/pkg/genesis"
	"github.com/luxfi/ids"
	"github.com/luxfi/utxo/secp256k1fx"
)

func rpc(url, method string, params interface{}) (json.RawMessage, error) {
	payload := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  method,
		"params":  params,
	}
	body, _ := json.Marshal(payload)
	resp, err := http.Post(url, "application/json", strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	var result struct {
		Result json.RawMessage `json:"result"`
		Error  *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("unmarshal: %w (body: %s)", err, string(data))
	}
	if result.Error != nil {
		return nil, fmt.Errorf("rpc error %d: %s", result.Error.Code, result.Error.Message)
	}
	return result.Result, nil
}

func TestTransferPChain(t *testing.T) {
	// Live-network integration test. Set LUX_LIVE_TESTS=1 to opt in;
	// otherwise skipped (default for `go test ./...`).
	if os.Getenv("LUX_LIVE_TESTS") != "1" {
		t.Skip("set LUX_LIVE_TESTS=1 to run live-network tests")
	}

	// fee0 private key
	privKeyHex := "abd51d463510cb17d7ba09e535828383d9c2c817aa386024aacce1660a1ee625"

	// Verify addresses
	keyInfo, err := genesis.ComputeValidatorKeyInfo(privKeyHex)
	if err != nil {
		t.Fatal(err)
	}

	srcAddr, _ := genesis.FormatChainAddress("P", "dev", keyInfo.ShortID)
	fmt.Printf("Source (genesis): %s  ShortID: %x\n", srcAddr, keyInfo.ShortID[:])

	// Target: P-dev1c7wevm4667l4umtzh93r25wpxlpsadkhyluakg (CLI-derived)
	// ShortID: c79d966ebad7bf5e6d62b9623551c137c30eb6d7
	targetHex := "c79d966ebad7bf5e6d62b9623551c137c30eb6d7"
	var targetShortID ids.ShortID
	targetBytes, _ := hex.DecodeString(targetHex)
	copy(targetShortID[:], targetBytes)

	dstAddr, _ := genesis.FormatChainAddress("P", "dev", targetShortID)
	fmt.Printf("Target (CLI): %s  ShortID: %x\n", dstAddr, targetShortID[:])

	// Get UTXOs for source
	pURL := "https://api.lux-dev.network/ext/bc/P"
	utxoResult, err := rpc(pURL, "platform.getUTXOs", map[string]interface{}{
		"addresses": []string{srcAddr},
		"limit":     10,
	})
	if err != nil {
		t.Fatalf("getUTXOs: %v", err)
	}
	fmt.Printf("UTXOs: %s\n", string(utxoResult))

	// Get balance
	balResult, err := rpc(pURL, "platform.getBalance", map[string]interface{}{
		"addresses": []string{srcAddr},
	})
	if err != nil {
		t.Fatalf("getBalance: %v", err)
	}
	fmt.Printf("Balance: %s\n", string(balResult))

	// For the actual transfer, I need to use the SDK wallet
	// But the SDK wallet derives its own address from the key
	// So I need to build the transaction manually

	// Load the key
	privKeyB, _ := hex.DecodeString(privKeyHex)
	privKey, _ := secp256k1.ToPrivateKey(privKeyB)

	// Create keychain
	kc := secp256k1fx.NewKeychain()
	kc.Add(privKey)

	// Print what the SDK derives vs what genesis derives
	sdkAddr := privKey.PublicKey().Address()
	fmt.Printf("\nSDK-derived ShortID: %x\n", sdkAddr[:])
	fmt.Printf("Genesis-derived ShortID: %x\n", keyInfo.ShortID[:])
	fmt.Printf("Match: %v\n", sdkAddr == keyInfo.ShortID)

	_ = context.Background
	_ = time.Second
}
