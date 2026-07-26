// Copyright (C) 2026, Lux Industries Inc. All rights reserved.
// See the file LICENSE for licensing terms.

// Command createchaintx is the focused, immediate-unblock tool for registering
// the 5 brand L2 EVMs (hanzo, zoo, pars, spc, osage) on a Quasar-era P-Chain.
//
// Decoupled from luxfi/cli: this tool does not call info.getBlockchainID(alias=X)
// anywhere — it goes through luxfi/node/wallet/network/primary.MakeWallet which
// fail-softs to a P-only XCTX (BlockchainID = ids.Empty) when the network has no
// X-Chain registered. That's the actual unblock — every other path you might
// hit in luxfi/cli's PublicDeployer.loadWallet currently crashes because the
// upstream sdk/wallet/primary still tries to look up X first.
//
// What this tool does, end-to-end:
//
//  1. Loads a BIP-39 mnemonic from one of three sources, in order of priority:
//     a. --mnemonic-file=<path>          (raw file, mnemonic is the only content)
//     b. LUX_MNEMONIC env var
//     c. KMS via env vars KMS_ENDPOINT + KMS_TOKEN + KMS_ORG, fetching the
//     secret at path lux/<env>/staking named `mnemonic`:
//     GET ${KMS_ENDPOINT}/v1/kms/orgs/<org>/secrets/lux/<env>/staking/mnemonic
//
//  2. Derives a single secp256k1 funding key on the canonical Lux BIP-44 UTXO
//     path m/44'/9000'/0'/0/<index> (purpose=44', coin_type=9000', account=0',
//     change=0, leaf=<index>, default index=5 → first non-staking-locked
//     allocation slot per the mainnet genesis layout). 9000 is the Lux/Avalanche
//     BIP-44 coin type — do not confuse with 96369, which is the Lux mainnet
//     EVM chainID and never appears in a derivation path. Override with --index
//     to use a different funded allocation slot.
//
//     Distinct paths exist for other key purposes — they are NOT this funding key:
//     * device Lux key  (Lux-internal hardened): m/44'/9000'/<nid>'/1'/<i>'
//     * device PQ key   (ML-DSA-65 staking):     m/44'/9000'/<nid>'/0'/<i>'
//     where <nid> is the primary-network ID (1/2/3/1337), NOT an EVM chainID.
//
//  3. For each of the 5 brand specs, loads the canonical brand genesis bytes
//     from disk (paths are not relative to --configs-dir; the tool ships a
//     compiled-in canonical mapping per brand-genesis-homes.md), then:
//     a. IssueCreateNetworkTx(owner) → per-chain validator network
//     b. IssueCreateChainTx(networkID, genesis, vmID, fxIDs=[], chainName=brand)
//     Skips both steps if a chain with chainName=brand already exists (the
//     idempotency contract — re-running this tool is safe).
//
//  4. Emits JSON of produced blockchain IDs to stdout (or --output=<path>).
//
//  5. With --apply-aliases, writes a kubectl-ready Patch JSON that updates the
//     `luxd-chain-aliases` (or `chain-aliases`) ConfigMap in namespace
//     lux-<env> with the new {blockchainID: [brand]} mapping.
//
// The tool deliberately omits AddChainValidatorTx, bootstrap-wait, EVM
// heartbeat, and eth_blockNumber probing — those are the bootstrap-chain
// concerns, not the immediate-unblock concerns. Once the chains are
// registered on P-chain, the running luxd will pick them up on the next
// roll (after the chain-aliases CM is updated and StatefulSet is rolled).
//
// Build:
//
//	cd ~/work/lux/genesis/cmd && GOWORK=off go build -o /tmp/createchaintx ./createchaintx/
//
// Example invocations:
//
//	# Mainnet, mnemonic in a local file, write output to /tmp/result.json
//	createchaintx \
//	  --uri=https://api.lux.network \
//	  --env=mainnet \
//	  --mnemonic-file=/tmp/lux-master.mnemonic \
//	  --output=/tmp/result.json
//
//	# Devnet, mnemonic from KMS, also emit aliases patch for kubectl apply
//	export KMS_ENDPOINT=https://kms.hanzo.ai
//	export KMS_TOKEN=...
//	export KMS_ORG=lux
//	createchaintx \
//	  --uri=http://luxd-0.lux-devnet.svc.cluster.local:9650 \
//	  --env=devnet \
//	  --output=/tmp/result.json \
//	  --apply-aliases=/tmp/aliases-patch.json
package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
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
	"github.com/luxfi/node/wallet/network/primary"
	"github.com/luxfi/sdk/info"
	"github.com/luxfi/sdk/platformvm"
	"github.com/luxfi/utxo/secp256k1fx"
)

