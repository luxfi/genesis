# LLM Documentation - Lux Genesis Tool

## Overview

This document provides comprehensive technical documentation for the Lux Genesis tool, a unified CLI for managing genesis configurations, blockchain operations, and data migrations in the Lux ecosystem.

## Recent Architecture Refactoring (August 2025)

### Command Consolidation
The project has undergone significant refactoring to consolidate ~40+ individual command files into a cleaner modular structure:

**Before**: Scattered scripts and individual command files
- `cmd/check_balance.go`, `cmd/check_both_balances.go`, `cmd/check_db_keys.go`
- `cmd/verify_migration.go`, `cmd/migrate_simple/`, `cmd/migrate_full/`
- ~90 scripts in `scripts/` directory

**After**: Organized command hierarchy with subcommands
- `cmd/check.go` - Parent command for all verification operations
  - `check balance` - Check account balances directly from database
  - `check migration` - Verify migration integrity (future)
  - `check db` - Database integrity checks (future)
- `cmd/db.go` - Database inspection and management
  - `db scan` - Scan database keys with optional filtering
  - `db inspect` - Deep inspection (future)
- `cmd/migrate_cmd.go` - Consolidated migration operations

### Package Structure
Core functionality extracted into reusable packages:
- `pkg/balance/checker.go` - Balance checking functionality
- `pkg/db/inspector.go` - Database inspection utilities
- `pkg/migration/migrator.go` - Migration logic
- `pkg/extract/` - Data extraction tools
- `pkg/genesis/` - Genesis file generation

### Script Organization
Utility scripts moved to `tools/scripts/` for reference while keeping the main command structure clean.

## Command Structure

### Usage
```bash
# Build the unified CLI
go build -o bin/genesis .

# Check balance
genesis check balance --db-path /path/to/db --address 0x...

# Scan database
genesis db scan --db-path /path/to/db --prefix 0x61 --limit 100

# Run migration
genesis migrate --source /path/to/pebbledb --target /path/to/badgerdb
```

## Migration Architecture

### Source: SubnetEVM (PebbleDB)

**Database Location**: `/Users/z/work/lux/state/chaindata/lux-mainnet-96369/db/pebbledb`

**Key Structure**: Plain namespaced keys (NOT hashed)
- Namespace: `337fb73f9bcdac8c31a2d5f7b877ab1e8a2b7f2a1e9bf02a0a0e6c6fd164f1d1` (32 bytes)

**Key Patterns**:
```
Headers:    namespace + 'h' + be8(blockNumber) + blockHash
Bodies:     namespace + 'b' + be8(blockNumber) + blockHash  
Receipts:   namespace + 'r' + be8(blockNumber) + blockHash
H Mapping:  namespace + 'H' + blockHash → be8(blockNumber)
Tip Hash:   namespace + "AcceptorTipKey" → 32-byte hash
Tip Height: namespace + "AcceptorTipHeightKey" → be8(height)
```

**Header Format**: 17 fields (pre-Cancun)
```go
type SubnetEVMHeader struct {
    ParentHash    common.Hash    // Field 0
    UncleHash     common.Hash    // Field 1
    Coinbase      common.Address // Field 2
    Root          common.Hash    // Field 3: State root
    TxHash        common.Hash    // Field 4
    ReceiptHash   common.Hash    // Field 5
    Bloom         types.Bloom    // Field 6: 256 bytes
    Difficulty    *big.Int       // Field 7: Always 1
    Number        *big.Int       // Field 8
    GasLimit      uint64         // Field 9
    GasUsed       uint64         // Field 10
    Time          uint64         // Field 11
    Extra         []byte         // Field 12
    MixDigest     common.Hash    // Field 13
    Nonce         types.BlockNonce // Field 14
    BaseFee       *big.Int       // Field 15: EIP-1559
    ExtDataHash   common.Hash    // Field 16: SubnetEVM specific
}
```

### Target: Coreth (BadgerDB)

**Database Location**: `~/.luxd/network-96369/chains/X6CU5qgMJfzsTB9UWxj2ZY5hd68x41HfZ4m4hCBWbHuj1Ebc3/ethdb`

**Key Structure**: Simple keys (no namespace)
```
Headers:    'h' + be8(blockNumber) + blockHash
Bodies:     'b' + be8(blockNumber) + blockHash
Receipts:   'r' + be8(blockNumber) + blockHash
TD:         't' + be8(blockNumber) + blockHash
Canonical:  'h' + be8(blockNumber) + 'n' → blockHash
Number:     'H' + blockHash → RLP(blockNumber)
```

**VM Metadata**: Separate database at `vm/`
```
lastAccepted:       32-byte hash
lastAcceptedHeight: 8-byte BE number
initialized:        1 byte (0x01)
```

## Migration Algorithm

### 1. Read Tip Information
```go
// Read tip hash from AcceptorTipKey
tipHash := readFromPebble(namespace + "AcceptorTipKey")

// Get height from H mapping
tipHeight := readHashToNumber(namespace + 'H' + tipHash)
```

### 2. Walk Chain Backwards
```go
for n := tipHeight; ; n-- {
    // Read from SubnetEVM
    headerRLP := read(namespace + 'h' + be8(n) + hash)
    bodyRLP := read(namespace + 'b' + be8(n) + hash)
    receiptsRLP := read(namespace + 'r' + be8(n) + hash)
    
    // Extract parent without full decode
    parent := extractParentHash(headerRLP)
    
    // Write to Coreth (using batch for efficiency)
    batch.Put('h' + be8(n) + hash, headerRLP)
    batch.Put('b' + be8(n) + hash, bodyRLP)
    batch.Put('r' + be8(n) + hash, receiptsRLP)
    
    // Write mappings
    WriteCanonicalHash(hash, n)
    WriteHeaderNumber(hash, n)
    WriteTd(hash, n, n+1)
    
    if n == 0 {
        genesisHash = hash
        break
    }
    hash = parent
}
```

