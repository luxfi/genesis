// Copyright (C) 2019-2026, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package configs

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestGetGenesis_CChainShardPresentEmbedsCChainGenesis confirms that for
// every embedded network that ships a cchain.json shard, the marshalled
// primary genesis JSON contains a non-empty cChainGenesis field — i.e. the
// builder will emit a C-Chain CreateChainTx for that network.
//
// This is the new contract that replaced the LUX_DISABLE_CCHAIN env knob:
// chain presence is data-driven (does the shard exist?) rather than
// runtime-toggled (is an env set?). Adding/removing a chain from a
// network's primary genesis is a file-tree edit, not an operator switch.
func TestGetGenesis_CChainShardPresentEmbedsCChainGenesis(t *testing.T) {
	for _, name := range []string{"mainnet", "testnet", "localnet"} {
		t.Run(name, func(t *testing.T) {
			data, err := GetGenesis(networkIDFromName(t, name))
			if err != nil {
				t.Fatalf("GetGenesis(%s): %v", name, err)
			}
			var m map[string]any
			if err := json.Unmarshal(data, &m); err != nil {
				t.Fatalf("parse %s: %v", name, err)
			}
			got, _ := m["cChainGenesis"].(string)
			if got == "" {
				t.Fatalf("%s: expected non-empty cChainGenesis (embedded shard present)", name)
			}
		})
	}
}

// TestGetGenesis_XChainShardPresentEmbedsXChainGenesis is the X-Chain
// equivalent of the C-Chain test above. Every Lux network ships an
// xchain.json shard (the small asset descriptor
// `{"symbol":"LUX","name":"Lux","denomination":6}`); the loader must
// thread it into the marshalled primary genesis JSON as the
// xChainGenesis field, where the builder later parses it to construct
// the XVM genesis and CreateChainTx.
//
// Together with TestBuildGenesisFromDir_AbsentShardEmptyOptIn (which
// covers the absent-shard case for X), this enforces the same data-
// driven contract for X that we already have for C/Q/Z: chain
// presence is a file-tree edit, not an operator switch. The previous
// hardcoded `Symbol: "LUX", Name: "Lux", Denomination: 6` literal in
// builder.FromConfig is now sourced from the shard, so a P-only
// network (P-only shape) can opt out by simply omitting the file.
func TestGetGenesis_XChainShardPresentEmbedsXChainGenesis(t *testing.T) {
	for _, name := range []string{"mainnet", "testnet", "localnet"} {
		t.Run(name, func(t *testing.T) {
			data, err := GetGenesis(networkIDFromName(t, name))
			if err != nil {
				t.Fatalf("GetGenesis(%s): %v", name, err)
			}
			var m map[string]any
			if err := json.Unmarshal(data, &m); err != nil {
				t.Fatalf("parse %s: %v", name, err)
			}
			got, _ := m["xChainGenesis"].(string)
			if got == "" {
				t.Fatalf("%s: expected non-empty xChainGenesis (embedded shard present)", name)
			}
			// Sanity-check the shard parses and pins the canonical Lux asset.
			var asset struct {
				Symbol       string `json:"symbol"`
				Name         string `json:"name"`
				Denomination byte   `json:"denomination"`
			}
			if err := json.Unmarshal([]byte(got), &asset); err != nil {
				t.Fatalf("%s: xChainGenesis shard unparseable: %v", name, err)
			}
			if asset.Symbol != "LUX" || asset.Name != "Lux" || asset.Denomination != 6 {
				t.Fatalf("%s: xChainGenesis shard drift: got %+v want {LUX Lux 6}", name, asset)
			}
		})
	}
}

