#!/bin/bash

echo "==================================================="
echo "  LAUNCHING SINGLE NODE LUX GENESIS (K=1)"
echo "  Patient Zero - The First Validator"
echo "==================================================="

# Configuration
LUXD="/Users/z/work/lux/node/build/luxd"
DATA_DIR="/tmp/lux-genesis-k1"
NETWORK_ID=96369

# Clean up
echo "Stopping any existing luxd processes..."
pkill -f luxd 2>/dev/null || true
sleep 2

# Check if we have the migrated database
if [ ! -d "/tmp/lux-mainnet-final/chainData/2XpgdN3WNtM6AuzGgnXW7S6BqbH7DYY8CKwqaUiDUj67vYGvfC/db" ]; then
    echo "ERROR: Migrated database not found!"
    echo "Expected at: /tmp/lux-mainnet-final/chainData/2XpgdN3WNtM6AuzGgnXW7S6BqbH7DYY8CKwqaUiDUj67vYGvfC/db"
    exit 1
fi

# Setup directory structure
echo "Setting up genesis node directory..."
rm -rf $DATA_DIR
mkdir -p "$DATA_DIR/staking"
mkdir -p "$DATA_DIR/db"
mkdir -p "$DATA_DIR/logs"
mkdir -p "$DATA_DIR/chainData"

# Copy the migrated C-Chain database
echo "Copying migrated C-Chain database..."
cp -r "/tmp/lux-mainnet-final/chainData/2XpgdN3WNtM6AuzGgnXW7S6BqbH7DYY8CKwqaUiDUj67vYGvfC" "$DATA_DIR/chainData/"

