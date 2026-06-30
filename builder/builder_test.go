// Copyright (C) 2019-2025, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package builder

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/luxfi/constants"
	"github.com/luxfi/genesis/configs"
	"github.com/luxfi/ids"
	pgenesis "github.com/luxfi/proto/p/genesis"
)

func TestGetStakingConfig(t *testing.T) {
	tests := []struct {
		name      string
		networkID uint32
	}{
		{"Mainnet", constants.MainnetID},
		{"Testnet", constants.TestnetID},
		{"CustomID", constants.CustomID},
		{"Custom", 12345},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := GetStakingConfig(tt.networkID)

			// Verify basic constraints
			require.GreaterOrEqual(t, cfg.UptimeRequirement, 0.0)
			require.LessOrEqual(t, cfg.UptimeRequirement, 1.0)
			require.Greater(t, cfg.MinValidatorStake, uint64(0))
			require.GreaterOrEqual(t, cfg.MaxValidatorStake, cfg.MinValidatorStake)
			require.Greater(t, cfg.MinDelegatorStake, uint64(0))
			require.Greater(t, cfg.MinStakeDuration, time.Duration(0))
			require.GreaterOrEqual(t, cfg.MaxStakeDuration, cfg.MinStakeDuration)

			// RewardConfig is populated by builder with node-specific types
			// The genesis package only provides the base staking parameters
		})
	}
}

func TestGetTxFeeConfig(t *testing.T) {
	tests := []struct {
		name      string
		networkID uint32
	}{
		{"Mainnet", constants.MainnetID},
		{"Testnet", constants.TestnetID},
		{"CustomID", constants.CustomID},
		{"Custom", 12345},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := GetTxFeeConfig(tt.networkID)

			// Verify basic fee config
			require.Greater(t, cfg.TxFee, uint64(0))
			require.Greater(t, cfg.CreateAssetTxFee, uint64(0))

			// Verify dynamic fee config
			require.Greater(t, uint64(cfg.DynamicFeeConfig.MaxCapacity), uint64(0))
			require.Greater(t, uint64(cfg.DynamicFeeConfig.MaxPerSecond), uint64(0))

			// Verify validator fee config
			require.Greater(t, uint64(cfg.ValidatorFeeConfig.Capacity), uint64(0))
			require.Greater(t, uint64(cfg.ValidatorFeeConfig.Target), uint64(0))
		})
	}
}

func TestGetBootstrappers(t *testing.T) {
	tests := []struct {
		name      string
		networkID uint32
	}{
		{"Mainnet", constants.MainnetID},
		{"Testnet", constants.TestnetID},
		{"CustomID", constants.CustomID},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bootstrappers, err := GetBootstrappers(tt.networkID)
			require.NoError(t, err)

			// Bootstrappers may not be configured for all networks
			// This is acceptable - they can be provided via config

			// Verify each bootstrapper has valid ID and IP
			for _, b := range bootstrappers {
				require.NotEqual(t, b.ID.String(), "")
				require.True(t, b.IP.IsValid())
			}
		})
	}
}

func TestSampleBootstrappers(t *testing.T) {
	tests := []struct {
		name      string
		networkID uint32
		count     int
	}{
		{"Mainnet_5", constants.MainnetID, 5},
		{"Mainnet_10", constants.MainnetID, 10},
		{"Testnet_3", constants.TestnetID, 3},
		{"Custom_0", constants.CustomID, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sampled, err := SampleBootstrappers(tt.networkID, tt.count)
			require.NoError(t, err)

			// Should not exceed requested count
			require.LessOrEqual(t, len(sampled), tt.count)

			// Should not exceed available bootstrappers
			all, err := GetBootstrappers(tt.networkID)
			require.NoError(t, err)
			require.LessOrEqual(t, len(sampled), len(all))
		})
	}
}

func TestGetConfig(t *testing.T) {
	tests := []struct {
		name      string
		networkID uint32
	}{
		{"Mainnet", constants.MainnetID},
		{"Testnet", constants.TestnetID},
		{"MainnetChainID", constants.MainnetChainID},
		{"TestnetChainID", constants.TestnetChainID},
		{"CustomID", constants.CustomID},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := GetConfig(tt.networkID)
			require.NotNil(t, cfg)
			// Config should have a valid NetworkID
			require.Greater(t, cfg.NetworkID, uint32(0))
		})
	}
}

func TestVMAliases(t *testing.T) {
	// Verify all expected VMs have aliases
	require.Contains(t, VMAliases, constants.PlatformVMID)
	require.Contains(t, VMAliases, constants.XVMID)
	require.Contains(t, VMAliases, constants.EVMID)

	// Verify aliases are non-empty
	for vmID, aliases := range VMAliases {
		require.NotEmpty(t, aliases, "VM %s should have aliases", vmID)
	}
}

