# Brand genesis canonical homes (2026-06-05)

For mainnet network reset and any future bootstrap-chain runs, read the
brand genesis from these paths (NOT from the dead `_orphan-l2/` bucket):

| Brand | Canonical source                                        | State-repo provenance                              |
|-------|---------------------------------------------------------|----------------------------------------------------|
| hanzo | `~/work/hanzo/universe/configs/genesis/<env>/genesis.json` | `~/work/lux/state/chains/hanzo-<env>/genesis.json` |
| zoo   | `~/work/zoo/universe/configs/genesis/<env>/genesis.json`   | `~/work/lux/state/chains/zoo-<env>/genesis.json`   |
| pars  | `~/work/pars/genesis/networks/<env>/genesis.json`         | `~/work/lux/state/chains/pars-<env>/genesis.json`  |
| spc   | `~/work/spc/genesis/networks/<env>/genesis.json`          | `~/work/lux/state/chains/spc-<env>/genesis.json`   |
| osage | TBD (no historical chain yet)                            | TBD                                                |

Brand-home file = byte-identical mirror of state-repo. State-repo is the
empirical provenance archive. Brand homes are deployment-side consumers
(bootstrap-chain, operator tenantImports, etc.).

For `bootstrap-chain --configs-dir`, assemble a temp dir mirroring
`<configs-dir>/<brand>-<env>/genesis.json` from these sources:

```bash
mkdir -p /tmp/bootstrap-configs/{hanzo,zoo,pars,spc}-mainnet
cp ~/work/hanzo/universe/configs/genesis/mainnet/genesis.json /tmp/bootstrap-configs/hanzo-mainnet/
cp ~/work/zoo/universe/configs/genesis/mainnet/genesis.json   /tmp/bootstrap-configs/zoo-mainnet/
cp ~/work/pars/genesis/networks/mainnet/genesis.json          /tmp/bootstrap-configs/pars-mainnet/
cp ~/work/spc/genesis/networks/mainnet/genesis.json           /tmp/bootstrap-configs/spc-mainnet/
```

The historical `_orphan-l2/` path is dead (tombstoned MOVED.txt at
`~/work/lux/genesis/configs/_orphan-l2/`).
