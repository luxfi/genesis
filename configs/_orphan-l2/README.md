# `_orphan-l2/` — brand L1 genesis without a dedicated universe repo

Per **Task #109** (2026-06-04), brand L1 genesis was moved out of
`luxfi/genesis/configs/` into the per-brand universe repos:

| Brand | New canonical owner                            |
|-------|------------------------------------------------|
| hanzo | `hanzoai/universe → configs/genesis/{env}/`    |
| zoo   | `zooai/universe → configs/genesis/{env}/`      |
| pars  | (no `parsdao/universe` yet — lives here)       |
| spc   | (no separate repo yet — lives here)            |

`pars-*` and `spc-*` genesis blobs cohabitate `luxfi/genesis/` only
because they do not yet have a dedicated universe repo. When one is
spun up, move them out and delete this directory.

Do **not** put `hanzo-*` or `zoo-*` here — they belong in their
own universe repos. The `no_brand_shadow_test.go` test in
`pkg/genesis/` enforces this.
