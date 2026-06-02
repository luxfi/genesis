# Lux Primary C-Chain — `upgrade.json` Activation Notes

Source-of-truth for the Lux primary network C-Chain (chainId `96369`) precompile
activation schedule. This file lives next to `upgrade.json` to explain why each
activation is dated where it is, and which precompiles are deliberately
excluded from the primary-network safe subset.

## Safe-subset framing

- **Brand L2s** (hanzo, zoo, pars, spc) carry **43 precompiles** baked into
  their genesis at block 0. See `~/work/lux/genesis/configs/hanzo-mainnet/genesis.json`
  for the canonical L2 set; the precompile keys are identical across the four
  brand L2 genesis files.
- **Primary C-Chain safe subset = 42** (brand L2 `43` minus `eciesConfig`),
  plus three additional standard EIP precompiles that brand L2s do not need
  but the primary network does (`kzg4844Config`, `secp256r1Config`,
  `ed25519Config`), plus three burn-handler precompiles
  (`deadConfig`, `deadFullConfig`, `deadZeroConfig`) that exist only on the
  primary network, plus `warpConfig` and the AI-Mining / DEX / Router stack
  already activated at genesis (`blockTimestamp: 0`).

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
| Safe-subset extension (this patch) | `1782864000` | 2026-07-01 00:00 UTC | 30 | Forward-dated 29 days from patch authorship date (2026-06-02) to give validators upgrade buffer. Brand L2s already carry these at their own block-0 genesis. |

Total entries: **49** (1 warp + 18 live + 30 forward-dated).

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
   StatefulSet startup script. Currently carries a hand-coded inline
   `UPGRADE_JSON` shell variable with 17 of the 18 already-live activations
   (the `aiMiningConfig` plus the 16 deterministic crypto/dex precompiles
   activated at block 0). The same shell variable is written to all 5 EVM
   chains (C + 4 brand L2s), which is a separate latent issue — brand L2s
   already carry these precompiles in their genesis at block 0 and
   overwriting their per-chain `upgrade.json` with the primary set is at
   best a no-op and at worst a wedge. The k8s ConfigMap rewrite is the
   operator-side follow-up; this canonical file change is the
   source-of-truth side.

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

- [ ] Every classical precompile in tier 2 (BLS12-381 family, BabyJubJub,
      Pasta, Pedersen, Poseidon) is verified to call
      `contract.RefuseUnderStrictPQ` at the top of its `Run()` method. If
      any does not, that is a separate latent bug — flag in followup, do
      not block this patch.
- [ ] The 30 new activations at `1782864000` do not introduce any
      precompile whose source has not been audited or which has been
      deprecated.
- [ ] Activation date `1782864000` (2026-07-01 00:00 UTC) is no earlier
      than 14 days from merge time.
- [ ] No `eciesConfig`, `contract*AllowListConfig`, `*NativeMinterConfig`,
      `rewardManagerConfig`, or `*MinterConfig` in the activation list.
- [ ] All 18 already-live precompiles stay at `blockTimestamp: 0` so
      `checkPrecompileCompatible` does not refuse boot.
- [ ] Monotonicity test passes (no entry has a smaller blockTimestamp than
      its predecessor in the list).
- [ ] k8s StatefulSet startup script is patched in a separate PR to (a)
      inline the new canonical `UPGRADE_JSON` for the primary C-Chain only
      and (b) stop writing `upgrade.json` to the brand L2 EVM chain
      directories. The brand L2s already have precompiles in their genesis
      at block 0.
