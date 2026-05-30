// Copyright (C) 2019-2026, Lux Industries Inc. All rights reserved.
// See the file LICENSE for licensing terms.

// Command bootstrap-l2 creates one or more L2 EVM subnets on a luxd
// devnet/testnet network using the canonical BIP44 m/44'/9000'/0'/0/<idx>
// derivation. For each chain it:
//
//  1. IssueCreateNetworkTx (P-chain CreateSubnetTx) — owner threshold=1
//  2. IssueCreateChainTx — vmID=nyGCobireNhxFB7iM5bxV74hAY6j9nQX6wizxfWomnMMtztkr,
//     genesis read from the genesis configs dir
//  3. IssueAddChainValidatorTx — adds every primary network validator
//  4. Probes eth_blockNumber and info.isBootstrapped — fails if either is bad
//
// Designed to bootstrap the four canonical Lux devnet L2 EVMs (hanzo, zoo,
// pars, spc) in one pass, but the chains list is data-driven so it can
// bootstrap any subset.
//
// Usage:
//
//	MNEMONIC="..." bootstrap-l2 \
//	  --uri=http://luxd-0.lux-devnet.svc.cluster.local:9650 \
//	  --hrp=dev \
//	  --bip44-idx=5 \
//	  --network-label=devnet \
//	  --configs-dir=/path/to/genesis/configs \
//	  --chains=hanzo,zoo,pars,spc \
//	  --output=/dev/stdout
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	luxbip32 "github.com/luxfi/go-bip32"
	luxbip39 "github.com/luxfi/go-bip39"

	"github.com/luxfi/crypto/secp256k1"
	"github.com/luxfi/ids"
	"github.com/luxfi/math/set"
	"github.com/luxfi/node/utils/formatting/address"
	"github.com/luxfi/node/vms/platformvm/txs"
	"github.com/luxfi/node/wallet/network/primary"
	"github.com/luxfi/sdk/info"
	"github.com/luxfi/sdk/platformvm"
	"github.com/luxfi/utxo/secp256k1fx"
)

// defaultSubnetEVMID is the canonical subnet-evm VM ID present in the luxd
// node image's plugin dir. Devnet, testnet, mainnet all expose this plugin
// natively (verified via `ls /data/plugins`). Mainnet's L2 EVM subnets
// (hanzo/zoo/pars/spc) were created against this same VM ID.
//
// The brand-namespaced alias nyGCobireNhxFB7iM5bxV74hAY6j9nQX6wizxfWomnMMtztkr
// referenced in the 2026-05-27 devnet chain-aliases CM required a runtime
// symlink that didn't survive PVC remount; using the native VM ID removes
// that failure mode.
const defaultSubnetEVMID = "mgj786NP7uDwBCcq6YwThhaN8FLyybkCa4zBWTQbNgmK6k9A6"

// resultChain is the per-chain output written to --output.
type resultChain struct {
	Name           string `json:"name"`
	SubnetID       string `json:"subnetId"`
	BlockchainID   string `json:"blockchainId"`
	EVMChainID     uint64 `json:"evmChainId"`
	FirstBlockHex  string `json:"firstBlockHex"`
	BootstrappedAt string `json:"bootstrappedAt"`
}

type result struct {
	Network     string        `json:"network"`
	URI         string        `json:"uri"`
	ControlKey  string        `json:"controlKey"`
	BIP44Index  uint32        `json:"bip44Index"`
	VMID        string        `json:"vmId"`
	GeneratedAt string        `json:"generatedAt"`
	Chains      []resultChain `json:"chains"`
}

