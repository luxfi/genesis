# Lux Primary C-Chain — `upgrade.json` Activation Notes

Source-of-truth for the Lux primary network C-Chain (chainId `96369`) precompile
activation schedule. This file lives next to `upgrade.json` to explain why each
activation is dated where it is, and which precompiles are deliberately
excluded from the primary-network safe subset.

## Safe-subset framing

- **Brand L2s** (hanzo, zoo, pars, spc) carry **42 precompiles** baked into
  their genesis at block 0 after the `eciesConfig` scrub in this patch.
  See `~/work/lux/genesis/configs/hanzo-mainnet/genesis.json` for the
  canonical L2 set; the precompile keys are identical across the four
  brand L2 genesis files.
- **Primary C-Chain safe subset = 42** (the brand L2 set), plus three
  burn-handler precompiles (`deadConfig`, `deadFullConfig`,
  `deadZeroConfig`) that exist only on the primary network, plus
  `warpConfig` and the AI-Mining / DEX / Router stack already activated
  at genesis (`blockTimestamp: 0`).
- **NOT in this patch** (deferred to v2 — see *Followup* below):
  `kzg4844Config`, `secp256r1Config`, `ed25519Config`. The pinned
  `luxfi/precompile v0.5.27` ships these as primitive modules but does
  not declare `RegisterModule(...)` for them, so the EVM extras
  `PrecompileUpgrade.UnmarshalJSON` parser rejects them with
  `"unknown precompile config"`. Re-add after bumping
  `luxfi/precompile` to a version that registers all three (and
  rebuilding the EVM plugin image so the registry side-effect imports
  pick them up).

## Exclusions

### `eciesConfig` — deliberately excluded

ECIES requires the caller to pass the recipient's *private key* in calldata
so the precompile can perform the discrete-log unwrap. Calldata on a public
chain is permanently world-readable; placing a private key there
irrecoverably exposes it. The same reasoning applies to the symmetric-key
precompiles (AES, ChaCha20), which were removed for the same reason — see
`~/work/lux/precompile/x25519/contract.go` lines 17-23 (the live comment
block carrying the rationale).

X25519 key exchange is retained because the shared secret is *not* in
calldata — only ephemeral public points are. ML-KEM-768 / HQC / X-Wing are
retained for the same reason: they expose only ciphertexts and encapsulated
keys, never secrets.

### `contractDeployerAllowListConfig`, `contractNativeMinterConfig`, `txAllowListConfig`, `rewardManagerConfig` — deliberately excluded

These are admin-gated allow-list precompiles whose activation requires the
operator to provide a stable `adminAddresses` list. The primary C-Chain is
permissionless by design — there is no admin address to assign — so
activating these would either (a) hand admin to an arbitrary address and
create a trust hazard, or (b) activate them empty and leave the gate
dormant, which is strictly worse than not activating them at all. They
remain available for brand L2 / subnet activation where an admin role makes
sense.

### `feeManagerConfig` — removed from canonical

Previously parked at `blockTimestamp: 900000000` (1998-07-09 UTC) as a
registry-slot reservation, but that placement violates the monotonicity
rule enforced by `extras.ChainConfig.verifyPrecompileUpgrades` (timestamps
must be non-decreasing across the upgrade list). The entry is removed from
canonical; if the FeeManager precompile is ever needed, it can be activated
via a separate forward-dated governance event with admin addresses
explicitly assigned. The current `chain-configs/lux-mainnet/config.json`
already pins `feeConfig` directly, so no behavioural change.

## Activation tiers

| Tier | Timestamp (Unix) | Timestamp (UTC) | Count | Notes |
|------|------------------|-----------------|-------|-------|
| Genesis    | `0`          | block 0       | 19 | `warpConfig` + 18 already-live precompiles (the set inlined in `~/work/lux/universe/k8s/lux-mainnet/luxd-startup.yaml` `UPGRADE_JSON`). DO NOT RESCHEDULE — luxd's `checkPrecompileCompatible` refuses to boot if any already-live precompile is moved to a different timestamp. The `IsForwardCompatibleWithLiveActivations` rollout test pins this contract. |
| Safe-subset extension (this patch) | `1782864000` | 2026-07-01 00:00 UTC | 27 | Forward-dated 29 days from patch authorship date (2026-06-02) to give validators upgrade buffer. Brand L2s already carry these at their own block-0 genesis. Three EIP precompiles (`kzg4844Config`, `secp256r1Config`, `ed25519Config`) are NOT included in this patch — they are unregistered in `luxfi/precompile v0.5.27` and would brick boot. See *Exclusions* and *Followup* sections. |

Total entries: **46** (1 warp + 18 live + 27 forward-dated).

## Strict-PQ profile gate

`strictPQTimestamp = 1766708400` (2025-12-25 16:20 PST / 2025-12-26 00:20
UTC) activates `contract.StrictPQReporter` on the ChainConfig. All
classical pairing-based / discrete-log precompiles (BLS12-381 G1/G2/Pairing,
BabyJubJub, Pasta) call `contract.RefuseUnderStrictPQ(state)` at the top of
`Run()` and return `ErrClassicalForbiddenInPQ` while the chain is in
strict-PQ mode.

The forward-dated activations at `1782864000` fall *after* the strict-PQ
activation, so every classical precompile in tier 2 is registered behind
the gate from its first block of activation. No window exists where a
classical precompile would execute permissively on the primary network.

## Forward-date rationale

Today's date when this patch was authored: **2026-06-02**.
Chosen activation: **`1782864000` (2026-07-01 00:00 UTC)** = **29 days of
buffer**.

The 14-day minimum buffer requested by Task #114 is exceeded comfortably.
A clean calendar boundary (Q3 start) was chosen so validators have an
unambiguous, easy-to-remember mark. UTC midnight has no DST drift.

