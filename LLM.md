# Genesis

Canonical genesis configurations for all Lux blockchain networks. **This repo is the single source of truth for chain IDs, genesis hashes, state roots, and required precompiles.** Other repos (`~/work/lux/{node,evm,state}`) reference these values; do not duplicate them.

**Latest Tag**: v1.9.6

## Post-E2E-PQ State (current)

Genesis now pins a `ChainSecurityProfile` for every chain it generates,
and resolves it at load time. The profile hash is committed into the
chain's bootstrap so a profile-flip is a wrong-network event, not a
silent classification change.

### Recent significant commits

| SHA | Tag | Impact |
|-----|-----|--------|
| `85a8fc8` | v1.9.6 | Pin ChainSecurityProfile into Config + Resolve at load (closes F102 genesis layer) |
| `1d73c8d` | v1.9.6 | Swap mldsa65 keygen from cloudflare/circl to luxfi/crypto/pq/mldsa |
| `e698307` | v1.9.5 | HD branch hardening + LIGHT_MNEMONIC production refusal (F12/F30/F31/F35) |
| `fe4781c` | v1.9.5 | `LUX_DISABLE_CCHAIN=1` omits C-Chain from primary genesis |
| `2b16f45` | v1.9.5 | Operator-overridable C-Chain genesis via `LUX_CCHAIN_GENESIS_FILE` |

### Active versions
- Repo: `v1.9.6` (next: `v1.9.7`).
- Pinned by: `luxfi/node v1.26.10`.
- Pulls: `luxfi/consensus v1.23.4+` (for `ChainSecurityProfile`
  definition + `ComputeHash`), `luxfi/crypto v1.18.4+` (mldsa65 keygen).

### Cross-repo dependencies
- `luxfi/consensus/config` → `ChainSecurityProfile`, `Resolve`,
  `ComputeHash`.
- `luxfi/crypto/pq/mldsa/mldsa65` → validator key generation.
- `luxfi/node/genesis/builder` → consumes this repo's JSON config and
  converts types (string → ids.NodeID, uint64 → time.Duration).

### Where to look for X
- ChainSecurityProfile pin: `pkg/genesis/security_profile.go`
- Profile hash verification: `pkg/genesis/security_profile.go:Resolve`
- HD branch hardening: `pkg/wallet/hd/*.go`
- Per-network configs: `configs/{mainnet,testnet,devnet,local}/`

### Open follow-ups
- Mainnet/testnet/devnet genesis JSONs still reference the
  `legacy-compat` profile while operators migrate validator keys. The
  strict-PQ profile is encoded but not yet the default genesis pin.

---

## Canonical Requirements

Every Lux chain genesis MUST satisfy these, or block import / consensus will fail:

| Requirement | Mainnet C-Chain | How to verify |
|---|---|---|
| chainId | 96369 (NOT 1) | `jq -r .chainId configs/mainnet/cchain.json` |
| Genesis hash | `0x3f4fa2a0b0ce089f52bf0ae9199c75ffdd76ecafc987794050cb0d286f1ec61e` | `geth init --datadir /tmp/v configs/mainnet/cchain.json 2>&1 \| grep "genesis hash"` |
| State root | `0x2d1cedac263020c5c56ef962f6abe0da1f5217bdc6468f8c9258a0ea23699e80` | `jq -r .stateRoot configs/mainnet/cchain.json` |
| Warp precompile in alloc | `0200000000000000000000000000000000000005` `nonce:0x1` `code:0x01` | `jq '.alloc."0200000000000000000000000000000000000005"' configs/mainnet/cchain.json` |
| `warpConfig.blockTimestamp` | `1730446786` (== genesis ts `0x672485c2`) | must equal genesis timestamp |
| Treasury | `0x9011E888251AB053B7bD1cdB598Db4f9DEd94714` = 2T LUX | `jq '.alloc."0x9011…"' configs/mainnet/cchain.json` |
| File determinism | sorted, sha256 matches Canonical Genesis Checksums table below | `shasum -a 256 configs/mainnet/cchain.json` |

