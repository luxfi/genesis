#!/bin/bash

# Launch script for Lux mainnet with migrated C-chain data
# Network ID: 96369

# Configuration
NETWORK_ID=96369
DATA_DIR="${HOME}/.luxd-mainnet-${NETWORK_ID}"
GENESIS_CONFIG="${HOME}/work/lux/genesis/mainnet-96369-config"
CHAIN_DATA="${HOME}/work/lux/state/chaindata/lux-mainnet-96369/db"
LUXD_BINARY="${HOME}/work/lux/node/build/luxd"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}Lux Mainnet Launcher${NC}"
echo "========================"
echo "Network ID: ${NETWORK_ID}"
echo "Data Directory: ${DATA_DIR}"
echo ""

# Check if luxd binary exists
if [ ! -f "${LUXD_BINARY}" ]; then
    echo -e "${RED}Error: luxd binary not found at ${LUXD_BINARY}${NC}"
    echo "Please build luxd first by running:"
    echo "  cd ~/work/lux/node && make build"
    exit 1
fi

# Create data directory structure
echo -e "${YELLOW}Setting up data directory...${NC}"
mkdir -p "${DATA_DIR}"/{db,configs,staking}

# Copy genesis configuration
if [ -d "${GENESIS_CONFIG}" ]; then
    echo -e "${YELLOW}Copying genesis configuration...${NC}"
    cp -r "${GENESIS_CONFIG}"/* "${DATA_DIR}/configs/"
fi

# Copy chain data if using migrated database
if [ -d "${CHAIN_DATA}" ]; then
    echo -e "${YELLOW}Copying chain database...${NC}"
    # For BadgerDB
    if [ -d "${CHAIN_DATA}/badgerdb" ]; then
        cp -r "${CHAIN_DATA}/badgerdb" "${DATA_DIR}/db/c-chain"
    # For PebbleDB
    elif [ -d "${CHAIN_DATA}/pebbledb" ]; then
        cp -r "${CHAIN_DATA}/pebbledb" "${DATA_DIR}/db/c-chain"
    fi
fi

# Generate staking keys if they don't exist
STAKING_KEY="${DATA_DIR}/staking/staker.key"
STAKING_CERT="${DATA_DIR}/staking/staker.crt"

if [ ! -f "${STAKING_KEY}" ] || [ ! -f "${STAKING_CERT}" ]; then
    echo -e "${YELLOW}Generating staking keys...${NC}"
    openssl req -x509 -newkey rsa:4096 \
        -keyout "${STAKING_KEY}" \
        -out "${STAKING_CERT}" \
        -days 365 \
        -nodes \
        -subj "/C=US/ST=State/L=City/O=LuxNetwork/CN=mainnet-validator" \
        2>/dev/null
    chmod 600 "${STAKING_KEY}"
    echo -e "${GREEN}Staking keys generated${NC}"
fi

# Launch parameters
echo -e "${GREEN}Starting Lux node...${NC}"
echo ""

# Build launch command
LAUNCH_CMD="${LUXD_BINARY} \
    --data-dir=${DATA_DIR} \
    --network-id=${NETWORK_ID} \
    --http-port=9650 \
    --staking-port=9651 \
    --public-ip=127.0.0.1 \
    --http-host=0.0.0.0 \
    --log-level=info \
    --api-admin-enabled=true \
    --api-metrics-enabled=true \
    --index-enabled=true \
    --db-type=badgerdb \
    --staking-tls-key-file=${STAKING_KEY} \
    --staking-tls-cert-file=${STAKING_CERT}"

# Add bootstrap nodes for mainnet
# These would be the actual mainnet bootstrap nodes
# For now, we'll run without bootstrap for testing
if [ "$1" != "--no-bootstrap" ]; then
    echo -e "${YELLOW}Note: Running without bootstrap nodes. Use --no-bootstrap to suppress this warning.${NC}"
    LAUNCH_CMD="${LAUNCH_CMD} --bootstrap-ips= --bootstrap-ids="
fi

# Show the command
echo "Launch command:"
echo "${LAUNCH_CMD}"
echo ""

# Create log file
LOG_FILE="${DATA_DIR}/luxd.log"
echo "Logs will be written to: ${LOG_FILE}"
echo ""

# Launch the node
echo -e "${GREEN}Launching node...${NC}"
${LAUNCH_CMD} > "${LOG_FILE}" 2>&1 &
NODE_PID=$!

echo "Node started with PID: ${NODE_PID}"
echo ""
echo "Monitor logs with: tail -f ${LOG_FILE}"
echo "Stop node with: kill ${NODE_PID}"
echo ""
echo -e "${GREEN}RPC Endpoints:${NC}"
echo "  HTTP: http://localhost:9650"
echo "  C-Chain: http://localhost:9650/ext/bc/C/rpc"
echo "  P-Chain: http://localhost:9650/ext/P"
echo "  X-Chain: http://localhost:9650/ext/X"
echo ""
echo -e "${GREEN}Node is running!${NC}"