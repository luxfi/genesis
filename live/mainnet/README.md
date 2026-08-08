# live/mainnet — what lux mainnet actually runs

These two files are the **only** artifacts that reproduce live lux mainnet. Nothing
else in this repository does, and until they were committed they existed in exactly
one place: a base64 blob inside the Kubernetes ConfigMap `luxd-startup` in namespace
`lux-mainnet`.

```
genesis.json    523,815 B   sha256 0304701fbd8ccc9c173e5d394d6944fb3f13cd0199603ee6ed4b27833ee2851b
genesis.bytes 2,458,686 B   sha256 4c506d7880b6ae9fe73547c490f0280063a3f8d63054e64a96ccdb843d0a7ddb
```

`genesis.json` is the source document, decoded from that ConfigMap and written to
`/data/genesis.json` on every boot by `startup.sh`. `genesis.bytes` is the built
P-Chain genesis blob it produces — the artifact that created the current P-Chain
state, and therefore the thing that defines every chain id on mainnet.

Building `genesis.json` with the node's own builder reproduces `genesis.bytes`
byte-for-byte, and all nine chain ids match `platform.getBlockchains` read in-pod:

```
C  25td8att1CPFPxXZptSfjWS7s7U2rNtaTQP7YmKK97HxorsnpH
```

## Why these are not templates

**No file under `configs/` reproduces live mainnet.** Four sources give four
different C-Chain ids; only this one is real:

| source | C-Chain id |
|---|---|
| **`live/mainnet/genesis.json`** | **`25td8att…orsnpH`** ✅ |
| `configs/mainnet/genesis.json` @ v1.16.4 | `VbydisH6…zoFpg` |
| `configs/mainnet/cchain.json` shard | `2rjwBZTs…kvRvZ` |
| `GetConfig(1)` shard fallback | `pd38gJLo…BjrB`, and **11 chains** |

They are genuinely different chains, not different formatting: this document carries
`durangoTimestamp`/`quasarTimestamp`/`fortunaTimestamp`/`graniteTimestamp` and a 15M
`targetGas`, while the shard carries `warpConfig`, `feeSplitTimestamp`, `cancunTime`,
`shanghaiTime`, `blobSchedule` and 500M `targetGas`.

A chain id is `sha256` over the CreateChainTx, which carries **no credentials** —
nothing is signed. So the id is a pure function of networkID, VMID, chain name, fxIDs
and the exact genesis-data bytes. **One byte of whitespace moves it.** Do not
reformat these files. Do not run them through a JSON pretty-printer.

## The operational hazard these protect against

luxd reaches this document only because `startup.sh` passes
`--genesis-file=/data/genesis.json`. **If that flag is ever dropped**, `config.go`
falls through to the shard loader, which yields **11 chains of which only X-Chain
matches live — ten phantom chains injected into mainnet's P-Chain state on the next
boot**, and re-introduces `warpConfig`, the key previously recorded as having bricked
the C-Chain.

The risk is one line in a ConfigMap, not in any binary. Guard the ConfigMap.

Before rolling any node build to mainnet, check:

1. `--genesis-file=/data/genesis.json` is still in `luxd-startup`
2. the embedded blob still hashes to `0304701f…`
3. building this document with the candidate binary yields `4c506d78…`,
   2,458,686 bytes, C-Chain `25td8att…`

Step 3 runs offline in seconds and needs no cluster.

## Also note

This document declares ten chain-genesis keys; the builder emits nine.
`TChainGenesis` has no field in the config struct and the parse uses stdlib
`encoding/json` with no `DisallowUnknownFields`, so **`tChainGenesis` is dropped
silently**. That is the same silent-drop that left M and F unmounted on devnet.
