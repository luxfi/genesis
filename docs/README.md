# Lux Network Genesis Management System

A unified, DRY (Don't Repeat Yourself) system for managing Lux Network genesis configurations, validators, and network launches. This system provides a comprehensive suite of tools for developers, network operators, and community members to interact with the Lux blockchain ecosystem.

## Project Philosophy

The core philosophy of this toolset is to provide a single, reliable, and well-documented way to perform critical network operations. It replaces a multitude of disparate scripts with a unified `genesis` CLI tool, ensuring that all functionality is predictable, discoverable, and thoroughly tested.

## Key Features

- **Unified CLI Tool**: A single `genesis` binary for all genesis, validator, and data migration operations.
- **Comprehensive Genesis Generation**: Create genesis files for the L1 (P-Chain, C-Chain, X-Chain) and all L2 networks (ZOO, SPC, Hanzo).
- **Advanced Data Migration**: Robust tools to migrate existing Subnet EVM data to either the C-Chain or a new L2, handling real-world data issues like state-only exports and key format inconsistencies.
- **Validator Management**: Securely generate and manage validator keys, including BLS and TLS components.
- **Flexible Launch Options**: Launch networks using `make`, a unified launch script, or Docker for production-like environments.
- **Cross-Chain Support**: Built-in functionality to scan external chains (like BSC and Ethereum) for token burns and NFT ownership to incorporate into genesis allocations.

## Quick Start

This section is for experienced users. If you are new, please follow the [Getting Started Guide](./2_getting_started.md).

### 1. Prerequisites

- Go 1.24.5+
- `make`
- Docker (for Docker-based launch)

### 2. Install Dependencies & Build Tools

```bash
make all
```

### 3. Generate Validators and Genesis

```bash
# Generate 11 validators from a mnemonic
# (Replace with your own secure mnemonic)
MNEMONIC="spirit level garage dot typical page asset abstract embark primary vendor right" make validators-generate

# Generate genesis configuration for all networks
make genesis-generate
```

### 4. Launch the Network (Docker Recommended)

```bash
# Launch the full network stack in a detached mode
make launch-docker
```

### 5. Verify Network Status

```bash
# Check the C-Chain block number
curl -X POST -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"eth_blockNumber","params":[]}' \
  http://localhost:9630/ext/bc/C/rpc
```

## Documentation Structure

This project's documentation is organized to serve different needs.

- **[1. Concepts](./1_concepts.md)**: Understand the "what and why" of the Lux Network architecture and genesis process.
- **[2. Getting Started](./2_getting_started.md)**: A hands-on tutorial for setting up a local development network.
- **[3. Genesis CLI Reference](./3_genesis_cli_reference.md)**: The definitive command reference for the `genesis` tool.
- **[4. Migration Guide](./4_migration_guide.md)**: An advanced guide for migrating historical blockchain data.
- **[5. Operator Runbook](./5_operator_runbook.md)**: A guide for running and maintaining a production network.
- **[6. Development](./6_development.md)**: Information for contributors, including how to run tests and use the project as a library.

## License

This project is licensed under the MIT License. See the `LICENSE` file for details.

