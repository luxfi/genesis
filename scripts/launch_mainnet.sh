#!/bin/bash

# Lux Mainnet Local Launch Script
# This script monitors the migration process and launches a local mainnet when ready

echo "=== Lux Mainnet Local Launch Script ==="
echo "Monitoring migration progress..."

# Check if migration is still running
while pgrep -f "genesis migrate" > /dev/null; do
    BLOCKS=$(tail -1 migration.log | grep -oE '[0-9]+ blocks' | grep -oE '[0-9]+' || echo "0")
    STATE=$(tail -1 migration.log | grep -oE '[0-9]+ state entries' | grep -oE '[0-9]+' | head -1 || echo "0") 
    echo "Migration in progress: ${BLOCKS:-0} blocks, ${STATE:-0} state entries migrated..."
    sleep 10
done

echo "Migration completed! Checking results..."

# Verify migration output
if [ ! -d "migrated/badgerdb" ]; then
    echo "ERROR: Migration output directory not found!"
    exit 1
fi

DB_SIZE=$(du -sh migrated/badgerdb | cut -f1)
echo "Migrated database size: $DB_SIZE"

# Setup directories for luxd
echo "Setting up luxd directories..."
mkdir -p ~/.luxd-mainnet/chains/C

# Copy migrated data to luxd directory
echo "Copying migrated data to luxd directory..."
cp -r migrated/badgerdb ~/.luxd-mainnet/chains/C/

# Create node configuration
echo "Creating node configuration..."
cat > ~/.luxd-mainnet/config.json <<EOF
{
  "network-id": "96369",
  "db-type": "badgerdb",
  "http-host": "0.0.0.0",
  "http-port": 9650,
  "staking-port": 9651,
  "log-level": "info",
  "snow-mixed-query-num-push-vdr": 1,
  "consensus-shutdown-timeout": "1s",
  "consensus-gossip-frequency": "1s",
  "network-compression-type": "zstd",
  "c-chain-config": {
    "snowman-api-enabled": true,
    "coreth-admin-api-enabled": true,
    "eth-apis": [
      "eth",
      "eth-filter",
      "net",
      "web3",
      "internal-eth",
      "internal-blockchain",
      "internal-transaction",
      "internal-tx-pool",
      "internal-account",
      "internal-personal",
      "debug",
      "debug-handler"
    ],
    "pruning-enabled": false,
    "local-txs-enabled": true,
    "api-max-duration": "0",
    "allow-unfinalized-queries": true,
    "allow-unprotected-txs": true,
    "remote-tx-gossip-only-enabled": false,
    "log-level": "debug"
  }
}
EOF

# Launch luxd with single validator (k=1) for local testing
echo "Launching luxd with single validator configuration..."
cd /Users/z/work/lux/node

# Kill any existing luxd processes
pkill -f luxd || true
sleep 2

# Launch luxd
echo "Starting luxd..."
./build/luxd \
  --data-dir=~/.luxd-mainnet \
  --network-id=96369 \
  --db-type=badgerdb \
  --http-host=0.0.0.0 \
  --http-port=9650 \
  --staking-port=9651 \
  --log-level=info \
  --snow-mixed-query-num-push-vdr=1 \
  --consensus-shutdown-timeout=1s \
  --consensus-gossip-frequency=1s \
  --network-compression-type=zstd \
  > ~/.luxd-mainnet/node.log 2>&1 &

NODE_PID=$!
echo "Luxd started with PID: $NODE_PID"

# Wait for node to be ready
echo "Waiting for node to be ready..."
sleep 10

# Check node status
echo "Checking node status..."
curl -X POST --data '{
    "jsonrpc":"2.0",
    "id"     :1,
    "method" :"info.isBootstrapped",
    "params": {
        "chain":"C"
    }
}' -H 'content-type:application/json;' http://127.0.0.1:9650/ext/info

echo ""
echo "Checking block height..."
curl -X POST --data '{
    "jsonrpc":"2.0",
    "id"     :1,
    "method" :"eth_blockNumber"
}' -H 'content-type:application/json;' http://127.0.0.1:9650/ext/bc/C/rpc

echo ""
echo "Checking balance of treasury account 0x9011E888251AB053B7bD1cdB598Db4f9DEd94714..."
curl -X POST --data '{
    "jsonrpc":"2.0",
    "id"     :1,
    "method" :"eth_getBalance",
    "params": ["0x9011E888251AB053B7bD1cdB598Db4f9DEd94714", "latest"]
}' -H 'content-type:application/json;' http://127.0.0.1:9650/ext/bc/C/rpc

echo ""
echo "=== Node launched successfully ==="
echo "Node PID: $NODE_PID"
echo "Log file: ~/.luxd-mainnet/node.log"
echo "RPC endpoint: http://127.0.0.1:9650/ext/bc/C/rpc"
echo ""
echo "To stop the node: kill $NODE_PID"
echo "To view logs: tail -f ~/.luxd-mainnet/node.log"