If genesis hash drifts → check `cChainGenesis` in `genesis.json` is the literal canonical `cchain.json` content. Common drift: `chainId=1` + missing warp = wrong-network template.

## Networks

### C-Chain (Primary Network)

| Network | Network ID | Chain ID | Genesis Hash | Treasury |
|---------|------------|----------|--------------|----------|
| Mainnet | 1 | 96369 | `0x3f4fa2a0b0ce089f52bf0ae9199c75ffdd76ecafc987794050cb0d286f1ec61e` | 2T LUX |
| Testnet | 2 | 96368 | `0x1c5fe37764b8bc146dc88bc1c2e0259cd8369b07a06439bcfa1782b5d4fb0995` | 2T LUX |
| Devnet | 3 | 96370 | `0x53fe8be293555d20de41847f96081f4e8beca1ee2c206999ffbf7c70e497cf43` | 2T LUX |
| Local | 1337 | 1337 | - | 2T LUX |

### L2 Chains

| Chain | Mainnet ID | Testnet ID | Devnet ID | Treasury |
|-------|-----------|-----------|-----------|----------|
| Zoo | 200200 | 200201 | 200202 | 2T ZOO |
| SPC | 36911 | 36910 | 36912 | 1B SPC |
| Hanzo | 36963 | 36962* | 36964 | 1B AI |
| Pars | 494949 | 494950 | 494951 | 2T PARS |

*Note: Hanzo testnet was deployed with chain ID 36964 (devnet ID) due to a deployment error. The intended ID is 36962.

### Deployed Genesis Hashes (All Chains)

| Chain | Mainnet | Testnet | Devnet |
|-------|---------|---------|--------|
| C-Chain | `0x3f4fa2a...1ec61e` | `0x53fe8be...97cf43` | `0x53fe8be...97cf43` |
| Zoo | `0x7c548af...14933` | `0x0652fb2...d1979` | `0x0652fb2...d1979` |
| SPC | `0x4dc9fd5...cfac8` | `0x210fcc1...0b692` | `0x0652fb2...d1979` |
| Hanzo | `0x6cf786a...48e67` | `0x6cf786a...48e67` | `0x6cf786a...48e67` |
| Pars | `0xa32921a...b2f34` | `0xa32921a...b2f34` | `0xa32921a...b2f34` |

## Genesis Treasury

All networks use the same production treasury address:

```
Address: 0x9011E888251AB053B7bD1cdB598Db4f9DEd94714
Balance: 2,000,000,000,000,000,000,000,000,000,000 wei (2T LUX)
Hex:     0x193e5939a08ce9dbd480000000
```

## Directory Structure

```
configs/
├── mainnet/
│   ├── cchain.json         # C-Chain genesis (chain ID 96369)
│   ├── pchain.json         # P-Chain genesis (ProtocolVM)
│   ├── network.json        # Network ID, message, startTime
│   └── bootstrappers.json  # Bootstrap nodes
├── testnet/
│   ├── cchain.json         # C-Chain genesis (chain ID 96368)
│   └── genesis.json
├── devnet/
│   └── cchain.json
├── local/
│   ├── cchain.json
│   └── genesis.json
├── zoo-{mainnet,testnet,devnet}/
│   └── genesis.json        # Zoo (200200/200201/200202)
├── spc-{mainnet,testnet,devnet}/
│   └── genesis.json        # SPC (36911/36910/36912)
├── hanzo-{mainnet,testnet,devnet}/
│   └── genesis.json        # Hanzo (36963/36962/36964)
└── pars-{mainnet,testnet,devnet}/
    └── genesis.json        # Pars (494949/494950/494951)
```

## C-Chain Configuration

### Mainnet (96369)

```json
{
  "chainId": 96369,
  "gasLimit": 12000000,
  "targetBlockRate": 2,
  "minBaseFee": 25000000000,
  "stateRoot": "0x2d1cedac263020c5c56ef962f6abe0da1f5217bdc6468f8c9258a0ea23699e80",
  "baseFeePerGas": "0x5d21dba00"
}
```

