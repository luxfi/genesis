// Copyright (C) 2019-2026, Lux Industries Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package genesis

// security_profile.go — pure-data carriers for the chain-wide
// ChainSecurityProfile pin. The Resolve / Validate / ComputeHash gate
// that actually consults luxfi/consensus lives in pkg/genesis/security
// to keep this package consensus-dep-free (one and only one direction:
// consensus consumes genesis, never the other way).
//
// Wire form is pin-by-ID + pin-by-hash:
//   - ProfileID names the canonical ChainSecurityProfile (e.g. 0x01 =
//     StrictPQ).
//   - ProfileHashHex is the 48-byte SHA3-384 ComputeHash of the
//     canonical profile, hex-encoded. The verifier in
//     pkg/genesis/security recomputes the hash at boot and refuses to
//     start if the hex does not match. Any drift in the canonical
//     profile content invalidates every prior genesis that pinned its
//     hash.

// SecurityProfile is the genesis-level pin for a chain's locked
// ChainSecurityProfile. Pure data — no methods that touch consensus.
// Verification lives in pkg/genesis/security.
type SecurityProfile struct {
	// ProfileID is the wire byte that names the canonical profile.
	// 0x01 = StrictPQ, 0x02 = Permissive, 0x03 = FIPS.
	// 0x80+ is reserved for downstream / white-label profiles (which
	// must register with the consensus team to obtain a byte).
	ProfileID uint8 `json:"profileID"`

	// ProfileHashHex is the SHA3-384 ComputeHash of the canonical
	// ChainSecurityProfile at the time genesis was sealed, hex-encoded
	// (96 hex chars, no 0x prefix). The verifier in
	// pkg/genesis/security rejects a startup whose live ComputeHash
	// does not match.
	ProfileHashHex string `json:"profileHashHex"`
}