func TestChainAliases(t *testing.T) {
	require.NotEmpty(t, PChainAliases)
	require.NotEmpty(t, XChainAliases)
	require.NotEmpty(t, CChainAliases)

	require.Contains(t, PChainAliases, "P")
	require.Contains(t, XChainAliases, "X")
	require.Contains(t, CChainAliases, "C")
}

// TestFromConfig_ChainSetIsShardDriven exercises the data-driven
// chain-presence contract end-to-end:
//
//   - Lux mainnet ships every primary-network chain shard
//     (X/C/D/Q/A/B/T/Z/G/K) → FromConfig must emit all 10
//     CreateChainTx entries and a non-empty utxoAssetID (X is the home of
//     the LUX asset).
//
//   - Strip every chain shard from the same Config — same allocations,
//     same validators, same network ID — and FromConfig must succeed,
//     return ids.Empty for utxoAssetID, and produce a P-Chain genesis
//     with zero CreateChainTx entries. This is the primary-from-P-only
//     boot path: a sovereign primary network (lqd-style) ships only
//     pchain.json + network.json and gets a primary-network of just
//     P-Chain, with funded addresses and weighted validators ready to
//     issue CreateChainTx for whatever chains it wants post-genesis.
//
// "Shard set = chain set" is the one-way contract; this test pins it.
func TestFromConfig_ChainSetIsShardDriven(t *testing.T) {
	cfg, err := configs.GetConfig(constants.MainnetID)
	require.NoError(t, err)
	require.NotNil(t, cfg)
	require.NotEmpty(t, cfg.XChainGenesis, "mainnet ships xchain.json")

	// Full Lux mainnet: 10 chains + real LUX asset ID.
	bytes, utxoAssetID, err := FromConfig(cfg)
	require.NoError(t, err)
	require.NotEqual(t, ids.Empty, utxoAssetID, "mainnet has X-Chain → real asset ID")

	pgc, err := pvmGenesisCodec()
	require.NoError(t, err)

	full, err := pgenesis.Parse(pgc, bytes)
	require.NoError(t, err)
	require.Len(t, full.Chains, 11, "mainnet emits all 11 primary chains (LP-134: X,C,D,Q,A,B,F,Z,G,K,M)")

	// Strip every chain shard. Allocations + validators stay intact —
	// the P-Chain itself is the foundation, not a CreateChainTx entry.
	cfg.XChainGenesis = ""
	cfg.CChainGenesis = ""
	cfg.DChainGenesis = ""
	cfg.QChainGenesis = ""
	cfg.AChainGenesis = ""
	cfg.BChainGenesis = ""
	cfg.FChainGenesis = ""
	cfg.ZChainGenesis = ""
	cfg.GChainGenesis = ""
	cfg.KChainGenesis = ""
	cfg.MChainGenesis = ""

	bytes, utxoAssetID, err = FromConfig(cfg)
	require.NoError(t, err, "P-only build must succeed")
	require.Equal(t, ids.Empty, utxoAssetID, "no X-Chain → no LUX asset → ids.Empty")

	pOnly, err := pgenesis.Parse(pgc, bytes)
	require.NoError(t, err, "P-only genesis bytes must parse")
	require.Empty(t, pOnly.Chains, "P-only network has zero CreateChainTx entries")

	// "If wallet/address funded for keys" — the P-Chain still carries
	// the Lux mainnet allocations (encoded as P-Chain UTXOs) and
	// validator stakes, so a sovereign L1 booted from this shape has
	// funded addresses and weighted validators ready to operate.
	require.NotEmpty(t, pOnly.Validators, "P-only must have validators (funded staker keys)")
	require.NotEmpty(t, pOnly.UTXOs, "P-only must have funded addresses (UTXOs)")
}

func TestDefaultFeeConfigs(t *testing.T) {
	// Test dynamic fee configs
	configs := []struct {
		name   string
		config interface{}
	}{
		{"MainnetDynamic", MainnetDynamicFeeConfig},
		{"TestnetDynamic", TestnetDynamicFeeConfig},
		{"LocalDynamic", LocalDynamicFeeConfig},
		{"MainnetValidator", MainnetValidatorFeeConfig},
		{"TestnetValidator", TestnetValidatorFeeConfig},
		{"LocalValidator", LocalValidatorFeeConfig},
	}

	for _, tt := range configs {
		t.Run(tt.name, func(t *testing.T) {
			require.NotNil(t, tt.config)
		})
	}
}
