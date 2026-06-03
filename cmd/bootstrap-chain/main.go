// Copyright (C) 2019-2026, Lux Industries Inc. All rights reserved.
// See the file LICENSE for licensing terms.

// Command bootstrap-chain creates one or more chains on a luxd network using the
// canonical BIP44 m/44'/9000'/0'/0/<idx> derivation. For each chain it:
//
//  1. IssueCreateNetworkTx — chain-owner network with threshold=1
//  2. IssueCreateChainTx — vmID=<--vm-id>, genesis read from the configs dir
//  3. IssueAddChainValidatorTx — adds every primary network validator
//  4. Probes eth_blockNumber and info.isBootstrapped — fails if either is bad
//
// Designed to bootstrap the four canonical Lux devnet chains (hanzo, zoo,
// pars, spc) in one pass, but the list is data-driven so it can bootstrap
// any subset.
//
// Idempotency: before any P-chain spend, the tool queries
// platform.getBlockchains and skips any chain whose alias already exists.
// Re-running the tool is safe — already-bootstrapped chains are detected and
// only probed for liveness, never re-created.
//
// Vocabulary: this tool speaks "chain" — the polymorphic primitive produced
// by CreateChainTx. Three IDs, three roles, never aliased:
//
//   - `networkID` — identifies a validator network. Comes in two scopes:
//       * primary networkID (uint32: 1=mainnet, 2=testnet, 3=local, 1337=dev)
//       * per-chain networkID (ids.ID 32 bytes) — the CreateNetworkTx ID
//         that owns one or more chains.
//   - `chainID` — the blockchain's own globally unique ID (ids.ID 32 bytes).
//   - `evmChainID` — EIP-155 chain ID (uint64). EVM JSON-RPC only.
//
// Usage:
//
//	LUX_MNEMONIC="..." bootstrap-chain \
//	  --uri=http://luxd-0.lux-devnet.svc.cluster.local:9650 \
//	  --hrp=dev \
//	  --bip44-idx=5 \
//	  --network-label=devnet \
//	  --configs-dir=/path/to/genesis/configs \
//	  --track-chain-ids=hanzo,zoo,pars,spc \
//	  --output=/dev/stdout
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	luxbip32 "github.com/luxfi/go-bip32"
	luxbip39 "github.com/luxfi/go-bip39"

	"github.com/luxfi/crypto/secp256k1"
	gethcommon "github.com/luxfi/geth/common"
	gethtypes "github.com/luxfi/geth/core/types"
	"github.com/luxfi/ids"
	"github.com/luxfi/math/set"
	"github.com/luxfi/node/utils/formatting/address"
	"github.com/luxfi/node/vms/platformvm/txs"
	"github.com/luxfi/node/wallet/network/primary"
	"github.com/luxfi/sdk/info"
	"github.com/luxfi/sdk/platformvm"
	"github.com/luxfi/utxo/secp256k1fx"
)

// defaultEVMID is the canonical EVM VM ID present in the luxd node image's
// plugin dir. Devnet, testnet, mainnet all expose this plugin natively
// (verified via `ls /data/plugins`). The brand-namespaced alias
// nyGCobireNhxFB7iM5bxV74hAY6j9nQX6wizxfWomnMMtztkr referenced in the
// 2026-05-27 devnet chain-aliases CM required a runtime symlink that didn't
// survive PVC remount; using the native VM ID removes that failure mode.
const defaultEVMID = "mgj786NP7uDwBCcq6YwThhaN8FLyybkCa4zBWTQbNgmK6k9A6"

