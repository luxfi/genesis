# Genesis SDK Examples

This directory contains example code demonstrating how to use the Genesis SDK in your Go applications.

## Examples

### 1. Network Launch (`network_launch.go`)

Demonstrates how to programmatically configure and launch a test network with multiple nodes.

```bash
go run network_launch.go
```

Features:
- Network configuration with custom parameters
- Automatic consensus parameter calculation
- Credential generation for multiple nodes
- Network launching with custom data directories

### 2. Database Inspector (`database_inspect.go`)

Shows how to open and inspect a blockchain database, categorize keys, and extract blockchain information.

```bash
go run database_inspect.go /path/to/chaindata
```

Features:
- Open PebbleDB databases
- Iterate through all keys
- Categorize keys by type
- Find latest block information
- Decode common value types

## Building Examples

```bash
# Build all examples
go build -o network_launch network_launch.go
go build -o database_inspect database_inspect.go

# Run examples
./network_launch
./database_inspect /path/to/chaindata
```

## SDK Import Paths

```go
// Core types and structures
import "github.com/luxfi/genesis/pkg/core"

// Network launching
import "github.com/luxfi/genesis/pkg/launch"

// Credential generation
import "github.com/luxfi/genesis/pkg/credentials"

// Consensus configurations
import "github.com/luxfi/genesis/pkg/consensus"

// Database operations
import "github.com/luxfi/database"
```

## Common Patterns

### Error Handling

Always check errors returned by SDK functions:

```go
if err := network.Validate(); err != nil {
    log.Fatalf("Validation failed: %v", err)
}
```

### Resource Cleanup

Always defer cleanup of resources:

```go
db, err := database.Open(path, 0, 0, "pebbledb", true)
if err != nil {
    return err
}
defer db.Close()
```

### Configuration Validation

Validate configurations before use:

```go
network := &core.Network{...}
if err := network.Validate(); err != nil {
    return err
}
network.Normalize() // Apply defaults
```