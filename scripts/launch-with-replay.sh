#!/bin/bash

# Launch Lux mainnet with LUX_GENESIS=1 automatic block replay

# Kill any existing luxd
pkill -f luxd || true
sleep 2

# Configuration
DATA_DIR="/home/z/work/lux/genesis/runs/lux-mainnet-96369"
LUXD_BIN="/home/z/work/lux/build/luxd"  # Working binary
CHAIN_ID=96369  # Mainnet chain ID

echo "🚀 Launching Lux Network Mainnet with LUX_GENESIS Replay"
echo "Binary: $LUXD_BIN"
echo "Database: $DATA_DIR/C/db (1,082,780 blocks)"
echo "RPC: http://localhost:9630/ext/bc/C/rpc"
echo ""

# Check binary exists
if [ ! -f "$LUXD_BIN" ]; then
    echo "❌ Error: Binary not found at $LUXD_BIN"
    exit 1
fi

# Make sure it's executable
chmod +x $LUXD_BIN

# Copy genesis from state repo
echo "📋 Setting up genesis configuration..."
cp /home/z/work/lux/genesis/state/configs/lux-mainnet-96369/C/genesis.json \
   $DATA_DIR/configs/chains/C/genesis.json

# Create C-Chain config
cat > "$DATA_DIR/configs/chains/C/config.json" << EOF
{
  "snowman-api-enabled": false,
  "coreth-admin-api-enabled": false,
  "chain-id": $CHAIN_ID,
  "network-id": $CHAIN_ID,
  "pruning-enabled": true,
  "local-txs-enabled": true,
  "api-max-duration": 0,
  "state-sync-enabled": false,
  "allow-unfinalized-queries": true,
  "skip-upgrade-check": true,
  "api-require-auth": false,
  "consensus-timeout": 5000000000,
  "eth-apis": ["eth", "personal", "admin", "debug", "web3", "internal-debug", "internal-blockchain", "internal-transaction-pool", "net"]
}
EOF

# Create node config
cat > "$DATA_DIR/node-config.json" << EOF
{
  "network-id": "$CHAIN_ID",
  "db-type": "leveldb",
  "db-dir": "$DATA_DIR",
  "chain-data-dir": "$DATA_DIR",
  "log-level": "info",
  "log-dir": "$DATA_DIR/logs",
  "http-host": "",
  "http-port": 9630,
  "public-ip": "127.0.0.1",
  "chain-config-dir": "$DATA_DIR/configs/chains",
  "vm-aliases": {
    "mgj786NP7uDwBCcq6YwThhaN8FLyybkCa4zBWTQbNgmK6k9A6": ["evm"]
  }
}
EOF

# Clean aliases
mkdir -p "$DATA_DIR/configs/chains/aliases"
echo "{}" > "$DATA_DIR/configs/chains/aliases.json"

# Show configuration
echo "📋 Configuration Summary:"
echo "  Network ID: $CHAIN_ID"
echo "  Chain ID: $CHAIN_ID"
echo "  Genesis Chain ID: $(cat $DATA_DIR/configs/chains/C/genesis.json | jq -r '.config.chainId')"
echo ""

# Set environment for genesis replay
export LUX_GENESIS=1
export LUX_IMPORTED_HEIGHT=1082780
export LUX_IMPORTED_BLOCK_ID=32dede1fc8e0f11ecde12fb42aef7933fc6c5fcf863bc277b5eac08ae4d461f0

echo "🔄 LUX_GENESIS=1 set for automatic block replay"
echo "📊 Imported blockchain data:"
echo "  Height: $LUX_IMPORTED_HEIGHT"
echo "  Block Hash: $LUX_IMPORTED_BLOCK_ID"
echo ""

# Launch node
echo "🚀 Starting node..."
exec $LUXD_BIN \
  --config-file="$DATA_DIR/node-config.json" \
  --network-id=$CHAIN_ID