func main() {
	uri := flag.String("uri", "", "luxd API URI (e.g. http://luxd-0.lux-devnet.svc.cluster.local:9650)")
	networkLabel := flag.String("network-label", "devnet", "human label for logs (testnet|devnet)")
	hrp := flag.String("hrp", "dev", "P-chain bech32 HRP: test|dev")
	bipIdx := flag.Uint("bip44-idx", 5, "BIP44 derivation index at m/44'/9000'/0'/0/<idx>")
	configsDir := flag.String("configs-dir", "", "directory containing <chain>-<network>/genesis.json files (required)")
	chainsCSV := flag.String("chains", "hanzo,zoo,pars,spc", "comma-separated chain names")
	vmIDStr := flag.String("vm-id", defaultSubnetEVMID, "subnet-evm VM ID present in luxd's --plugin-dir")
	output := flag.String("output", "/dev/stdout", "result JSON output path")
	skipValidators := flag.Bool("skip-validators", false, "skip adding primary validators as subnet validators")
	probeTimeout := flag.Duration("probe-timeout", 90*time.Second, "max wait per chain for eth_blockNumber>0 and isBootstrapped")
	probeInterval := flag.Duration("probe-interval", 3*time.Second, "polling interval inside probe-timeout")
	subnetSettleDelay := flag.Duration("subnet-settle-delay", 10*time.Second, "delay after IssueCreateNetworkTx before re-syncing wallet")
	printAddrOnly := flag.Bool("print-addr-only", false, "derive the BIP44 key from MNEMONIC, print the P-chain address, exit")
	flag.Parse()

	if *printAddrOnly {
		mn := os.Getenv("MNEMONIC")
		if mn == "" {
			log.Fatal("MNEMONIC env var required")
		}
		key, err := deriveLuxKey(mn, uint32(*bipIdx))
		if err != nil {
			log.Fatalf("derive: %v", err)
		}
		fmt.Println(formatPAddr(*hrp, key.PublicKey().Address()))
		return
	}

	if *uri == "" || *configsDir == "" {
		log.Fatal("--uri and --configs-dir are required")
	}

	vmID, err := ids.FromString(*vmIDStr)
	if err != nil {
		log.Fatalf("invalid --vm-id: %v", err)
	}

	mnemonic := os.Getenv("MNEMONIC")
	if mnemonic == "" {
		log.Fatal("MNEMONIC env var required")
	}

	key, err := deriveLuxKey(mnemonic, uint32(*bipIdx))
	if err != nil {
		log.Fatalf("derive key: %v", err)
	}
	addr := key.PublicKey().Address()
	controlKey := formatPAddr(*hrp, addr)
	log.Printf("[%s] derived key m/44'/9000'/0'/0/%d -> %s", *networkLabel, *bipIdx, controlKey)

	chains := strings.Split(*chainsCSV, ",")
	for i := range chains {
		chains[i] = strings.TrimSpace(chains[i])
	}

	// Pre-flight: load every genesis up front. A bad path is fatal before
	// we burn any P-chain LUX on a CreateNetwork that we can't follow with
	// a CreateChain.
	type chainSpec struct {
		Name       string
		GenesisRaw []byte
		EVMChainID uint64
	}
	specs := make([]chainSpec, 0, len(chains))
	for _, name := range chains {
		path := filepath.Join(*configsDir, fmt.Sprintf("%s-%s", name, *networkLabel), "genesis.json")
		raw, err := os.ReadFile(path)
		if err != nil {
			log.Fatalf("read genesis %s: %v", path, err)
		}
		var doc map[string]any
		if err := json.Unmarshal(raw, &doc); err != nil {
			log.Fatalf("invalid genesis json %s: %v", path, err)
		}
		cfg, _ := doc["config"].(map[string]any)
		if cfg == nil {
			log.Fatalf("genesis %s has no .config", path)
		}
		var evmChainID uint64
		switch v := cfg["chainId"].(type) {
		case float64:
			evmChainID = uint64(v)
		case int:
			evmChainID = uint64(v)
		case int64:
			evmChainID = uint64(v)
		default:
			log.Fatalf("genesis %s: .config.chainId not a number (got %T)", path, v)
		}
		specs = append(specs, chainSpec{Name: name, GenesisRaw: raw, EVMChainID: evmChainID})
		log.Printf("[%s] loaded %s genesis (evm chainId=%d, %d bytes)", *networkLabel, name, evmChainID, len(raw))
	}

	ctx := context.Background()
	kc := primary.NewKeychainAdapter(secp256k1fx.NewKeychain(key))

	infoClient := info.NewClient(*uri)
	if myNodeID, _, err := infoClient.GetNodeID(ctx); err == nil {
		log.Printf("[%s] connected to node %s", *networkLabel, myNodeID)
	}

	pClient := platformvm.NewClient(*uri)
	if balResp, err := pClient.GetBalance(ctx, []ids.ShortID{addr}); err != nil {
		log.Fatalf("getBalance: %v", err)
	} else {
		log.Printf("[%s] P-chain balance of control key: %+v", *networkLabel, balResp)
	}

	var nodeIDs []ids.NodeID
	var minPrimaryEnd uint64
	if !*skipValidators {
		validators, err := pClient.GetCurrentValidators(ctx, ids.Empty, nil)
		if err != nil {
			log.Printf("WARN: GetCurrentValidators failed: %v (skipping validators)", err)
			*skipValidators = true
		} else {
			for _, v := range validators {
				nodeIDs = append(nodeIDs, v.NodeID)
				if minPrimaryEnd == 0 || v.EndTime < minPrimaryEnd {
					minPrimaryEnd = v.EndTime
				}
			}
			log.Printf("[%s] discovered %d primary validators (min end = %d)", *networkLabel, len(nodeIDs), minPrimaryEnd)
		}
	}

	// Initial wallet sync. We re-sync after every subnet creation so the
	// builder sees the new subnetID UTXOs.
	w, err := primary.MakeWallet(ctx, &primary.WalletConfig{
		URI:         *uri,
		LUXKeychain: kc,
		EVMKeychain: kc,
	})
	if err != nil {
		log.Fatalf("wallet sync: %v", err)
	}
	luxAssetID := w.X().Builder().Context().XAssetID
	pBal, err := w.P().Builder().GetBalance()
	if err != nil {
		log.Fatalf("P balance: %v", err)
	}
	log.Printf("[%s] initial wallet P-chain LUX = %d nLUX", *networkLabel, pBal[luxAssetID])

	// Empirical: ~1 LUX for createSubnet + ~0.5 LUX for createChain per L2,
	// plus ~0.005 LUX per addValidatorTx. Floor at 1.5 LUX * len(chains)
	// to refuse work we cannot fund.
	minRequired := uint64(1_500_000_000) * uint64(len(specs))
	if pBal[luxAssetID] < minRequired {
		log.Fatalf("insufficient P-chain balance for %d chains, need >= %d nLUX, have %d nLUX",
			len(specs), minRequired, pBal[luxAssetID])
	}

	owner := &secp256k1fx.OutputOwners{
		Threshold: 1,
		Addrs:     []ids.ShortID{addr},
	}

	out := result{
		Network:     *networkLabel,
		URI:         *uri,
		ControlKey:  controlKey,
		BIP44Index:  uint32(*bipIdx),
		VMID:        vmID.String(),
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Chains:      make([]resultChain, 0, len(specs)),
	}

	for _, spec := range specs {
		log.Printf("[%s] === bootstrapping %s ===", *networkLabel, spec.Name)

		log.Printf("[%s/%s] IssueCreateNetworkTx", *networkLabel, spec.Name)
		createNetTx, err := w.P().IssueCreateNetworkTx(owner)
		if err != nil {
			log.Fatalf("[%s] create network: %v", spec.Name, err)
		}
		subnetID := createNetTx.ID()
		log.Printf("[%s/%s] subnet ID = %s", *networkLabel, spec.Name, subnetID)
		time.Sleep(*subnetSettleDelay)

		// Re-sync wallet with the new subnetID tx fetched, so the builder
		// can authorize the CreateChain spend.
		var w2 primary.Wallet
		for i := 0; i < 5; i++ {
			w2, err = primary.MakeWallet(ctx, &primary.WalletConfig{
				URI:              *uri,
				LUXKeychain:      kc,
				EVMKeychain:      kc,
				PChainTxsToFetch: set.Of(subnetID),
			})
			if err == nil {
				break
			}
			log.Printf("[%s] wallet re-sync attempt %d: %v", spec.Name, i+1, err)
			time.Sleep(5 * time.Second)
		}
		if err != nil {
			log.Fatalf("[%s] wallet re-sync failed: %v", spec.Name, err)
		}

		log.Printf("[%s/%s] IssueCreateChainTx (vmID=%s)", *networkLabel, spec.Name, vmID)
		createChainTx, err := w2.P().IssueCreateChainTx(subnetID, spec.GenesisRaw, vmID, nil, spec.Name)
		if err != nil {
			log.Fatalf("[%s] create chain: %v", spec.Name, err)
		}
		bcID := createChainTx.ID()
		log.Printf("[%s/%s] blockchain ID = %s", *networkLabel, spec.Name, bcID)

		if !*skipValidators && len(nodeIDs) > 0 {
			time.Sleep(3 * time.Second)
			w3, err := primary.MakeWallet(ctx, &primary.WalletConfig{
				URI:              *uri,
				LUXKeychain:      kc,
				EVMKeychain:      kc,
				PChainTxsToFetch: set.Of(subnetID),
			})
			if err != nil {
				log.Printf("[%s] WARN validator wallet sync failed: %v", spec.Name, err)
			} else {
				startTime := time.Now().Add(60 * time.Second)
				endTime := startTime.Add(300 * 24 * time.Hour)
				if minPrimaryEnd > 0 {
					primaryEnd := time.Unix(int64(minPrimaryEnd), 0)
					safe := primaryEnd.Add(-1 * time.Hour)
					if safe.Before(endTime) {
						endTime = safe
					}
				}
				for _, nid := range nodeIDs {
					_, err := w3.P().IssueAddChainValidatorTx(&txs.ChainValidator{
						Validator: txs.Validator{
							NodeID: nid,
							Start:  uint64(startTime.Unix()),
							End:    uint64(endTime.Unix()),
							Wght:   20,
						},
						Chain: subnetID,
					})
					if err != nil {
						log.Printf("[%s] WARN add validator %s: %v", spec.Name, nid, err)
					} else {
						log.Printf("[%s] added subnet validator: %s", spec.Name, nid)
					}
					time.Sleep(1 * time.Second)
				}
			}
		}

		// Use w (not w2) for the next chain: w still has the parent wallet
		// state for issuing a fresh CreateNetwork. Re-sync w so it picks up
		// the spent UTXOs from this iteration.
		w, err = primary.MakeWallet(ctx, &primary.WalletConfig{
			URI:         *uri,
			LUXKeychain: kc,
			EVMKeychain: kc,
		})
		if err != nil {
			log.Fatalf("[%s] post-chain wallet re-sync: %v", spec.Name, err)
		}

		// Probe loop — chain must report isBootstrapped + eth_blockNumber>0.
		firstBlock, err := probeChain(ctx, *uri, bcID.String(), *probeTimeout, *probeInterval)
		if err != nil {
			log.Fatalf("[%s] probe failed: %v", spec.Name, err)
		}
		log.Printf("[%s/%s] PROBE OK: first eth_blockNumber=%s", *networkLabel, spec.Name, firstBlock)

		out.Chains = append(out.Chains, resultChain{
			Name:           spec.Name,
			SubnetID:       subnetID.String(),
			BlockchainID:   bcID.String(),
			EVMChainID:     spec.EVMChainID,
			FirstBlockHex:  firstBlock,
			BootstrappedAt: time.Now().UTC().Format(time.RFC3339),
		})
	}

	// Atomic write — temp + rename so a partial file never leaks.
	body, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		log.Fatalf("marshal result: %v", err)
	}
	if *output == "/dev/stdout" {
		fmt.Println(string(body))
	} else {
		tmp := *output + ".tmp"
		if err := os.WriteFile(tmp, body, 0o644); err != nil {
			log.Fatalf("write output: %v", err)
		}
		if err := os.Rename(tmp, *output); err != nil {
			log.Fatalf("rename output: %v", err)
		}
		log.Printf("wrote %s", *output)
	}
}