### 3. Write Metadata
```go
// Head pointers in ethdb/
WriteHeadHeaderHash(tipHash)
WriteHeadBlockHash(tipHash)
WriteHeadFastBlockHash(tipHash)

// VM metadata in vm/
vmDB.Put("lastAccepted", tipHash)
vmDB.Put("lastAcceptedHeight", be8(tipHeight))
vmDB.Put("initialized", []byte{1})

// Chain config under genesis
WriteChainConfig(genesisHash, chainConfig)
```

## Key Functions

### Big-Endian Encoding
```go
func be8(n uint64) []byte {
    var b [8]byte
    binary.BigEndian.PutUint64(b[:], n)
    return b[:]
}
```

### Plain Key Construction
```go
func plainKey(ns []byte, prefix byte, n uint64, h common.Hash) []byte {
    key := make([]byte, 0, 73)
    key = append(key, ns...)
    key = append(key, prefix)
    key = append(key, be8(n)...)
    key = append(key, h.Bytes()...)
    return key
}
```

### Parent Hash Extraction (without full decode)
```go
func extractParentHash(headerRLP []byte) (common.Hash, error) {
    s := rlp.NewStream(bytes.NewReader(headerRLP), 0)
    if _, err := s.List(); err != nil {
        return common.Hash{}, err
    }
    var parent common.Hash
    if err := s.Decode(&parent); err != nil {
        return common.Hash{}, err
    }
    return parent, nil
}
```

## Critical Discoveries

### 1. Key Format Not Hashed
Initial assumption was that SubnetEVM used hashed keys like:
```
namespace + keccak256('h' + blockNumber + hash)
```

**Reality**: Keys are plain:
```
namespace + 'h' + blockNumber + hash
```

### 2. Header Structure Mismatch
SubnetEVM headers have 17 fields, Coreth expects Cancun fields. Solution: Copy raw RLP without decoding.

### 3. Database Paths
Must write to exact paths that luxd opens:
- ethdb: `~/.luxd/network-96369/chains/*/ethdb`
- vm: `~/.luxd/network-96369/chains/*/vm`

## Performance Optimizations

### BadgerDB Batch Writes
```go
batch := db.NewBatch()
defer batch.Reset()

const batchSize = 1000
for i := 0; i < totalBlocks; i++ {
    batch.Put(key, value)
    if i % batchSize == 0 {
        batch.Write()
        batch.Reset()
    }
}
batch.Write() // Final flush
```

### Single Writer Pattern
BadgerDB requires single writer for transactions. Multiple readers are fine.

## Migration Statistics

- **Source**: 1,082,781 blocks (7.1GB PebbleDB)
- **Target**: ~8GB BadgerDB
- **Time**: 75 seconds
- **Rate**: 14,430 blocks/second
- **Memory**: ~2GB peak

## Verification Checklist

### Pre-boot Invariants
1. ✓ ReadHeadHeaderHash == tipHash
2. ✓ ReadHeaderNumber(tipHash) == tipHeight  
3. ✓ Header at tip exists
4. ✓ ReadCanonicalHash(tipHeight) == tipHash
5. ✓ ReadCanonicalHash(0) == genesisHash
6. ✓ Body at tip exists (or empty)
7. ✓ VM lastAccepted == tipHash
8. ✓ VM lastAcceptedHeight == tipHeight
9. ✓ VM initialized == true

### Post-boot RPC Checks
```bash
# Block number
curl -X POST -H "Content-Type: application/json" \
  --data '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' \
  http://localhost:9650/ext/bc/C/rpc

# Balance check
curl -X POST -H "Content-Type: application/json" \
  --data '{"jsonrpc":"2.0","method":"eth_getBalance","params":["0x9011E888251AB053B7bD1cdB598Db4f9DEd94714","0x10859c"],"id":1}' \
  http://localhost:9650/ext/bc/C/rpc
```

## Node Launch Commands

### Solo/Development
```bash
luxd \
  --network-id=96369 \
  --http-host=0.0.0.0 \
  --http-port=9650 \
  --consensus-sample-size=1 \
  --consensus-quorum-size=1
```

### Production (2+ validators)
```bash
luxd \
  --network-id=96369 \
  --http-host=0.0.0.0 \
  --http-port=9650 \
  --bootstrap-ips=<peer-ips> \
  --bootstrap-ids=<peer-ids>
```

## Common Issues and Solutions

### Issue: "Genesis mismatch"
**Cause**: ChainConfig written under wrong hash
**Solution**: Write config under genesis hash (block 0), not tip

### Issue: "Missing header" 
**Cause**: Wrong key format or namespace
**Solution**: Verify namespace is `337fb73f...` and keys are plain

### Issue: "RLP decode error"
**Cause**: Header structure mismatch (17 vs 21 fields)
**Solution**: Copy raw RLP without decoding

### Issue: "Database locked"
**Cause**: Multiple processes accessing PebbleDB
**Solution**: Ensure only one reader process

### Issue: "Node not bootstrapping"
**Cause**: Missing VM metadata or wrong paths
**Solution**: Verify vm/ directory has all 3 keys

## References

- SubnetEVM: https://github.com/ava-labs/subnet-evm
- Coreth: https://github.com/ava-labs/coreth  
- Lux Node: https://github.com/luxfi/node
- BadgerDB: https://github.com/dgraph-io/badger
- PebbleDB: https://github.com/cockroachdb/pebble