// defaultVMID is the canonical EVM plugin VM ID baked into every luxd image
// (verified via `ls /data/plugins`). Same value the 2026-06-06 inventory
// records, same value across mainnet/testnet/devnet. Override with --vm-id
// only if a non-default plugin is provisioned.
const defaultVMID = "mgj786NP7uDwBCcq6YwThhaN8FLyybkCa4zBWTQbNgmK6k9A6"

// stakingPath is the canonical Avalanche/Lux BIP-44 derivation path for
// the P-Chain UTXO key: m/44'/9000'/0'/0/<index>.
//
// purpose=44 (BIP-44), coinType=9000 (Avalanche/Lux platform), account=0,
// change=0, index=<funded slot>. C-Chain EVM keys use the same shape with
// coinType=60 (ETH) — out of scope here, CreateChainTx is a P-Chain op.
//
// The funded mainnet genesis allocations match this derivation:
//
//	index 0..4  -> initialStakedFunds (bonded as initial validator stake,
//	               500M LUX each, locked until validator end-time)
//	index 5..99 -> 100 unlock-schedule allocations, ~35M LUX unlocked
//	               per address at network start. CreateChainTx fee is
//	               trivial vs that — any non-staking slot is funded.
//
// Default --index = 5 (first free-spendable slot). Override with --index
// for any allocations[<N>].utxoAddr address you control.
type stakingPath struct {
	purpose, coinType, account, change, index uint32
}

var defaultStakingPath = stakingPath{
	purpose:  44,   // hardened — BIP-44
	coinType: 9000, // hardened — Avalanche/Lux platform
	account:  0,    // hardened
	change:   0,    // unhardened
	index:    5,    // unhardened — first non-staking-locked allocation
}

// brandSpec maps a brand identifier to its canonical genesis location.
// Single source of truth, lives inline so the binary is self-contained
// (no --configs-dir flag, no /tmp/bootstrap-configs shuffling).
//
// The pathTemplate is filled with the env name via fmt.Sprintf. Each
// brand owns its own genesis (per ~/work/lux/genesis/configs/inventory/
// brand-genesis-homes.md), so the home directories differ.
type brandSpec struct {
	brand        string
	pathTemplate string // %s = env name (mainnet|testnet|devnet)
}

// brandSpecs are the 5 brand L2 EVMs registered on Lux primary network.
// Order matters: CreateChainTx ordering produces deterministic chain IDs
// per spec list, which keeps re-runs aligned to the inventory file.
var brandSpecs = []brandSpec{
	{brand: "hanzo", pathTemplate: "%s/work/hanzo/universe/configs/genesis/%s/genesis.json"},
	{brand: "zoo", pathTemplate: "%s/work/zoo/universe/configs/genesis/%s/genesis.json"},
	{brand: "pars", pathTemplate: "%s/work/pars/genesis/networks/%s/genesis.json"},
	{brand: "spc", pathTemplate: "%s/work/spc/genesis/networks/%s/genesis.json"},
	{brand: "osage", pathTemplate: "%s/work/osage/genesis/networks/%s/genesis.json"},
}

// result is the JSON shape emitted to stdout/--output. Keyed by env so
// downstream consumers (chain-aliases CM templating, gateway routes,
// explorer-chains CM) can index into a single document for all envs.
type result struct {
	Env         string              `json:"env"`
	URI         string              `json:"uri"`
	ControlKey  string              `json:"controlKey"`
	VMID        string              `json:"vmId"`
	GeneratedAt string              `json:"generatedAt"`
	Chains      map[string]chainRow `json:"chains"`
}

// chainRow records one brand's outcome. NetworkID is the per-chain
// validator network (CreateNetworkTx ID, owner of the blockchain).
// ChainID is the blockchain's globally unique ID. Reused=true means
// the chain was already present on P-chain and we only recorded it
// without burning LUX on a duplicate CreateChainTx.
type chainRow struct {
	NetworkID string `json:"networkId"`
	ChainID   string `json:"chainId"`
	Reused    bool   `json:"reused"`
}