// deriveLuxKey derives a secp256k1 private key from a BIP39 mnemonic at the
// canonical Lux web-wallet hardened path m/44'/9000'/0'/0/<idx>.
func deriveLuxKey(mnemonic string, idx uint32) (*secp256k1.PrivateKey, error) {
	mnemonic = strings.TrimSpace(mnemonic)
	if !luxbip39.IsMnemonicValid(mnemonic) {
		return nil, fmt.Errorf("invalid BIP39 mnemonic")
	}
	seed := luxbip39.NewSeed(mnemonic, "")
	master, err := luxbip32.NewMasterKey(seed)
	if err != nil {
		return nil, err
	}
	purpose, err := master.NewChildKey(luxbip32.FirstHardenedChild + 44)
	if err != nil {
		return nil, err
	}
	coinType, err := purpose.NewChildKey(luxbip32.FirstHardenedChild + 9000)
	if err != nil {
		return nil, err
	}
	account, err := coinType.NewChildKey(luxbip32.FirstHardenedChild + 0)
	if err != nil {
		return nil, err
	}
	change, err := account.NewChildKey(0)
	if err != nil {
		return nil, err
	}
	child, err := change.NewChildKey(idx)
	if err != nil {
		return nil, err
	}
	return secp256k1.ToPrivateKey(child.Key)
}

