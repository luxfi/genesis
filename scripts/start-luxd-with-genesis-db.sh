#!/bin/bash

echo "========================================================="
echo "    Starting luxd with Genesis Database Replay          "
echo "========================================================="
echo ""

# Kill existing luxd
echo "Stopping any existing luxd process..."
pkill -f luxd
sleep 2

LUXD_BIN="/home/z/work/lux/node/build/luxd"
DATA_DIR="/tmp/luxd-with-genesis"
GENESIS_DB="/home/z/work/lux/state/chaindata/lux-mainnet-96369/db"
NETWORK_ID=96369

# Create data directory if it doesn't exist
mkdir -p $DATA_DIR

echo "Configuration:"
echo "  luxd binary:  $LUXD_BIN"
echo "  Data dir:     $DATA_DIR"
echo "  Genesis DB:   $GENESIS_DB"
echo "  Network ID:   $NETWORK_ID"
echo ""

echo "Starting luxd with genesis database..."

$LUXD_BIN \
    --network-id=$NETWORK_ID \
    --data-dir=$DATA_DIR \
    --genesis-db=$GENESIS_DB \
    --genesis-db-type=pebbledb \
    --http-host=0.0.0.0 \
    --http-port=9630 \
    --staking-port=9631 \
    --sybil-protection-enabled=false \
    --sybil-protection-disabled-weight=100 \
    --api-admin-enabled=true \
    --api-keystore-enabled=false \
    --api-metrics-enabled=false \
    --log-level=info \
    --skip-bootstrap &

PID=$!
echo "luxd started with PID $PID"
echo ""

echo "Waiting for RPC to be ready..."
sleep 10

echo "Checking C-chain status..."
curl -s -X POST -H "Content-Type: application/json" \
    -d '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' \
    http://localhost:9630/ext/bc/C/rpc | jq .

echo ""
echo "Checking available RPC modules..."
curl -s -X POST -H "Content-Type: application/json" \
    -d '{"jsonrpc":"2.0","id":1,"method":"rpc_modules","params":[]}' \
    http://localhost:9630/ext/bc/C/rpc | jq .result

echo ""
echo "========================================================="
echo "If the chain height is > 0, the genesis DB replay worked!"
echo "========================================================="