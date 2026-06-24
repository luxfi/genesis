// Copyright (C) 2019-2025, Lux Partners Limited. All rights reserved.
// See the file LICENSE for licensing terms.

// Command genalloc emits the canonical BIP44 wallet allocations array for a
// single network, in the exact AllocationJSON shape the primary-network
// genesis files use (initialAmount + a single locktime-0 unlockSchedule
// entry, utxoAddr in the network's bech32 HRP, evmAddr in 0x H160 form).
//
// It is deliberately scoped to ONLY the allocations array: it does not
// touch cChainGenesis or any other chain blob. Splice its output into an
// existing genesis.json to repair/replace the funded user-account set
// without disturbing the immutable C-Chain treasury or validator config.
//
// The derivation is the canonical BIP44 path m/44'/9000'/0'/0/i
// (LoadBIP44WalletKeysFromMnemonic) — the same keys every Lux web wallet,
// `lux key derive`, and the testnet/devnet genesis already fund. The
// network HRP comes from constants.GetHRP(networkID); the 20-byte UTXO and
// EVM payloads are HRP-independent, so the same mnemonic yields identical
// evmAddrs across all networks (only the utxoAddr HRP differs).
//
// Usage:
//
//	LUX_MNEMONIC="$(cat /path/to/mnemonic)" \
//	  genalloc -network-id 1 -keys 1000 -amount 50000000 > alloc.json
//
//	-network-id  Network ID (1=mainnet/lux, 2=testnet/test, 3=devnet/dev, 1337=local)
//	-keys        Number of canonical BIP44 wallet keys to fund
//	-amount      Allocation per key in WHOLE LUX (scaled by 1e6 microLUX internally)
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/luxfi/constants"
	"github.com/luxfi/genesis/pkg/genesis"
)

func main() {
	networkID := flag.Uint("network-id", 0, "Network ID (1=mainnet, 2=testnet, 3=devnet, 1337=local)")
	numKeys := flag.Int("keys", 1000, "Number of canonical BIP44 wallet keys to fund")
	amountLUX := flag.Uint64("amount", 50_000_000, "Allocation per key in WHOLE LUX")
	flag.Parse()

	if *networkID == 0 {
		fmt.Fprintln(os.Stderr, "Error: -network-id is required (1=mainnet, 2=testnet, 3=devnet, 1337=local)")
		os.Exit(2)
	}
	if *numKeys <= 0 {
		fmt.Fprintln(os.Stderr, "Error: -keys must be > 0")
		os.Exit(2)
	}

	mnemonic := os.Getenv(genesis.MnemonicEnvVar)
	if mnemonic == "" {
		fmt.Fprintf(os.Stderr, "Error: %s env var must be set\n", genesis.MnemonicEnvVar)
		os.Exit(2)
	}

	// Derive the canonical BIP44 wallet keys (m/44'/9000'/0'/0/i). This is
	// the same call BuildBIP44WalletAllocations uses internally, surfaced as
	// KeyInfo so we can feed the canonical ChainAllocations.PChain() path.
	keys, err := genesis.LoadBIP44WalletKeysFromMnemonic(mnemonic, *numKeys)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error deriving BIP44 wallet keys: %v\n", err)
		os.Exit(1)
	}

	// The 20-byte UTXO/EVM payloads are HRP-independent; the network HRP
	// only changes how the utxoAddr is bech32-formatted. constants.GetHRP
	// is the single source of truth (mainnet->"lux", testnet->"test", ...).
	hrp := constants.GetHRP(uint32(*networkID))

	// Convert KeyInfo -> ValidatorKeyInfo so we can reuse the canonical
	// PChain() allocation shaper (initialAmount + one locktime-0 unlock),
	// which is exactly what the existing genesis files carry.
	vkeys := make([]genesis.ValidatorKeyInfo, len(keys))
	for i, k := range keys {
		vkeys[i] = genesis.ValidatorKeyInfo{
			EVMAddr: fmt.Sprintf("0x%s", k.EVMAddr.Hex()),
			ShortID: k.StakingAddr,
		}
	}

	// 50_000_000 whole LUX * 1e6 microLUX = 5e13 base units per key — the
	// canonical DefaultAllocationPerAccount magnitude under the 6-decimal unit.
	amount := *amountLUX * genesis.Lux
	allocs, err := genesis.NewAllocations(vkeys, hrp).WithAmount(amount).PChain()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error building P-chain allocations: %v\n", err)
		os.Exit(1)
	}

	out, err := json.MarshalIndent(allocs, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error marshaling allocations: %v\n", err)
		os.Exit(1)
	}
	out = append(out, '\n')
	if _, err := os.Stdout.Write(out); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing output: %v\n", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "Emitted %d BIP44 allocations for networkID %d (hrp %q), %d LUX each\n",
		len(allocs), *networkID, hrp, *amountLUX)
}
