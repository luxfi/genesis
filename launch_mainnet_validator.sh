#!/bin/bash
set -e

echo "============================================"
echo "  LUX MAINNET SINGLE NODE VALIDATOR"
echo "============================================"
echo ""

# Configuration
LUXD_BIN="/Users/z/work/lux/node/build/luxd"
MIGRATED_DB="/tmp/lux-badger-migration"
DATA_DIR="/tmp/lux-mainnet-validator-$$"
HTTP_PORT=9650
STAKING_PORT=9651
NETWORK_ID=96369

# Check if luxd exists
if [ ! -f "$LUXD_BIN" ]; then
    echo "Error: luxd not found at $LUXD_BIN"
    echo "Building luxd..."
    cd /Users/z/work/lux/node && make
fi

# Check if migrated database exists
if [ ! -d "$MIGRATED_DB" ]; then
    echo "Error: Migrated database not found at $MIGRATED_DB"
    echo "Please run migration first"
    exit 1
fi

# Kill any existing luxd
echo "Stopping any existing luxd processes..."
pkill -f luxd 2>/dev/null || true
sleep 2

# Create data directory structure
echo "Setting up data directory: $DATA_DIR"
mkdir -p "$DATA_DIR/chainData"
mkdir -p "$DATA_DIR/db"
mkdir -p "$DATA_DIR/staking"

# Setup chain data for C-chain
CHAIN_ID="2f9gWKiw8VTE29NbiA6kUmETi6Rz8ikk8tUbaHEdhft7X8BvQo"
CHAIN_DIR="$DATA_DIR/chainData/$CHAIN_ID"
mkdir -p "$CHAIN_DIR"

# Link the migrated database
echo "Linking migrated database..."
ln -s "$MIGRATED_DB" "$CHAIN_DIR/ethdb"

# Generate ephemeral staking keys for single node
echo "Setting up ephemeral staking for single node operation..."

echo ""
echo "Configuration:"
echo "  Network ID: $NETWORK_ID (mainnet)"
echo "  Data Dir: $DATA_DIR"
echo "  Chain DB: $MIGRATED_DB"
echo "  HTTP Port: $HTTP_PORT"
echo "  Staking Port: $STAKING_PORT"
echo ""

echo "Starting validator node..."
echo "================================"

# Launch luxd with proper mainnet settings
$LUXD_BIN \
    --network-id=$NETWORK_ID \
    --data-dir="$DATA_DIR" \
    --db-dir="$DATA_DIR/db" \
    --chain-data-dir="$DATA_DIR/chainData" \
    --http-host=0.0.0.0 \
    --http-port=$HTTP_PORT \
    --staking-port=$STAKING_PORT \
    --staking-ephemeral-cert-enabled=true \
    --staking-ephemeral-signer-enabled=true \
    --bootstrap-ips="" \
    --bootstrap-ids="" \
    --log-level=info \
    --api-admin-enabled=true \
    --api-keystore-enabled=false \
    --api-metrics-enabled=true \
    --health-check-frequency=2s \
    --network-max-reconnect-delay=1s \
    --sybil-protection-enabled=false \
    --sybil-protection-disabled-weight=100 \
    --snow-sample-size=1 \
    --snow-quorum-size=1 \
    --snow-virtuous-commit-threshold=1 \
    --snow-rogue-commit-threshold=1 \
    --snow-concurrent-repolls=1 \
    --snow-optimal-processing=1 \
    --snow-max-processing=1 \
    --snow-max-time-processing=2s &

LUXD_PID=$!
echo ""
echo "luxd started with PID $LUXD_PID"
echo ""

# Wait for node to initialize
echo "Waiting for node to initialize (30 seconds)..."
sleep 30

# Check node health
echo ""
echo "Checking node health..."
echo "================================"
curl -s -X POST --data '{
    "jsonrpc":"2.0",
    "id":1,
    "method":"health.health"
}' -H 'content-type:application/json' http://localhost:$HTTP_PORT/ext/health | jq '.'

# Check C-Chain status
echo ""
echo "Checking C-Chain status..."
echo "================================"

# Get block height
HEIGHT=$(curl -s -X POST -H "Content-Type: application/json" \
    -d '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' \
    http://localhost:$HTTP_PORT/ext/bc/C/rpc 2>/dev/null | jq -r '.result' 2>/dev/null)

if [ -z "$HEIGHT" ] || [ "$HEIGHT" = "null" ]; then
    echo "❌ Failed to get block height - C-chain may not be ready yet"
    echo "   Check logs at: tail -f $DATA_DIR/logs/main.log"
else
    # Convert hex to decimal
    HEIGHT_DEC=$((16#${HEIGHT:2}))
    echo "✅ C-Chain block height: $HEIGHT_DEC"
    
    # Get genesis block
    echo ""
    echo "Genesis block info:"
    curl -s -X POST -H "Content-Type: application/json" \
        -d '{"jsonrpc":"2.0","method":"eth_getBlockByNumber","params":["0x0", false],"id":1}' \
        http://localhost:$HTTP_PORT/ext/bc/C/rpc | jq '.result | {number, hash, timestamp}'
    
    # Check a specific account balance
    echo ""
    echo "Checking validator address balance..."
    BALANCE=$(curl -s -X POST -H "Content-Type: application/json" \
        -d '{"jsonrpc":"2.0","method":"eth_getBalance","params":["0x9011E888251AB053B7bD1cdB598Db4f9DEd94714", "latest"],"id":1}' \
        http://localhost:$HTTP_PORT/ext/bc/C/rpc | jq -r '.result')
    
    if [ ! -z "$BALANCE" ] && [ "$BALANCE" != "null" ]; then
        # Convert hex to decimal and to LUX
        BALANCE_DEC=$((16#${BALANCE:2}))
        echo "  Address: 0x9011E888251AB053B7bD1cdB598Db4f9DEd94714"
        echo "  Balance: $BALANCE (wei)"
        echo "  Balance: $(echo "scale=18; $BALANCE_DEC / 1000000000000000000" | bc -l) LUX"
    fi
fi

echo ""
echo "============================================================"
echo "✅ Mainnet single node validator is running!"
echo ""
echo "Node PID: $LUXD_PID"
echo "Data directory: $DATA_DIR"
echo ""
echo "Commands:"
echo "  Stop node: kill $LUXD_PID"
echo "  View logs: tail -f $DATA_DIR/logs/main.log"
echo "  Check health: curl http://localhost:$HTTP_PORT/ext/health"
echo "  RPC endpoint: http://localhost:$HTTP_PORT/ext/bc/C/rpc"
echo "============================================================"