# Use existing staking keys or generate new ones
if [ -f "/tmp/lux-mainnet-final/staking/staker.crt" ]; then
    echo "Using existing staking keys..."
    cp /tmp/lux-mainnet-final/staking/* "$DATA_DIR/staking/"
else
    echo "Generating new staking keys..."
    openssl genrsa -out "$DATA_DIR/staking/staker.key" 4096 2>/dev/null
    openssl req -new -x509 -key "$DATA_DIR/staking/staker.key" \
        -out "$DATA_DIR/staking/staker.crt" -days 365 \
        -subj "/C=US/ST=State/L=City/O=Lux/CN=genesis" 2>/dev/null
fi

# Create genesis config with K=1 consensus (single node can validate)
cat > "$DATA_DIR/node-config.json" <<EOF
{
  "network-id": "$NETWORK_ID",
  "health-check-frequency": "2s",
  "network-max-reconnect-delay": "1s",
  "network-allow-private-ips": true,
  "consensus-shutdown-timeout": "5s",
  "consensus-gossip-frequency": "250ms",
  "min-stake-duration": "336h",
  "max-stake-duration": "8760h",
  "snow-sample-size": 1,
  "snow-quorum-size": 1,
  "snow-virtuous-commit-threshold": 1,
  "snow-rogue-commit-threshold": 1,
  "subnet-configs": {
    "11111111111111111111111111111111LpoYY": {
      "consensus-parameters": {
        "k": 1,
        "alpha-preference": 1,
        "alpha-confidence": 1,
        "beta": 1,
        "concurrent-repolls": 1,
        "optimal-processing": 10,
        "max-outstanding-items": 256,
        "max-item-processing-time": "30s"
      }
    }
  }
}
EOF

# Launch the genesis node with K=1
echo -e "\n=== Starting Genesis Node with K=1 Consensus ==="
$LUXD \
    --config-file="$DATA_DIR/node-config.json" \
    --data-dir="$DATA_DIR" \
    --db-dir="$DATA_DIR/db" \
    --chain-data-dir="$DATA_DIR/chainData" \
    --log-dir="$DATA_DIR/logs" \
    --http-host=0.0.0.0 \
    --http-port=9650 \
    --staking-port=9651 \
    --staking-tls-cert-file="$DATA_DIR/staking/staker.crt" \
    --staking-tls-key-file="$DATA_DIR/staking/staker.key" \
    --bootstrap-ips="" \
    --bootstrap-ids="" \
    --log-level=info \
    > "$DATA_DIR/node.log" 2>&1 &

PID=$!
echo "Genesis node started with PID: $PID"

# Wait for initialization
echo "Waiting for node to initialize..."
sleep 10

# Check if running
if ! ps -p $PID > /dev/null; then
    echo "Node failed to start! Last logs:"
    tail -50 "$DATA_DIR/node.log"
    exit 1
fi

# Get node info
echo -e "\n=== Genesis Node Info ==="
NODE_ID=$(curl -s -X POST http://127.0.0.1:9650/ext/info \
    -H "Content-Type: application/json" \
    -d '{"jsonrpc":"2.0","id":1,"method":"info.getNodeID"}' \
    | python3 -c "import sys, json; print(json.load(sys.stdin).get('result',{}).get('nodeID','Unknown'))" 2>/dev/null)

echo "Node ID: $NODE_ID"

# Check bootstrap status
echo -e "\n=== Bootstrap Status ==="
P_BOOT=$(curl -s -X POST http://127.0.0.1:9650/ext/info \
    -H "Content-Type: application/json" \
    -d '{"jsonrpc":"2.0","id":1,"method":"info.isBootstrapped","params":{"chain":"P"}}' \
    | python3 -c "import sys, json; print(json.load(sys.stdin).get('result',{}).get('isBootstrapped',False))" 2>/dev/null)

C_BOOT=$(curl -s -X POST http://127.0.0.1:9650/ext/info \
    -H "Content-Type: application/json" \
    -d '{"jsonrpc":"2.0","id":1,"method":"info.isBootstrapped","params":{"chain":"C"}}' \
    | python3 -c "import sys, json; print(json.load(sys.stdin).get('result',{}).get('isBootstrapped',False))" 2>/dev/null)

echo "P-Chain bootstrapped: $P_BOOT"
echo "C-Chain bootstrapped: $C_BOOT"

# Check C-Chain
echo -e "\n=== C-Chain Status ==="
RESPONSE=$(curl -s -X POST http://127.0.0.1:9650/ext/bc/C/rpc \
    -H "Content-Type: application/json" \
    -d '{"jsonrpc":"2.0","id":1,"method":"eth_blockNumber"}' 2>/dev/null)

if [[ "$RESPONSE" == *"result"* ]]; then
    BLOCK_HEX=$(echo "$RESPONSE" | python3 -c "import sys, json; print(json.load(sys.stdin)['result'])" 2>/dev/null)
    if [ -n "$BLOCK_HEX" ] && [ "$BLOCK_HEX" != "null" ]; then
        BLOCK_NUM=$((16#${BLOCK_HEX#0x}))
        echo "✓ C-Chain Block Height: $BLOCK_NUM"
        
        # Check treasury balance
        BAL_RESPONSE=$(curl -s -X POST http://127.0.0.1:9650/ext/bc/C/rpc \
            -H "Content-Type: application/json" \
            -d '{"jsonrpc":"2.0","id":1,"method":"eth_getBalance","params":["0x9011e888251ab053b7bd1cdb598db4f9ded94714","latest"]}' 2>/dev/null)
        
        if [[ "$BAL_RESPONSE" == *"result"* ]]; then
            BAL_HEX=$(echo "$BAL_RESPONSE" | python3 -c "import sys, json; print(json.load(sys.stdin)['result'])" 2>/dev/null)
            if [ -n "$BAL_HEX" ] && [ "$BAL_HEX" != "0x0" ]; then
                echo "✓ Treasury Balance: $(python3 -c "print(f'{int(\"$BAL_HEX\",16)/10**18:,.2f} LUX')" 2>/dev/null)"
            fi
        fi
    fi
else
    echo "C-Chain not ready yet"
fi

# Check recent logs
echo -e "\n=== Recent Activity ==="
grep -E "bootstrap|Bootstrap|consensus|Consensus" "$DATA_DIR/node.log" | tail -5

echo -e "\n========================================="
echo "GENESIS NODE LAUNCHED WITH K=1"
echo "========================================="
echo ""
echo "Node ID: $NODE_ID"
echo "API: http://127.0.0.1:9650"
echo "RPC: http://127.0.0.1:9650/ext/bc/C/rpc"
echo ""
echo "This node can validate alone with K=1 consensus."
echo "As more validators join, update consensus parameters:"
echo "  K=2 for 2 nodes"
echo "  K=13 for 21 nodes (mainnet)"
echo ""
echo "Logs: tail -f $DATA_DIR/node.log"
echo ""
echo "To stop: kill $PID"