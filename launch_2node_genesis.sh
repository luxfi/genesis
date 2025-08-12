#!/bin/bash

echo "==================================================="
echo "  LAUNCHING 2-NODE LUX GENESIS NETWORK"
echo "  Consensus K=2 for immediate bootstrap"
echo "==================================================="

# Configuration
LUXD="/Users/z/work/lux/node/build/luxd"
MIGRATED_DB="/tmp/lux-mainnet-final/chainData/2XpgdN3WNtM6AuzGgnXW7S6BqbH7DYY8CKwqaUiDUj67vYGvfC/db"
BASE_DIR="/tmp/lux-2node-genesis"
NETWORK_ID=96369

# Clean up
echo "Cleaning up old processes..."
pkill -f luxd 2>/dev/null || true
rm -rf $BASE_DIR
sleep 2

# Create directories for both nodes
echo "Setting up node directories..."
for i in 1 2; do
    NODE_DIR="$BASE_DIR/node0$i"
    mkdir -p "$NODE_DIR/staking"
    mkdir -p "$NODE_DIR/db"
    mkdir -p "$NODE_DIR/logs"
    mkdir -p "$NODE_DIR/chainData/2XpgdN3WNtM6AuzGgnXW7S6BqbH7DYY8CKwqaUiDUj67vYGvfC"
    
    # Copy the migrated database to both nodes
    echo "Copying C-Chain database to node0$i..."
    cp -r "$MIGRATED_DB" "$NODE_DIR/chainData/2XpgdN3WNtM6AuzGgnXW7S6BqbH7DYY8CKwqaUiDUj67vYGvfC/"
    
    # Generate staking keys
    echo "Generating staking keys for node0$i..."
    openssl genrsa -out "$NODE_DIR/staking/staker.key" 4096 2>/dev/null
    openssl req -new -x509 -key "$NODE_DIR/staking/staker.key" \
        -out "$NODE_DIR/staking/staker.crt" -days 365 \
        -subj "/C=US/ST=State/L=City/O=Lux/CN=node0$i" 2>/dev/null
done

# Create config for node01 with K=2 consensus
cat > "$BASE_DIR/node01/config.json" <<EOF
{
  "network-id": "$NETWORK_ID",
  "health-check-frequency": "2s",
  "network-max-reconnect-delay": "1s",
  "network-allow-private-ips": true,
  "consensus-shutdown-timeout": "10s",
  "consensus-gossip-frequency": "250ms",
  "min-stake-duration": "336h",
  "max-stake-duration": "8760h",
  "stake-minting-period": "8760h",
  "stake-max-consumption-rate": 120000,
  "stake-min-consumption-rate": 100000,
  "stake-supply-cap": 720000000000000000,
  "snow-sample-size": 2,
  "snow-quorum-size": 2,
  "snow-virtuous-commit-threshold": 5,
  "snow-rogue-commit-threshold": 10,
  "p-chain-config": {
    "K": 2,
    "alpha": 2,
    "beta": 2
  },
  "c-chain-config": {
    "K": 2,
    "alpha": 2,
    "beta": 2
  }
}
EOF

# Copy config to node02
cp "$BASE_DIR/node01/config.json" "$BASE_DIR/node02/config.json"

# Start Node 01 (Bootstrap node)
echo -e "\n=== Starting Node 01 (Bootstrap) ==="
$LUXD \
    --config-file="$BASE_DIR/node01/config.json" \
    --data-dir="$BASE_DIR/node01" \
    --db-dir="$BASE_DIR/node01/db" \
    --chain-data-dir="$BASE_DIR/node01/chainData" \
    --log-dir="$BASE_DIR/node01/logs" \
    --http-host=0.0.0.0 \
    --http-port=9650 \
    --staking-port=9651 \
    --staking-tls-cert-file="$BASE_DIR/node01/staking/staker.crt" \
    --staking-tls-key-file="$BASE_DIR/node01/staking/staker.key" \
    --bootstrap-ips="" \
    --bootstrap-ids="" \
    --log-level=info \
    > "$BASE_DIR/node01.log" 2>&1 &

NODE1_PID=$!
echo "Node 01 started with PID: $NODE1_PID"

# Wait for node01 to start
sleep 5