// resultChain is the per-chain output written to --output.
//
// Brand is the user-facing brand identifier ("hanzo", "zoo", "spc",
// "pars") that downstream consumers (chain-aliases CM, explorer-chains
// CM, gateway routes) map onto. ChainTxName is the P-chain-unique
// identifier passed to IssueCreateChainTx (content-hashed by default).
// NetworkID is the CreateNetworkTx ID — the per-chain validator network
// that owns the blockchain. ChainID is the blockchain's own globally
// unique ID. EVMChainID is the EIP-155 chain ID for EVM JSON-RPC.
// Five distinct concepts; never alias.
type resultChain struct {
	Brand          string `json:"brand"`
	ChainTxName    string `json:"chainTxName"`
	NetworkID      string `json:"networkId"`
	ChainID        string `json:"chainId"`
	EVMChainID     uint64 `json:"evmChainId"`
	FirstBlockHex  string `json:"firstBlockHex"`
	BootstrappedAt string `json:"bootstrappedAt"`
	Reused         bool   `json:"reused"` // true when the chain pre-existed and was only probed
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
	// --track-chain-ids mirrors luxd's existing --track-chain-ids flag. One
	// concept, one flag — this is the declared list of chains this network
	// serves. The tool reads it, queries the P-chain, and idempotently
	// creates the missing ones.
	trackChainIDs := flag.String("track-chain-ids", "hanzo,zoo,pars,spc", "comma-separated brand names (matches luxd --track-chain-ids). Each name resolves a (brand,env) → configs/<brand>-<env>/genesis.json and a content-hashed CreateChainTx name.")
	vmIDStr := flag.String("vm-id", defaultEVMID, "EVM VM ID present in luxd's --plugin-dir")
	// --chain-name-override pins an explicit CreateChainTx name for a single
	// brand (format: <brand>=<name>, repeatable). Bypasses the content-hash
	// default. Use only for legacy reattachment scenarios; the canonical
	// path is the content-hash default which keeps re-runs idempotent and
	// avoids collisions on P-chain.
	chainNameOverride := flag.String("chain-name-override", "", "comma-separated <brand>=<chainName> overrides (e.g. 'hanzo=hanzo-v2,zoo=zoo-final'). Empty = use content-hash default.")
	output := flag.String("output", "/dev/stdout", "result JSON output path")
	skipValidators := flag.Bool("skip-validators", false, "skip adding primary validators as chain validators")
	probeTimeout := flag.Duration("probe-timeout", 90*time.Second, "max wait per chain for eth_blockNumber>0 and isBootstrapped")
	probeInterval := flag.Duration("probe-interval", 3*time.Second, "polling interval inside probe-timeout")
	chainSettleDelay := flag.Duration("chain-settle-delay", 10*time.Second, "delay after IssueCreateNetworkTx before re-syncing wallet")
	printAddrOnly := flag.Bool("print-addr-only", false, "derive the BIP44 key from LUX_MNEMONIC, print the P-chain address, exit")
	evmHeartbeatKeyHex := flag.String("evm-heartbeat-key", "", "hex-encoded secp256k1 key funded on every EVM chain (LUX_PRIVATE_KEY). If set, the tool sends a 0-value self-tx after CreateChainTx to roll block 1 before probing eth_blockNumber>0")
	probeBootstrapOnly := flag.Bool("probe-bootstrap-only", false, "accept the chain as healthy if info.isBootstrapped=true regardless of eth_blockNumber; use when --evm-heartbeat-key is unavailable and the operator will trigger heartbeats out-of-band")
	// --skip-bootstrap-wait is the create-only path: issue CreateNetworkTx +
	// CreateChainTx + AddChainValidatorTx and exit immediately, without
	// waiting for the chain to bootstrap. This is the right tool when the
	// running luxd's --track-chains flag does not yet include the new chain
	// ID — the only way to make luxd track it is to update the ConfigMap and
	// roll the StatefulSet, which must happen AFTER we know the new chain ID.
	skipBootstrapWait := flag.Bool("skip-bootstrap-wait", false, "skip the post-CreateChainTx isBootstrapped+eth_blockNumber probe; use when the running luxd does not yet --track-chains the new chain")
	// --force-new-chains bypasses the platform.getBlockchains existing-chain
	// detection. Use when re-bootstrapping a brand whose name already exists
	// on P-chain but whose previous CreateChainTx used a wrong genesis blob.
	// The new chain will have a NEW blockchain ID; the old chain stays
	// orphaned on P-chain (no way to delete, no harm if untracked by luxd).
	forceNewChains := flag.Bool("force-new-chains", false, "ignore pre-existing chains with the same name on P-chain and always issue a fresh CreateChainTx")
	// To override the fee-payment asset (e.g. on lux-mainnet where staking
	// asset pmSJ7BfZ... differs from the legacy LUX asset HrJCm4yvN... held
	// on P-chain UTXOs), set the env var LUX_WALLET_UTXO_ASSET_ID_OVERRIDE
	// in the process environment before running. The override is read by
	// luxfi/node/wallet/chain/p.NewContextFromClients.
	// --key-file is an alternate entry to LUX_MNEMONIC+BIP44 for callers that
	// already hold the deployer's raw 32-byte secp256k1 key on disk (e.g. the
	// node{1..5}/ec/private.key files used to fund test+dev primary genesis).
	// When set, LUX_MNEMONIC and --bip44-idx are ignored.
	keyFile := flag.String("key-file", "", "path to a 64-char hex secp256k1 private key (alternative to LUX_MNEMONIC+BIP44 derivation)")
	flag.Parse()

	loadKey := func() (*secp256k1.PrivateKey, error) {
		if *keyFile != "" {
			raw, err := os.ReadFile(*keyFile)
			if err != nil {
				return nil, fmt.Errorf("read key-file %s: %w", *keyFile, err)
			}
			hexStr := strings.TrimSpace(strings.TrimPrefix(string(raw), "0x"))
			keyBytes, err := hex.DecodeString(hexStr)
			if err != nil {
				return nil, fmt.Errorf("parse key-file %s as hex: %w", *keyFile, err)
			}
			return secp256k1.ToPrivateKey(keyBytes)
		}
		mn := strings.TrimSpace(os.Getenv("LUX_MNEMONIC"))
		if mn == "" {
			return nil, fmt.Errorf("LUX_MNEMONIC env var or --key-file required")
		}
		return deriveLuxKey(mn, uint32(*bipIdx))
	}

	if *printAddrOnly {
		key, err := loadKey()
		if err != nil {
			log.Fatalf("load key: %v", err)
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

	key, err := loadKey()
	if err != nil {
		log.Fatalf("load key: %v", err)
	}
	addr := key.PublicKey().Address()
	controlKey := formatPAddr(*hrp, addr)
	log.Printf("[%s] derived key m/44'/9000'/0'/0/%d -> %s", *networkLabel, *bipIdx, controlKey)

	chains := strings.Split(*trackChainIDs, ",")
	for i := range chains {
		chains[i] = strings.TrimSpace(chains[i])
	}

	// Parse --chain-name-override (CSV of <brand>=<name>).
	overrideByBrand := map[string]string{}
	if strings.TrimSpace(*chainNameOverride) != "" {
		for _, kv := range strings.Split(*chainNameOverride, ",") {
			kv = strings.TrimSpace(kv)
			if kv == "" {
				continue
			}
			eq := strings.IndexByte(kv, '=')
			if eq <= 0 || eq == len(kv)-1 {
				log.Fatalf("--chain-name-override entry %q: expected <brand>=<name>", kv)
			}
			overrideByBrand[strings.TrimSpace(kv[:eq])] = strings.TrimSpace(kv[eq+1:])
		}
	}

	// Pre-flight: load every genesis up front. A bad path is fatal before
	// we burn any P-chain LUX on a CreateNetwork that we can't follow with
	// a CreateChain. The loader validates 0x-prefixing on alloc keys
	// (auto-fixes in memory, never rewrites the file on disk).
	//
	// Two-name decomposition (decomplected from brand identity):
	//   - Brand          — user-facing brand identifier ("hanzo", "zoo",
	//                      "spc", "pars"). Lives at the chain-aliases CM
	//                      layer (blockchainID → ["hanzo"]). NOT used as
	//                      the CreateChainTx name.
	//   - ChainTxName    — P-chain-unique identifier passed to
	//                      IssueCreateChainTx. Default is content-
	//                      addressed: "<brand>-<8hex>" where 8hex is
	//                      hex(SHA256("<brand>|<env>|"||genesisBytes))
	//                      [:4]. Same canonical genesis → same name (idem-
	//                      potent re-runs). Different genesis → different
	//                      name → no collision.
	//   - GenesisPath    — configs/<brand>-<env>/genesis.json (one
	//                      canonical path per (brand,env); no "fix"
	//                      suffix variants).
	type chainSpec struct {
		Brand       string // user-facing brand identifier
		ChainTxName string // P-chain unique name (content-hash or override)
		GenesisRaw  []byte
		EVMChainID  uint64
	}
	specs := make([]chainSpec, 0, len(chains))
	for _, brand := range chains {
		path := filepath.Join(*configsDir, fmt.Sprintf("%s-%s", brand, *networkLabel), "genesis.json")
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
		// Track D regression gate: brand L2 genesis blobs MUST NOT carry a
		// pre-computed `stateRoot` or `genesisHash`. These two fields in
		// luxfi/geth/core/genesis.go::Genesis.Commit override the result of
		// flushAlloc — meaning a stale value silently displaces the
		// canonical alloc trie. The only legitimate use was the historical
		// Lux primary C-Chain header workaround, now resolved via the
		// empirical block-0 hash in cchain.json. Any brand L2 carrying
		// these fields is a regression — fail loudly before burning LUX on
		// a CreateChainTx with poisoned GenesisData.
		if v, ok := doc["stateRoot"]; ok && v != nil && v != "" {
			log.Fatalf(
				"[%s] %s: genesis %s carries pre-computed stateRoot=%v — brand L2 JSON must let flushAlloc compute the root (see Track D root cause + geth/core/genesis.go:592-595)",
				*networkLabel, brand, path, v,
			)
		}
		if v, ok := doc["genesisHash"]; ok && v != nil && v != "" {
			log.Fatalf(
				"[%s] %s: genesis %s carries pre-computed genesisHash=%v — brand L2 JSON must let toBlockWithRoot compute the hash (see geth/core/genesis.go:602-605)",
				*networkLabel, brand, path, v,
			)
		}
		// Defensive 0x prefix + hex shape validation on alloc keys.
		// The EVM genesis loader rejects unprefixed keys. We normalize
		// and log for missing prefixes (recoverable), and fail loudly for
		// malformed keys (non-hex chars, wrong length) — never silently
		// mutate something we can't prove is just a missing prefix.
		//
		// Acceptance shape after this block:
		//   - every alloc key matches /^0x[0-9a-fA-F]{40}$/
		//   - if any key fails that AND can't be fixed by prepending
		//     `0x`, the whole bootstrap aborts before any CreateChainTx
		//     burns LUX
		alloc, _ := doc["alloc"].(map[string]any)
		if alloc != nil {
			fixed := 0
			ok := 0
			bad := make([]string, 0)
			repaired := make(map[string]any, len(alloc))
			for k, v := range alloc {
				canonical, repair, valid := normalizeAllocKey(k)
				if !valid {
					bad = append(bad, k)
					continue
				}
				repaired[canonical] = v
				if repair {
					fixed++
				} else {
					ok++
				}
			}
			if len(bad) > 0 {
				// Fail loud + give the operator everything they need to
				// fix the genesis at source. Show up to 5 bad keys so
				// the log line stays bounded but useful.
				preview := bad
				if len(preview) > 5 {
					preview = preview[:5]
				}
				log.Fatalf(
					"[%s] %s: %d malformed alloc keys (require canonical /^0x[0-9a-fA-F]{40}$/); sample: %v",
					*networkLabel, brand, len(bad), preview,
				)
			}
			doc["alloc"] = repaired
			if patched, perr := json.Marshal(doc); perr == nil {
				raw = patched
				log.Printf(
					"[%s] %s: alloc keys ok=%d 0x-repaired=%d (in-memory only, file unchanged)",
					*networkLabel, brand, ok, fixed,
				)
			}
		}
		// Decomplect chain.name from brand identity. Default is content-
		// addressed: same canonical genesis → same chain.name → idempotent
		// across re-runs. Different bytes → different chain.name → no
		// collision against stale "hanzo"/"zoo"/"spc"/"pars" chain rows
		// from prior CreateChainTx attempts on P-chain.
		chainTxName := overrideByBrand[brand]
		if chainTxName == "" {
			chainTxName = contentHashChainName(brand, *networkLabel, raw)
		}
		specs = append(specs, chainSpec{
			Brand:       brand,
			ChainTxName: chainTxName,
			GenesisRaw:  raw,
			EVMChainID:  evmChainID,
		})
		log.Printf("[%s] loaded %s genesis (chainTxName=%s, evm chainId=%d, %d bytes)",
			*networkLabel, brand, chainTxName, evmChainID, len(raw))
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

	// Single source of truth for "already created" — query the live
	// P-chain for the current blockchain set, then look up by the
	// content-hashed ChainTxName. This removes the operator burden of
	// maintaining a --existing-X flag in lockstep with cluster reality
	// AND makes the idempotency check exact: a pre-existing chain
	// matches IFF it shares the same canonical genesis bytes (because
	// chain.name encodes the content hash). A previous "hanzo" chain
	// from a different genesis is NOT matched and falls through to a
	// fresh CreateChainTx with a new, distinct chain.name.
	existingByChainTxName := map[string]struct {
		NetworkID ids.ID // parent validator-network (CreateNetworkTx ID)
		ChainID   ids.ID // the blockchain's own ID
	}{}
	if *forceNewChains {
		log.Printf("[%s] FORCE-NEW-CHAINS: skipping platform.getBlockchains existing-chain detection; every spec will issue a fresh CreateNetworkTx + CreateChainTx", *networkLabel)
	} else {
		blockchains, perr := platformGetBlockchains(ctx, *uri)
		if perr != nil {
			log.Printf("WARN: platform.getBlockchains failed: %v (assuming no pre-existing chains)", perr)
		} else {
			byName := map[string]platformBlockchain{}
			for _, b := range blockchains {
				byName[strings.ToLower(b.Name)] = b
			}
			for _, spec := range specs {
				if b, ok := byName[strings.ToLower(spec.ChainTxName)]; ok {
					existingByChainTxName[spec.ChainTxName] = struct {
						NetworkID ids.ID
						ChainID   ids.ID
					}{NetworkID: b.NetworkID, ChainID: b.ID}
					log.Printf("[%s/%s] pre-existing chain found via platform.getBlockchains: chainTxName=%s chainID=%s networkID=%s",
						*networkLabel, spec.Brand, spec.ChainTxName, b.ID, b.NetworkID)
				}
			}
		}
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

	// Initial wallet sync. We re-sync after every chain creation so the
	// builder sees the new chain-owner UTXOs.
	w, err := primary.MakeWallet(ctx, &primary.WalletConfig{
		URI:         *uri,
		LUXKeychain: kc,
		EVMKeychain: kc,
	})
	if err != nil {
		log.Fatalf("wallet sync: %v", err)
	}
	luxAssetID := w.X().Builder().Context().UTXOAssetID
	pBal, err := w.P().Builder().GetBalance()
	if err != nil {
		log.Fatalf("P balance: %v", err)
	}
	log.Printf("[%s] initial wallet P-chain LUX = %d nLUX (assetID=%s)", *networkLabel, pBal[luxAssetID], luxAssetID)

	// Refuse work we cannot fund. We only charge for the chains that don't
	// exist yet — pre-existing chains are skipped entirely.
	pendingCount := 0
	for _, s := range specs {
		if _, ok := existingByChainTxName[s.ChainTxName]; !ok {
			pendingCount++
		}
	}
	// Empirical: ~1 LUX for createNetwork + ~0.5 LUX for createChain per
	// pending chain, plus ~0.005 LUX per addValidator. Floor at 1.5 LUX *
	// pendingCount.
	minRequired := uint64(1_500_000_000) * uint64(pendingCount)
	if pBal[luxAssetID] < minRequired {
		log.Fatalf("insufficient P-chain balance for %d pending chains, need >= %d nLUX, have %d nLUX",
			pendingCount, minRequired, pBal[luxAssetID])
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
		log.Printf("[%s] === bootstrapping %s (chainTxName=%s) ===", *networkLabel, spec.Brand, spec.ChainTxName)

		var networkID ids.ID // chain-owner validator network (CreateNetworkTx ID)
		var chainID ids.ID   // the blockchain's own ID
		reused := false

		if ec, ok := existingByChainTxName[spec.ChainTxName]; ok {
			networkID = ec.NetworkID
			chainID = ec.ChainID
			reused = true
			log.Printf("[%s/%s] REUSE: networkID=%s chainID=%s (skipping both CreateNetworkTx and CreateChainTx)",
				*networkLabel, spec.Brand, networkID, chainID)
		} else {
			log.Printf("[%s/%s] IssueCreateNetworkTx", *networkLabel, spec.Brand)
			createNetTx, err := w.P().IssueCreateNetworkTx(owner)
			if err != nil {
				log.Fatalf("[%s] create network: %v", spec.Brand, err)
			}
			networkID = createNetTx.ID()
			log.Printf("[%s/%s] network ID = %s", *networkLabel, spec.Brand, networkID)
			time.Sleep(*chainSettleDelay)

			// Re-sync wallet with the new chain-owner tx fetched, so the
			// builder can authorize the CreateChain spend.
			var w2 primary.Wallet
			for i := 0; i < 5; i++ {
				w2, err = primary.MakeWallet(ctx, &primary.WalletConfig{
					URI:              *uri,
					LUXKeychain:      kc,
					EVMKeychain:      kc,
					PChainTxsToFetch: set.Of(networkID),
				})
				if err == nil {
					break
				}
				log.Printf("[%s] wallet re-sync attempt %d: %v", spec.Brand, i+1, err)
				time.Sleep(5 * time.Second)
			}
			if err != nil {
				log.Fatalf("[%s] wallet re-sync failed: %v", spec.Brand, err)
			}

			log.Printf("[%s/%s] IssueCreateChainTx (vmID=%s, chainTxName=%s)",
				*networkLabel, spec.Brand, vmID, spec.ChainTxName)
			createChainTx, err := w2.P().IssueCreateChainTx(networkID, spec.GenesisRaw, vmID, nil, spec.ChainTxName)
			if err != nil {
				log.Fatalf("[%s] create chain: %v", spec.Brand, err)
			}
			chainID = createChainTx.ID()
			log.Printf("[%s/%s] blockchain ID = %s", *networkLabel, spec.Brand, chainID)
		}

		if !*skipValidators && len(nodeIDs) > 0 && !reused {
			time.Sleep(3 * time.Second)
			w3, err := primary.MakeWallet(ctx, &primary.WalletConfig{
				URI:              *uri,
				LUXKeychain:      kc,
				EVMKeychain:      kc,
				PChainTxsToFetch: set.Of(networkID),
			})
			if err != nil {
				log.Printf("[%s] WARN validator wallet sync failed: %v", spec.Brand, err)
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
						Chain: networkID,
					})
					if err != nil {
						log.Printf("[%s] WARN add validator %s: %v", spec.Brand, nid, err)
					} else {
						log.Printf("[%s] added chain validator: %s", spec.Brand, nid)
					}
					time.Sleep(1 * time.Second)
				}
			}
		}

		// Use w (not w2) for the next chain: w still has the parent wallet
		// state for issuing a fresh CreateNetwork. Re-sync w so it picks up
		// the spent UTXOs from this iteration. Skip when we didn't touch
		// the P-chain (reused chain).
		if !reused {
			w, err = primary.MakeWallet(ctx, &primary.WalletConfig{
				URI:         *uri,
				LUXKeychain: kc,
				EVMKeychain: kc,
			})
			if err != nil {
				log.Fatalf("[%s] post-chain wallet re-sync: %v", spec.Brand, err)
			}
		}

		// Wait for bootstrap, send heartbeat if configured, then probe.
		// Bootstrap-only probe is required before sending a tx — otherwise
		// eth_sendRawTransaction returns "chain not bootstrapped".
		if *skipBootstrapWait {
			log.Printf("[%s/%s] SKIP-BOOTSTRAP-WAIT: chain registered on P-chain at chainID=%s; operator must update --track-chains and roll luxd to activate", *networkLabel, spec.Brand, chainID)
		} else {
			if err := waitBootstrap(ctx, *uri, chainID.String(), *probeTimeout, *probeInterval); err != nil {
				log.Fatalf("[%s] wait-bootstrap failed: %v", spec.Brand, err)
			}
			log.Printf("[%s/%s] isBootstrapped=true", *networkLabel, spec.Brand)
		}

		if *evmHeartbeatKeyHex != "" && !*skipBootstrapWait {
			log.Printf("[%s/%s] sending EVM heartbeat tx to roll block 1", *networkLabel, spec.Brand)
			if err := evmHeartbeat(ctx, *uri, chainID.String(), spec.EVMChainID, *evmHeartbeatKeyHex); err != nil {
				// Heartbeat failure on a reused chain is non-fatal: the
				// chain may already have blocks. Log and continue.
				if reused {
					log.Printf("[%s/%s] WARN heartbeat on reused chain failed (non-fatal): %v", *networkLabel, spec.Brand, err)
				} else {
					log.Fatalf("[%s] heartbeat failed: %v", spec.Brand, err)
				}
			}
		}

		firstBlock := "0x0"
		if *skipBootstrapWait {
			log.Printf("[%s/%s] SKIP-BOOTSTRAP-WAIT: firstBlockHex=0x0 (chain not yet tracked by luxd)", *networkLabel, spec.Brand)
		} else if !*probeBootstrapOnly {
			firstBlock, err = probeChain(ctx, *uri, chainID.String(), *probeTimeout, *probeInterval)
			if err != nil {
				log.Fatalf("[%s] probe failed: %v", spec.Brand, err)
			}
			log.Printf("[%s/%s] PROBE OK: first eth_blockNumber=%s", *networkLabel, spec.Brand, firstBlock)
		} else {
			// In bootstrap-only mode we still capture whatever
			// eth_blockNumber currently reports; the caller knows it may
			// be 0x0.
			httpc := &http.Client{Timeout: 5 * time.Second}
			if blk, berr := ethBlockNumber(ctx, httpc, *uri, chainID.String()); berr == nil {
				firstBlock = blk
			}
			log.Printf("[%s/%s] BOOTSTRAP-ONLY mode: eth_blockNumber=%s (heartbeat will roll block 1 out-of-band)", *networkLabel, spec.Brand, firstBlock)
		}

		out.Chains = append(out.Chains, resultChain{
			Brand:          spec.Brand,
			ChainTxName:    spec.ChainTxName,
			NetworkID:      networkID.String(),
			ChainID:        chainID.String(),
			EVMChainID:     spec.EVMChainID,
			FirstBlockHex:  firstBlock,
			BootstrappedAt: time.Now().UTC().Format(time.RFC3339),
			Reused:         reused,
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

// platformBlockchain is the subset of platform.getBlockchains we care about.
//
// `NetworkID` is the validator-network (CreateNetworkTx ID) that owns the
// blockchain. `ID` is the blockchain's own globally unique chain ID. Both
// are 32-byte hashes; they identify different things and never alias.
//
// Wire field name `networkID` matches the canonical upstream lux/node
// shape (see lux/node/vms/platformvm/service.go::APIBlockchain).
type platformBlockchain struct {
	ID        ids.ID `json:"id"`
	Name      string `json:"name"`
	NetworkID ids.ID `json:"networkID"`
	VMID      ids.ID `json:"vmID"`
}

// platformGetBlockchains queries the P-chain for the current set of
// blockchains. Returns the parsed list. This is the canonical
// already-exists check for idempotency.
func platformGetBlockchains(ctx context.Context, uri string) ([]platformBlockchain, error) {
	body := strings.NewReader(
		`{"jsonrpc":"2.0","id":1,"method":"platform.getBlockchains","params":{}}`)
	req, _ := http.NewRequestWithContext(ctx, "POST", uri+"/ext/P", body)
	req.Header.Set("Content-Type", "application/json")
	httpc := &http.Client{Timeout: 15 * time.Second}
	resp, err := httpc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var parsed struct {
		Result struct {
			Blockchains []platformBlockchain `json:"blockchains"`
		} `json:"result"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("decode platform.getBlockchains: %w (body=%s)", err, string(raw))
	}
	if parsed.Error != nil {
		return nil, fmt.Errorf("rpc error: %s", parsed.Error.Message)
	}
	return parsed.Result.Blockchains, nil
}

// contentHashChainName returns the P-chain CreateChainTx name for a
// brand-env pair, content-addressed against the canonical genesis bytes.
//
// Format: "<brand>-<8hex>" where 8hex is the first 8 hex chars (4 bytes)
// of SHA256("<brand>|<env>|"||genesisBytes).
//
// Properties:
//   - Deterministic. Same canonical genesis → same chain.name → idempotent
//     re-runs of bootstrap-chain see the chain already exists and skip.
//   - Collision-free against legacy "hanzo"/"zoo"/"spc"/"pars" rows from
//     prior CreateChainTx attempts. Old chains stay orphaned on P-chain;
//     the new chain.name is distinct so CreateChainTx succeeds.
//   - Brand-rooted. The "<brand>-" prefix keeps the value humanly legible
//     ("hanzo-3f4a1c8b") without conflating it with the user-facing
//     brand alias (which lives at the chain-aliases CM layer).
//
// 4 bytes (8 hex chars) gives a 2^32 collision space — overkill for the
// 4-chain set we bootstrap. Operators who need explicit names use the
// --chain-name-override flag.
func contentHashChainName(brand, env string, genesisBytes []byte) string {
	h := sha256.New()
	h.Write([]byte(brand))
	h.Write([]byte("|"))
	h.Write([]byte(env))
	h.Write([]byte("|"))
	h.Write(genesisBytes)
	digest := h.Sum(nil)
	return fmt.Sprintf("%s-%s", brand, hex.EncodeToString(digest[:4]))
}

// normalizeAllocKey returns the canonical 0x-prefixed form of an EVM
// genesis alloc key, a flag indicating whether the input needed repair,
// and a validity flag.
//
// Acceptance shape: /^0x[0-9a-fA-F]{40}$/ after normalization.
//
// Returns:
//   - canonical: the 0x-prefixed lowercase-or-mixed-case form (we don't
//     lowercase the hex itself — Ethereum addresses preserve case as a
//     checksum signal; if the caller mixed cases they want to keep it).
//   - repaired: true if the input was missing a 0x prefix.
//   - valid: false if the key fails the hex+length shape AFTER any prefix
//     repair attempt. Callers must fail loudly on !valid.
//
// Single source of truth for "is this a usable alloc key?" — both the
// in-memory repair in main and any future validation tooling should
// route through this function.
func normalizeAllocKey(k string) (canonical string, repaired bool, valid bool) {
	switch {
	case strings.HasPrefix(k, "0x"):
		canonical = k
	case strings.HasPrefix(k, "0X"):
		// Normalize the prefix to lowercase even when the body cases are
		// preserved — `0X` is non-standard for EVM addresses.
		canonical = "0x" + k[2:]
		repaired = true
	default:
		canonical = "0x" + k
		repaired = true
	}
	body := canonical[2:]
	if len(body) != 40 {
		return canonical, repaired, false
	}
	for _, c := range body {
		switch {
		case c >= '0' && c <= '9':
		case c >= 'a' && c <= 'f':
		case c >= 'A' && c <= 'F':
		default:
			return canonical, repaired, false
		}
	}
	valid = true
	return
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

// waitBootstrap polls /ext/info.isBootstrapped(chain=<id>) until true or timeout.
func waitBootstrap(ctx context.Context, uri, chainID string, timeout, interval time.Duration) error {
	deadline := time.Now().Add(timeout)
	httpc := &http.Client{Timeout: 10 * time.Second}
	for {
		ok, err := infoIsBootstrapped(ctx, httpc, uri, chainID)
		if ok {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("not bootstrapped within %s: %v", timeout, err)
		}
		time.Sleep(interval)
	}
}

// evmHeartbeat sends a 0-value self-transfer using the supplied secp256k1 key.
// Signs with the EVM chainID's EIP-155 v. Returns nil iff eth_sendRawTransaction
// returns a tx hash. The first tx on a fresh EVM chain produces block 1.
func evmHeartbeat(ctx context.Context, uri, chainID string, evmChainID uint64, keyHex string) error {
	keyHex = strings.TrimSpace(strings.TrimPrefix(keyHex, "0x"))
	keyBytes, err := hex.DecodeString(keyHex)
	if err != nil {
		return fmt.Errorf("parse evm key hex: %w", err)
	}
	priv, err := secp256k1.ToPrivateKey(keyBytes)
	if err != nil {
		return fmt.Errorf("parse evm key: %w", err)
	}
	ecdsaPriv := priv.ToECDSA()
	// EVM address derivation: keccak256(pubKey[1:65])[-20:]
	from := evmAddrFromPriv(priv)
	httpc := &http.Client{Timeout: 10 * time.Second}

	// Fetch nonce + suggested gas + chain baseFee
	rpc := fmt.Sprintf("%s/ext/bc/%s/rpc", uri, chainID)
	nonce, err := ethGetTransactionCount(ctx, httpc, rpc, from)
	if err != nil {
		return fmt.Errorf("nonce: %w", err)
	}
	// Use the genesis baseFeePerGas (25 gwei per devnet configs) bumped 100%
	// so the tx beats the min on the very first block.
	gasPrice := new(big.Int).SetUint64(50_000_000_000)

	tx := gethtypes.NewTransaction(nonce, from, big.NewInt(0), 21000, gasPrice, nil)
	signer := gethtypes.NewEIP155Signer(new(big.Int).SetUint64(evmChainID))
	signed, err := gethtypes.SignTx(tx, signer, ecdsaPriv)
	if err != nil {
		return fmt.Errorf("sign tx: %w", err)
	}
	raw, err := signed.MarshalBinary()
	if err != nil {
		return fmt.Errorf("marshal tx: %w", err)
	}
	rawHex := "0x" + hex.EncodeToString(raw)

	body := strings.NewReader(fmt.Sprintf(
		`{"jsonrpc":"2.0","id":1,"method":"eth_sendRawTransaction","params":["%s"]}`, rawHex))
	req, _ := http.NewRequestWithContext(ctx, "POST", rpc, body)
	req.Header.Set("Content-Type", "application/json")
	resp, err := httpc.Do(req)
	if err != nil {
		return fmt.Errorf("post: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	var parsed struct {
		Result string `json:"result"`
		Error  *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return fmt.Errorf("decode: %w (body=%s)", err, string(respBody))
	}
	if parsed.Error != nil {
		return fmt.Errorf("rpc error: %s", parsed.Error.Message)
	}
	log.Printf("    heartbeat tx hash=%s (from=%s nonce=%d)", parsed.Result, from.Hex(), nonce)
	return nil
}

// evmAddrFromPriv computes the EVM address as keccak256(pubKey)[-20:].
func evmAddrFromPriv(priv *secp256k1.PrivateKey) gethcommon.Address {
	ecdsa := priv.ToECDSA()
	luxAddr := secp256k1.PubkeyToAddress(ecdsa.PublicKey)
	return gethcommon.BytesToAddress(luxAddr[:])
}

func ethGetTransactionCount(ctx context.Context, c *http.Client, rpc string, addr gethcommon.Address) (uint64, error) {
	body := strings.NewReader(fmt.Sprintf(
		`{"jsonrpc":"2.0","id":1,"method":"eth_getTransactionCount","params":["0x%x","pending"]}`, addr))
	req, _ := http.NewRequestWithContext(ctx, "POST", rpc, body)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.Do(req)
	if err != nil {
		return 0, err
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
		return 0, fmt.Errorf("decode: %w (body=%s)", err, string(raw))
	}
	if parsed.Error != nil {
		return 0, fmt.Errorf("rpc: %s", parsed.Error.Message)
	}
	hexStr := strings.TrimPrefix(parsed.Result, "0x")
	if hexStr == "" {
		return 0, nil
	}
	n, ok := new(big.Int).SetString(hexStr, 16)
	if !ok {
		return 0, fmt.Errorf("bad hex nonce: %s", parsed.Result)
	}
	return n.Uint64(), nil
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