// TestBuildGenesisFromDir_AbsentShardEmptyOptIn confirms the FS-fallback
// loader honours the same "shard absent → opt-out" semantics as the
// embedded loader. An operator who wants a P+X-only network just omits
// the relevant chain shards from their dir; no env knob, no flag.
func TestBuildGenesisFromDir_AbsentShardEmptyOptIn(t *testing.T) {
	dir := t.TempDir()

	write := func(name, body string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	// Minimal shards: network + P-Chain only. No X/C/Q/Z. This is the
	// P-only shape — every primary-network chain (X included)
	// is opt-in by shard presence.
	write("network.json", `{"networkID":1337,"startTime":1735689600,"message":"P-only test"}`)
	write("pchain.json", `{
		"allocations":[],
		"initialStakeDuration":31536000,
		"initialStakeDurationOffset":5400,
		"initialStakedFunds":[],
		"initialStakers":[]
	}`)

	data, err := buildGenesisFromDir(dir)
	if err != nil {
		t.Fatalf("buildGenesisFromDir: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("parse: %v", err)
	}
	for _, field := range []string{"xChainGenesis", "cChainGenesis", "qChainGenesis", "zChainGenesis"} {
		if got, _ := m[field].(string); got != "" {
			t.Fatalf("absent shard should yield empty %s; got %q", field, got)
		}
	}
}

// networkIDFromName resolves a network directory name to its canonical
// network ID. Test helper that mirrors networkNameFromID's inverse.
func networkIDFromName(t *testing.T, name string) uint32 {
	t.Helper()
	switch name {
	case "mainnet":
		return MainnetID
	case "testnet":
		return TestnetID
	case "devnet":
		return DevnetID
	case "localnet":
		return LocalID
	default:
		t.Fatalf("unknown network name %q", name)
		return 0
	}
}

// TestGetGenesis_AllPrimaryChainsBakedIn locks the contract that every
// primary network (mainnet, testnet, devnet, localnet) ships ALL 11
// primary-network chain shards baked into its genesis. This means the
// chains spawn at network startup with NO post-launch CreateBlockchainTx —
// chain set is fully data-driven by which shards are committed.
//
// The 11 primary-network chains, in canonical order (LP-134: the legacy
// T-Chain/ThresholdVM is retired and split into F-Chain + M-Chain, both
// served by the same ThresholdVM substrate in FHE / MPC mode respectively):
//
//	X  XVM         exchange / UTXO asset
//	C  EVM         contracts
//	D  DexVM       decentralized exchange (CLOB + AMM + perps)
//	Q  QuantumVM   post-quantum consensus (Quasar / ML-DSA / ML-KEM)
//	A  AIVM        AI verification / attestation
//	B  BridgeVM    cross-chain bridge (MPC signing)
//	F  ThresholdVM threshold-FHE confidential compute (LP-134; FHE mode, T's slot)
//	Z  ZKVM        zero-knowledge proofs / private state
//	G  GraphVM     graph database
//	K  KeyVM       KMS — key management
//	M  ThresholdVM threshold-MPC bridge-custody signing (LP-134; MPC mode)
//
// P-Chain is the primary network itself and carries the validator set + chain
// registry; it has no shard slot because it isn't a CreateChainTx entry.
//
// If a future genesis intentionally needs to omit a chain (e.g. a P+X-only
// regulated-securities L1), that's a NEW config-tree, not a regression on the
// canonical Lux primary networks. Editing this test to drop a chain on
// mainnet/testnet/devnet is a load-bearing decision — bring it to design
// review.
func TestGetGenesis_AllPrimaryChainsBakedIn(t *testing.T) {
	required := []string{
		"xChainGenesis",
		"cChainGenesis",
		"dChainGenesis",
		"qChainGenesis",
		"aChainGenesis",
		"bChainGenesis",
		"fChainGenesis",
		"zChainGenesis",
		"gChainGenesis",
		"kChainGenesis",
		"mChainGenesis",
	}
	for _, name := range []string{"mainnet", "testnet", "devnet", "localnet"} {
		t.Run(name, func(t *testing.T) {
			data, err := GetGenesis(networkIDFromName(t, name))
			if err != nil {
				t.Fatalf("GetGenesis(%s): %v", name, err)
			}
			var m map[string]any
			if err := json.Unmarshal(data, &m); err != nil {
				t.Fatalf("parse %s: %v", name, err)
			}
			for _, field := range required {
				got, _ := m[field].(string)
				if got == "" {
					t.Fatalf("%s: required chain field %q is empty — primary network must ship every letter-chain shard", name, field)
				}
			}
		})
	}
}

// TestPrimaryChainShards_PerChainCanonicalChainID locks the canonical
// per-letter EVM chainId scheme so any drift between the network configs
// is caught at test time. Pattern:
//
//	letter base = 96369 + 100*(idx_in_alphabet_relative_to_C)
//	per-network offset: testnet = base-1, devnet = base+1 (mirrors C-Chain)
//
// Exception: the C-Chain devnet EVM chainID is the owner-locked canonical
// value 96367 (see DevnetChainID), NOT base+1 (96370). The +1 rule holds
// for the D/Q/A/... shards below; C is the one cell pinned by the
// canonical map rather than derived from the arithmetic.
//
// LP-134: the legacy T-Chain (idx 5, 96869-series) is retired. F-Chain and
// M-Chain take the next free indices 9 and 10 — fresh IDs, T's are not
// recycled. (localnet base = 31337 + 110*idx.)
//
// This produces:
//
//	C(96369/96368/96367), D(96469/96468/96470), Q(96569/96568/96570),
//	A(96669/96668/96670), B(96769/96768/96770), F(97269/97268/97270),
//	Z(96969/96968/96970), G(97069/97068/97070), K(97169/97168/97170),
//	M(97369/97368/97370)
//
// Each ID is unique across {network × letter} so a misrouted tx cannot
// be replayed against the wrong chain.
func TestPrimaryChainShards_PerChainCanonicalChainID(t *testing.T) {
	want := map[string]map[string]int{
		"mainnet": {
			"d": 96469, "q": 96569, "a": 96669, "b": 96769,
			"f": 97269, "z": 96969, "g": 97069, "k": 97169, "m": 97369,
		},
		"testnet": {
			"d": 96468, "q": 96568, "a": 96668, "b": 96768,
			"f": 97268, "z": 96968, "g": 97068, "k": 97168, "m": 97368,
		},
		"devnet": {
			"d": 96470, "q": 96570, "a": 96670, "b": 96770,
			"f": 97270, "z": 96970, "g": 97070, "k": 97170, "m": 97370,
		},
		"localnet": {
			"d": 31447, "q": 31557, "a": 31667, "b": 31777,
			"f": 32327, "z": 31997, "g": 32107, "k": 32217, "m": 32437,
		},
	}
	for net, byLetter := range want {
		net := net
		t.Run(net, func(t *testing.T) {
			for letter, wantID := range byLetter {
				path := filepath.Join("..", "configs", net, letter+"chain.json")
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatalf("%s/%schain.json: %v", net, letter, err)
				}
				var shard map[string]any
				if err := json.Unmarshal(data, &shard); err != nil {
					t.Fatalf("%s/%schain.json parse: %v", net, letter, err)
				}
				// chainId is stored as a JSON number → unmarshals to float64
				gotF, ok := shard["chainId"].(float64)
				if !ok {
					t.Fatalf("%s/%schain.json: chainId missing or wrong type, got %T", net, letter, shard["chainId"])
				}
				if int(gotF) != wantID {
					t.Fatalf("%s/%schain.json: chainId got %d want %d", net, letter, int(gotF), wantID)
				}
			}
		})
	}
}
