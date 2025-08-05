#!/bin/bash
set -e

echo "🚀 Launching BFT Node with k=1 consensus..."

# Kill any existing node
pkill luxd || true
sleep 2

# Create data directory
DATA_DIR="runs/bft-$(date +%s)"
mkdir -p "$DATA_DIR"

# Launch with dev mode which works
../node/build/luxd \
  --network-id=96369 \
  --dev \
  --data-dir="$DATA_DIR" \
  --http-host=0.0.0.0 \
  --http-port=9630 \
  --staking-port=9631 \
  --log-level=info \
  --api-admin-enabled=true