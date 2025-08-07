#!/bin/bash

echo "Stopping current luxd..."
pkill -f luxd
sleep 2

echo "Starting luxd with admin and debug APIs enabled..."

LUXD_BIN="/home/z/work/lux/node/build/luxd"
DATA_DIR="/tmp/luxd-replay"
NETWORK_ID=96369

$LUXD_BIN \
    --network-id=$NETWORK_ID \
    --data-dir=$DATA_DIR \
    --http-host=0.0.0.0 \
    --http-port=9630 \
    --staking-port=9631 \
    --staking-tls-cert-file=$DATA_DIR/staking/staker.crt \
    --staking-tls-key-file=$DATA_DIR/staking/staker.key \
    --staking-signer-key-file=$DATA_DIR/staking/signer.key \
    --sybil-protection-enabled=false \
    --sybil-protection-disabled-weight=100 \
    --api-admin-enabled=true \
    --api-keystore-enabled=false \
    --api-metrics-enabled=false \
    --chain-config-dir=$DATA_DIR/configs/chains \
    --coreth-admin-api-enabled=true \
    --coreth-admin-api-dir=$DATA_DIR/coreth-admin \
    --log-level=info &

echo "luxd started with PID $!"
echo "Waiting for RPC to be ready..."
sleep 5

# Check if admin/debug APIs are available
echo "Checking available RPC modules..."
curl -s -X POST -H "Content-Type: application/json" \
    -d '{"jsonrpc":"2.0","id":1,"method":"rpc_modules","params":[]}' \
    http://localhost:9630/ext/bc/C/rpc | jq .result

echo ""
echo "luxd is ready with admin APIs enabled!"
echo "Now run: go run complete-replay.go replay"