# Brand genesis canonical homes (2026-06-05)

For mainnet network reset and any future bootstrap-chain runs, read the
brand genesis from these paths (NOT from the dead `_orphan-l2/` bucket):

| Brand | Canonical source                                          | State-repo provenance                              |
|-------|-----------------------------------------------------------|----------------------------------------------------|
| hanzo | `~/work/hanzo/universe/configs/genesis/<env>/genesis.json` | `~/work/lux/state/chains/hanzo-<env>/genesis.json` |
| zoo   | `~/work/zoo/universe/configs/genesis/<env>/genesis.json`   | `~/work/lux/state/chains/zoo-<env>/genesis.json`   |
| pars  | `~/work/pars/genesis/networks/<env>/genesis.json`         | `~/work/lux/state/chains/pars-<env>/genesis.json`  |
| spc   | `~/work/spc/genesis/networks/<env>/genesis.json`          | `~/work/lux/state/chains/spc-<env>/genesis.json`   |
| osage | `~/work/osage/genesis/networks/<env>/genesis.json`        | `~/work/lux/state/chains/osage-<env>/genesis.json` |

Brand-home file = byte-identical mirror of state-repo. State-repo is the
empirical provenance archive. Brand homes are deployment-side consumers
(bootstrap-chain, operator tenantImports, etc.).

Chain ID assignments:
- hanzo:  mainnet 36963, testnet 36962, devnet 36964
- zoo:    mainnet 200200, testnet 200201, devnet 200202
- pars:   mainnet 494949, testnet 494950, devnet 494951
- spc:    mainnet 36911,  testnet 36910,  devnet 36912
- osage:  mainnet 1872,   testnet 1871,   devnet 1873

For `bootstrap-chain --configs-dir`, assemble a temp dir mirroring
`<configs-dir>/<brand>-<env>/genesis.json` from these sources:

```bash
mkdir -p /tmp/bootstrap-configs/{hanzo,zoo,pars,spc,osage}-mainnet
cp ~/work/hanzo/universe/configs/genesis/mainnet/genesis.json /tmp/bootstrap-configs/hanzo-mainnet/
cp ~/work/zoo/universe/configs/genesis/mainnet/genesis.json   /tmp/bootstrap-configs/zoo-mainnet/
cp ~/work/pars/genesis/networks/mainnet/genesis.json          /tmp/bootstrap-configs/pars-mainnet/
cp ~/work/spc/genesis/networks/mainnet/genesis.json           /tmp/bootstrap-configs/spc-mainnet/
cp ~/work/osage/genesis/networks/mainnet/genesis.json         /tmp/bootstrap-configs/osage-mainnet/
```

The historical `_orphan-l2/` path is dead (tombstoned MOVED.txt at
`~/work/lux/genesis/configs/_orphan-l2/`).
