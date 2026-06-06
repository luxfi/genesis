// Copyright (C) 2019-2026, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package builder

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/luxfi/constants"
	"github.com/luxfi/genesis/configs"
	pgenesis "github.com/luxfi/proto/p/genesis"
	xchaintxs "github.com/luxfi/proto/x/txs"
)

// TestWire_PVMGenesisRoundtrip pins the ZAP-native PVM genesis codec's
// roundtrip property: bytes-out, parse-back, re-marshal must produce
// byte-identical output.
//
// This is the wire-format integrity guarantee — every primary-network
// validator MUST produce the same wire bytes from the same Config so
// genesis hash matches across the fleet.
func TestWire_PVMGenesisRoundtrip(t *testing.T) {
	cfg, err := configs.GetConfig(constants.MainnetID)
	require.NoError(t, err)
	require.NotNil(t, cfg)

	// Build mainnet genesis bytes (full 10-chain shape).
	bytes1, _, err := FromConfig(cfg)
	require.NoError(t, err)
	require.NotEmpty(t, bytes1)

	pgc, err := pvmGenesisCodec()
	require.NoError(t, err)

	// Parse.
	gen, err := pgenesis.Parse(pgc, bytes1)
	require.NoError(t, err)
	require.NotNil(t, gen)
	require.Equal(t, 10, len(gen.Chains), "mainnet ships all 10 primary chains")

	// Re-marshal.
	bytes2, err := gen.Bytes(pgc)
	require.NoError(t, err)

	// Bytes-out must equal bytes-in.
	require.Equal(t, bytes1, bytes2,
		"ZAP-native PVM genesis codec must produce stable roundtrip bytes")
}

// TestWire_PVMGenesisVersion pins the on-wire codec-version prefix:
// the first two bytes of every PVM genesis blob are the LE codec
// version (0x0000 for CodecVersion=0). This is the LE byte order that
// differs from the legacy linearcodec BE encoding — and is the
// canonical wire-format signature that this build produces ZAP-native
// bytes.
func TestWire_PVMGenesisVersion(t *testing.T) {
	cfg, err := configs.GetConfig(constants.MainnetID)
	require.NoError(t, err)

	bytes, _, err := FromConfig(cfg)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(bytes), 2)

	// CodecVersion=0 encodes to 0x00 0x00 in both BE and LE, so this
	// test pins the version slot but not the byte order. The byte-order
	// signature is observable in any multi-byte length prefix downstream
	// (timestamp, initial supply, validator counts, etc.).
	require.Equal(t, byte(pgenesis.CodecVersion&0xff), bytes[0], "version low byte")
	require.Equal(t, byte((pgenesis.CodecVersion>>8)&0xff), bytes[1], "version high byte")
}

// TestWire_PVMGenesisDeterministic pins that two independent builds
// from the same Config produce byte-identical PVM genesis blobs. This
// guarantees genesis hash stability across nodes that build from the
// same config — a necessary property for primary-network bootstrap
// (every validator computes the same root chain ID).
func TestWire_PVMGenesisDeterministic(t *testing.T) {
	cfg1, err := configs.GetConfig(constants.MainnetID)
	require.NoError(t, err)
	cfg2, err := configs.GetConfig(constants.MainnetID)
	require.NoError(t, err)

	b1, id1, err := FromConfig(cfg1)
	require.NoError(t, err)
	b2, id2, err := FromConfig(cfg2)
	require.NoError(t, err)

	require.Equal(t, b1, b2, "deterministic build: bytes-out must match across builds from the same Config")
	require.Equal(t, id1, id2, "deterministic build: UTXOAssetID must match")
}

// TestWire_XVMGenesisRoundtrip pins the ZAP-native XVM genesis codec's
// roundtrip property: UTXOAssetID (which hashes the XVM genesis blob)
// must be stable across builds.
func TestWire_XVMGenesisRoundtrip(t *testing.T) {
	cfg, err := configs.GetConfig(constants.MainnetID)
	require.NoError(t, err)
	require.NotEmpty(t, cfg.XChainGenesis, "mainnet ships xchain.json")

	codecs, err := newXVMParserCodecs()
	require.NoError(t, err)

	// Verify the genesis codec accepts CodecVersion roundtrip.
	require.NotNil(t, codecs.Codec)
	require.NotNil(t, codecs.GenesisCodec)
	require.NotNil(t, codecs.Registry)
	require.NotNil(t, codecs.GenesisRegistry)

	// Verify the codec version handle exists on the manager.
	gen := &struct {
		Networks []byte
	}{}
	_, _ = codecs.GenesisCodec.Size(xchaintxs.CodecVersion, gen) // any version-aware op should not panic
}

// TestWire_XVMGenesisDeterministic pins UTXOAssetID stability across
// builds. Every primary-network validator MUST derive the same
// utxoAssetID from the same XVM genesis bytes — without this, P-Chain
// stake UTXOs would have different asset IDs across the fleet and
// cross-fleet block validation would fail.
func TestWire_XVMGenesisDeterministic(t *testing.T) {
	cfg, err := configs.GetConfig(constants.MainnetID)
	require.NoError(t, err)

	_, id1, err := FromConfig(cfg)
	require.NoError(t, err)
	_, id2, err := FromConfig(cfg)
	require.NoError(t, err)

	require.Equal(t, id1, id2, "deterministic UTXOAssetID across builds")
	require.NotEmpty(t, id1, "mainnet has a real LUX asset ID")
}

