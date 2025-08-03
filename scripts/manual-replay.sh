#!/bin/bash
set -euo pipefail

# Manual blockchain replay script for Lux mainnet

# Configuration
CHAIN_ID=96369
DATA_DIR="/home/z/work/lux/genesis/runs/lux-mainnet-${CHAIN_ID}"
NODE_URL="http://localhost:9630/ext/bc/C/rpc"

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

echo -e "${GREEN}🔄 Manual Blockchain Replay Script${NC}"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

# Check if node is running
echo -e "\n${YELLOW}Checking if node is running...${NC}"
if ! curl -s -X POST -H "Content-Type: application/json" \
    -d '{"jsonrpc":"2.0","id":1,"method":"eth_blockNumber","params":[]}' \
    $NODE_URL 2>/dev/null | grep -q result; then
    echo -e "${RED}❌ Error: Node is not running or RPC is not accessible${NC}"
    echo "Please start the node first with a working luxd binary"
    exit 1
fi

# Get current block height
HEX_BLOCK=$(curl -s -X POST -H "Content-Type: application/json" \
    -d '{"jsonrpc":"2.0","id":1,"method":"eth_blockNumber","params":[]}' \
    $NODE_URL | jq -r .result)
CURRENT_BLOCK=$((16#${HEX_BLOCK#0x}))

echo -e "${GREEN}✓ Node is running. Current block: $CURRENT_BLOCK${NC}"

if [ "$CURRENT_BLOCK" -gt 0 ]; then
    echo -e "${YELLOW}⚠️  Chain already has blocks. Manual replay not needed.${NC}"
    exit 0
fi

echo -e "\n${YELLOW}📊 Starting manual blockchain replay...${NC}"
echo "This will replay blocks from the imported database into the running chain."

# Use the genesis tool to trigger replay
cd /home/z/work/lux/genesis

echo -e "\n${GREEN}🚀 Triggering blockchain replay...${NC}"
./bin/genesis replay \
    --data-dir="$DATA_DIR" \
    --chain-id=$CHAIN_ID \
    --rpc-url=$NODE_URL

echo -e "\n${GREEN}✓ Replay complete!${NC}"

# Check final block height
HEX_BLOCK=$(curl -s -X POST -H "Content-Type: application/json" \
    -d '{"jsonrpc":"2.0","id":1,"method":"eth_blockNumber","params":[]}' \
    $NODE_URL | jq -r .result)
FINAL_BLOCK=$((16#${HEX_BLOCK#0x}))

echo -e "Final block height: ${GREEN}$FINAL_BLOCK${NC}"