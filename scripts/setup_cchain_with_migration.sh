#!/bin/bash
set -e

echo "============================================"
echo "  SETUP C-CHAIN WITH MIGRATED DATABASE"
echo "============================================"
echo ""

# Configuration
LUXD_BIN="/Users/z/work/lux/node/build/luxd"
MIGRATED_DB="/tmp/lux-badger-migration"
DATA_DIR="/tmp/lux-mainnet-cchain-$$"
HTTP_PORT=9650
STAKING_PORT=9651
NETWORK_ID=96369

# The C-Chain ID for mainnet
# This is the actual C-Chain ID we need to use
CCHAIN_ID="2q9e4r6Mu3U68nU1fYjgbR6JvwrRx36CohpAX5UQxse55x1Q5"

# Kill any existing luxd
echo "Stopping any existing luxd processes..."
pkill -f luxd 2>/dev/null || true
sleep 2

# Create data directory structure
echo "Setting up data directory: $DATA_DIR"
mkdir -p "$DATA_DIR/db"
mkdir -p "$DATA_DIR/chainData/$CCHAIN_ID"

# Copy the migrated database to the correct location
echo "Setting up C-Chain database..."
echo "  C-Chain ID: $CCHAIN_ID"
echo "  Source DB: $MIGRATED_DB"
echo "  Target: $DATA_DIR/chainData/$CCHAIN_ID/"

# Copy the BadgerDB files to the C-chain directory
cp -r "$MIGRATED_DB" "$DATA_DIR/chainData/$CCHAIN_ID/db"

# Create chain config for C-Chain
cat > "$DATA_DIR/C-config.json" << 'EOF'
{
  "snowman-api-enabled": false,
  "coreth-admin-api-enabled": true,
  "eth-apis": ["eth","eth-filter","net","web3","internal-eth","internal-blockchain","internal-transaction"],
  "rpc-gas-cap": 50000000,
  "rpc-tx-fee-cap": 100,
  "pruning-enabled": false,
  "local-txs-enabled": true,
  "api-max-duration": 0,
  "api-max-blocks-per-request": 0,
  "allow-unfinalized-queries": true,
  "allow-unprotected-txs": true,
  "tx-pool-price-limit": 1,
  "tx-pool-price-bump": 0,
  "metrics-enabled": true,
  "metrics-expensive-enabled": true
}
EOF

# Create network upgrade config
cat > "$DATA_DIR/upgrade.json" << 'EOF'
{
  "precompileUpgrades": [],
  "stateUpgrades": []
}
EOF

echo ""
echo "Configuration:"
echo "  Network ID: $NETWORK_ID"
echo "  Data Dir: $DATA_DIR"
echo "  C-Chain DB: $DATA_DIR/chainData/$CCHAIN_ID/db"
echo "  HTTP Port: $HTTP_PORT"
echo "  Staking Port: $STAKING_PORT"
echo ""

echo "Starting node with C-Chain..."
echo "================================"

# Launch luxd with C-Chain configuration
$LUXD_BIN \
    --network-id=$NETWORK_ID \
    --data-dir="$DATA_DIR" \
    --db-dir="$DATA_DIR/db" \
    --chain-data-dir="$DATA_DIR/chainData" \
    --chain-config-dir="$DATA_DIR" \
    --http-host=0.0.0.0 \
    --http-port=$HTTP_PORT \
    --staking-port=$STAKING_PORT \
    --staking-ephemeral-cert-enabled=true \
    --staking-ephemeral-signer-enabled=true \
    --bootstrap-ips="" \
    --bootstrap-ids="" \
    --log-level=debug \
    --log-display-level=info \
    --api-admin-enabled=true \
    --api-metrics-enabled=true \
    --health-check-frequency=2s \
    --network-max-reconnect-delay=1s \
    --sybil-protection-enabled=false \
    --sybil-protection-disabled-weight=100 > "$DATA_DIR/luxd.log" 2>&1 &

LUXD_PID=$!
echo ""
echo "luxd started with PID $LUXD_PID"
echo ""

