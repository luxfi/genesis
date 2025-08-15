#!/bin/bash

# Script to run Lux mainnet using genesis launch command
# This uses the migrated C-chain data with proper genesis configuration

echo "🚀 Launching Lux Mainnet (96369) with migrated C-chain data"
echo "============================================================"

# Build genesis binary if needed
if [ ! -f "./bin/genesis" ]; then
    echo "Building genesis binary..."
    CGO_ENABLED=0 go build -o bin/genesis main.go
fi

# Use genesis launch command with mainnet configuration
./bin/genesis launch \
    --network-id 96369 \
    --data-dir ~/.luxd-mainnet-96369 \
    --genesis ~/work/lux/state/configs/lux-mainnet-96369/C/genesis.json \
    --chain-data ~/work/lux/state/chaindata/lux-mainnet-96369/db/pebbledb \
    --http-port 9650 \
    --staking-port 9651 \
    --no-bootstrap \
    --log-level info \
    --binary ~/work/lux/node/build/luxd

# Note: Update --binary path once luxd is successfully built