### Network Parameters

| Parameter | Mainnet | Testnet | Devnet | Local |
|-----------|---------|---------|--------|-------|
| Block Rate | 2s | 2s | 1s | 1s |
| Gas Limit | 12M | 12M | 20M | 15M |
| Min Base Fee | 25 gwei | 25 gwei | 1 gwei | 1 gwei |
| Chain ID | 96369 | 96368 | 96370 | 1337 |

### Warp Precompile (Required for Genesis Hash)

Both mainnet and testnet C-Chain genesis MUST include the warp precompile:

```json
{
  "alloc": {
    "0200000000000000000000000000000000000005": {
      "balance": "0x0",
      "nonce": "0x1",
      "code": "0x01"
    },
    "0x9011E888251AB053B7bD1cdB598Db4f9DEd94714": {
      "balance": "0x193e5939a08ce9dbd480000000"
    }
  }
}
```

This produces the canonical state root: `0x2d1cedac263020c5c56ef962f6abe0da1f5217bdc6468f8c9258a0ea23699e80`

### Upgrade Timestamps

C-Chain protocol upgrades activated Dec 25, 2024 @ 4:20pm UTC (timestamp 1735143600):

```json
{
  "evmTimestamp": 0,
  "durangoTimestamp": 0,
  "etnaTimestamp": 1735143600,
  "fortunaTimestamp": 1735143600,
  "graniteTimestamp": 1735143600
}
```

## Genesis Package (`pkg/genesis/`)

### Core Types

```go
// Config - Top-level genesis configuration
type Config struct {
    NetworkID                  uint32
    Allocations                []Allocation
    StartTime                  uint64
    InitialStakeDuration       uint64
    InitialStakeDurationOffset uint64
    InitialStakedFunds         []ids.ShortID
    InitialStakers             []Staker
    CChainGenesis              string
    Message                    string
}

// Staker - Initial validator
type Staker struct {
    NodeID        ids.NodeID
    RewardAddress ids.ShortID
    DelegationFee uint32
    Signer        *ProofOfPossession
    Weight        uint64
    StartTime     uint64
    EndTime       uint64
}

// Allocation - Genesis allocation
type Allocation struct {
    ETHAddr        ids.ShortID
    LUXAddr        ids.ShortID
    InitialAmount  uint64
    UnlockSchedule []LockedAmount
}
```

### Bech32 Address Format

Addresses use chain prefix + bech32 encoding:

```
Format: {chain}-{hrp}1{data}{checksum}
Example: P-lux1ck0t9h5u7jvvzhx29n99guqjsfkpzt67wgx7wg

Chain Prefixes:
- P-lux1...  (P-Chain — ProtocolVM)
- X-lux1...  (X-Chain/Exchange)
- C-0x...    (C-Chain/EVM hex)
```

**Important**: The bech32 checksum is computed using only the HRP ("lux"), not the chain prefix ("P-lux").

## CLI Tools

### genesis (`cmd/genesis/`)

Generate genesis configurations:

```bash
# Generate from mnemonic
MNEMONIC="test test..." ./genesis -network-id 96369 -validators 5 -output genesis.json

# From existing keys
./genesis -network mainnet -keys-dir ~/.lux/keys -output genesis.json
```

### extract-genesis (`cmd/extract-genesis/`)

Extract genesis from existing chain data:

```bash
./extract-genesis -chaindata /path/to/chaindata -output cchain.json
```

## State Repository Integration

