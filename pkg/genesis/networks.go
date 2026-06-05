// Copyright (C) 2019-2026, Lux Industries Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package genesis

import "github.com/luxfi/constants"

// Canonical network ID map — the only legal values for the primary
// network ID. Anything else is "custom" and a bug.
//
//	  primary network ID            EVM (C-Chain) chain ID
//	  ──────────────────            ──────────────────────
//	  constants.MainnetID  (1)      constants.MainnetChainID (96369)
//	  constants.TestnetID  (2)      constants.TestnetChainID (96368)
//	  constants.DevnetID   (3)      constants.DevnetChainID  (96370)
//	  constants.LocalID    (1337)   constants.LocalChainID   (31337)
//
// The primary network ID is SHARED across all Lux-derived L1s (Lux,
// Hanzo, Zoo, Pars, …). EVM chain IDs are per-chain, per-env.
//
// Mnemonic source (LUX_MNEMONIC):
//   - mainnet/testnet/devnet (1/2/3): private hardware-RNG mnemonic from
//     KMS; RefuseLightMnemonicOnProduction enforces this.
//   - local (1337): LightMnemonic is the well-known dev seed value.
//
// All four envs read the mnemonic from the same env var (LUX_MNEMONIC);
// per-env isolation comes from a different mnemonic per env, not from
// the env-var name.
var CanonicalPrimaryNetworkIDs = []uint32{
	constants.MainnetID, // 1
	constants.TestnetID, // 2
	constants.DevnetID,  // 3
	constants.LocalID,   // 1337
}

// IsCanonicalPrimaryNetwork reports whether networkID is one of the four
// canonical primary network IDs. Custom IDs (e.g. UnitTestID) are not
// canonical and callers should treat them explicitly.
func IsCanonicalPrimaryNetwork(networkID uint32) bool {
	switch networkID {
	case constants.MainnetID, constants.TestnetID, constants.DevnetID, constants.LocalID:
		return true
	}
	return false
}
