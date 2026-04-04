# Lux Mainnet Genesis Configuration for geth

**Last Updated**: 2025-12-15

## Overview

This document describes the genesis configuration required for importing SubnetEVM blocks into `luxfi/geth`. The genesis MUST match the original SubnetEVM genesis exactly to ensure cryptographic integrity.

## Genesis Hash

**Canonical Genesis Hash**: `0x3f4fa2a0b0ce089f52bf0ae9199c75ffdd76ecafc987794050cb0d286f1ec61e`

This hash is computed from the canonical genesis configuration below. Any deviation will produce a different hash, and all subsequent state roots will mismatch.

## Canonical Genesis Configuration

```json
{
  "config": {
    "chainId": 96369,
    "homesteadBlock": 0,
    "eip150Block": 0,
    "eip155Block": 0,
    "eip158Block": 0,
    "byzantiumBlock": 0,
    "constantinopleBlock": 0,
    "petersburgBlock": 0,
    "istanbulBlock": 0,
    "berlinBlock": 0,
    "londonBlock": 0,
    "terminalTotalDifficulty": 0,
    "terminalTotalDifficultyPassed": true,
    "subnetEVMTimestamp": 0,
    "durangoTimestamp": 0
  },
  "nonce": "0x0",
  "timestamp": "0x672485c2",
  "extraData": "0x",
  "gasLimit": "0xb71b00",
  "difficulty": "0x0",
  "mixHash": "0x0000000000000000000000000000000000000000000000000000000000000000",
  "coinbase": "0x0000000000000000000000000000000000000000",
  "alloc": {
    "0x9011E888251AB053B7bD1cdB598Db4f9DEd94714": {
      "balance": "2000000000000000000000000000000"
    },
    "0x0200000000000000000000000000000000000005": {
      "nonce": "1",
      "balance": "0",
      "code": "0x01"
    }
  },
  "number": "0x0",
  "gasUsed": "0x0",
  "parentHash": "0x0000000000000000000000000000000000000000000000000000000000000000",
  "baseFeePerGas": "0x5d21dba00"
}
```

## Critical Configuration Fields

### Chain Configuration

| Field | Value | Description |
|-------|-------|-------------|
| `chainId` | 96369 | Lux Mainnet chain ID |
| `terminalTotalDifficulty` | 0 | Post-merge (PoS) from genesis |
| `terminalTotalDifficultyPassed` | true | Merge already happened |
| `subnetEVMTimestamp` | 0 | SubnetEVM features active from genesis |
| `durangoTimestamp` | 0 | Durango upgrade (Shanghai EIPs) from genesis |

### SubnetEVM-Specific Fields

These fields are **REQUIRED** in `luxfi/geth` for SubnetEVM compatibility:

#### `subnetEVMTimestamp`

Enables SubnetEVM gas accounting:
- Coinbase receives full `gasPrice * gasUsed` (no EIP-1559 burning)
- Gas refunds are disabled (ApricotPhase1 behavior)
- No base fee burning to coinbase

#### `durangoTimestamp`

Enables Shanghai EIPs via Durango upgrade:
- EIP-3651: Warm coinbase
- EIP-3855: PUSH0 instruction
- EIP-3860: Init code metering (2 gas per 32-byte word)
- EIP-4895: Beacon chain withdrawals (unused)

## Genesis Parameters Explanation

### Timestamp
```
"timestamp": "0x672485c2"
```
Decimal: 1730447810 (Unix timestamp of genesis block)

### Gas Limit
```
"gasLimit": "0xb71b00"
```
Decimal: 12,000,000 gas

### Base Fee
```
"baseFeePerGas": "0x5d21dba00"
```
Decimal: 25,000,000,000 (25 Gwei)

### Difficulty
```
"difficulty": "0x0"
```
Must be 0 for post-merge chains.

## Alloc Section

### Treasury Account
```json
"0x9011E888251AB053B7bD1cdB598Db4f9DEd94714": {
  "balance": "2000000000000000000000000000000"
}
```
2 billion LUX (with 18 decimals)

### Precompile Contract
```json
"0x0200000000000000000000000000000000000005": {
  "nonce": "1",
  "balance": "0",
  "code": "0x01"
}
```
SubnetEVM native minter precompile placeholder.

## Initialization Command

```bash
# Initialize geth database with genesis
./geth init --datadir /path/to/datadir /path/to/genesis.json

# Verify genesis hash in output:
# INFO [...] Successfully wrote genesis state database=chaindata hash=3f4fa2..1ec61e
```

## Genesis Hash Verification

The genesis hash MUST be `0x3f4fa2a0b0ce089f52bf0ae9199c75ffdd76ecafc987794050cb0d286f1ec61e`.

If you see a different hash, check:
1. `difficulty` must be `"0x0"` (not `"0x1"`)
2. All fork blocks must be 0
3. `terminalTotalDifficultyPassed` must be `true`
4. Timestamp must be exactly `"0x672485c2"`
5. No extra whitespace in alloc addresses

## Common Mistakes

### Wrong Genesis Hash

| Mistake | Effect |
|---------|--------|
| `difficulty: "0x1"` | Different hash, all state roots fail |
| Missing `subnetEVMTimestamp` | Gas accounting mismatch |
| Missing `durangoTimestamp` | EIP-3860 gas mismatch (876 gas per contract deploy) |
| Different timestamp | Different genesis hash |
| Different alloc balances | Different genesis state root |

### State Root Mismatch at Block N

If state roots match until block N then fail:
1. Check if block N deploys a contract (EIP-3860)
2. Check if block N has gas refunds (disabled in SubnetEVM)
3. Check coinbase balance accumulation

## Code References

### geth Configuration
- `params/config.go`: ChainConfig with SubnetEVMTimestamp, DurangoTimestamp
- `params/config.go:1518`: Rules() enables Shanghai when Durango active

### State Transition
- `core/state_transition.go:570`: SubnetEVM coinbase payment
- `core/state_transition.go:665`: Gas refund disabling

## Version Requirements

- `luxfi/geth` >= v1.16.55
- Must include SubnetEVM gas accounting patches
