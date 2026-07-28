// Copyright (C) 2019-2026, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package configs

import (
	"encoding/json"
	"testing"
)

// The 50/50 fee split decides HOW MUCH of a transaction fee is kept; the
// RewardManager precompile decides WHERE the kept half goes. Enabling the first
// without the second burns half of every fee and strands the other half at the
// keyless blackhole 0x0100..00 — the C-Chain already holds ~3868 LUX there.
//
// This test enforces the pairing here, from the two files alone, because no
// shipped luxd enforces it and none reads the field yet:
//
//   - extras.ChainConfig.verifyFeeSplit (luxfi/evm main) refuses an unpaired
//     config, but exists in no evm tag, so no binary carries the guard.
//   - plugin/evm parseGenesis populates extras by naming each genesis config key
//     it supports (evmTimestamp, durangoTimestamp, quasarTimestamp,
//     fortunaTimestamp, graniteTimestamp, precompile keys, feeConfig,
//     allowFeeRecipients). feeSplitTimestamp is in that list on no tag and not on
//     main either, and luxfi/evm holds extras in a side map rather than in the
//     ChainConfig JSON, so the key cannot arrive by plain unmarshal. Until
//     parseGenesis names it, feeSplitTimestamp in a cchain.json is inert: it
//     neither activates the split nor fails a boot.
//
// So the timestamps below are a SCHEDULE, not an activation. Activating the
// split takes an evm change plus a node roll; this test only guarantees that
// when a build does read the field, the reward half has a governed destination.
//
// The two files split the concern deliberately and must stay in step:
//
//	cchain.json  -> ChainConfig (genesis-only; feeSplitTimestamp lives here
//	                because the split is not a NetworkUpgrade and so cannot be
//	                staged through upgrade.json)
//	upgrade.json -> UpgradeConfig (upgradeBytes; precompile activations)
type cChainFile struct {
	Config struct {
		ChainID            uint64  `json:"chainId"`
		FeeSplitTimestamp  *uint64 `json:"feeSplitTimestamp"`
		AllowFeeRecipients bool    `json:"allowFeeRecipients"`
	} `json:"config"`
}

type upgradeFile struct {
	PrecompileUpgrades []map[string]struct {
		BlockTimestamp *uint64 `json:"blockTimestamp"`
		Disable        bool    `json:"disable"`
		InitialReward  struct {
			RewardAddress string `json:"rewardAddress"`
		} `json:"initialRewardConfig"`
	} `json:"precompileUpgrades"`
}

// embeddedNetworks are the directories compiled into the binary by the
// //go:embed directive in configs.go — i.e. every network a node can boot
// without a --genesis-file.
var embeddedNetworks = []string{"mainnet", "testnet", "devnet", "localnet"}

func readEmbedded(t *testing.T, network, name string, out any) {
	t.Helper()
	b, err := embeddedGenesis.ReadFile(network + "/" + name)
	if err != nil {
		t.Fatalf("%s/%s: %v", network, name, err)
	}
	if err := json.Unmarshal(b, out); err != nil {
		t.Fatalf("%s/%s: %v", network, name, err)
	}
}

func TestFeeSplitHasGovernedDestination(t *testing.T) {
	for _, network := range embeddedNetworks {
		t.Run(network, func(t *testing.T) {
			var cchain cChainFile
			readEmbedded(t, network, "cchain.json", &cchain)
			split := cchain.Config.FeeSplitTimestamp
			if split == nil {
				t.Logf("chain %d: fee split dormant", cchain.Config.ChainID)
				return
			}
			if cchain.Config.AllowFeeRecipients {
				t.Logf("chain %d: split @%d, destination = block builder", cchain.Config.ChainID, *split)
				return
			}

			var upgrade upgradeFile
			readEmbedded(t, network, "upgrade.json", &upgrade)
			for _, entry := range upgrade.PrecompileUpgrades {
				cfg, ok := entry["rewardManagerConfig"]
				switch {
				case !ok, cfg.Disable, cfg.BlockTimestamp == nil, cfg.InitialReward.RewardAddress == "":
					continue
				case *cfg.BlockTimestamp > *split:
					t.Fatalf("chain %d: rewardManagerConfig activates at %d, AFTER feeSplitTimestamp %d: "+
						"the reward half would be stranded at the keyless blackhole until then",
						cchain.Config.ChainID, *cfg.BlockTimestamp, *split)
				}
				t.Logf("chain %d: split @%d, reward half -> %s from @%d",
					cchain.Config.ChainID, *split, cfg.InitialReward.RewardAddress, *cfg.BlockTimestamp)
				return
			}
			t.Fatalf("chain %d: cchain.json sets feeSplitTimestamp %d but upgrade.json enables no "+
				"rewardManagerConfig with a rewardAddress at or before it — half of every fee would "+
				"be burned and the other half credited to the keyless blackhole 0x0100..00",
				cchain.Config.ChainID, *split)
		})
	}
}

// TestPrecompileUpgradesMonotonic pins the ordering rule luxd enforces at boot
// (extras.verifyPrecompileUpgrades): activations must be non-decreasing in
// blockTimestamp. Appending a newly scheduled precompile — rewardManagerConfig,
// say — anywhere but the end silently makes the whole schedule unloadable.
func TestPrecompileUpgradesMonotonic(t *testing.T) {
	for _, network := range embeddedNetworks {
		t.Run(network, func(t *testing.T) {
			var upgrade upgradeFile
			readEmbedded(t, network, "upgrade.json", &upgrade)
			var prev uint64
			for i, entry := range upgrade.PrecompileUpgrades {
				for key, cfg := range entry {
					if cfg.BlockTimestamp == nil {
						t.Fatalf("precompileUpgrades[%d][%q]: no blockTimestamp", i, key)
					}
					if *cfg.BlockTimestamp < prev {
						t.Fatalf("precompileUpgrades[%d][%q]: blockTimestamp %d < previous %d",
							i, key, *cfg.BlockTimestamp, prev)
					}
					prev = *cfg.BlockTimestamp
				}
			}
		})
	}
}
