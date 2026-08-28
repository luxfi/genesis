// Copyright (C) 2024-2026, Lux Industries Inc. All rights reserved.
// See the file LICENSE for licensing terms.

// LP-018 §Canonical EVM chainID map — per (network family, environment)
// tuple. This is the source of truth that operator CRs and per-network
// genesis blobs must agree with; a misconfigured `NetworkSpec.EVMChainID`
// that does not appear in this map is an authoring error.
//
// One source of truth, one way to look up: pass a (family, env) pair,
// get back the EVMChainID (or 0 for "unknown / TBD"). The map covers
// every brand-X-environment cell at LP-018 publication time; new cells
// land here when the corresponding genesis configs land in the
// embedded tree.

package configs

import "strings"

// NetworkFamily names the brand of an L1/L2 in the canonical chainID
// map. Lowercase, no spaces — the operator stores this as the kind's
// `spec.family` label when set explicitly; absence implies "lux"
// (the primary network family).
type NetworkFamily string

const (
	// FamilyLux is the Lux primary network family (C-Chain).
	FamilyLux NetworkFamily = "lux"
	// FamilyHanzo is the Hanzo L2 family hosted on the Lux primary
	// network's validator set (validators==0 per LP-018).
	FamilyHanzo NetworkFamily = "hanzo"
	// FamilyZoo is the Zoo L2 family hosted on the Lux primary network's
	// validator set (validators==0 per LP-018).
	FamilyZoo NetworkFamily = "zoo"
	// FamilySPC is the SPC L2 family hosted on the Lux primary network's
	// validator set (validators==0 per LP-018).
	FamilySPC NetworkFamily = "spc"
	// FamilyPars is the Pars network family. Runs as Anchored on a Lux
	// primary or as Independent with its own networkID.
	FamilyPars NetworkFamily = "pars"
	// FamilyOsage is the Osage network family (placeholder pending
	// genesis publication).
	FamilyOsage NetworkFamily = "osage"
)

// EVMChainID returns the canonical EVM chainID for a (family, env)
// tuple per LP-018 §Canonical EVM chainID map. The env argument MUST be one of
// MainnetID / TestnetID / DevnetID / LocalID (the four canonical
// primary network IDs). Any other value returns (0, false).
//
// The lookup is total over the closed set published in the LP. An
// unknown family or an env outside the canonical four returns (0,
// false). Adding a new family is one entry in the switch below plus
// a constant declaration above — no other branch changes.
//
// Why this is a function and not a map literal: the data is sparse
// (some cells are TBD), the env axis is closed (we know all four
// values), and we want unknown-cell access to be a typed (0, false)
// — not a silent zero-with-no-error from a map miss.
func EVMChainID(family NetworkFamily, env uint32) (uint64, bool) {
	switch family {
	case FamilyLux:
		switch env {
		case MainnetID:
			return MainnetChainID, true
		case TestnetID:
			return TestnetChainID, true
		case DevnetID:
			return DevnetChainID, true
		case LocalID:
			return LocalChainID, true
		}
	case FamilyHanzo:
		switch env {
		case MainnetID:
			return 36963, true
		case TestnetID:
			return 36962, true
		case DevnetID:
			return 36964, true
		}
	case FamilyZoo:
		switch env {
		case MainnetID:
			return 200200, true
		case TestnetID:
			return 200201, true
		case DevnetID:
			return 200202, true
		}
	case FamilySPC:
		switch env {
		case MainnetID:
			return 36911, true
		case TestnetID:
			return 36910, true
		case DevnetID:
			return 36912, true
		}
	case FamilyPars:
		switch env {
		case MainnetID:
			// 494949 = live api.pars.network eth_chainId 0x78d65 / net_version.
			// (Was 7070 — a stale value the live chain never adopted.)
			return 494949, true
		case TestnetID:
			return 494950, true
		case DevnetID:
			return 494951, true
		}
		// Osage cells are TBD as of LP-018 publication; lookup returns
		// (0, false) until the genesis configs land here.
	}
	return 0, false
}

// NetworkFamilyOf returns the canonical NetworkFamily for a chainID,
// per LP-018 §Canonical EVM chainID map. The first uint64 → family mapping that
// matches wins. The boolean is false when no family claims the
// chainID (caller must surface this as an authoring error).
//
// Useful for two checks: (a) reject CRs whose `evmChainID` is
// outside the canonical map (likely a typo); (b) on genesis-blob
// reads, surface a warning when the embedded chainID doesn't match
// the CR's declared family.
func NetworkFamilyOf(chainID uint64) (NetworkFamily, bool) {
	switch chainID {
	case MainnetChainID, TestnetChainID, DevnetChainID, LocalChainID:
		return FamilyLux, true
	case 36963, 36962, 36964:
		return FamilyHanzo, true
	case 200200, 200201, 200202:
		return FamilyZoo, true
	case 36911, 36910, 36912:
		return FamilySPC, true
	case 494949, 494950, 494951:
		return FamilyPars, true
	}
	return "", false
}

// ParseNetworkFamily accepts a free-form string and returns the
// canonical NetworkFamily. Case-insensitive. Returns ("", false) for
// any unknown input — callers must surface this rather than silently
// defaulting to FamilyLux.
func ParseNetworkFamily(s string) (NetworkFamily, bool) {
	switch NetworkFamily(strings.ToLower(s)) {
	case FamilyLux:
		return FamilyLux, true
	case FamilyHanzo:
		return FamilyHanzo, true
	case FamilyZoo:
		return FamilyZoo, true
	case FamilySPC:
		return FamilySPC, true
	case FamilyPars:
		return FamilyPars, true
	case FamilyOsage:
		return FamilyOsage, true
	}
	return "", false
}

// EVMChainIDFor is the string-input convenience over EVMChainID for
// callers that hold brand+env as text (CRD validators, operator-side
// helpers, k8s admission webhooks). Case-insensitive on brand.
//
// env must be one of "mainnet", "testnet", "devnet", "localnet" (case-
// insensitive). Any other env or unknown brand returns (0, false). One
// way to look up by string; delegates to the typed EVMChainID — there
// is no parallel data table.
func EVMChainIDFor(brand, env string) (uint64, bool) {
	fam, ok := ParseNetworkFamily(brand)
	if !ok {
		return 0, false
	}
	var envID uint32
	switch strings.ToLower(env) {
	case "mainnet", "main":
		envID = MainnetID
	case "testnet", "test":
		envID = TestnetID
	case "devnet", "dev":
		envID = DevnetID
	case "localnet", "local":
		envID = LocalID
	default:
		return 0, false
	}
	return EVMChainID(fam, envID)
}
