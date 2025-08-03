# 3. Genesis CLI Reference

This document is the definitive reference guide for the unified `genesis` CLI tool. It covers its command structure, options, and provides examples for common operations.

## Overview

The `genesis` tool is the single entry point for all genesis-related operations. It replaces dozens of older, individual scripts with a consistent, hierarchical command structure.

**Key Features:**
- **DRY Principle**: No duplicate functionality across different scripts.
- **Predictable**: Consistent command structure and naming conventions.
- **Discoverable**: Built-in help for every command (`genesis help <command>`).
- **Comprehensive**: All genesis, migration, and analysis operations in one tool.
- **Well-Tested**: Integrated test suite for all commands.

## Using as a `lux-cli` Plugin

While the `genesis` tool can be run as a standalone binary (`./bin/genesis`), it is designed to function as a plugin for `lux-cli`. After running `make install-plugin`, you can invoke all commands through `lux-cli`:

-   `./bin/genesis generate` becomes `lux-cli genesis generate`
-   `./bin/genesis import chain-data` becomes `lux-cli genesis import chain-data`

This guide will use the standalone `./bin/genesis` syntax for clarity, but all commands are compatible with the `lux-cli` plugin invocation.

## Top-Level Commands

The `genesis` tool's functionality is organized into the following top-level commands. You can see this list by running `./bin/genesis tools`.

-   `generate`: Generate genesis files for all chains (P, C, X).
-   `validators`: Manage validator configurations.
-   `extract`: Extract blockchain data from various sources.
-   `import`: Import blockchain data and cross-chain assets.
-   `analyze`: Analyze blockchain data and structures.
-   `inspect`: Inspect database contents in detail.
-   `scan`: Scan external blockchains for assets.
-   `migrate`: Migrate data between formats and chains.
-   `repair`: Fix and repair blockchain data issues.
-   `launch`: Launch Lux nodes with various configurations.
-   `export`: Export blockchain data in various formats.
-   `validate`: Validate configurations and data.

---

## Command Details

### `genesis generate`

Generates genesis files for P-Chain, C-Chain, and X-Chain.

**Usage:**
`./bin/genesis generate [options]`

**Key Options:**
-   `--network`: Network type (`mainnet`, `testnet`, `local`). Default: `mainnet`.
-   `--output`: Output directory. Default: `configs/{network}`.
-   `--validators`: Path to the validator JSON file (e.g., `configs/mainnet-validators.json`).
-   `--import-cchain`: Path to an existing C-Chain genesis file to import for data continuity.
-   `--import-allocations`: Path to a CSV or JSON file with token allocations to include.
-   `--treasury-amount`: Set the treasury amount (e.g., `2T`, `500B`, `100M`).

**Example:**
```bash
# Generate mainnet genesis using a validators file and importing allocations
./bin/genesis generate \
    --network mainnet \
    --validators configs/mainnet-validators.json \
    --import-allocations exports/all_allocations.csv \
    --treasury-amount 2T
```

---

### `genesis validators`

Manages the validator set.

**Subcommands:**
-   `list`: List the current validators in the configuration.
-   `add`: Add a new validator to the set.
-   `remove`: Remove a validator from the set.
-   `generate`: Generate new validator keys.

**Example: `validators generate`**
```bash
# Generate 5 new validator keys from a mnemonic, saving the public info
# and the private keys to the git-ignored validator-keys/ directory.
./bin/genesis validators generate \
    --mnemonic "your twelve word mnemonic phrase here" \
    --offsets "0,1,2,3,4" \
    --save-keys configs/mainnet-validators.json \
    --save-keys-dir validator-keys/
```

---

### `genesis extract`

Extracts data from a blockchain database (PebbleDB/LevelDB). This is a crucial first step for any migration.

**Subcommands:**
-   `state`: Extracts all data, removing the 33-byte subnet namespace prefix. This is the most common command.
-   `genesis`: Extracts only the genesis configuration from a database.
-   `blocks`: Extracts a range of blocks.

**Example: `extract state`**
```bash
# Extract data from a historic subnet DB, including all state (balances, contracts).
# This prepares the data for migration to C-Chain or L2.
./bin/genesis extract state /path/to/source/pebbledb /path/to/output/dir \
    --network 96369 \
    --state
```

---

### `genesis import`

Imports data into a Lux node.

**Subcommands:**
-   `chain-data`: The primary command to import a full chain history into a node.
-   `consensus`: Imports consensus data (advanced use).