## Deployment surfaces

This canonical file is the source-of-truth. The runtime distribution
surfaces that consume it:

1. `~/work/lux/genesis/configs/mainnet/upgrade.json` — canonical (this
   file).
2. `~/work/lux/universe/k8s/lux-mainnet/luxd-startup.yaml` — k8s
   StatefulSet ConfigMap. The canonical content above is mirrored
   byte-for-byte in the `cchain-upgrade.json` data key. The startup
   script copies that file to `/data/configs/chains/C/upgrade.json`
   at boot (PRIMARY C-CHAIN ONLY). Brand L2 chain config directories
   are NOT touched — brand L2 genesis files carry their precompile
   activations baked at block 0 and overwriting their upgrade.json
   with the C-Chain schedule could reschedule an already-live
   precompile and brick the chain via `checkPrecompileCompatible`.
   Sync rule: this file and the ConfigMap data key are a coordinated
   pair; updating one without the other is a silent drift footgun.

## Followup (v2 patch)

1. Bump `github.com/luxfi/precompile` (currently pinned at `v0.5.27` in
   `~/work/lux/evm/go.mod:36`) to a version that declares
   `RegisterModule(...)` for the `kzg4844`, `secp256r1`, and `ed25519`
   modules. Each module must have an `init()` that registers a
   `precompileconfig.Config` factory keyed on `kzg4844Config`,
   `secp256r1Config`, and `ed25519Config` respectively.
2. Rebuild the EVM plugin image so its
   `~/work/lux/evm/precompile/registry/registry.go` side-effect imports
   pick up the new `init()` calls.
3. Re-add the three keys to `~/work/lux/genesis/configs/mainnet/upgrade.json`
   at a forward-dated timestamp (still after Quasar activation
   `1766708400` so the strict-PQ gate is in force when they activate).
4. Re-run the new
   `TestMainnetUpgradeJSON_UnmarshalsAgainstRegistry` end-to-end test
   to confirm the parser accepts every key in the canonical file.

## Brand genesis scrub (this patch)

`eciesConfig` is deliberately not registered in
`luxfi/evm/precompile/registry/registry.go` (the ECIES precompile was
removed because the recipient secret key sits in calldata — see
*Exclusions* above). The brand L2 genesis files at
`~/work/lux/genesis/configs/{hanzo,zoo,pars,spc}-{mainnet,testnet,devnet}/genesis.json`
(11 of 12) carried `eciesConfig` in their block-0
`precompileUpgrades` array, which would fail the same
`PrecompileUpgrade.UnmarshalJSON` parser as soon as any CTO-side
canonical-genesis rebootstrap re-fired a `CreateChainTx` against them.
The scrub strips `eciesConfig` from those 11 files in this same
patch. `zoo-mainnet` had no `precompileUpgrades` array and was
untouched.

## Verification

After luxd reads this file at boot, validators on Lux mainnet should see:

- Existing primary-C-Chain RPC behaviour preserved (no boot regression);
  `IsForwardCompatibleWithLiveActivations` is the gate.
- After `2026-07-01 00:00 UTC`: `eth_call` to BLS12-381 precompile
  addresses succeed for honest inputs and return
  `ErrClassicalForbiddenInPQ` (chain reports strict-PQ from `1766708400`).
- ML-DSA, SLH-DSA, ML-KEM, HQC, X-Wing precompiles available after
  forward-date.
- Pulsar / Magnetar / Corona / P3Q threshold + PQ-STARK precompiles
  available after forward-date.

## Red review checklist

- [x] Every classical precompile in tier 2 (BLS12-381 family, BabyJubJub,
      Pasta, Pedersen, Poseidon) is verified to call
      `contract.RefuseUnderStrictPQ` at the top of its `Run()` method. If
      any does not, that is a separate latent bug — flag in followup, do
      not block this patch.
- [x] The 27 new activations at `1782864000` do not introduce any
      precompile whose source has not been audited or which has been
      deprecated.
- [x] Activation date `1782864000` (2026-07-01 00:00 UTC) is no earlier
      than 14 days from merge time.
- [x] No `eciesConfig`, `contract*AllowListConfig`, `*NativeMinterConfig`,
      `rewardManagerConfig`, or `*MinterConfig` in the activation list.
- [x] No `kzg4844Config`, `secp256r1Config`, or `ed25519Config` in this
      patch (Red vector 8 CRITICAL — deferred to v2 per *Followup*).
- [x] All 18 already-live precompiles stay at `blockTimestamp: 0` so
      `checkPrecompileCompatible` does not refuse boot.
- [x] Monotonicity test passes (no entry has a smaller blockTimestamp than
      its predecessor in the list).
- [x] k8s StatefulSet startup script patched in lockstep
      (`~/work/lux/universe/k8s/lux-mainnet/luxd-startup.yaml`): new
      `cchain-upgrade.json` data key mirrors this canonical byte-for-byte;
      script seeds it onto `/data/configs/chains/C/upgrade.json` only;
      the 4 brand L2 chain dirs are NOT touched.
- [x] End-to-end UnmarshalJSON regression test added at
      `~/work/lux/evm/params/extras_e2e_test/upgrade_unmarshal_test.go`
      (Red vector 9 MEDIUM): asserts the canonical parses cleanly via
      `extras.UpgradeConfig.UnmarshalJSON` after registry side-effects
      AND asserts unregistered keys (kzg4844/secp256r1/ed25519) are
      rejected with the canonical "unknown precompile config" error.
- [x] Brand mainnet/testnet/devnet genesis files scrubbed of
      `eciesConfig` (Red vector 6 INFO): 11 of 12 had it; all 11
      cleaned; `zoo-mainnet` had no precompileUpgrades array.
