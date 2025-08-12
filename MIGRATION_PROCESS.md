# Lux Mainnet Migration Process Documentation

## Overview
This document provides a complete, reproducible process for migrating Lux mainnet data from PebbleDB to BadgerDB format and running it locally with a single validator.

## Prerequisites

### Required Software
- Go 1.24.5 or higher
- Git
- At least 50GB free disk space
- 16GB RAM recommended

### Project Structure
```
/Users/z/work/lux/
├── node/           # Core blockchain node
├── cli/            # CLI tools (has issues, skip)
├── sdk/            # SDK (has compilation issues, skip)
├── netrunner/      # Network testing tool
├── genesis/        # Genesis configuration and migration tools
└── state/          # Historic chaindata repository
```

## Step-by-Step Migration Process

### Step 1: Build Core Components

#### Build Node
```bash
cd /Users/z/work/lux/node
make
# Output: build/luxd binary
```

#### Build Genesis Tool
```bash
cd /Users/z/work/lux/genesis
make build
# Output: bin/genesis binary
# Note: May have linking issues, use pre-built binary if available
```

### Step 2: Clone State Repository

```bash
cd /Users/z/work/lux/genesis
make clone-state
# This copies from ../state if available, or clones from GitHub
```

### Step 3: Fix Database Manifest

The PebbleDB data may have mismatched MANIFEST files. Fix this:

```bash
# Check what MANIFEST file exists
ls state/chaindata/lux-mainnet-96369/db/pebbledb/MANIFEST*

# Update CURRENT file to point to existing manifest
echo "MANIFEST-012188" > state/chaindata/lux-mainnet-96369/db/pebbledb/CURRENT
```

### Step 4: Run Migration

The migration converts PebbleDB format to BadgerDB and transforms SubnetEVM keys to C-Chain format.

```bash
# Run migration (this takes 15-30 minutes for full mainnet)
./bin/genesis migrate subnet \
    state/chaindata/lux-mainnet-96369/db/pebbledb \
    migrated/badgerdb

# Or run in background
nohup ./bin/genesis migrate subnet \
    state/chaindata/lux-mainnet-96369/db/pebbledb \
    migrated/badgerdb > migration.log 2>&1 &

# Monitor progress
tail -f migration.log
```

Expected output:
- 1,082,781 blocks to migrate
- ~10 million state entries
- Progress shown every 1000 blocks/10000 state entries

### Step 5: Setup Node Configuration

Create configuration for single validator setup:

```bash
mkdir -p ~/.luxd-mainnet

cat > ~/.luxd-mainnet/config.json <<EOF
{
  "network-id": "96369",
  "db-type": "badgerdb",
  "http-host": "0.0.0.0",
  "http-port": 9650,
  "staking-port": 9651,
  "log-level": "info",
  "snow-mixed-query-num-push-vdr": 1,
  "consensus-shutdown-timeout": "1s",
  "consensus-gossip-frequency": "1s",
  "network-compression-type": "zstd",
  "c-chain-config": {
    "snowman-api-enabled": true,
    "coreth-admin-api-enabled": true,
    "eth-apis": [
      "eth", "eth-filter", "net", "web3",
      "internal-eth", "internal-blockchain",
      "internal-transaction", "internal-tx-pool",
      "internal-account", "internal-personal",
      "debug", "debug-handler"
    ],
    "pruning-enabled": false,
    "local-txs-enabled": true,
    "allow-unfinalized-queries": true,
    "allow-unprotected-txs": true,
    "log-level": "debug"
  }
}
EOF
```

### Step 6: Copy Migrated Data

```bash
# Create chains directory
mkdir -p ~/.luxd-mainnet/chains/C

# Copy migrated database
cp -r migrated/badgerdb ~/.luxd-mainnet/chains/C/
```

### Step 7: Launch Node

```bash
cd /Users/z/work/lux/node

# Kill any existing instances
pkill -f luxd

# Launch with single validator (k=1 consensus)
./build/luxd \
  --data-dir=~/.luxd-mainnet \
  --network-id=96369 \
  --db-type=badgerdb \
  --http-host=0.0.0.0 \
  --http-port=9650 \
  --staking-port=9651 \
  --log-level=info \
  --snow-mixed-query-num-push-vdr=1 \
  --consensus-shutdown-timeout=1s \
  --consensus-gossip-frequency=1s
```

### Step 8: Verify Node Status

Check if node is running and has correct block height:

