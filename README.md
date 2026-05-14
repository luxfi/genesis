# Lux Network Genesis Configurations

This repository contains the canonical genesis configurations for all Lux networks.

## Directory Structure

```
genesis/
├── mainnet/           # LUX Mainnet (Chain ID: 96369)
│   ├── cchain.json    # C-Chain genesis (EVM compatible)
│   ├── pchain.json    # P-Chain genesis (ProtocolVM)
│   ├── xchain.json    # X-Chain genesis (Exchange chain)
│   ├── network.json   # Network configuration
│   └── genesis.json   # Full network genesis
├── testnet/           # LUX Testnet (Chain ID: 96368)
│   ├── cchain.json    # C-Chain genesis (EVM compatible)
│   └── genesis.json   # Full testnet genesis
├── devnet/            # LUX Devnet (Chain ID: 96370) - Fast iteration network
│   └── cchain.json    # C-Chain genesis (EVM compatible)
├── local/             # Local development network (Chain ID: 1337)
│   ├── cchain.json    # C-Chain genesis (EVM compatible)
│   └── genesis.json   # Local network genesis
├── bootstrappers.json # Network bootstrapper nodes
└── checkpoints.json   # Network checkpoints
```

## Mainnet C-Chain Genesis

The mainnet C-Chain was initialized with a single genesis allocation:

| Address | Balance |
|---------|---------|
| `0x9011e888251ab053b7bd1cdb598db4f9ded94714` | 2,000,000,000,000,000,000,000,000,000,000 wei (2T LUX) |

### Key Parameters

- **Chain ID**: 96369
- **Gas Limit**: 30,000,000 (0x1C9C380)
- **Min Base Fee**: 25 gwei
- **Block Rate**: 2 seconds

## Network IDs

| Network | Chain ID | Network ID | Purpose |
|---------|----------|------------|---------|
| LUX Mainnet | 96369 | 96369 | Production network |
| LUX Testnet | 96368 | 96368 | Public test network |
| LUX Devnet | 96370 | 96370 | Fast iteration dev network |
| Local | 1337 | 1337 | Local development |

## Network Configurations

### Mainnet (96369)
- **Block Rate**: 2 seconds
- **Gas Limit**: 30M
- **Min Base Fee**: 25 gwei
- **Genesis Account**: `0x9011...` with 2T LUX

### Testnet (96368)
- **Block Rate**: 2 seconds
- **Gas Limit**: 12M
- **Min Base Fee**: 25 gwei
- **Genesis Account**: `0x9011...` with 2T LUX (same as mainnet)

### Devnet (96370)
- **Block Rate**: 1 second (faster)
- **Gas Limit**: 20M
- **Min Base Fee**: 1 gwei (cheaper)
- **Genesis Account**: `0x9011...` with 2T LUX
- All upgrades enabled at genesis (Shanghai, Cancun, etc.)

### Local (1337)
- **Block Rate**: 1 second
- **Gas Limit**: 15M
- **Min Base Fee**: 1 gwei
- **Genesis Account**: `0x9011...` with 2T LUX
- All upgrades enabled at genesis

## Usage

These genesis files are used by:
- `luxd` node for network initialization
- `coreth` for C-Chain configuration
- Block explorers for genesis block verification
- Migration tools for state reconstruction

## RPC Endpoints

- **Mainnet**: `http://localhost:9630/ext/bc/C/rpc`
- **Testnet**: `http://localhost:9630/ext/bc/C/rpc` (testnet node)
- **Devnet**: `http://localhost:9630/ext/bc/C/rpc` (devnet node)

## Genesis Account

All networks use the same production genesis account:

| Account | Balance | Networks |
|---------|---------|----------|
| `0x9011e888251ab053b7bd1cdb598db4f9ded94714` | 2T LUX | All networks |

## State Data & RLP Exports

Blockchain state exports and RLP-encoded blocks are maintained in a separate repository:

**Repository**: [github.com/luxfi/state](https://github.com/luxfi/state)

### Available Data

| Network | Chain ID | Blocks | RLP Location |
|---------|----------|--------|--------------|
| Lux Mainnet | 96369 | 1,082,780 | `rlp/lux-mainnet/lux-mainnet-96369.rlp` |
| Lux Testnet | 96368 | 219 | `rlp/lux-testnet/lux-testnet-96368.rlp` |
| Zoo Mainnet | 200200 | 799 | `rlp/zoo-mainnet/zoo-mainnet-200200.rlp` |
| Zoo Testnet | 200201 | 85 | `rlp/zoo-testnet/zoo-testnet-200201.rlp` |

### Import Workflow

```bash
# Clone repos
git clone https://github.com/luxfi/genesis
git clone https://github.com/luxfi/state

# Initialize with genesis
geth init --datadir /path/to/db genesis/mainnet/cchain.json

# Import blocks from state repo
geth import --datadir /path/to/db state/rlp/lux-mainnet/lux-mainnet-96369.rlp
```

### Source Data

The state repo also contains:
- `pebbledb/` - Original SubnetEVM PebbleDB databases
- `docs/` - State verification and recovery documentation
- `pkg/` - Go tools for archaeology, bridge, and scanning
