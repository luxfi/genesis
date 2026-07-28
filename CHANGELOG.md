# genesis CHANGELOG

## Unreleased

### Fee split: scheduled on all three public C-Chains, with a governed destination

`feeSplitTimestamp` activates luxfi/evm's `creditTxFee` split — half of every
transaction fee is truly burned (credited to no account, so total supply drops)
and half is credited to the configured coinbase, i.e. to whatever the
`rewardManagerConfig` precompile points at. The split changes HOW MUCH is kept;
RewardManager still decides WHERE. It lives in `cchain.json` (the genesis
ChainConfig) because it is deliberately not a NetworkUpgrade and therefore
cannot be staged through `upgrade.json`.

Activation timestamps (UTC), staged devnet → testnet → mainnet:

| network | chain | feeSplitTimestamp | = | reward half → |
|---------|-------|-------------------|---|---------------|
| devnet  | 96367 | 1785974400 | 2026-08-06T00:00:00Z | `0x8E29b816c6C35b13cE1ff68D33E245C2bda8ac3D` (DAO Safe) |
| testnet | 96368 | 1786320000 | 2026-08-10T00:00:00Z | `0xEAbCC110fAcBfebabC66Ad6f9E7B67288e720B59` |
| mainnet | 96369 | 1786924800 | 2026-08-17T00:00:00Z | `0x8E29b816c6C35b13cE1ff68D33E245C2bda8ac3D` (DAO Safe) |

Both previously committed timestamps were stale, and each would be wrong the
moment a build reads the field:

- devnet was `1785129000` (2026-07-27T05:10Z) — in the PAST. A fork timestamp
  that moves from `nil` to a value at or before the head timestamp is a
  `ConfigCompatError` (`extras.isForkTimestampIncompatible`), so
  `SetupGenesisBlock` fails and the C-Chain does not initialize.
- mainnet was `1785715200` (2026-08-03T00:00:00Z) with **no** `rewardManagerConfig`
  anywhere in its schedule — half of every fee burned, the other half credited to
  the keyless blackhole `0x0100..00`, which already holds ~3868 LUX on 96369 and
  grows with every block.

Neither of those failures can happen on a shipped binary today, and neither can
the split fire. This is a SCHEDULE, not an activation:

- `plugin/evm.parseGenesis` populates the extras config by naming each genesis
  `config` key it supports (`evmTimestamp`, `durangoTimestamp`,
  `quasarTimestamp`, `fortunaTimestamp`, `graniteTimestamp`, precompile keys,
  `feeConfig`, `allowFeeRecipients`). `feeSplitTimestamp` is in that list on no
  luxfi/evm tag and not on `main` either, and luxfi/evm keeps extras in a side
  map rather than in the ChainConfig JSON (libevm integration was removed), so
  the key cannot arrive by plain unmarshal. It is inert: it neither activates
  the split nor fails a boot.
- `extras.verifyFeeSplit`, the guard that refuses an unpaired split, exists on
  `main` only — `git tag --contains` is empty for it.
- `core/fee_split.go` first appears at evm `v1.104.14`. In every tag that has it
  (through `v1.104.22`) the kept half is credited to the compiled-in
  `extras.FeeRewardVault` = `0x0100..02`, NOT to the RewardManager coinbase;
  crediting the coinbase is a `main`-only change. So the destination promised
  above is only what `main` does.
- Deployed reality: mainnet runs luxd `1.36.2` → chains `v1.7.2` → evm
  `v1.99.51`; testnet `1.36.24` and devnet `1.36.25` → chains `v1.7.9` → evm
  `v1.104.10`. None of the three contains a `FeeSplitTimestamp` field at all.

Activating the split therefore takes an evm change (name the key in
`parseGenesis`) plus a node roll — and the roll must be ordered after every live
C-Chain genesis carries a FUTURE timestamp, or the compat rule above bites. The
live `lux-devnet/luxd-genesis` ConfigMap currently carries `1785133547`
(2026-07-27T06:25Z, already past) and that value exists in no repo.

`configs/<net>/upgrade.json` therefore now carries `rewardManagerConfig`,
appended last so the activation schedule stays monotonic:

- devnet `1784057456` and testnet `1784063913` are the values **already live**
  on those fleets, committed here verbatim; an already-activated precompile
  upgrade may not be modified or absent from a later config.
- mainnet activates at `1786320000` (2026-08-10T00:00:00Z), a week before its
  split, so fee routing to the DAO Safe is observable at the full rate before
  it halves. Until then 100% of mainnet fees keep landing at the blackhole.

`configs/fee_split_test.go` pins the pairing here, from the two files alone,
precisely because no shipped luxd enforces it — so a split with no governed
destination fails in CI instead of on a validator. It also pins the monotonic
ordering that `verifyPrecompileUpgrades` requires.

Deployment note: `feeSplitTimestamp` reaches a running fleet only through the
C-Chain genesis luxd loads at boot (`--genesis-file`, or these embedded configs).
It must be loaded **before** the timestamp passes, or the config becomes
unbootable by the rule above. It is not a `NetworkUpgrade` and so cannot be
staged through `upgrade.json`; a policy fork that becomes unbootable once it
passes unloaded belongs in `UpgradeConfig` like every other one, and moving it
there is the durable fix.

### Mnemonic env unification — one var, one way

The genesis mnemonic is now read from exactly one env var: `LUX_MNEMONIC`.
The previous two-env fallback chain (`MNEMONIC` then `LIGHT_MNEMONIC`)
is gone — there is one and only one canonical name.

