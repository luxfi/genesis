#!/bin/bash

echo "======================================"
echo "  LUX MAINNET BOOTSTRAP VALIDATOR"
echo "======================================"

# Configuration
LUXD="/Users/z/work/lux/node/build/luxd"
DATA_DIR="/tmp/lux-mainnet-bootstrap"
MIGRATED_DB="/tmp/lux-mainnet-final/chainData/2XpgdN3WNtM6AuzGgnXW7S6BqbH7DYY8CKwqaUiDUj67vYGvfC/db"
C_CHAIN_CONFIG="/tmp/c-chain-config.json"

# Clean up
echo "Stopping existing processes..."
pkill -f luxd 2>/dev/null || true
sleep 2

# Check database exists
if [ ! -d "$MIGRATED_DB" ]; then
    echo "ERROR: Migrated database not found at $MIGRATED_DB"
    exit 1
fi

# Setup directories
echo "Setting up data directories..."
rm -rf $DATA_DIR
mkdir -p "$DATA_DIR/staking"
mkdir -p "$DATA_DIR/db"
mkdir -p "$DATA_DIR/logs"
mkdir -p "$DATA_DIR/chainData/2XpgdN3WNtM6AuzGgnXW7S6BqbH7DYY8CKwqaUiDUj67vYGvfC"
mkdir -p "$DATA_DIR/configs/C"

# Copy migrated database
echo "Copying migrated C-Chain database..."
cp -r "$MIGRATED_DB" "$DATA_DIR/chainData/2XpgdN3WNtM6AuzGgnXW7S6BqbH7DYY8CKwqaUiDUj67vYGvfC/db"

# Generate staking keys
echo "Generating staking certificates..."
openssl genrsa -out "$DATA_DIR/staking/staker.key" 4096 2>/dev/null
openssl req -new -x509 -key "$DATA_DIR/staking/staker.key" \
    -out "$DATA_DIR/staking/staker.crt" -days 365 \
    -subj "/C=US/ST=State/L=City/O=Lux/CN=genesis-validator" 2>/dev/null

# Create C-Chain config
echo "Creating C-Chain configuration..."
cat > "$DATA_DIR/configs/C/config.json" <<'CCONFIG'
{
  "snowman-api-enabled": false,
  "coreth-admin-api-enabled": false,
  "eth-apis": ["eth", "eth-filter", "net", "web3"],
  "rpc-gas-cap": 50000000,
  "rpc-tx-fee-cap": 100,
  "pruning-enabled": false,
  "state-sync-enabled": false,
  "allow-unprotected-txs": true
}
CCONFIG

# Create node config
cat > "$DATA_DIR/config.json" <<EOF
{
  "network-id": "mainnet",
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

# Launch with C-Chain config via chain-config flag
echo ""
echo "Starting Lux mainnet bootstrap validator..."
echo ""

$LUXD \
    --config-file="$DATA_DIR/config.json" \
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
    --chain-config-dir="$DATA_DIR/configs" \
    --log-level=info \
    > "$DATA_DIR/node.log" 2>&1 &

PID=$!
echo "Validator started with PID: $PID"

# Wait for initialization
echo "Waiting for node to initialize..."
sleep 10

# Check if running
if ! ps -p $PID > /dev/null; then
    echo ""
    echo "ERROR: Node failed to start!"
    echo "Last 30 lines of log:"
    tail -30 "$DATA_DIR/node.log"
    exit 1
fi

# Get node info
echo ""
echo "=== NODE STATUS ==="
NODE_ID=$(curl -s -X POST http://127.0.0.1:9650/ext/info \
    -H "Content-Type: application/json" \
    -d '{"jsonrpc":"2.0","id":1,"method":"info.getNodeID"}' \
    | python3 -c "import sys, json; print(json.load(sys.stdin).get('result',{}).get('nodeID','Unknown'))" 2>/dev/null)

echo "Node ID: $NODE_ID"

# Check C-Chain
echo ""
echo "=== C-CHAIN STATUS ==="
RESPONSE=$(curl -s -X POST http://127.0.0.1:9650/ext/bc/C/rpc \
    -H "Content-Type: application/json" \
    -d '{"jsonrpc":"2.0","id":1,"method":"eth_blockNumber"}' 2>/dev/null)

if [[ "$RESPONSE" == *"result"* ]]; then
    BLOCK_HEX=$(echo "$RESPONSE" | python3 -c "import sys, json; print(json.load(sys.stdin)['result'])" 2>/dev/null)
    if [ -n "$BLOCK_HEX" ] && [ "$BLOCK_HEX" != "null" ]; then
        BLOCK_NUM=$((16#${BLOCK_HEX#0x}))
        echo "✓ Block Height: $BLOCK_NUM"
        
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

echo ""
echo "======================================"
echo "  BOOTSTRAP VALIDATOR RUNNING"
echo "======================================"
echo ""
echo "Node ID:  $NODE_ID"
echo "PID:      $PID"
echo "API:      http://127.0.0.1:9650"
echo "RPC:      http://127.0.0.1:9650/ext/bc/C/rpc"
echo ""
echo "Logs:     tail -f $DATA_DIR/node.log"
echo "Stop:     kill $PID"
echo ""
echo "Other validators can now join using:"
echo "  --bootstrap-ips=127.0.0.1:9651"
echo "  --bootstrap-ids=$NODE_ID"