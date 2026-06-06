// Copyright (C) 2019-2026, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

// Package builder local ZAP-native codec helper.
//
// This file is a STAGING SHIM until Wave 2G-Wallet (in-flight at
// proto/zap_codec) lands and publishes
// `github.com/luxfi/proto/zap_codec`. The contents below mirror that
// package's public surface EXACTLY — same Manager type, same
// constructor signatures, same wire layout (zapcodec little-endian
// reflection codec with a 2-byte LE codec-version prefix).
//
// TODO(wave-2g-wallet): once proto tags the zap_codec subpackage, this
// file is deleted and builder.go's pvmGenesisCodec /
// newXVMParserCodecs swap to `github.com/luxfi/proto/zap_codec`
// imports. The two helper functions in builder.go are the ONLY in-tree
// consumers — no other file in this module depends on this shim.
//
// Wire format identity: the bytes this file's Manager emits are
// byte-identical to bytes proto/zap_codec.Manager would emit at the
// same CodecVersion + same registered type set. The eventual swap is
// therefore purely a code-level move — no genesis blob re-issue, no
// network coordination needed.
//
// Wire-format relationship to legacy linearcodec (the codec this
// package displaces):
//
//   - All multi-byte integers are little-endian. linearcodec is
//     big-endian. x86_64 and arm64 hardware is LE-native, so LE writes
//     map to single MOV instructions where BE writes need BSWAP.
//   - Slice/map length prefixes (uint32) and string length prefixes
//     (uint16) flip to LE.
//   - Interface type-id prefixes (uint32) flip to LE.
//   - Reflection layout (struct-tag order, field encoding) is
//     identical to linearcodec.
//
// This is a WIRE-FORMAT BREAK from the legacy linearcodec bytes that
// previously fed proto/{p,x} consumers. The break is intentional and
// aligned with LP-023 ZAP-native activation
// (proto/zap_native/codec_select.go: ZAPActivationUnix=0 → "ZAP is
// mandatory from genesis"). Forward-only — there is no dual-mode
// emission and no rollback path.
//
// Architecture (Hickey decomplection):
//
//   - VALUE: the wire codec choice. Today: zapcodec LE. (Was:
//     linearcodec BE.) The value lives here, qualified by namespace,
//     not braided into the call sites.
//   - COMPOSITION: zapCodecManager wraps a zapcodec.Codec with a
//     version-prefix outer layer. Outer + inner are independently
//     complete primitives.
//   - ORTHOGONAL: this file neither knows about proto/{p,x} types nor
//     about genesis semantics. Per-type registration happens at the
//     callers (pvmGenesisCodec / newXVMParserCodecs in builder.go),
//     which RegisterTypes onto the Manager.
package builder

import (
	"errors"
	"math"

	"github.com/luxfi/codec/wrappers"
	"github.com/luxfi/codec/zapcodec"
)

// zapCodecMaxSize is the default maximum wire-payload size for
// runtime txs. Mirrors `proto/zap_codec.MaxSize` (1 MiB) so per-tx
// envelope is unchanged vs the legacy linearcodec configuration.
const zapCodecMaxSize = 1024 * 1024

// zapCodecVersionSize is the on-wire length of the codec-version
// prefix the outer manager prepends. Two bytes, uint16 little-endian.
const zapCodecVersionSize = 2

// Sentinel errors. Shape-compatible with `proto/zap_codec` errors so
// callers asserting on the shim's sentinels get equivalent behaviour
// after the swap.
var (
	errZapCodecCantPackVersion   = errors.New("zap_codec: couldn't pack codec version")
	errZapCodecCantUnpackVersion = errors.New("zap_codec: couldn't unpack codec version")
	errZapCodecUnknownVersion    = errors.New("zap_codec: unknown codec version")
	errZapCodecExtraSpace        = errors.New("zap_codec: trailing buffer space")
	errZapCodecMaxSizeExceeded   = errors.New("zap_codec: wire payload exceeds max size")
)

// zapCodecManager is the version-prefix wire codec implementation. It
// owns the codec.Manager-shaped surface (Marshal/Unmarshal/Size) that
// proto/{p,x}'s local Codec interfaces structurally match.
//
// zapCodecManager also satisfies the Registry surface by delegating
// RegisterType / SkipRegistrations to the inner zapcodec.Codec. This
// is the same contract every Wave 2-era consumer expects: hand back
// ONE value that is BOTH the wire codec AND the type registry.
//
// Thread-safety: the inner zapcodec.Codec is concurrency-safe (its
// RWMutex protects the type registry). zapCodecManager itself is
// stateless past construction — callers may share *zapCodecManager
// across goroutines without external sync.
type zapCodecManager struct {
	inner   zapcodec.Codec
	version uint16
	maxSize int
}