**Example: `import chain-data`**
```bash
# Import previously exported chain data into a fresh node instance.
# This will start the node, run the import, and restart it in normal mode.
./bin/genesis import chain-data /path/to/source/chaindata \
  --data-dir=$HOME/.luxd-import \
  --network-id=96369
```

---

### `genesis migrate`

Performs complex data migrations. This is covered in detail in the [Migration Guide](./4_migration_guide.md).

**Subcommands:**
-   `subnet`: Migrates a subnet's data to be compatible with the C-Chain.
-   `zoo`: A specialized command to handle the ZOO token migration from BSC, including token burns and Egg NFT allocations.
-   `add-evm-prefix`: A utility to add the `evm` prefix to database keys.

**Example: `migrate zoo`**
```bash
# Run the full ZOO migration pipeline
./bin/genesis migrate zoo-migrate \
    --burns-csv exports/zoo-bsc-burns.csv \
    --eggs-csv exports/egg-nft-holders.csv \
    --output genesis/zoo-genesis-allocations.json
```

---

### `genesis scan`

Scans external blockchains (like Ethereum or BSC) for data relevant to genesis creation.

**Subcommands:**
-   `tokens`: Scan for token information.
-   `nfts`: Scan for NFT holders.
-   `burns`: Scan for token burn events at a specific address.
-   `holders`: Scan for all token holders.

**Example: `scan burns`**
```bash
# Scan the BSC mainnet for ZOO token burn events
./bin/genesis scan token-burns \
    --chain bsc \
    --token 0x0a6045b79151d0a54dbd5227082445750a023af2 \
    --burn-address 0x000000000000000000000000000000000000dEaD \
    --output exports/zoo-bsc-burns.csv
```

---

### `genesis launch`

Launches a Lux node with various configurations. Often used for testing.

**Subcommands:**
-   `clean`: Launch with a clean state (fresh genesis).
-   `mainnet`/`testnet`: Launch with the respective network configuration.
-   `migrated`: Launch a node using previously migrated data.
-   `verify`: Launch a node and run RPC checks to verify it's working.

**Example: `launch migrated`**
```bash
# Launch a node using the data in a specified directory
./bin/genesis launch migrated --data-dir /path/to/migrated/data
```

---

### `genesis inspect`

Provides low-level inspection of the blockchain database. Useful for debugging complex migration issues.

**Subcommands:**
-   `keys`: Inspect the raw keys in the database.
-   `blocks`: Inspect the details of stored blocks.
-   `tip`: Find the highest block (the "tip") of the chain.
-   `prefixes`: Scan the database to see the distribution of key prefixes (e.g., `evmh`, `evmb`).

**Example: `inspect tip`**
```bash
# Find the block height of a database
./bin/genesis inspect tip --db /path/to/db/pebbledb
```

---

## Migration from Old Scripts

This tool replaces many older scripts. If you are familiar with the old workflow, use this table to find the new command.

| Old Script/Tool                 | New `genesis` Command                     |
| ------------------------------- | ----------------------------------------- |
| `analyze-subnet-blocks.go`      | `genesis analyze blocks --subnet`         |
| `migrate-subnet-to-cchain.go`   | `genesis migrate subnet`                  |
| `check-head-pointers.go`        | `genesis inspect tip`                     |
| `fix-snowman-ids.go`            | `genesis repair snowman`                  |
| `add-evm-prefix-to-blocks.go`   | `genesis repair prefix`                   |
| `rebuild-canonical.go`          | `genesis repair canonical`                |
| `export-state-to-genesis.go`    | `genesis export genesis`                  |
| `import-consensus.go`           | `genesis import consensus`                |
| `scan-db-prefixes.go`           | `genesis inspect prefixes`                |
| `launch-mainnet-automining.sh`  | `genesis launch mainnet --automining`     |
| `launch-clean-cchain.sh`        | `genesis launch clean`                    |
| `analyze-keys-detailed.go`      | `genesis analyze keys --detailed`         |
| `find-highest-block.go`         | `genesis inspect tip`                     |
| `check-block-format.go`         | `genesis inspect blocks`                  |
| `migrate_evm.go`                | `genesis migrate evm`                     |
| `namespace`                     | `genesis extract state`                   |
| `teleport` commands             | `genesis scan` and `genesis migrate`      |
| `archeology` / `evmarchaeology` | `genesis analyze` and `genesis inspect`   |
