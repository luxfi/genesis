#!/bin/bash
# Launch mainnet with genesis database replay
# This script runs from the genesis directory and uses relative paths

set -e

# Base directories (relative to genesis repo)
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
NODE_DIR="${SCRIPT_DIR}/../node"
GENESIS_DB="${SCRIPT_DIR}/state/chaindata/lux-mainnet-96369"
RUN_DIR="${SCRIPT_DIR}/runs/mainnet-replay"

# Configuration
NETWORK_ID="96369"  # Mainnet
HTTP_PORT="9630"
STAKING_PORT="9631"

echo "=== Lux Mainnet Replay Launcher ==="
echo "Working from: ${SCRIPT_DIR}"
echo "Genesis DB: ${GENESIS_DB}"
echo "Run directory: ${RUN_DIR}"
echo ""

# Check if genesis database exists
if [ ! -d "$GENESIS_DB" ]; then
    echo "❌ Genesis database not found at $GENESIS_DB"
    echo "Please ensure state/chaindata exists with the blockchain data to replay"
    exit 1
fi

# Clean and create run directory
rm -rf "$RUN_DIR"
mkdir -p "$RUN_DIR/staking"

# Step 1: Generate staking keys
echo "Step 1: Generating staking keys with BLS..."
cd "$SCRIPT_DIR"
./bin/genesis staking keygen --output "$RUN_DIR/staking"

# Extract node ID
NODE_ID=$(cat "$RUN_DIR/staking/genesis-staker.json" | jq -r '.nodeID')
echo "Generated NodeID: $NODE_ID"

# Step 2: Create chain configurations
echo ""
echo "Step 2: Creating chain configurations..."
mkdir -p "$RUN_DIR/configs/chains/C"

cat > "$RUN_DIR/configs/chains/C/config.json" <<EOF
{
  "db-type": "badgerdb",
  "log-level": "info",
  "state-sync-enabled": false,
  "offline-pruning-enabled": false,
  "allow-unprotected-txs": true
}
EOF

# Step 3: Launch luxd with replay
echo ""
echo "Step 3: Starting luxd with genesis replay..."
echo "Configuration:"
echo "  - Network ID: $NETWORK_ID (mainnet)"
echo "  - NodeID: $NODE_ID"
echo "  - HTTP Port: $HTTP_PORT"
echo "  - Staking Port: $STAKING_PORT"
echo "  - Genesis DB: PebbleDB (replay from $GENESIS_DB)"
echo "  - Runtime DB: BadgerDB (all chains)"
echo "  - Consensus: Single node (k=1)"
echo ""

cd "$NODE_DIR"

# Ensure luxd is built
if [ ! -f "./build/luxd" ]; then
    echo "Building luxd..."
    make build
fi

# Launch with genesis database replay
# NOTE: We don't specify --genesis-file when using --genesis-db
exec ./build/luxd \
    --network-id="$NETWORK_ID" \
    --data-dir="$RUN_DIR/data" \
    --db-type=badgerdb \
    --chain-config-dir="$RUN_DIR/configs/chains" \
    --staking-tls-cert-file="$RUN_DIR/staking/staker.crt" \
    --staking-tls-key-file="$RUN_DIR/staking/staker.key" \
    --staking-signer-key-file="$RUN_DIR/staking/signer.key" \
    --genesis-db="$GENESIS_DB" \
    --genesis-db-type=pebbledb \
    --c-chain-db-type=badgerdb \
    --p-chain-db-type=badgerdb \
    --x-chain-db-type=badgerdb \
    --http-host=0.0.0.0 \
    --http-port="$HTTP_PORT" \
    --staking-port="$STAKING_PORT" \
    --log-level=info \
    --api-admin-enabled=true \
    --sybil-protection-enabled=true \
    --consensus-sample-size=1 \
    --consensus-quorum-size=1 \
    --consensus-commit-threshold=1 \
    --consensus-concurrent-repolls=1 \
    --consensus-optimal-processing=1 \
    --consensus-max-processing=1 \
    --consensus-max-time-processing=2s \
    --bootstrap-beacon-connection-timeout=10s \
    --health-check-frequency=2s \
    --network-max-reconnect-delay=1s