# Genesis Configuration Tool

A lightweight CLI tool for managing genesis configurations and blockchain data migration for L1, L2, L3, and Quantum chains in the Lux ecosystem.

## Overview

This tool provides a clean interface for:
- Migrating SubnetEVM blockchain data to C-Chain format
- Generating genesis configurations for different chain types  
- Managing validators and allocations
- Launching chains with migrated blockchain data
- Validating genesis configurations

## Quick Start

```bash
# Run full pipeline and launch node
make launch
```

This single command will:
1. Build the genesis tools
2. Extract blockchain data from state repository
3. Migrate SubnetEVM data to C-Chain format
4. Launch the node with 1,082,781 blocks

### Individual Commands

```bash
make build          # Build tools only
make pipeline-lux   # Run data pipeline (extract + migrate)
make node           # Launch node with existing data
```

## Documentation

- [Blockchain Migration Guide](BLOCKCHAIN_MIGRATION.md) - Complete guide for migrating SubnetEVM data to C-Chain
- [Storage Structure](BLOCKCHAIN_MIGRATION.md#storage-structure) - How blockchain data is organized

## Installation

```bash
# Build the tool
make build

# Install to system path
make install
```

## Quick Start

### Generate Genesis Configurations

```bash
# Generate L1 genesis (Lux mainnet)
make lux-mainnet

# Generate L2 genesis (Zoo on Lux)
make zoo-mainnet

# Generate L3 app chain genesis
./bin/genesis generate --type l3 --network myapp --base-chain zoo

# Generate Quantum chain genesis
make quantum-genesis
```

### Access Historic Chaindata

```bash
# Clone chaindata from state repo (on-demand)
make clone-state

# For local development with SSH
STATE_REPO=git@github.com:luxfi/state.git make clone-state

# Update existing chaindata
make update-state
```

### Launch Chains

```bash
# Launch L1 network
make launch-l1

# Launch L2 on Lux
make launch-l2

# Launch L3 app chain
make launch-l3

# Launch Quantum chain
make quantum-launch
```

## Chain Types

### L1 - Sovereign Chains
Independent blockchain networks with their own validators and consensus.

```bash
./bin/genesis generate --type l1 --network lux-mainnet --chain-id 96369
```

### L2 - Based Rollups
Layer 2 solutions built on top of L1 chains (e.g., Zoo on Lux).

```bash
./bin/genesis generate --type l2 --network zoo-mainnet --chain-id 200200 --base-chain lux
```

### L3 - App Chains
Application-specific chains built on L2s for specialized use cases.

```bash
./bin/genesis generate --type l3 --network myapp --base-chain zoo
```

### Quantum Chains
Next-generation chains with quantum-resistant cryptography and enhanced consensus.

```bash
./bin/genesis generate --type quantum --network quantum-mainnet
```

## Directory Structure

```
genesis-new/
├── cmd/genesis/        # CLI tool source
├── configs/           # Generated genesis configurations
├── pkg/              # Shared packages
│   ├── config/       # Configuration utilities
│   ├── validator/    # Validator management
│   ├── allocation/   # Token allocation logic
│   └── cchain/       # C-Chain specific utilities
├── scripts/          # Helper scripts
├── state/            # Cloned chaindata (on-demand, git-ignored)
└── Makefile          # Build and management commands
```

## Configuration

### Genesis Configuration Format

```json
{
  "chainId": 96369,
  "type": "l1",
  "network": "lux-mainnet",
  "timestamp": 1699564800,
  "gasLimit": 30000000,
  "difficulty": "0x1",
  "alloc": {
    "0x1000...": {
      "balance": "1000000000000000000000000000"
    }
  },
  "validators": [
    {
      "address": "0x1234...",
      "weight": 100
    }
  ]
}
```

### L2 Configuration

```json
{
  "l2Config": {
    "baseChain": "lux",
    "sequencerUrl": "https://sequencer.zoo.network",
    "batcherAddress": "0x4567...",
    "rollupAddress": "0x5678..."
  }
}
```

### Quantum Configuration

```json
{
  "quantumConfig": {
    "quantumProof": "0xQUANTUM_PROOF",
    "entanglementKey": "0xENTANGLEMENT_KEY",
    "consensusMode": "quantum-byzantine"
  }
}
```

## Commands

### CLI Commands

```bash
# Generate genesis configuration
genesis generate --type l1 --network mainnet --chain-id 96369

# Validate genesis file
genesis validate ./configs/l1-mainnet-genesis.json

# Launch chain with genesis
genesis launch --type l1 --genesis ./configs/l1-mainnet-genesis.json

# Show version
genesis version
```

### Makefile Targets

```bash
# Build targets
make build          # Build the genesis CLI
make install        # Install to /usr/local/bin
make clean          # Clean build artifacts
make deep-clean     # Clean everything including state

# State management
make clone-state    # Clone historic chaindata
make update-state   # Update cloned state

# Genesis generation
make gen-l1         # Generate L1 genesis
make gen-l2         # Generate L2 genesis
make gen-l3         # Generate L3 genesis

# Quick configs
make lux-mainnet    # Lux mainnet genesis
make lux-testnet    # Lux testnet genesis
make zoo-mainnet    # Zoo mainnet genesis
make zoo-testnet    # Zoo testnet genesis

# Launch commands
make launch-l1      # Launch L1 network
make launch-l2      # Launch L2 on Lux
make launch-l3      # Launch L3 app chain

# Quantum chain
make quantum-genesis # Generate quantum genesis
make quantum-launch  # Launch quantum chain
```

## Default Allocations

The tool automatically includes standard allocations:

- **Treasury**: 1,000,000,000 tokens at `0x1000...0000`
- **Development Fund**: 500,000,000 tokens at `0x2000...0000`
- **Ecosystem Fund**: 300,000,000 tokens at `0x3000...0000`

## Integration

### With Lux Node

```bash
# Generate genesis
./bin/genesis generate --type l1 --network mainnet

# Use with lux node
lux node run --genesis ./configs/l1-mainnet-genesis.json
```

### With Docker

```bash
# Build image with genesis
docker build -t lux-genesis .

# Run with custom genesis
docker run -v $(pwd)/configs:/genesis lux-genesis
```

## Development

### Running Tests

```bash
make test
```

### Adding New Chain Types

1. Add type to `cmd/genesis/main.go`
2. Implement configuration function
3. Add launch logic
4. Update validation rules

### Contributing

1. Keep the repository lightweight
2. Don't commit chaindata or large files
3. Use the state repository for historic data
4. Follow existing patterns for new features

## License

MIT License - see LICENSE file for details