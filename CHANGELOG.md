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
| mainnet | 96369 | 1789430400 | 2026-09-15T00:00:00Z | `0xF66B025b46844AFA5d6df54cf0C00E1583cE1abA` (DAO Safe, deployed CREATE2) |

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
  `feeConfig`, `allowFeeRecipients`). `feeSplitTimestamp` is now named there on
  `main` (`TestParseGenesisReadsFeeSplitTimestamp` /
  `TestParseGenesisRefusesSplitWithoutRewardManager` pin both the round-trip and
  the refusal); on every SHIPPED tag it is still absent, and luxfi/evm keeps
  extras in a side map rather than in the ChainConfig JSON (libevm integration
  was removed), so on those tags the key cannot arrive by plain unmarshal. On a
  shipped binary it stays inert — it neither activates the split nor fails a boot.
- `extras.verifyFeeSplit`, the guard that refuses an unpaired split, exists on
  `main` only — `git tag --contains` is empty for it.
- `core/fee_split.go` first appears at evm `v1.104.14`. In every tag that has it
  (through `v1.104.22`) the kept half is credited to the compiled-in
  `extras.FeeRewardVault` = `0x0100..02`, NOT to the RewardManager coinbase;
  crediting the coinbase is a `main`-only change. So the destination promised
  above is only what `main` does.
- Deployed reality (verified live 2026-08-22 via `info.getNodeVersion` + git):
  mainnet luxd `v1.36.148` → chains `v1.7.26` → evm `v1.104.30`; testnet
  `v1.36.147` (same chains/evm); devnet `v1.36.139` → chains `v1.7.24` → evm
  `v1.104.30`. All three carry the `FeeSplitTimestamp` field + `verifyFeeSplit` +
  `creditTxFee` (present since evm `v1.104.14`), but NOT the `parseGenesis` reader
  that populates the field from config — that is only on evm `main`/`v1.104.50`.
  Even current node `main` (`v1.36.177` → chains `v1.7.33`) still pins evm
  `v1.104.30`, so nothing shipped reads `feeSplitTimestamp` yet.

The evm change has landed on `main` (see the parseGenesis note above), so
activation now takes only a node roll carrying that binary — and the roll must
be ordered after every live C-Chain genesis carries a FUTURE timestamp, or the
compat rule above bites. The live `lux-devnet/luxd-genesis` ConfigMap currently
carries `1785133547` (2026-07-27T06:25Z, already past) and that value exists in
no repo. The ordered flag-day procedure is written out under "Fee-split roll"
below; it is owner-gated because it rolls the mainnet validator set.

`configs/<net>/upgrade.json` therefore now carries `rewardManagerConfig`,
appended last so the activation schedule stays monotonic:

- devnet `1784057456` and testnet `1784063913` are the values **already live**
  on those fleets, committed here verbatim; an already-activated precompile
  upgrade may not be modified or absent from a later config.
- mainnet carries `rewardManagerConfig` (with the rest of its 49 precompiles) at
  `1766708400`, the Dec-25 clean-slate reboot timestamp — so on the reboot chain
  the reward destination is live from genesis and 100% of fees route to the DAO
  Safe `0xF66B025b46844AFA5d6df54cf0C00E1583cE1abA` before the split ever halves
  them. The split then forks on at `1789430400` (2026-09-15). Rolling the split
  onto the EXISTING chain instead (no reboot) is the other path, and there both
  the precompile and the split need timestamps ahead of the live head — that
  branch is the owner-gated "Fee-split roll" below.

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

### Fee-split roll — the ordered flag-day (owner-gated)

The DAO Safe is deployed and both files point at it. What remains is one roll,
and it is owner-gated because it cycles the mainnet validator set. Do it in this
order; each step has a check that must pass before the next.

1. Get the fix into a deployable luxd. evm is a Go module, not the deployable;
   luxd bundles it through `luxfi/chains`. The fix is on evm `main`, tagged
   `v1.104.50` (parseGenesis reads the key; `core/fee_split.go` credits the
   coinbase `GetCoinbaseAt` resolves from RewardManager). Propagate it: bump
   `luxfi/chains`' evm dep to `v1.104.50` and tag chains (it pins `v1.104.30`
   today), then bump `luxfi/node`'s chains dep to that tag and tag node (`v1.36.177`
   → next patch). CI then builds the luxd image. The deployed fleet runs evm
   `v1.104.30` via chains `v1.7.26` (luxd v1.36.148), so the gap to `v1.104.50`
   is ~20 patches / 75 files — a normal patch-range bump, smaller than once
   feared, but review the `.31`–`.50` range, not just the fee-split commit.
   DONE-WHEN: a luxd image exists whose bundled `evm.parseGenesis` carries
   `feeSplitTimestamp` — the two `TestParseGenesis*` tests are in that evm.

2. Confirm the two destinations agree, from the two files alone, before anything
   rolls: `rewardManagerConfig.initialRewardConfig.rewardAddress` in
   `configs/mainnet/upgrade.json` == the deployed DAO Safe
   `0xF66B025b46844AFA5d6df54cf0C00E1583cE1abA`, and the Safe has code on 96369.
   `configs/fee_split_test.go` already pins RewardManager-enabled-at-the-split
   from the files; run it. DONE-WHEN: that test is green and `eth_getCode` on the
   Safe is non-empty.

3. Read the LIVE head time on a mainnet validator (in-pod, not a public LB) and
   pick the branch:
   - Clean-slate reboot: the reboot genesis makes every precompile (RewardManager
     included) active at block 0, so only `feeSplitTimestamp` must be future
     relative to the reboot — it is (`1789430400`, 2026-09-15). No timestamp edit.
   - Roll onto the existing chain: BOTH must be ahead of the live head at roll
     time, RewardManager ≤ split. RewardManager currently sits at `1766708400`
     (past); to add it to a running chain, bump it to a future value ≤
     `1789430400` and keep it the last, highest entry so
     `verifyPrecompileUpgrades` stays monotonic. Then bump `feeSplitTimestamp`
     too if 2026-09-15 is no longer comfortably ahead of the head.
   DONE-WHEN: live head time < RewardManager activation ≤ `feeSplitTimestamp`.

4. Roll the fleet on the new binary + config, owner-gated and one node at a time,
   counting `(nodes at tip − α)` before each step — never below the safety
   margin. `verifyFeeSplit` runs at parse, so a node handed a split with no
   governed destination REFUSES to boot rather than burning-and-stranding; a
   failed parse on one node is a safe stop, not a chain halt. DONE-WHEN: all
   validators are on the new binary and at the tip, `latest == finalized`.

5. After the split activates, confirm the money moves: the DAO Safe balance rises
   with block production and the blackhole `0x0100..00` stops growing. DONE-WHEN:
   the Safe's balance delta over a window is positive and equals ~half of fees
   spent in that window; the blackhole delta is ~zero.

Reversibility: steps 1–3 touch only code, config, and reads — fully reversible.
Step 4 is the point of no return for the burn (burning is irreversible by
design), so step 3's check is the gate that must hold. The destination is not:
RewardManager's admin (`0x9011` today) can `setRewardAddress` to move where the
kept half lands afterward, no roll and no fork.

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
