# SubnetEVM to Coreth Migration Guide

## Overview

This guide documents the successful migration of SubnetEVM blockchain data to Coreth format for the Lux mainnet.

## Key Discoveries

1. **SubnetEVM Key Structure**: Uses plain keys, not hashed:
   - Headers: `namespace + 'h' + be8(number) + hash`
   - Bodies: `namespace + 'b' + be8(number) + hash`
   - Receipts: `namespace + 'r' + be8(number) + hash`
   - Hash→Number mapping: `namespace + 'H' + hash` → 8-byte BE number
   - AcceptorTipKey: `namespace + "AcceptorTipKey"` → tip hash

2. **Header Structure**: SubnetEVM headers have 17 fields (including BaseFee but no Cancun fields)
3. **Database Backend**: SubnetEVM uses PebbleDB, Coreth uses BadgerDB

## Migration Requirements

### Key Format Translation

SubnetEVM PebbleDB keys are formatted as:
- **Hashed mode**: `namespace(32 bytes) + keccak256('h' + blockNumber + blockHash)`
- **Namespace**: `337fb73f9bcdac8c31a2d5f7b877ab1e8a2b7f2a1e9bf02a0a0e6c6fd164f1d1` for Lux mainnet

### Header Structure Upgrade

SubnetEVM headers must be upgraded from London/Shanghai format to Cancun format:

```go
// Legacy SubnetEVM Header (16-17 fields)
type LegacyHeader struct {
    ParentHash  common.Hash
    UncleHash   common.Hash
    Coinbase    common.Address
    Root        common.Hash
    TxHash      common.Hash
    ReceiptHash common.Hash
    Bloom       types.Bloom
    Difficulty  *big.Int
    Number      *big.Int
    GasLimit    uint64
    GasUsed     uint64
    Time        uint64
    Extra       []byte
    MixDigest   common.Hash
    Nonce       types.BlockNonce
    BaseFee     *big.Int // optional
}

// Modern Coreth Header (adds 4 fields)
type Header struct {
    // ... all legacy fields ...
    WithdrawalsHash    *common.Hash    // New: Cancun
    BlobGasUsed        *uint64         // New: Cancun
    ExcessBlobGas      *uint64         // New: Cancun
    ParentBeaconRoot   *common.Hash    // New: Cancun
}
```

## Migration Approaches

### Approach 1: Raw RLP Copy (Recommended)

Copy headers as raw RLP bytes without decoding:

```go
// Read raw RLP from SubnetEVM
headerRLP := readRaw(sourceDB, hashedKey)

// Write raw RLP to Coreth
db.Put(corethKey, headerRLP)
```

**Pros**: 
- Preserves exact header hash
- No decoding/encoding errors
- Fastest migration

**Cons**:
- Headers remain in old format
- May cause issues with Coreth expecting Cancun fields

### Approach 2: Decode and Upgrade

Decode to legacy struct, add Cancun fields, re-encode:

```go
// Decode SubnetEVM header
var legacy LegacyHeader
rlp.DecodeBytes(headerRLP, &legacy)

// Upgrade to modern format
modern := &types.Header{
    // Copy all legacy fields
    ...
    // Add Cancun fields with defaults
    WithdrawalsHash:  nil, // No withdrawals
    BlobGasUsed:      nil, // No blob gas
    ExcessBlobGas:    nil, // No blob gas
    ParentBeaconRoot: nil, // No beacon chain
}

// Write upgraded header
rawdb.WriteHeader(db, modern)
```

**Pros**:
- Headers compatible with Cancun-aware Coreth
- Future-proof

**Cons**:
- Changes header hash (breaks canonical mapping)
- Requires re-deriving all block hashes

## Migration Solution

The successful migration uses:

1. **AcceptorTipKey** to find the tip hash
2. **H mapping** to get block numbers from hashes
3. **Plain key construction** for headers, bodies, and receipts
4. **Raw RLP copying** to avoid header structure issues
5. **Parent hash extraction** without full decode

## Implementation

The migration tool (`cmd/migrate_final.go`) performs:

1. Reads tip hash from `AcceptorTipKey`
2. Gets tip height from H mapping
3. Walks backwards from tip to genesis
4. Copies raw RLP data without modification
5. Writes VM metadata for Coreth
6. Verifies invariants

## Results

- **Blocks migrated**: 1,082,781 (genesis to block 1,082,780)
- **Migration time**: 75 seconds
- **Average rate**: 14,430 blocks/second
- **Genesis hash**: `0x3f4fa2a0b0ce089f52bf0ae9199c75ffdd76ecafc987794050cb0d286f1ec61e`
- **Tip hash**: `0x32dede1fc8e0f11ecde12fb42aef7933fc6c5fcf863bc277b5eac08ae4d461f0`

## Usage

```bash
# Build the migration tool
go build -o bin/migrate_final cmd/migrate_final.go

# Run migration
./bin/migrate_final

# Boot node with migrated data
cd ~/work/lux/node
./build/luxd --network-id=96369 --consensus-k=1
```

## References

- SubnetEVM source: `/Users/z/work/lux/state/chaindata/lux-mainnet-96369/db/pebbledb`
- Target C-chain: `/Users/z/.luxd/network-96369/chains/X6CU5qgMJfzsTB9UWxj2ZY5hd68x41HfZ4m4hCBWbHuj1Ebc3/ethdb`
- Block height: 1,082,780
- Tip hash: `0x32dede1fc8e0f11ecde12fb42aef7933fc6c5fcf863bc277b5eac08ae4d461f0`