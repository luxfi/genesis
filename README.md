# Genesis - Lux Blockchain Configuration Tool

[![CI](https://github.com/luxfi/genesis/actions/workflows/ci.yml/badge.svg)](https://github.com/luxfi/genesis/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/luxfi/genesis)](https://goreportcard.com/report/github.com/luxfi/genesis)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)

Genesis is a unified CLI tool for managing genesis configurations and launching networks in the Lux blockchain ecosystem. It provides comprehensive support for P-Chain, C-Chain, and X-Chain genesis generation, blockchain data migration, and network bootstrapping.

## Features

- **Multi-Chain Genesis Generation**: Create genesis files for P-Chain, C-Chain, and X-Chain
- **Blockchain Data Migration**: Import existing blockchain data from SubnetEVM to C-Chain
- **State Extraction**: Extract and analyze blockchain state from PebbleDB databases
- **Network Bootstrapping**: Launch Lux nodes with custom genesis configurations
- **Database Tools**: Inspect and manipulate blockchain databases
- **L2 Management**: Create and manage L2 networks

## Installation

### From Source

```bash
# Clone the repository
git clone https://github.com/luxfi/genesis.git
cd genesis

# Build the binary
go build -o genesis ./cmd/genesis

# Install to PATH
sudo mv genesis /usr/local/bin/
```

### From Release

Download the latest release for your platform from the [releases page](https://github.com/luxfi/genesis/releases).

```bash
# Linux
wget https://github.com/luxfi/genesis/releases/latest/download/genesis-linux-amd64
chmod +x genesis-linux-amd64
sudo mv genesis-linux-amd64 /usr/local/bin/genesis

# macOS
wget https://github.com/luxfi/genesis/releases/latest/download/genesis-darwin-amd64
chmod +x genesis-darwin-amd64
sudo mv genesis-darwin-amd64 /usr/local/bin/genesis
```

## Quick Start

### Generate Mainnet Genesis Files

The genesis tool automatically generates all three required chain genesis files (P-Chain, C-Chain, X-Chain) for Lux mainnet. These files are essential for bootstrapping a working network.

```bash
# Using make
make lux-mainnet

# Or using the binary directly
genesis generate --network mainnet --chain-id 96369

# Output:
# configs/lux-mainnet-96369/P/genesis.json
# configs/lux-mainnet-96369/C/genesis.json
# configs/lux-mainnet-96369/X/genesis.json
```

### Replay Blockchain Data

Import existing SubnetEVM blockchain data into C-Chain format:

```bash
# Replay blocks from a SubnetEVM database
genesis replay /path/to/subnet/pebbledb

# With specific options
genesis replay /path/to/subnet/pebbledb \
  --start-block 1000 \
  --end-block 5000 \
  --output /path/to/output
```

### Extract Blockchain State

Extract state from an existing blockchain:

```bash
# Extract full state
genesis extract state /path/to/chaindata /output/path --state

# Extract specific block range
genesis extract blockchain /path/to/chaindata \
  --start-block 1000 \
  --end-block 2000
```

### Inspect Database

Inspect blockchain database contents:

```bash
# Show chain tip
genesis inspect tip /path/to/chaindata

# Inspect specific blocks
genesis inspect blocks /path/to/chaindata --start 100 --limit 10

# Debug database keys
genesis debug-keys /path/to/chaindata
```

## Command Reference

### Core Commands

#### `genesis version`
Display version information.

```bash
genesis version
# Output: Genesis CLI v1.0.0
```

#### `genesis replay`
Replay blockchain blocks from SubnetEVM format.

```bash
genesis replay <source-db> [flags]

Flags:
  --start-block uint    Starting block number (default: 0)
  --end-block uint      Ending block number (default: latest)
  --output string       Output directory
  --batch-size int      Batch size for processing (default: 1000)
```

#### `genesis extract`
Extract data from blockchain databases.

```bash
# Extract blockchain data
genesis extract blockchain <source> [flags]

# Extract genesis state
genesis extract genesis <source> [flags]

# Extract state at specific height
genesis extract state <source> <output> [flags]
  --network uint        Network ID
  --state              Include state data
  --height uint        Block height for state extraction
```

#### `genesis inspect`
Inspect blockchain database contents.

```bash
# Show chain tip
genesis inspect tip <chaindata-path>

# Inspect blocks
genesis inspect blocks <chaindata-path> [flags]
  --start uint         Start block number
  --limit int          Number of blocks to show

# Inspect database keys
genesis inspect keys <chaindata-path> [flags]
  --prefix string      Key prefix to filter
```

#### `genesis database`
Database management commands.

```bash
# List database contents
genesis database list <path>

# Compact database
genesis database compact <source> <destination>

# Verify database integrity
genesis database verify <path>
```

#### `genesis l2`
L2 network management.

```bash
# Create new L2 network
genesis l2 create <name> [flags]
  --chain-id uint      Chain ID for the L2
  --base-chain string  Base chain (default: "lux")

# List L2 networks
genesis l2 list

# Get L2 network info
genesis l2 info <name>
```

### Advanced Commands

#### `genesis compact-ancient`
Compact ancient blockchain data.

```bash
genesis compact-ancient <source> <destination> [flags]
  --batch-size int     Batch size for compaction
  --compress           Enable compression
```

#### `genesis convert`
Convert between blockchain formats.

```bash
# Convert to Coreth format
genesis convert coreth <source> <destination>

# Convert to L2 format
genesis convert l2 <source> <destination>
```

#### `genesis import-blockchain`
Import blockchain data into node.

```bash
genesis import-blockchain <source> [flags]
  --node-path string   Path to node data directory
  --chain string       Chain to import into (C, X, or P)
```

#### `genesis setup-chain-state`
Set up chain state for node initialization.

```bash
genesis setup-chain-state <genesis-file> [flags]
  --data-dir string    Node data directory
  --chain-id uint      Chain ID
```

## Go SDK Usage

### Installation

```go
import "github.com/luxfi/genesis/pkg/core"
import "github.com/luxfi/genesis/pkg/launch"
import "github.com/luxfi/genesis/pkg/credentials"
```

### Basic Usage

```go
package main

import (
    "log"
    "github.com/luxfi/genesis/pkg/core"
    "github.com/luxfi/genesis/pkg/launch"
)

func main() {
    // Create network configuration
    network := &core.Network{
        Name:      "mynetwork",
        NetworkID: 12345,
        ChainID:   12345,
        Nodes:     5,
        Genesis: core.GenesisConfig{
            Source: "fresh",
            Allocations: map[string]uint64{
                "0x1234567890123456789012345678901234567890": 1000000000000000000,
            },
        },
    }

    // Validate configuration
    if err := network.Validate(); err != nil {
        log.Fatal(err)
    }

    // Apply defaults
    network.Normalize()

    // Launch network
    launcher := launch.NewLauncher()
    if err := launcher.LaunchNetwork(network); err != nil {
        log.Fatal(err)
    }
}
```

### Working with Credentials

```go
package main

import (
    "log"
    "github.com/luxfi/genesis/pkg/credentials"
)

func main() {
    // Generate new credentials
    gen := credentials.NewGenerator()
    creds, err := gen.Generate()
    if err != nil {
        log.Fatal(err)
    }

    // Save credentials
    if err := gen.Save(creds, "./staking"); err != nil {
        log.Fatal(err)
    }

    log.Printf("Generated NodeID: %s", creds.NodeID)
}
```

### Database Operations

```go
package main

import (
    "log"
    "github.com/luxfi/database"
    "github.com/luxfi/genesis/pkg/ancient"
)

func main() {
    // Open database
    db, err := database.Open("/path/to/chaindata", 0, 0)
    if err != nil {
        log.Fatal(err)
    }
    defer db.Close()

    // Iterate through keys
    it := db.NewIterator()
    defer it.Release()
    
    for it.Next() {
        key := it.Key()
        value := it.Value()
        log.Printf("Key: %x, Value: %x", key, value)
    }
}
```

## Architecture

Genesis is built with a modular architecture:

- **cmd/genesis**: CLI commands and entry points
- **pkg/core**: Core data structures and interfaces
- **pkg/launch**: Network launching functionality
- **pkg/credentials**: Credential generation and management
- **pkg/consensus**: Consensus parameter configurations
- **pkg/ancient**: Ancient data format handling

## Development

### Requirements

- Go 1.24.5 or higher
- Git

### Building from Source

```bash
# Clone repository
git clone https://github.com/luxfi/genesis.git
cd genesis

# Download dependencies
go mod download

# Run tests
go test -v ./...

# Build binary
go build -o genesis ./cmd/genesis
```

### Running Tests

```bash
# Run all tests
go test -v ./...

# Run tests with coverage
go test -v -coverprofile=coverage.txt ./...

# Run specific package tests
go test -v ./pkg/core

# Run with race detection
go test -v -race ./...
```

### Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## Configuration

Genesis uses a YAML configuration file for complex operations. Create `genesis.yaml`:

```yaml
networks:
  mainnet:
    network_id: 96369
    chain_id: 96369
    consensus:
      k: 20
      alpha: 15
      beta: 20
    
  testnet:
    network_id: 96368
    chain_id: 96368
    consensus:
      k: 1
      alpha: 1
      beta: 1

defaults:
  output_dir: "./output"
  data_dir: "./data"
```

## Make Targets

```bash
# Build and install
make build          # Build the genesis CLI
make install        # Install to /usr/local/bin
make clean          # Clean build artifacts

# Quick network generation
make lux-mainnet    # Generate Lux mainnet genesis
make lux-testnet    # Generate Lux testnet genesis
make zoo-mainnet    # Generate Zoo L2 genesis
make spc-mainnet    # Generate SPC L2 genesis

# Development
make test           # Run all tests
make fmt            # Format code
make lint           # Run linters

# Pipeline operations
make pipeline       # Run full genesis pipeline
make launch         # Launch node with migrated data
```

## License

This project is licensed under the Apache License 2.0 - see the [LICENSE](LICENSE) file for details.

## Support

- Documentation: [https://docs.lux.network](https://docs.lux.network)
- Issues: [GitHub Issues](https://github.com/luxfi/genesis/issues)
- Discord: [Lux Community](https://discord.gg/lux)