- `pkg/genesis.MnemonicEnvVar` constant exposes the name (`"LUX_MNEMONIC"`).
- `getMnemonicEnv()` reads only `LUX_MNEMONIC`. No fallback.
- All CLI help, error messages and tests now reference `LUX_MNEMONIC`.
- `LightMnemonic` constant still holds the well-known dev seed value:
  pass it as the value of `LUX_MNEMONIC` to bootstrap a local network.

Per-env isolation comes from a DIFFERENT mnemonic per env (loaded from
KMS for production), not from the env-var name.

### Dead-code removal — vesting rot

The "50M free + 50M vesting at 1%/year over 100y from Jan 1 2020"
docstring was a lie: the canonical builder `buildConfigFromKeyInfos`
never wired the 100-period schedule (the prior code admits the schedule
"overflows zapdb batch limit"). Drop the unreachable plumbing:

- `StakingStartTime`, `UnlockInterval`, `VestingPeriods` constants
- `buildUnlockSchedule()` function (pkg/genesis/keys.go)
- `VestingConfig` struct + `DefaultVesting()` + `WithVesting()`
- vesting branches in `ChainAllocations.PChain()` / `PChainMap()`
- `GeneratePChainAllocationsWithVesting()` function
- `MainnetAllocations` no longer calls `WithVesting(DefaultVesting())`

The validator stake-lock (3-entry 5y/10y/20y schedule attached to the
validator's UTXO inside `buildConfigFromKeyInfos`) is unchanged — that
locked-stake bucket is the only `UnlockSchedule` in the canonical
genesis path and the ProtocolVM needs it.

`pkg/genesis/networks.go` adds the canonical primary-network-id table
(`MainnetID` 1, `TestnetID` 2, `DevnetID` 3, `LocalID` 1337) as the
single source of truth for "what envs exist".

### Breaking — HIP-0077 §"Identity" HD path alignment

`pkg/genesis/keys.go` now derives keys on the per-network hardened
branches mandated by HIP-0077:

| derived              | new path                          | curve / scheme | old path (removed)                  |
|----------------------|-----------------------------------|----------------|-------------------------------------|
| `device_pq_key[i]`   | `m/44'/9000'/nid'/0'/i'`          | ML-DSA-65      | n/a (was loaded from disk only)     |
| `device_lux_key[i]`  | `m/44'/9000'/nid'/1'/i'`          | secp256k1      | `m/44'/9000'/0'/0/i`                |

All five levels (`44'`, `9000'`, `nid'`, branch, index) are now hardened
on the secp256k1 branch. Per-network hardening (`nid'` at the account
level) means the same mnemonic on different network ids derives fully
independent keypairs.

The ML-DSA-65 keypair is generated by expanding the 32-byte BIP-32
child seed via `SHAKE-256("LUX/HIP-0077/mldsa65" || child_seed)` into
the 32-byte ξ that FIPS 204 §5.1 KeyGen consumes. The expansion is
domain-separated so future schemes sharing the same BIP-32 child seed
cannot collide.

#### Function signature changes

```
// before
LoadKeysFromMnemonic(mnemonic string, numAccounts int) ([]KeyInfo, error)
LoadKeysFromMnemonicEnv(numAccounts int)                ([]KeyInfo, error)
BuildWalletAllocations(numKeys int, amountPerKey uint64) ([]Allocation, error)
BuildWalletKeyHex(index int)                             (string, error)

// after — every public derivation entry point now takes the network id
LoadKeysFromMnemonic(mnemonic string, nid uint32, numAccounts int) ([]KeyInfo, error)
LoadKeysFromMnemonicEnv(nid uint32, numAccounts int)                ([]KeyInfo, error)
BuildWalletAllocations(nid uint32, numKeys int, amountPerKey uint64) ([]Allocation, error)
BuildWalletKeyHex(nid uint32, index int)                             (string, error)
```

`LoadKeysFromMnemonicEnvForNetwork(networkID, numAccounts)` keeps its
signature; it now passes `networkID` through as the `nid` for the
underlying derivation, preserving the F30 public-mnemonic guard.

`KeyInfo.MLDSAPublicKey` is now populated from the deterministic
mnemonic derivation. Old loads from `nodeDir/mldsa/public.key` still
work for keys-on-disk paths (`LoadKeysFromDir`).

#### Migration

Any address derived under the old `m/44'/9000'/0'/0/i` path will NOT
reproduce under the new layout. Specifically:

- Treasury and any other addresses pinned via the mnemonic (validator
  ETH/P-chain addresses, fee reserve keys, wallet allocations) must be
  re-derived after upgrading.
- If you need to keep the old addresses spendable, export their private
  keys (`BuildWalletKeyHex` on the old code) and load them via
  `KEYS_DIR` instead of `LUX_MNEMONIC` — `LoadKeysFromDir` is unchanged.

Per CLAUDE.md `no backwards compatibility, only forwards perfection`:
this is the correct break; clients deriving from the HIP-0077 spec
agree with `luxd` going forward, and nobody is left guessing which
historical path to use.

#### Library note

ML-DSA-65 keygen currently uses `github.com/cloudflare/circl/sign/mldsa/mldsa65`
because `github.com/luxfi/crypto/pq/mldsa` (v1.17.44) does not yet
expose a public `NewKeyFromSeed` entry point. Migration is tracked by
a `TODO(canonical)` at the import site in `pkg/genesis/keys.go`; the
canonical replacement MUST adopt the same
`SHAKE-256("LUX/HIP-0077/mldsa65" || child_seed)` expansion to preserve
determinism.