# Get Node01 ID
NODE1_ID=$(curl -s -X POST http://127.0.0.1:9650/ext/info \
    -H "Content-Type: application/json" \
    -d '{"jsonrpc":"2.0","id":1,"method":"info.getNodeID"}' \
    | python3 -c "import sys, json; print(json.load(sys.stdin)['result']['nodeID'])" 2>/dev/null)

echo "Node 01 ID: $NODE1_ID"

# Start Node 02 (connects to Node 01)
echo -e "\n=== Starting Node 02 ==="
$LUXD \
    --config-file="$BASE_DIR/node02/config.json" \
    --data-dir="$BASE_DIR/node02" \
    --db-dir="$BASE_DIR/node02/db" \
    --chain-data-dir="$BASE_DIR/node02/chainData" \
    --log-dir="$BASE_DIR/node02/logs" \
    --http-host=0.0.0.0 \
    --http-port=9652 \
    --staking-port=9653 \
    --staking-tls-cert-file="$BASE_DIR/node02/staking/staker.crt" \
    --staking-tls-key-file="$BASE_DIR/node02/staking/staker.key" \
    --bootstrap-ips="127.0.0.1:9651" \
    --bootstrap-ids="$NODE1_ID" \
    --log-level=info \
    > "$BASE_DIR/node02.log" 2>&1 &

NODE2_PID=$!
echo "Node 02 started with PID: $NODE2_PID"

# Wait for nodes to connect
echo -e "\n=== Waiting for network formation ==="
sleep 10

# Check node status
echo -e "\n=== Node Status ==="
echo "Node 01:"
curl -s -X POST http://127.0.0.1:9650/ext/info \
    -H "Content-Type: application/json" \
    -d '{"jsonrpc":"2.0","id":1,"method":"info.peers"}' | python3 -m json.tool 2>/dev/null | head -20

echo -e "\nNode 02:"
curl -s -X POST http://127.0.0.1:9652/ext/info \
    -H "Content-Type: application/json" \
    -d '{"jsonrpc":"2.0","id":1,"method":"info.peers"}' | python3 -m json.tool 2>/dev/null | head -20

# Check bootstrap status
echo -e "\n=== Bootstrap Status ==="
echo "Node 01 P-Chain:"
curl -s -X POST http://127.0.0.1:9650/ext/info \
    -H "Content-Type: application/json" \
    -d '{"jsonrpc":"2.0","id":1,"method":"info.isBootstrapped","params":{"chain":"P"}}' | python3 -m json.tool

echo "Node 01 C-Chain:"
curl -s -X POST http://127.0.0.1:9650/ext/info \
    -H "Content-Type: application/json" \
    -d '{"jsonrpc":"2.0","id":1,"method":"info.isBootstrapped","params":{"chain":"C"}}' | python3 -m json.tool

# Check C-Chain
echo -e "\n=== C-Chain Check ==="
for port in 9650 9652; do
    echo "Node on port $port:"
    RESPONSE=$(curl -s -X POST http://127.0.0.1:$port/ext/bc/C/rpc \
        -H "Content-Type: application/json" \
        -d '{"jsonrpc":"2.0","id":1,"method":"eth_blockNumber"}' 2>/dev/null)
    
    if [[ "$RESPONSE" == *"result"* ]]; then
        BLOCK_HEX=$(echo "$RESPONSE" | python3 -c "import sys, json; print(json.load(sys.stdin)['result'])" 2>/dev/null)
        if [ -n "$BLOCK_HEX" ]; then
            BLOCK_NUM=$((16#${BLOCK_HEX#0x}))
            echo "  ✓ Block height: $BLOCK_NUM"
        fi
    else
        echo "  C-Chain not ready: $RESPONSE"
    fi
done

echo -e "\n=== 2-NODE GENESIS NETWORK LAUNCHED ==="
echo "Node 01: PID=$NODE1_PID, API=http://127.0.0.1:9650"
echo "Node 02: PID=$NODE2_PID, API=http://127.0.0.1:9652"
echo ""
echo "Logs:"
echo "  tail -f $BASE_DIR/node01.log"
echo "  tail -f $BASE_DIR/node02.log"
echo ""
echo "To scale up: Add more nodes with updated consensus parameters"