// Copyright (C) 2019-2026, Lux Industries Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package genesis

// cert_policy.go — pure-data carrier for the per-chain CertPolicy pin
// described by LP-217 §"Operator config". The Validate gate that
// actually consults luxfi/consensus lives in
// ~/work/lux/node/genesis/cert_policy.go to keep pkg/genesis
// consensus-dep-free (same direction the SecurityProfile pin uses —
// consensus consumes genesis, never the other way).
//
// YAML wire form mirrors LP-217 §"YAML form" exactly:
//
//	quasar:
//	  cert_policy:
//	    mode:        PQ-strict   # PQ-off | PQ-fast | PQ-strict | PQ-heavy
//	    variant:     hybrid      # hybrid | strict
//	    timeout_ms:  1000
//	    fallback:    PQ-fast
//
// One JSON object per pin. JSON keys camel-case to match the rest of
// genesis.Config. The four-field shape is fixed by LP-217 and matches
// consensus/config.CertPolicy 1:1; node/genesis converts string→enum
// via consensusconfig.ParseCertPolicy at boot, which runs the four
// LP-217 validation rules.

// CertPolicy is the genesis-level pin for a chain's cert posture.
// Pure data — no methods that touch consensus. Verification lives in
// node/genesis (via consensus/config.ParseCertPolicy + Validate).
//
// One CertPolicy per L1 (carried on Config). One optional CertPolicy
// override per chain-VM (carried on ChainEntry). LP-204 inheritance
// applies: a ChainEntry that omits a cert_policy block inherits the
// parent L1's policy verbatim. A ChainEntry override MUST NOT name a
// Mode stronger than the parent L1's Mode — chain-VMs cannot
// synthesise legs the parent L1 never produces (LP-204 §"L1 chain-VM
// mode selection").
type CertPolicy struct {
	// Mode is one of: "PQ-off" | "PQ-fast" | "PQ-strict" | "PQ-heavy".
	// Maps to consensus/config.CertMode via ParseCertPolicy.
	Mode string `json:"mode" yaml:"mode"`

	// Variant is one of: "hybrid" | "strict" (or "" → "hybrid").
	// Maps to consensus/config.CertVariant via ParseCertPolicy.
	Variant string `json:"variant" yaml:"variant"`

	// TimeoutMs is the max wait in milliseconds for the full-mode
	// cert before LP-202 tier degradation fires. MUST be ≥ 2 ×
	// expected_floor_latency(Mode) per LP-217 rule 3.
	TimeoutMs uint32 `json:"timeoutMs" yaml:"timeout_ms"`

	// Fallback is one of the four mode names; the tier the chain
	// settles at when Mode's legs do not arrive within TimeoutMs.
	// MUST satisfy Fallback ≤ Mode (LP-217 rule 1) and MUST itself
	// be a valid (Mode, Variant) under the chain's Variant (rule 4).
	Fallback string `json:"fallback" yaml:"fallback"`
}