Historical blockchain state is maintained separately in [luxfi/state](https://github.com/luxfi/state).

### Available State Data

| Network | Blocks | RLP Size | Location |
|---------|--------|----------|----------|
| Lux Mainnet | 1,082,780 | 1.2 GB | `rlp/lux-mainnet/lux-mainnet-96369.rlp` |
| Lux Testnet | 218 | 695 KB | `rlp/lux-testnet/lux-testnet-96368.rlp` |
| Zoo Mainnet | 799 | 1.3 MB | `rlp/zoo-mainnet/zoo-mainnet-200200.rlp` |
| Zoo Testnet | 84 | 156 KB | `rlp/zoo-testnet/zoo-testnet-200201.rlp` |
| SPC Mainnet | 11 | 7.8 KB | `rlp/spc-mainnet/spc-mainnet-36911.rlp` |

### Import Workflow

```bash
# Clone repos
git clone https://github.com/luxfi/genesis
git clone https://github.com/luxfi/state

# Initialize with genesis
geth init --datadir /tmp/lux genesis/configs/mainnet/cchain.json

# Import blocks
geth import --datadir /tmp/lux state/rlp/lux-mainnet/lux-mainnet-96369.rlp
```

### LuxEVM Header Formats

State repo RLP uses LuxEVM header encoding:

| Fields | Format | Usage |
|--------|--------|-------|
| 16 | Genesis block | No optional fields |
| 17-19 | Post-genesis | ExtDataHash, ExtDataGasUsed, BlockGasCost |

**Coreth field order** (positions 15+):
```
15: ExtDataHash    (common.Hash - value type, required)
16: BaseFee        (*big.Int)
17: ExtDataGasUsed (*big.Int)
18: BlockGasCost   (*big.Int)
```

## Mainnet Validators

| Node | NodeID | P-Chain Address |
|------|--------|-----------------|
| 1 | NodeID-FrtEjhat6RUqjEWCJgYZKqBaxY2Woyy5G | P-lux1ck0t9h5u7jvvzhx29n99guqjsfkpzt67wgx7wg |
| 2 | NodeID-9hq49qGVZN7M7tXxdpF3AqptQGdmPCFnQ | P-lux1dclruwcn9ug8u0jjk3ukh676jr3lsy4er9m3l5 |
| 3 | NodeID-8osEnSC4LQFdG1LMit12CBsE6BfKGHNAw | P-lux1qdv9zns0gpfesw0h28jqp2up6h77du2damqf88 |
| 4 | NodeID-MrdgTgPuddyo7anomKr4akMcoKzVKcgbG | P-lux1jjs4nx7ul4d6pnsjtpv2khzu8p4yctegvass46 |
| 5 | NodeID-Mgd5yHs4pe6qRjkBcW5Y7oqHA51j4afcC | P-lux1970ngvf6s6rsndrkvjzr6lfvf2tdl30529c435 |

## Token Decimals

```go
// P-Chain/X-Chain: 6 decimals (microLUX as base unit)
const (
    MicroLux uint64 = 1            // 0.000001 LUX
    MilliLux uint64 = 1_000        // 0.001 LUX
    Lux      uint64 = 1_000_000    // 1 LUX
)

// C-Chain: 18 decimals (wei, standard EVM)
// 1 LUX = 10^18 wei on C-Chain
```

## Verification

### Genesis Hash Check

```bash
# Verify mainnet C-Chain genesis
geth init --datadir /tmp/verify configs/mainnet/cchain.json 2>&1 | grep "genesis hash"
# Expected: 0x3f4fa2a0b0ce089f52bf0ae9199c75ffdd76ecafc987794050cb0d286f1ec61e

# Verify state root
cat configs/mainnet/cchain.json | jq -r '.stateRoot'
# Expected: 0x2d1cedac263020c5c56ef962f6abe0da1f5217bdc6468f8c9258a0ea23699e80
```

### Network ID Verification

```bash
cat configs/mainnet/network.json | jq '.networkID'
# Expected: 96369

cat configs/testnet/network.json | jq '.networkID'  
# Expected: 96368
```

## Fortuna Timestamp Fix (2025-12-25)

### Problem
Block import failed with gas limit mismatch when importing Zoo chain blocks:
```
invalid gas limit: have 12000000, want 10000000
```

### Root Cause
Zoo genesis files had `fortunaTimestamp: 0` which activates Fortuna from block 0. Fortuna uses dynamic gas limits via `state.MaxCapacity()` returning 10M, but original blocks had 12M gas limit (Cortina rules).

### Solution
Set far-future timestamps for Fortuna, Etna, and Granite upgrades:

```json
{
  "etnaTimestamp": 253399622400,
  "fortunaTimestamp": 253399622400,
  "graniteTimestamp": 253399622400
}
```

### Files Updated
- `configs/zoo-mainnet/genesis.json`
- `configs/zoo-testnet/genesis.json`

### Verified Block Counts (2025-12-28)

| Network | Chain ID | Blocks | Genesis Hash |
|---------|----------|--------|--------------|
| Lux Mainnet | 96369 | **1,082,780** | `0x3f4fa2a0b0ce089f52bf0ae9199c75ffdd76ecafc987794050cb0d286f1ec61e` |
| Lux Testnet | 96368 | **218** | `0x1c5fe37764b8bc146dc88bc1c2e0259cd8369b07a06439bcfa1782b5d4fb0995` |
| Zoo Mainnet | 200200 | **799** | `0x7c548af47de27560779ccc67dda32a540944accc71dac3343da3b9cd18f14933` |
| Zoo Testnet | 200201 | **84** | `0x0652fb2fde1460544a5893e5eba5095ff566861cbc87fcb1c73be2b81d6d1979` |
| SPC Mainnet | 36911 | **11** | `0x4dc9fd5cf4ee64609f140ba0aa50f320cadf0ae8b59a29415979bc05b17cfac8` |

---

## Canonical Genesis Checksums (SHA256)

**IMPORTANT**: All genesis files are sorted alphabetically by key for determinism.
These checksums are the canonical reference - if they don't match, the file has been modified.

| File | SHA256 |
|------|--------|
| `mainnet/cchain.json` | `968e9f3cd19a9b88ca157db3894622272ded02f0f184b5386493ff55897001be` |
| `testnet/cchain.json` | `ba211e1bbbd7807c2224d240628facb2dfb6c2d6cde2e274eb881a57609ed038` |
| `zoo-mainnet/genesis.json` | `14c01e8bb5ac13144a3730c695847d53506647efbaada3f3240a16e7b10684f7` |
| `zoo-testnet/genesis.json` | `74357b85e6968ca70d8907b2e940e5e234fd6b288749865d91ab69a6083e6dae` |
| `zoo-devnet/genesis.json` | `dd604fee5c78f7367b314a9a7d7ee1d5f8c42839165164262368afc7dd91fc5e` |
| `spc-mainnet/genesis.json` | `9c01115559556fc245d4993b193889370bd99edd22a7fa1e9e3d7ddafbb63535` |
| `spc-testnet/genesis.json` | `af12bb81dd9abce26b7a311f00fa2ac7f5adfb996e698b9f473252199f747b2c` |
| `spc-devnet/genesis.json` | `317d54ac7846b0f174964552b4949d858072f6ffbc37f6c10092e01d595ce46f` |
| `hanzo-mainnet/genesis.json` | `adae3c441b34db937dbcd961450bff5bcabd388a5479fe9ba7380ef61f083637` |
| `hanzo-testnet/genesis.json` | `a2d00685bf7f414815dc3579552e19045183428c520f4f5b23a6726cf4002c32` |
| `hanzo-devnet/genesis.json` | `1c489d29c08857696909f029a1b2cda554e059e86c630ae64f35a5243f4bd25c` |
| `pars-mainnet/genesis.json` | `ca35306a73f4cbfeccc8018b71596b122f698a23f3c038993a2ad34ca315e539` |
| `pars-testnet/genesis.json` | `9d7ff05187bb2570d2a1af8ccfa41a99dfaf8b56558decf127209ac6d5faec85` |
| `pars-devnet/genesis.json` | `3fc55f6be7b46dd6955ab2ec95186a4aa7da13991650909cb5f9ecbaa51b06c7` |

### Verify Checksum

```bash
cd ~/work/lux/genesis/configs
shasum -a 256 zoo-mainnet/genesis.json
# Should output: 241fd0209253e7a5a04c55c7a6f4f0ba864a83dd9840c4969d5a1b5b5f6c3366
```

### Re-sort if Modified

```bash
# If file was modified, re-sort it:
jq -S '.' genesis.json > genesis.sorted.json && mv genesis.sorted.json genesis.json
```

---

## Genesis Hash Fix (2025-12-28)

### Problem
Mainnet genesis hash was computing as `0xc3069db32ef986d8e287458e44066eb04205648e602dc2fede561c424a6516de` instead of the expected `0x3f4fa2a0b0ce089f52bf0ae9199c75ffdd76ecafc987794050cb0d286f1ec61e` from historical RLP blocks.

### Root Cause
The `warpConfig.blockTimestamp` was set to 0, which caused `ApplyPrecompileActivations()` in coreth to add the warp precompile to genesis state BEFORE the alloc was applied. When warp wasn't in alloc, this changed the state trie and produced a different genesis hash.

**Key insight from `coreth/core/genesis.go:271`:**
```go
// ApplyPrecompileActivations runs FIRST (line 271)
ApplyPrecompileActivations(g.Config, nil, blockContext, statedb)

// Then alloc is applied (lines 276-283)
for addr, account := range g.Alloc {
    // This overwrites precompile if in alloc, leaves it if not
}
```

### Solution
The original mainnet genesis was created with warp precompile IN the alloc section:

```json
{
  "alloc": {
    "0200000000000000000000000000000000000005": {
      "balance": "0x0",
      "nonce": "0x1", 
      "code": "0x01"
    }
  },
  "warpConfig": {
    "blockTimestamp": 1730446786,
    "quorumNumerator": 67,
    "requirePrimaryNetworkSigners": false
  }
}
```

The `warpConfig.blockTimestamp` equals the genesis timestamp (`0x672485c2` = 1730446786), meaning warp is activated at genesis. Having warp in both alloc AND warpConfig ensures consistent state.

### Files Restored
- `configs/mainnet/cchain.json` - Restored from git with warp in alloc
- `configs/mainnet/genesis.json` - Updated cChainGenesis to match

### Verification
```bash
# Start mainnet
netrunner start --networks mainnet

# Verify genesis hash
curl -X POST -H 'Content-Type: application/json' \
  --data '{"jsonrpc":"2.0","method":"eth_getBlockByNumber","params":["0x0",false],"id":1}' \
  http://127.0.0.1:9630/ext/bc/C/rpc | jq '.result.hash'
# Expected: "0x3f4fa2a0b0ce089f52bf0ae9199c75ffdd76ecafc987794050cb0d286f1ec61e"

# Verify warp precompile in state
curl -X POST -H 'Content-Type: application/json' \
  --data '{"jsonrpc":"2.0","method":"eth_getCode","params":["0x0200000000000000000000000000000000000005","0x0"],"id":1}' \
  http://127.0.0.1:9630/ext/bc/C/rpc | jq '.result'
# Expected: "0x01"
```

---

## Known Issues

### Documentation Discrepancies

1. **Network IDs**: Some older docs reference Network ID 1/2 instead of 96369/96368
2. **File paths**: Docs may reference `chain.rlp.gz` but actual files are `{network}-{chainid}.rlp`
3. **Block counts**: Documentation may be outdated - verify against actual RLP files

### State Repo Sync

The state repo should be periodically updated with:
- Latest block exports
- Updated metadata.json files
- Corrected documentation

---

## RLP Import via luxd `--import-chain-data`

End-to-end auto-import of historical chain data on luxd startup. Replaces the
prior geth-sidecar workflow.

### Flow

1. `luxd --import-chain-data=<path.rlp>` (existing flag, was unwired).
2. `node/config/config.go` *config bridge* injects `import-chain-data` into
   `nodeConfig.ChainConfigs["C"].Config` JSON map. Merges with any existing
   C-Chain config.
3. EVM plugin reads `vm.config.ImportChainData` in `Initialize()` after
   `initializeChain()` returns. If non-empty, calls `importBlocksFromFile`
   (same code path as `admin_importChain` RPC).
4. RLP stream decoded in batches of 2,500 blocks. State trie committed every
   `CommitInterval` (4,096) blocks; `acceptedBlockDB` updated atomically with
   each commit so restart-safe.
5. Final state commit + last-accepted update at end-of-stream.

### Required cChainGenesis (NON-NEGOTIABLE)

`genesis.json` `cChainGenesis` MUST be the canonical mainnet config:
- `chainId`: 96369 (NOT 1 — chainId 1 in cChainGenesis means genesis.json was
  overwritten with a wrong-network template; restore from `cchain.json`)
- Warp precompile in `alloc`: address `0200000000000000000000000000000000000005`,
  `nonce: "0x1"`, `code: "0x01"`
- `warpConfig.blockTimestamp`: 1730446786 (== genesis timestamp 0x672485c2)

If any of these drift, the EVM computes a non-canonical genesis hash (commonly
`0xcd5083cb…`) and RLP block 1's `parentHash` won't match → import fails with
`batch 0: block 1 parent mismatch - expected genesis 0xcd…, got 0x3f4fa2a0…`.

Restore from canonical:
```bash
python3 -c "import json
canon = json.load(open('configs/mainnet/cchain.json'))
g = json.load(open('configs/mainnet/genesis.json'))
g['cChainGenesis'] = json.dumps(canon, separators=(',', ':'), sort_keys=True)
json.dump(g, open('configs/mainnet/genesis.json', 'w'), indent=2, sort_keys=True)"
```

### Wiring sites (when re-implementing)

| Layer | File | What |
|-------|------|------|
| Flag | `node/config/keys.go` `ImportChainDataKey` | Already exists |
| Flag | `node/config/flags.go:370` | Already exists |
| Bridge | `node/config/config.go:2012-2028` | NEW: inject into `chainConfigs["C"]` after `getChainConfigs` |
| Plugin config | `evm/plugin/evm/config/config.go` `Config.ImportChainData` | NEW: matches JSON key `import-chain-data` |
| Plugin init | `evm/plugin/evm/vm.go` `Initialize()` post-`initializeChain` | NEW: calls `importBlocksFromFile` |
| Import logic | `evm/plugin/evm/admin_api.go:220` `importBlocksFromFile` | Pre-existing — used by `admin_importChain` |

### macOS gotcha (kills luxd silently)

After `cp` of the luxd binary to a new path, macOS gatekeeper can SIGKILL on
launch (exit 137, no output). The provenance attribute (`com.apple.provenance`)
plus an invalidated ad-hoc signature is the cause. Fix:
```bash
codesign --force --sign - ~/work/lux/node/build/luxd
codesign --force --sign - ~/work/lux/node/build/plugins/*
```

### Plugin filename / VM ID

The Lux EVM plugin has one canonical VM ID for both C-Chain and EVM-based L2 chains:

```
mgj786NP7uDwBCcq6YwThhaN8FLyybkCa4zBWTQbNgmK6k9A6
```

This is `CB58(ids.ID{'e','v','m'})` per `luxfi/constants.EVMID`. luxd's chain manager, the lux-operator, and `luxfi/cli` all reference this one ID. Install the plugin binary at `~/.lux/plugins/mgj786NP7uDwBCcq6YwThhaN8FLyybkCa4zBWTQbNgmK6k9A6` — no other filename, no aliases.

### Verified output (mainnet)

```
read last accepted hash=0x3f4fa2a0b0ce089f52bf0ae9199c75ffdd76ecafc987794050cb0d286f1ec61e height=0
Loaded most recent local header  hash=0x3f4fa2a0…
Initializing snapshots  headHash=0x3f4fa2a0…  headRoot=0x2d1cedac263020c5c56ef962f6abe0da1f5217bdc6468f8c9258a0ea23699e80
Auto-importing chain data at startup  path=…/lux-mainnet-96369.rlp
ImportChain: inserting batch  batch=0 blocks=2500 firstNum=1
ImportChain: batch done  batch=0 imported=2500 height=2500
…
```

Throughput: ~550 blocks/sec early, ~150 blocks/sec by ~100k blocks (state-trie
growth). 1.08M blocks → ~60-90 min wall time on M-series.

---