// newZapCodecVersionedManager constructs a zapCodecManager that
// emits/accepts wire bytes prefixed by a uint16 little-endian codec
// version. The inner zapcodec instance is allocated fresh;
// SkipRegistrations and RegisterType go directly to it.
func newZapCodecVersionedManager(version uint16, maxSize uint64) *zapCodecManager {
	size := maxSize
	if size > math.MaxInt {
		size = math.MaxInt
	}
	return &zapCodecManager{
		inner:   zapcodec.NewDefault(),
		version: version,
		maxSize: int(size),
	}
}

// RegisterType registers val with the inner reflection codec.
func (m *zapCodecManager) RegisterType(val interface{}) error {
	return m.inner.RegisterType(val)
}

// SkipRegistrations bumps the inner codec's next-type-id by n.
func (m *zapCodecManager) SkipRegistrations(n int) {
	m.inner.SkipRegistrations(n)
}

// Marshal serializes source into a fresh byte slice with the
// manager's codec version (uint16 LE) prepended. Caller-supplied
// version MUST match the manager's bound version.
func (m *zapCodecManager) Marshal(version uint16, source interface{}) ([]byte, error) {
	if version != m.version {
		return nil, errZapCodecUnknownVersion
	}
	p := &wrappers.Packer{MaxSize: m.maxSize}
	if err := zapCodecWriteVersionLE(p, version); err != nil {
		return nil, err
	}
	if err := m.inner.MarshalInto(source, p); err != nil {
		return nil, err
	}
	if p.Err != nil {
		return nil, p.Err
	}
	return p.Bytes[:p.Offset], nil
}

// Unmarshal deserializes bytes into dest. Returns the codec version
// read from the wire prefix.
func (m *zapCodecManager) Unmarshal(bytes []byte, dest interface{}) (uint16, error) {
	if len(bytes) < zapCodecVersionSize {
		return 0, errZapCodecCantUnpackVersion
	}
	if len(bytes) > m.maxSize {
		return 0, errZapCodecMaxSizeExceeded
	}
	p := &wrappers.Packer{Bytes: bytes, MaxSize: m.maxSize}
	version, err := zapCodecReadVersionLE(p)
	if err != nil {
		return 0, err
	}
	if version != m.version {
		return version, errZapCodecUnknownVersion
	}
	if err := m.inner.UnmarshalFrom(p, dest); err != nil {
		return version, err
	}
	if p.Offset != len(bytes) {
		return version, errZapCodecExtraSpace
	}
	return version, nil
}

// Size returns the on-wire size of value INCLUDING the manager's
// version-prefix bytes.
func (m *zapCodecManager) Size(version uint16, value interface{}) (int, error) {
	if version != m.version {
		return 0, errZapCodecUnknownVersion
	}
	size, err := m.inner.Size(value)
	if err != nil {
		return 0, err
	}
	return zapCodecVersionSize + size, nil
}

// newZapCodecPVMGenesis returns a Manager wired for proto/p PVM
// genesis blobs. Budget is math.MaxInt32 (genesis can be very large
// — full validator set + every chain's initial state).
//
// Mirrors `proto/zap_codec.NewPVMGenesis` exactly.
func newZapCodecPVMGenesis(version uint16) *zapCodecManager {
	return newZapCodecVersionedManager(version, math.MaxInt32)
}

// newZapCodecXVMParser returns the two codec/registry values
// proto/x's txs.NewParser requires: runtime Codec, genesis Codec.
// Each returned Manager is independent.
//
// Mirrors `proto/zap_codec.NewXVMParser` exactly.
func newZapCodecXVMParser(version uint16) (runtime, genesis *zapCodecManager) {
	return newZapCodecVersionedManager(version, zapCodecMaxSize),
		newZapCodecVersionedManager(version, math.MaxInt32)
}

// zapCodecWriteVersionLE writes a uint16 codec version in
// little-endian. The wrappers.Packer's built-in PackShort is BE — we
// cannot reuse it because the zapcodec body that follows is LE.
func zapCodecWriteVersionLE(p *wrappers.Packer, version uint16) error {
	p.PackFixedBytes([]byte{byte(version), byte(version >> 8)})
	if p.Err != nil {
		return errZapCodecCantPackVersion
	}
	return nil
}

// zapCodecReadVersionLE reads a uint16 codec version in little-endian.
func zapCodecReadVersionLE(p *wrappers.Packer) (uint16, error) {
	b := p.UnpackFixedBytes(zapCodecVersionSize)
	if p.Err != nil {
		return 0, errZapCodecCantUnpackVersion
	}
	return uint16(b[0]) | uint16(b[1])<<8, nil
}