# Wait for node to initialize
echo "Waiting for node to initialize (30 seconds)..."
for i in {1..30}; do
    echo -n "."
    sleep 1
done
echo ""

# Check node health
echo ""
echo "Checking node health..."
echo "================================"
HEALTH=$(curl -s -X POST --data '{
    "jsonrpc":"2.0",
    "id":1,
    "method":"health.health"
}' -H 'content-type:application/json' http://localhost:$HTTP_PORT/ext/health 2>/dev/null)

if [ ! -z "$HEALTH" ]; then
    echo "$HEALTH" | jq '.'
fi

# Check C-Chain status
echo ""
echo "Checking C-Chain status..."
echo "================================"

# Try to get the C-Chain block height
HEIGHT=$(curl -s -X POST -H "Content-Type: application/json" \
    -d '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' \
    http://localhost:$HTTP_PORT/ext/bc/C/rpc 2>/dev/null | jq -r '.result' 2>/dev/null)

if [ -z "$HEIGHT" ] || [ "$HEIGHT" = "null" ]; then
    echo "❌ C-Chain not responding yet"
    echo ""
    echo "Checking logs for errors..."
    tail -20 "$DATA_DIR/luxd.log" | grep -i "error\|warn\|fail" || true
else
    # Convert hex to decimal
    HEIGHT_DEC=$((16#${HEIGHT:2}))
    echo "✅ C-Chain block height: $HEIGHT_DEC"
    
    if [ "$HEIGHT_DEC" -gt 0 ]; then
        echo ""
        echo "SUCCESS! C-Chain is running with migrated data!"
        
        # Get more details
        echo ""
        echo "Getting blockchain details..."
        
        # Get genesis block
        GENESIS=$(curl -s -X POST -H "Content-Type: application/json" \
            -d '{"jsonrpc":"2.0","method":"eth_getBlockByNumber","params":["0x0", false],"id":1}' \
            http://localhost:$HTTP_PORT/ext/bc/C/rpc 2>/dev/null | jq -r '.result.hash' 2>/dev/null)
        echo "  Genesis hash: $GENESIS"
        
        # Get latest block
        LATEST=$(curl -s -X POST -H "Content-Type: application/json" \
            -d '{"jsonrpc":"2.0","method":"eth_getBlockByNumber","params":["latest", false],"id":1}' \
            http://localhost:$HTTP_PORT/ext/bc/C/rpc 2>/dev/null)
        
        if [ ! -z "$LATEST" ]; then
            echo ""
            echo "Latest block info:"
            echo "$LATEST" | jq '.result | {number, hash, timestamp, gasUsed, miner}'
        fi
        
        # Check a known address balance
        echo ""
        echo "Checking known address balance..."
        BALANCE=$(curl -s -X POST -H "Content-Type: application/json" \
            -d '{"jsonrpc":"2.0","method":"eth_getBalance","params":["0x0100000000000000000000000000000000000000", "latest"],"id":1}' \
            http://localhost:$HTTP_PORT/ext/bc/C/rpc 2>/dev/null | jq -r '.result' 2>/dev/null)
        
        if [ ! -z "$BALANCE" ] && [ "$BALANCE" != "null" ] && [ "$BALANCE" != "0x0" ]; then
            BALANCE_DEC=$((16#${BALANCE:2}))
            BALANCE_LUX=$(echo "scale=4; $BALANCE_DEC / 1000000000000000000" | bc -l)
            echo "  Fee collector (0x0100...0000): $BALANCE_LUX LUX"
        fi
    else
        echo "⚠️  C-Chain height is 0 - database may not be loaded correctly"
    fi
fi

echo ""
echo "============================================================"
echo "Node PID: $LUXD_PID"
echo "Data directory: $DATA_DIR"
echo ""
echo "Commands:"
echo "  Stop node: kill $LUXD_PID"
echo "  View logs: tail -f $DATA_DIR/luxd.log"
echo "  Check health: curl http://localhost:$HTTP_PORT/ext/health"
echo "  C-Chain RPC: http://localhost:$HTTP_PORT/ext/bc/C/rpc"
echo "============================================================"