func formatPAddr(hrp string, a ids.ShortID) string {
	b32, err := address.FormatBech32(hrp, a[:])
	if err != nil {
		return ""
	}
	return "P-" + b32
}

// probeChain polls /ext/info isBootstrapped + /ext/bc/<id>/rpc eth_blockNumber
// until both return success or timeout elapses. Returns the first eth_blockNumber
// observed (hex). Fails with a descriptive error if the deadline is missed.
func probeChain(ctx context.Context, uri, chainID string, timeout, interval time.Duration) (string, error) {
	deadline := time.Now().Add(timeout)
	httpc := &http.Client{Timeout: 10 * time.Second}

	for {
		bootstrapped, bootErr := infoIsBootstrapped(ctx, httpc, uri, chainID)
		var blockHex string
		var blockErr error
		if bootstrapped {
			blockHex, blockErr = ethBlockNumber(ctx, httpc, uri, chainID)
		}

		if bootstrapped && blockErr == nil && isPositiveHex(blockHex) {
			return blockHex, nil
		}

		if time.Now().After(deadline) {
			return "", fmt.Errorf("probe deadline exceeded: bootstrapped=%v(%v) blockNumber=%q(%v)",
				bootstrapped, bootErr, blockHex, blockErr)
		}
		time.Sleep(interval)
	}
}

