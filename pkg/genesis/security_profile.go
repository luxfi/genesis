// Copyright (C) 2019-2026, Lux Industries Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package genesis

import (
	"encoding/hex"
	"errors"
	"fmt"

	consensusconfig "github.com/luxfi/consensus/config"
)

// security_profile.go — genesis-level binding of the chain-wide
// ChainSecurityProfile. Closes red-team finding F102: previously
// neither this package nor luxfi/node imported the locked profile,
// so the profile was pure documentation that no consumer checked.
//
// Wire form is pin-by-ID + pin-by-hash:
//   - ProfileID names the canonical ChainSecurityProfile (e.g. 0x01 =
//     StrictPQ).
//   - ProfileHashHex is the 48-byte SHA3-384 ComputeHash of the
//     canonical profile, hex-encoded. The genesis loader recomputes
//     the hash at boot from consensusconfig.ProfileByID(ProfileID) and
//     refuses to start if the hex does not match. Any drift in the
//     canonical profile content invalidates every prior genesis that
//     pinned its hash.
//
// One-way load: deserialise → resolve → validate → ComputeHash →
// constant-time compare. There is no upgrade path that re-derives the
// hash from the pinned ID alone; that would defeat the lock.

// SecurityProfile is the genesis-level pin for a chain's locked
// ChainSecurityProfile. ProfileID identifies the canonical row in the
// consensusconfig registry; ProfileHashHex pins the expected content
// hash. Both fields are mandatory on a profile-bound genesis.
type SecurityProfile struct {
	// ProfileID is the wire byte that names the canonical profile.
	// 0x01 = StrictPQ, 0x02 = Permissive, 0x03 = FIPS.
	// 0x80+ is reserved for downstream / white-label profiles (which
	// must register with the consensus team to obtain a byte).
	ProfileID uint8 `json:"profileID"`

	// ProfileHashHex is the SHA3-384 ComputeHash of the canonical
	// ChainSecurityProfile at the time genesis was sealed, hex-encoded
	// (96 hex chars, no 0x prefix). The genesis loader rejects a
	// startup whose live ComputeHash does not match.
	ProfileHashHex string `json:"profileHashHex"`
}

// SecurityProfileLoadErr is returned by Resolve when the pin fails to
// resolve to a known canonical profile or its content hash mismatches.
var (
	ErrSecurityProfileMissing         = errors.New("genesis: security profile pin is absent on a profile-required genesis")
	ErrSecurityProfileInvalidID       = errors.New("genesis: SecurityProfile.ProfileID is not a known consensus profile")
	ErrSecurityProfileInvalidHashHex  = errors.New("genesis: SecurityProfile.ProfileHashHex is not 48 bytes of hex")
	ErrSecurityProfileValidateFailed  = errors.New("genesis: canonical profile failed Validate() at boot")
	ErrSecurityProfileHashMismatch    = errors.New("genesis: live ComputeHash() does not match genesis-pinned ProfileHashHex")
	ErrSecurityProfileComputeHashFail = errors.New("genesis: canonical profile ComputeHash() returned error")
)

// Resolve loads the canonical ChainSecurityProfile named by ProfileID,
// validates it, computes its hash, and confirms the hash matches the
// genesis pin. Returns the canonical profile on success.
//
// This is the single load-and-verify entry point. Every consumer of
// the locked profile (node bootstrap, signer registration, peer-cert
// gate, mempool, validator registry, bridge oracle) calls Resolve once
// at startup and threads the returned *ChainSecurityProfile through
// the rest of the process — no re-resolution per request.
//
// The constant-time hash comparison is intentional: a runtime mutation
// of the canonical profile that produces a different hash MUST fail
// the genesis pin, and we don't want to leak which byte of the hash
// matched via an early-exit byte loop. (Hash comparisons are not
// secrets, but consistency with the rest of the security gate is
// worth more than the four-line shortcut.)
func (s *SecurityProfile) Resolve() (*consensusconfig.ChainSecurityProfile, error) {
	if s == nil {
		return nil, ErrSecurityProfileMissing
	}

	pinned, err := hex.DecodeString(s.ProfileHashHex)
	if err != nil || len(pinned) != 48 {
		return nil, fmt.Errorf("%w: have %d bytes (%v)", ErrSecurityProfileInvalidHashHex, len(pinned), err)
	}

	profile, err := consensusconfig.ProfileByID(consensusconfig.ProfileID(s.ProfileID))
	if err != nil {
		return nil, fmt.Errorf("%w: 0x%02x: %v", ErrSecurityProfileInvalidID, s.ProfileID, err)
	}
	if err := profile.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrSecurityProfileValidateFailed, err)
	}
	live, err := profile.ComputeHash()
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrSecurityProfileComputeHashFail, err)
	}
	if !constantTimeEqual48(live[:], pinned) {
		return nil, fmt.Errorf("%w: pinned=%s live=%x",
			ErrSecurityProfileHashMismatch, s.ProfileHashHex, live[:])
	}
	// Stamp the live hash so consumers always see it on the struct.
	profile.ProfileHash = live
	return profile, nil
}

// constantTimeEqual48 returns true iff the two 48-byte hashes are equal,
// in constant time over the byte slice contents. Subtle.ConstantTimeCompare
// is generic over byte slices but pinned to 48 bytes here so a length
// mismatch is caught explicitly above.
func constantTimeEqual48(a, b []byte) bool {
	if len(a) != 48 || len(b) != 48 {
		return false
	}
	var v byte
	for i := 0; i < 48; i++ {
		v |= a[i] ^ b[i]
	}
	return v == 0
}