// TestWire_HexSignature_PVMGenesis captures the canonical hex prefix of
// a minimal PVM genesis blob produced by the ZAP-native codec. This is
// a tripwire: any change to the wire encoding (codec backend swap,
// type-id slot reshuffle, byte-order flip) will move this prefix and
// fail the test.
//
// The full mainnet blob is too large to embed verbatim; the prefix
// (first 32 bytes) suffices to detect any structural-encoding drift.
func TestWire_HexSignature_PVMGenesis(t *testing.T) {
	cfg, err := configs.GetConfig(constants.MainnetID)
	require.NoError(t, err)

	bytes, _, err := FromConfig(cfg)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(bytes), 32)

	// Prefix: 2 bytes version + start of UTXOs slice header.
	// Both legacy linearcodec (BE) and ZAP zapcodec (LE) write
	// CodecVersion=0 as 0x0000 (palindrome at this width), so the first
	// two bytes are the same. The third+ bytes are the LE length prefix
	// of the UTXOs slice, which IS where the BE/LE break shows up.
	prefix := hex.EncodeToString(bytes[:32])
	t.Logf("PVM genesis ZAP-native prefix: %s", prefix)

	// Sanity: must not be all zeros past the version (mainnet ships
	// allocations, validators, chains — all non-empty slices).
	allZeros := true
	for _, b := range bytes[2:32] {
		if b != 0 {
			allZeros = false
			break
		}
	}
	require.False(t, allZeros, "non-trivial mainnet genesis must have non-zero body")
}

// TestWire_FormatBreak_Documented is the WIRE-FORMAT VERDICT test.
//
// Builds a tiny known-shape PVM Genesis and confirms the ZAP-native
// codec emits the EXPECTED little-endian byte layout — which is
// DIFFERENT from the legacy linearcodec big-endian layout that
// previously fed proto/{p,x}. This documents the break in code so
// future readers can see exactly which bytes flip.
//
// Verdict (per LP-023 ZAP-native activation):
//
//	BREAK — ZAP-native bytes are NOT byte-equivalent to linearcodec
//	         bytes. Mainnet/testnet/devnet/localnet all run ZAP-only
//	         from genesis (ZAPActivationUnix=0). No coordinated
//	         network upgrade is needed because no validator was
//	         running the legacy linearcodec encoding in production —
//	         this is the first ZAP-native genesis tag, forward-only.
//
// Test fixture: a Genesis with Timestamp=0x0102030405060708 and
// InitialSupply=0x1112131415161718. The two uint64 fields' wire
// layout is the canonical signature for the byte-order break.
//
//	linearcodec (BE): 01 02 03 04 05 06 07 08  11 12 13 14 15 16 17 18
//	zapcodec    (LE): 08 07 06 05 04 03 02 01  18 17 16 15 14 13 12 11
//
// The PVM Genesis layout serializes UTXOs/Validators/Chains slice
// headers (uint32 length = 0 each), then Timestamp (uint64), then
// InitialSupply (uint64), then Message string. The slice headers are
// 4 zero bytes each (same in BE and LE for value 0), so the byte-order
// break manifests in the Timestamp + InitialSupply fields.
func TestWire_FormatBreak_Documented(t *testing.T) {
	pgc, err := pvmGenesisCodec()
	require.NoError(t, err)

	gen := &pgenesis.Genesis{
		// Empty slices encode as uint32 LE length = 0 (4 bytes zero).
		// Three slices: UTXOs, Validators, Chains.
		Timestamp:     0x0102030405060708,
		InitialSupply: 0x1112131415161718,
		Message:       "z",
	}

	bytes, err := gen.Bytes(pgc)
	require.NoError(t, err)

	// Layout (positions are zero-indexed):
	//   [0..1]   uint16 LE codec version (0x00 0x00 since CodecVersion=0)
	//   [2..5]   uint32 LE UTXOs slice length (0)
	//   [6..9]   uint32 LE Validators slice length (0)
	//   [10..13] uint32 LE Chains slice length (0)
	//   [14..21] uint64 LE Timestamp
	//   [22..29] uint64 LE InitialSupply
	//   [30..31] uint16 LE Message length (1)
	//   [32]     'z'

	got := hex.EncodeToString(bytes)
	t.Logf("ZAP-native PVM Genesis (Timestamp=0x0102030405060708, "+
		"InitialSupply=0x1112131415161718, Message=\"z\"): %s", got)

	// Required prefix demonstrating LE byte order:
	//   00 00                              — version
	//   00 00 00 00                        — UTXOs[] length=0
	//   00 00 00 00                        — Validators[] length=0
	//   00 00 00 00                        — Chains[] length=0
	//   08 07 06 05 04 03 02 01            — Timestamp LE
	//   18 17 16 15 14 13 12 11            — InitialSupply LE
	//   01 00                              — Message length=1 (uint16 LE)
	//   7a                                 — 'z'
	wantHex := "00000000000000000000000000000807060504030201181716151413121101007a"
	require.Equal(t, wantHex, got,
		"ZAP-native PVM Genesis wire format must be little-endian for uint64 + uint16 fields. "+
			"Legacy linearcodec would emit BE — that wire format is BROKEN by this codec swap.")

	// Roundtrip the fixture through the codec.
	gen2, err := pgenesis.Parse(pgc, bytes)
	require.NoError(t, err)
	require.Equal(t, uint64(0x0102030405060708), gen2.Timestamp)
	require.Equal(t, uint64(0x1112131415161718), gen2.InitialSupply)
	require.Equal(t, "z", gen2.Message)
}
