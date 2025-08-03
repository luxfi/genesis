#!/bin/bash
set -euo pipefail

# Launch script using avalanchego binary (temporary workaround for protobuf issue)

# Configuration
CHAIN_ID=96369
LUXD_BIN="/home/z/.avalanche-cli.current/bin/avalanchego/avalanchego-v1.11.12/avalanchego"
DATA_DIR="/home/z/work/lux/genesis/runs/lux-mainnet-${CHAIN_ID}"

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

echo -e "${GREEN}🚀 Lux Network Launch (with avalanchego) - Chain ID ${CHAIN_ID}${NC}"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo -e "${YELLOW}Note: Using avalanchego binary as workaround for protobuf issue${NC}"
echo -e "${YELLOW}LUX_GENESIS automatic replay not available with this binary${NC}"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

# Check if binary exists
if [ ! -f "$LUXD_BIN" ]; then
    echo -e "${RED}❌ Error: avalanchego binary not found at $LUXD_BIN${NC}"
    exit 1
fi

# Start the node
echo -e "\n${GREEN}🚀 Starting node...${NC}"
echo "Data directory: $DATA_DIR"
echo "Binary: $LUXD_BIN"
echo "Chain ID: $CHAIN_ID"
echo "RPC endpoint: http://localhost:9630/ext/bc/C/rpc"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

# Launch node
exec $LUXD_BIN \
    --config-file="$DATA_DIR/node-config.json" \
    --network-id=$CHAIN_ID