func infoIsBootstrapped(ctx context.Context, c *http.Client, uri, chainID string) (bool, error) {
	body := strings.NewReader(fmt.Sprintf(
		`{"jsonrpc":"2.0","id":1,"method":"info.isBootstrapped","params":{"chain":%q}}`, chainID))
	req, _ := http.NewRequestWithContext(ctx, "POST", uri+"/ext/info", body)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var parsed struct {
		Result struct {
			IsBootstrapped bool `json:"isBootstrapped"`
		} `json:"result"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return false, fmt.Errorf("decode info.isBootstrapped: %w (body=%s)", err, string(raw))
	}
	if parsed.Error != nil {
		return false, fmt.Errorf("rpc error: %s", parsed.Error.Message)
	}
	return parsed.Result.IsBootstrapped, nil
}

func ethBlockNumber(ctx context.Context, c *http.Client, uri, chainID string) (string, error) {
	body := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"eth_blockNumber","params":[]}`)
	url := fmt.Sprintf("%s/ext/bc/%s/rpc", uri, chainID)
	req, _ := http.NewRequestWithContext(ctx, "POST", url, body)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var parsed struct {
		Result string `json:"result"`
		Error  *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", fmt.Errorf("decode eth_blockNumber: %w (body=%s)", err, string(raw))
	}
	if parsed.Error != nil {
		return "", fmt.Errorf("rpc error: %s", parsed.Error.Message)
	}
	return parsed.Result, nil
}

func isPositiveHex(h string) bool {
	if !strings.HasPrefix(h, "0x") {
		return false
	}
	// Accept "0x0" as bootstrapped-without-blocks-yet only if the RPC itself
	// succeeded — our acceptance criteria say >0 is required.
	if h == "0x0" {
		return false
	}
	return true
}
