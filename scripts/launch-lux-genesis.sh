#\!/bin/bash
set -e

echo "Launching Lux mainnet with LUX_GENESIS=1..."

# Set environment variable
export LUX_GENESIS=1

# Set paths
DATA_DIR="$(pwd)/runs/lux-mainnet-96369"
CONFIG_FILE="$DATA_DIR/config.json"
GENESIS_FILE="$(pwd)/state/configs/lux-mainnet-96369/genesis.json"

# Ensure data directory exists
mkdir -p "$DATA_DIR"

# Create config if it doesn't exist
if [ \! -f "$CONFIG_FILE" ]; then
  echo "Creating config file..."
  cat > "$CONFIG_FILE" << EOCFG
{
  "network-id": 96369,
  "log-level": "info",
  "data-dir": "$DATA_DIR",
  "chain-data-dir": "$DATA_DIR",
  "genesis": "$GENESIS_FILE",
  "http-port": 9630,
  "staking-port": 9671,
  "staking-enabled": false,
  "sybil-protection-enabled": false,
  "index-enabled": false,
  "pruning-enabled": true,
  "snow-sample-size": 1,
  "snow-quorum-size": 1,
  "db-type": "leveldb",
  "chain-config-dir": "$(pwd)/state/configs/lux-mainnet-96369",
  "api-admin-enabled": true,
  "api-auth-required": false,
  "keystore-directory": "$DATA_DIR/keystore",
  "plugin-dir": "/home/z/work/lux/node/build/plugins"
}
EOCFG
fi

# Launch luxd
echo "Starting luxd with LUX_GENESIS=$LUX_GENESIS..."
exec /home/z/work/lux/build/luxd \
  --data-dir="$DATA_DIR" \
  --config-file="$CONFIG_FILE"