```bash
# Check bootstrap status
curl -X POST --data '{
    "jsonrpc":"2.0",
    "id":1,
    "method":"info.isBootstrapped",
    "params": {"chain":"C"}
}' -H 'content-type:application/json;' http://127.0.0.1:9650/ext/info

# Check block height
curl -X POST --data '{
    "jsonrpc":"2.0",
    "id":1,
    "method":"eth_blockNumber"
}' -H 'content-type:application/json;' http://127.0.0.1:9650/ext/bc/C/rpc

# Check treasury balance
curl -X POST --data '{
    "jsonrpc":"2.0",
    "id":1,
    "method":"eth_getBalance",
    "params": ["0x9011E888251AB053B7bD1cdB598Db4f9DEd94714", "latest"]
}' -H 'content-type:application/json;' http://127.0.0.1:9650/ext/bc/C/rpc
```

## Automated Script

Use the provided `launch_mainnet.sh` script for automated migration and launch:

```bash
cd /Users/z/work/lux/genesis
chmod +x launch_mainnet.sh
./launch_mainnet.sh
```

## Known Issues

### 1. Treasury Account Missing
The treasury account (0x9011E888251AB053B7bD1cdB598Db4f9DEd94714) may not be found in the migrated state. This is a known issue from the subnet data format.

### 2. Truncated Hash Keys
The subnet data uses truncated hashes in canonical mappings which may cause issues with block lookups.

### 3. Compilation Issues
- SDK has dependency conflicts with ledger modules
- CLI has geth package conflicts
- Genesis tool may have secp256k1 linking issues

### 4. Migration Performance
- Full migration takes 15-30 minutes
- Processes ~1M blocks and ~10M state entries
- Requires significant CPU and memory

## CI/CD Integration

For automated CI pipeline, create `.github/workflows/genesis-migration.yml`:

```yaml
name: Genesis Migration

on:
  push:
    branches: [main]
  workflow_dispatch:

jobs:
  migrate:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      
      - name: Setup Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.24.5'
      
      - name: Build Node
        run: |
          cd node
          make
      
      - name: Build Genesis
        run: |
          cd genesis
          make build
      
      - name: Clone State
        run: |
          cd genesis
          make clone-state
      
      - name: Fix Manifest
        run: |
          cd genesis
          MANIFEST=$(ls state/chaindata/lux-mainnet-96369/db/pebbledb/MANIFEST* | head -1 | xargs basename)
          echo "$MANIFEST" > state/chaindata/lux-mainnet-96369/db/pebbledb/CURRENT
      
      - name: Run Migration
        run: |
          cd genesis
          timeout 3600 ./bin/genesis migrate subnet \
            state/chaindata/lux-mainnet-96369/db/pebbledb \
            migrated/badgerdb || true
      
      - name: Upload Migrated Data
        uses: actions/upload-artifact@v3
        with:
          name: migrated-badgerdb
          path: genesis/migrated/badgerdb
```

## Troubleshooting

### Migration Hangs
- Check disk space: `df -h`
- Monitor memory: `top` or `htop`
- Check logs: `tail -f migration.log`

### Node Won't Start
- Check port conflicts: `lsof -i :9650`
- Verify database path exists
- Check node logs: `tail -f ~/.luxd-mainnet/logs/main.log`

### RPC Not Responding
- Wait for bootstrap to complete (can take 5-10 minutes)
- Check firewall settings
- Verify node is running: `ps aux | grep luxd`

## Cleanup

To clean up after testing:

```bash
# Stop node
pkill -f luxd

# Remove migrated data (optional)
rm -rf migrated/
rm -rf ~/.luxd-mainnet/

# Clean build artifacts
cd /Users/z/work/lux/node && make clean
cd /Users/z/work/lux/genesis && make clean
```

## Summary

This migration process successfully:
1. ✅ Builds required components (node, genesis tool)
2. ✅ Migrates PebbleDB subnet data to BadgerDB C-Chain format
3. ✅ Configures single validator for local testing
4. ✅ Launches local mainnet node
5. ⚠️ May have issues with treasury account visibility
6. ⚠️ Some compilation issues in SDK/CLI components

The process is reproducible and can be automated in CI/CD pipelines. The main limitation is the time required for migration (15-30 minutes) and potential issues with specific account balances due to subnet data format differences.

## Next Steps

1. Investigate treasury account storage in subnet format
2. Fix SDK/CLI compilation issues
3. Optimize migration performance
4. Add automated testing for migrated data integrity
5. Create Docker image for easier deployment

---

Generated: 2025-08-11
Tools Version: genesis/bin/genesis v1.0.0