func main() {
	uri := flag.String("uri", "", "luxd API URI (e.g. https://api.lux.network)")
	env := flag.String("env", "", "network env: mainnet|testnet|devnet (required)")
	hrp := flag.String("hrp", "", "P-chain bech32 HRP (auto-resolved from --env if empty)")
	mnemonicFile := flag.String("mnemonic-file", "", "path to a file whose contents are the BIP-39 mnemonic (alternative to LUX_MNEMONIC env or KMS)")
	vmIDStr := flag.String("vm-id", defaultVMID, "EVM plugin VM ID present in luxd's --plugin-dir")
	output := flag.String("output", "/dev/stdout", "result JSON output path")
	applyAliases := flag.String("apply-aliases", "", "if set, write a kubectl-ready JSON patch for the chain-aliases ConfigMap to this path")
	homeOverride := flag.String("home", "", "override $HOME for the brand-genesis path template (empty = use real $HOME)")
	forceNew := flag.Bool("force-new", false, "ignore pre-existing chains with the same brand name on P-chain; always issue a fresh CreateChainTx")
	derivIndex := flag.Uint("index", uint(defaultStakingPath.index), "BIP-44 leaf index at m/44'/9000'/0'/0/<index> (default 5: first allocation slot not bonded to initial validator stake)")
	brandsCSV := flag.String("brands", "", "comma-separated subset of brands to process in list order (empty = all five). E.g. --brands=hanzo,zoo")
	flag.Parse()

	if *uri == "" || *env == "" {
		log.Fatal("--uri and --env are required (try --help)")
	}

	// Optional brand subset: process only the named brands (preserving the
	// canonical list order so chain-creation ordering stays deterministic).
	// Unknown brand names are a hard error — a typo must not silently create
	// a different set of chains than intended.
	if csv := strings.TrimSpace(*brandsCSV); csv != "" {
		want := map[string]bool{}
		for _, b := range strings.Split(csv, ",") {
			want[strings.ToLower(strings.TrimSpace(b))] = true
		}
		filtered := brandSpecs[:0:0]
		seen := map[string]bool{}
		for _, bs := range brandSpecs {
			if want[bs.brand] {
				filtered = append(filtered, bs)
				seen[bs.brand] = true
			}
		}
		for b := range want {
			if !seen[b] {
				log.Fatalf("--brands: unknown brand %q (known: hanzo,zoo,pars,spc,osage)", b)
			}
		}
		brandSpecs = filtered
		log.Printf("brand subset: %s", csv)
	}
	resolvedHRP := *hrp
	if resolvedHRP == "" {
		var err error
		resolvedHRP, err = hrpForEnv(*env)
		if err != nil {
			log.Fatalf("resolve hrp: %v", err)
		}
	}
	vmID, err := ids.FromString(*vmIDStr)
	if err != nil {
		log.Fatalf("invalid --vm-id %q: %v", *vmIDStr, err)
	}

	mnemonic, mnemonicSource, err := loadMnemonic(*mnemonicFile, *env)
	if err != nil {
		log.Fatalf("load mnemonic: %v", err)
	}
	log.Printf("[%s] loaded mnemonic from %s (%d words)", *env, mnemonicSource, countWords(mnemonic))

	derivPath := defaultStakingPath
	derivPath.index = uint32(*derivIndex)
	key, err := deriveStakingKey(mnemonic, derivPath)
	if err != nil {
		log.Fatalf("derive funding key: %v", err)
	}
	addr := key.PublicKey().Address()
	controlKey := formatPAddr(resolvedHRP, addr)
	log.Printf("[%s] funding key m/44'/9000'/0'/0/%d -> %s", *env, derivPath.index, controlKey)

	home := *homeOverride
	if home == "" {
		home, err = os.UserHomeDir()
		if err != nil {
			log.Fatalf("user home: %v", err)
		}
	}

	// Pre-flight: load all 5 genesis blobs. Fail before any P-chain spend if
	// any path is missing or malformed — the alternative (start spending LUX
	// then crash at chain #3) is exactly the failure mode this tool exists
	// to avoid.
	type loadedSpec struct {
		brand   string
		genesis []byte
	}
	specs := make([]loadedSpec, 0, len(brandSpecs))
	for _, bs := range brandSpecs {
		path := filepath.Clean(fmt.Sprintf(bs.pathTemplate, home, *env))
		// pathTemplate has a leading "%s/work/..." — the first %s is the
		// home dir. Strip a stray leading "/" if home already ends in "/".
		path = strings.ReplaceAll(path, "//", "/")
		raw, err := os.ReadFile(path)
		if err != nil {
			log.Fatalf("[%s] read genesis %s: %v", bs.brand, path, err)
		}
		if !json.Valid(raw) {
			log.Fatalf("[%s] genesis %s: invalid JSON", bs.brand, path)
		}
		specs = append(specs, loadedSpec{brand: bs.brand, genesis: raw})
		log.Printf("[%s] %s genesis loaded (%d bytes) from %s", *env, bs.brand, len(raw), path)
	}

	ctx := context.Background()
	kc := primary.NewKeychainAdapter(secp256k1fx.NewKeychain(key))

	infoClient := info.NewClient(*uri)
	if nodeID, _, err := infoClient.GetNodeID(ctx); err == nil {
		log.Printf("[%s] connected to luxd node %s", *env, nodeID)
	}
	pClient := platformvm.NewClient(*uri)
	if balResp, err := pClient.GetBalance(ctx, []ids.ShortID{addr}); err != nil {
		log.Fatalf("getBalance: %v", err)
	} else {
		log.Printf("[%s] P-chain balance of %s: %+v", *env, controlKey, balResp)
	}

	// Tighten HTTP timeouts so DNS entries with mixed live/dead IPs don't
	// stall the wallet for minutes per dead IP. Matches the timeout shape
	// PublicDeployer.loadWallet sets — keeping behaviour predictable.
	if t, ok := http.DefaultTransport.(*http.Transport); ok {
		t.DialContext = (&net.Dialer{Timeout: 5 * time.Second}).DialContext
		t.TLSHandshakeTimeout = 10 * time.Second
		t.TLSClientConfig = &tls.Config{InsecureSkipVerify: false}
	}

	// Idempotency: query the live P-chain for existing blockchains. Any brand
	// whose chainName already matches a registered chain is skipped (we record
	// the existing IDs in the result, no new CreateChainTx). --force-new
	// bypasses this check.
	existing := map[string]struct {
		NetworkID ids.ID
		ChainID   ids.ID
	}{}
	if *forceNew {
		log.Printf("[%s] FORCE-NEW: skipping pre-existing-chain detection; every brand will issue a fresh CreateChainTx", *env)
	} else {
		rows, err := platformGetBlockchains(ctx, *uri)
		if err != nil {
			log.Printf("WARN platform.getBlockchains failed: %v (assuming no pre-existing chains)", err)
		} else {
			byName := map[string]platformBlockchain{}
			for _, b := range rows {
				byName[strings.ToLower(b.Name)] = b
			}
			for _, s := range specs {
				if b, ok := byName[strings.ToLower(s.brand)]; ok {
					existing[s.brand] = struct {
						NetworkID ids.ID
						ChainID   ids.ID
					}{NetworkID: b.NetworkID, ChainID: b.ID}
					log.Printf("[%s/%s] pre-existing chain: chainID=%s networkID=%s (skipping CreateChainTx)",
						*env, s.brand, b.ID, b.NetworkID)
				}
			}
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
	bal, err := w.P().Builder().GetBalance()
	if err != nil {
		log.Fatalf("P balance: %v", err)
	}
	log.Printf("[%s] initial P-chain wallet LUX=%d nLUX (assetID=%s)", *env, bal[luxAssetID], luxAssetID)

	out := result{
		Env:         *env,
		URI:         *uri,
		ControlKey:  controlKey,
		VMID:        vmID.String(),
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Chains:      make(map[string]chainRow, len(specs)),
	}

	for _, s := range specs {
		log.Printf("[%s/%s] === processing ===", *env, s.brand)

		var (
			networkID ids.ID
			chainID   ids.ID
			reused    bool
		)
		if ec, ok := existing[s.brand]; ok {
			networkID, chainID, reused = ec.NetworkID, ec.ChainID, true
			log.Printf("[%s/%s] REUSE: chainID=%s networkID=%s", *env, s.brand, chainID, networkID)
		} else {
			// Each brand gets a Lux-native network with an owner OutputOwners
			// set to our funding key. Required because IssueCreateChainTx
			// needs a "subnet owner" UTXO to authorize the spend — the
			// primary network has no owner tx in P-Chain history, so a
			// chain on primary cannot be authored that way.
			//
			// Owner network created here is tracked by luxd via
			// chainTracking.trackAllChains=true in the LuxNetwork CR, so
			// chains on these owner networks serve under their aliases
			// without needing a per-subnet validator registration.
			owner := &secp256k1fx.OutputOwners{
				Threshold: 1,
				Addrs:     []ids.ShortID{addr},
			}
			log.Printf("[%s/%s] IssueCreateNetworkTx", *env, s.brand)
			netTx, err := w.P().IssueCreateNetworkTx(owner)
			if err != nil {
				log.Fatalf("[%s/%s] create network: %v", *env, s.brand, err)
			}
			networkID = netTx.ID()
			log.Printf("[%s/%s] networkID=%s", *env, s.brand, networkID)
			time.Sleep(8 * time.Second) // mempool settle for owner tx

			// Wallet re-sync: pick up the new owner UTXO so the chain
			// auth can be built.
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
				log.Printf("[%s/%s] wallet re-sync attempt %d: %v", *env, s.brand, i+1, err)
				time.Sleep(5 * time.Second)
			}
			if err != nil {
				log.Fatalf("[%s/%s] wallet re-sync failed: %v", *env, s.brand, err)
			}

			log.Printf("[%s/%s] IssueCreateChainTx vmID=%s chainName=%s genesisBytes=%d",
				*env, s.brand, vmID, s.brand, len(s.genesis))
			chainTx, err := w2.P().IssueCreateChainTx(networkID, s.genesis, vmID, nil, s.brand)
			if err != nil {
				log.Fatalf("[%s/%s] create chain: %v", *env, s.brand, err)
			}
			chainID = chainTx.ID()
			log.Printf("[%s/%s] chainID=%s", *env, s.brand, chainID)

			// Re-sync parent wallet for next brand iteration.
			w, err = primary.MakeWallet(ctx, &primary.WalletConfig{
				URI:         *uri,
				LUXKeychain: kc,
				EVMKeychain: kc,
			})
			if err != nil {
				log.Fatalf("[%s/%s] post-chain wallet re-sync: %v", *env, s.brand, err)
			}
		}

		out.Chains[s.brand] = chainRow{
			NetworkID: networkID.String(),
			ChainID:   chainID.String(),
			Reused:    reused,
		}
	}

	// Marshal and write the result.
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

	if *applyAliases != "" {
		if err := writeAliasesPatch(*applyAliases, *env, out.Chains); err != nil {
			log.Fatalf("write aliases patch: %v", err)
		}
		log.Printf("wrote aliases patch %s", *applyAliases)
		log.Printf("APPLY: kubectl --context do-sfo3-lux-k8s -n lux-%s patch cm chain-aliases --type merge --patch-file %s",
			*env, *applyAliases)
	}
}

// hrpForEnv resolves the bech32 HRP for the given network env. lux primary
// uses HRPs `lux` (mainnet, networkID=1), `test` (testnet, networkID=2),
// `dev` (devnet/localnet, networkID=3 and 1337). The mapping lives in
// luxfi/constants — we mirror it here so the tool is self-contained.
func hrpForEnv(env string) (string, error) {
	switch strings.ToLower(env) {
	case "mainnet":
		return "lux", nil
	case "testnet":
		return "test", nil
	case "devnet", "local", "localnet":
		return "dev", nil
	default:
		return "", fmt.Errorf("unknown env %q (expected mainnet|testnet|devnet)", env)
	}
}

// countWords returns the BIP-39 word count of mnemonic, used purely for
// the log line — never for validation.
func countWords(mnemonic string) int {
	return len(strings.Fields(mnemonic))
}

// loadMnemonic resolves the mnemonic from one of three sources in priority
// order: --mnemonic-file, LUX_MNEMONIC env var, KMS HTTP GET via Infisical's
// v3 raw secrets API. Returns the mnemonic and a human-readable source
// label for logging. Always validates with bip39 before returning.
func loadMnemonic(mnemonicFile, env string) (string, string, error) {
	if mnemonicFile != "" {
		raw, err := os.ReadFile(mnemonicFile)
		if err != nil {
			return "", "", fmt.Errorf("read %s: %w", mnemonicFile, err)
		}
		mn := strings.TrimSpace(string(raw))
		if !luxbip39.IsMnemonicValid(mn) {
			return "", "", fmt.Errorf("file %s: invalid BIP-39 mnemonic", mnemonicFile)
		}
		return mn, fmt.Sprintf("file %s", mnemonicFile), nil
	}
	if mn := strings.TrimSpace(os.Getenv("LUX_MNEMONIC")); mn != "" {
		if !luxbip39.IsMnemonicValid(mn) {
			return "", "", errors.New("env LUX_MNEMONIC: invalid BIP-39 mnemonic")
		}
		return mn, "env LUX_MNEMONIC", nil
	}
	endpoint := strings.TrimRight(strings.TrimSpace(os.Getenv("KMS_ENDPOINT")), "/")
	token := strings.TrimSpace(os.Getenv("KMS_TOKEN"))
	org := strings.TrimSpace(os.Getenv("KMS_ORG"))
	if org == "" {
		org = "lux"
	}
	if endpoint == "" || token == "" {
		return "", "", errors.New("no mnemonic source: set --mnemonic-file, LUX_MNEMONIC env, or KMS_ENDPOINT+KMS_TOKEN env")
	}
	mn, err := fetchKMSMnemonic(endpoint, token, org, env)
	if err != nil {
		return "", "", fmt.Errorf("kms fetch: %w", err)
	}
	if !luxbip39.IsMnemonicValid(mn) {
		return "", "", errors.New("kms-returned mnemonic: invalid BIP-39 mnemonic")
	}
	return mn, fmt.Sprintf("KMS %s lux/%s/staking[mnemonic]", endpoint, env), nil
}

// fetchKMSMnemonic reads the `mnemonic` secret at path lux/<env>/staking from
// luxfi/kms — the one place secrets live.
//
//	GET ${KMS_ENDPOINT}/v1/kms/orgs/<org>/secrets/lux/<env>/staking/mnemonic
//	Authorization: Bearer ${KMS_TOKEN}   (IAM-signed, resolved to <org>)
//	-> {"secret": {"value": "..."}}
//
// The server splits the path at its last "/" into (path, name), so the secret
// path disambiguates the luxd env; the ?env= slug is the KMS environment and
// defaults to "default", matching the luxfi/kms client's KMS_ENV.
//
// This previously called Infisical's /api/v3/secrets/raw. That endpoint is not
// served by kms.lux.network or kms.hanzo.ai — a request there returns the
// console SPA's HTML, so the JSON decode failed and no mnemonic was ever
// retrievable through this path.
func fetchKMSMnemonic(endpoint, token, org, env string) (string, error) {
	kmsEnv := strings.TrimSpace(os.Getenv("KMS_ENV"))
	if kmsEnv == "" {
		kmsEnv = "default"
	}
	q := url.Values{}
	q.Set("env", kmsEnv)
	reqURL := fmt.Sprintf("%s/v1/kms/orgs/%s/secrets/lux/%s/staking/mnemonic?%s",
		endpoint, url.PathEscape(org), url.PathEscape(env), q.Encode())
	req, err := http.NewRequest(http.MethodGet, reqURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	httpc := &http.Client{Timeout: 20 * time.Second}
	resp, err := httpc.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("kms http %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var doc struct {
		Secret struct {
			Value string `json:"value"`
		} `json:"secret"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return "", fmt.Errorf("decode kms response: %w", err)
	}
	if v := strings.TrimSpace(doc.Secret.Value); v != "" {
		return v, nil
	}
	return "", errors.New("kms returned an empty mnemonic")
}

// deriveStakingKey derives a secp256k1 private key from a BIP-39 mnemonic
// at the supplied BIP-32 path. Hardened nodes have FirstHardenedChild
// (0x80000000) OR'd in by the bip32 library when called via NewChildKey
// with the hardened index already pre-set.
func deriveStakingKey(mnemonic string, p stakingPath) (*secp256k1.PrivateKey, error) {
	mnemonic = strings.TrimSpace(mnemonic)
	if !luxbip39.IsMnemonicValid(mnemonic) {
		return nil, errors.New("invalid BIP-39 mnemonic")
	}
	seed := luxbip39.NewSeed(mnemonic, "")
	master, err := luxbip32.NewMasterKey(seed)
	if err != nil {
		return nil, err
	}
	// Hardened steps: purpose'/coinType'/account'
	purpose, err := master.NewChildKey(luxbip32.FirstHardenedChild + p.purpose)
	if err != nil {
		return nil, err
	}
	coinType, err := purpose.NewChildKey(luxbip32.FirstHardenedChild + p.coinType)
	if err != nil {
		return nil, err
	}
	account, err := coinType.NewChildKey(luxbip32.FirstHardenedChild + p.account)
	if err != nil {
		return nil, err
	}
	// Unhardened steps: change/index
	change, err := account.NewChildKey(p.change)
	if err != nil {
		return nil, err
	}
	leaf, err := change.NewChildKey(p.index)
	if err != nil {
		return nil, err
	}
	return secp256k1.ToPrivateKey(leaf.Key)
}

// formatPAddr returns the bech32-encoded P-chain address string with the
// "P-" prefix. Returns "" on error so log lines never carry partial data.
func formatPAddr(hrp string, a ids.ShortID) string {
	b32, err := address.FormatBech32(hrp, a[:])
	if err != nil {
		return ""
	}
	return "P-" + b32
}

// platformBlockchain is the subset of platform.getBlockchains we care
// about. ID is the blockchain's own globally unique chain ID; NetworkID
// is the validator-network (CreateNetworkTx ID) that owns it; Name is
// the human-readable chainName that CreateChainTx records.
type platformBlockchain struct {
	ID        ids.ID `json:"id"`
	Name      string `json:"name"`
	NetworkID ids.ID `json:"networkID"`
	VMID      ids.ID `json:"vmID"`
}

// platformGetBlockchains issues a single JSON-RPC POST to
// /v1/bc/P with method="platform.getBlockchains" and parses the
// response. We hand-roll this RPC because info.getBlockchainID(alias)
// would re-introduce the X-Chain coupling we exist to avoid.
func platformGetBlockchains(ctx context.Context, uri string) ([]platformBlockchain, error) {
	body := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"platform.getBlockchains","params":{}}`)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(uri, "/")+"/v1/bc/P", body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	httpc := &http.Client{Timeout: 15 * time.Second}
	resp, err := httpc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("http %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	var doc struct {
		Result struct {
			Blockchains []platformBlockchain `json:"blockchains"`
		} `json:"result"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return nil, err
	}
	if doc.Error != nil {
		return nil, errors.New(doc.Error.Message)
	}
	return doc.Result.Blockchains, nil
}

// writeAliasesPatch emits a kubectl JSON patch that, when applied with
// `kubectl patch cm chain-aliases --type merge --patch-file`, updates
// data.aliases.json to map the new blockchainIDs to their brand alias.
//
// We use a `merge` patch (not strategic) because ConfigMap data is a
// freeform map[string]string — strategic merge is undefined for it.
// The patch overwrites the whole aliases.json key; if the operator
// wants to PRESERVE pre-existing aliases (e.g. legacy chain IDs from
// a prior bootstrap), they must hand-merge before applying. This is
// the safe default for the post-reset case where the CM should
// contain ONLY the 5 fresh L2s.
func writeAliasesPatch(outPath, env string, chains map[string]chainRow) error {
	aliases := map[string][]string{}
	// Stable brand order matches brandSpecs for deterministic patch bytes.
	for _, bs := range brandSpecs {
		row, ok := chains[bs.brand]
		if !ok || row.ChainID == "" {
			continue
		}
		aliases[row.ChainID] = []string{bs.brand}
	}
	aliasesJSON, err := json.MarshalIndent(aliases, "    ", "  ")
	if err != nil {
		return err
	}
	patch := map[string]any{
		"data": map[string]any{
			"aliases.json": string(aliasesJSON) + "\n",
		},
	}
	_ = env // env unused in the patch body; kubectl --namespace carries it
	buf := &bytes.Buffer{}
	enc := json.NewEncoder(buf)
	enc.SetIndent("", "  ")
	if err := enc.Encode(patch); err != nil {
		return err
	}
	return os.WriteFile(outPath, buf.Bytes(), 0o644)
}
