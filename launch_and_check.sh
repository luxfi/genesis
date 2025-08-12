#!/bin/bash
set -e

echo "============================================"
echo "  LUX MAINNET VALIDATOR LAUNCH & CHECK"
echo "============================================"
echo ""

# Configuration
LUXD_BIN="/Users/z/work/lux/node/build/luxd"
MIGRATED_DB="/tmp/lux-badger-migration"
DATA_DIR="/tmp/lux-mainnet-validator-$$"
HTTP_PORT=9650
STAKING_PORT=9651
NETWORK_ID=96369

# Kill any existing luxd
echo "Stopping any existing luxd processes..."
pkill -f luxd 2>/dev/null || true
sleep 2

# Create data directory structure
echo "Setting up data directory: $DATA_DIR"
mkdir -p "$DATA_DIR/chainData"
mkdir -p "$DATA_DIR/db"

# Setup chain data for C-chain
CHAIN_ID="2f9gWKiw8VTE29NbiA6kUmETi6Rz8ikk8tUbaHEdhft7X8BvQo"
CHAIN_DIR="$DATA_DIR/chainData/$CHAIN_ID"
mkdir -p "$CHAIN_DIR"
ln -s "$MIGRATED_DB" "$CHAIN_DIR/ethdb"

echo "Starting validator node..."
$LUXD_BIN \
    --network-id=$NETWORK_ID \
    --data-dir="$DATA_DIR" \
    --db-dir="$DATA_DIR/db" \
    --chain-data-dir="$DATA_DIR/chainData" \
    --http-host=0.0.0.0 \
    --http-port=$HTTP_PORT \
    --staking-port=$STAKING_PORT \
    --staking-ephemeral-cert-enabled=true \
    --staking-ephemeral-signer-enabled=true \
    --bootstrap-ips="" \
    --bootstrap-ids="" \
    --log-level=info \
    --api-admin-enabled=true \
    --health-check-frequency=2s \
    --network-max-reconnect-delay=1s \
    --sybil-protection-enabled=false \
    --sybil-protection-disabled-weight=100 > "$DATA_DIR/luxd.log" 2>&1 &

LUXD_PID=$!
echo "luxd started with PID $LUXD_PID"
echo "Waiting for node to initialize (20 seconds)..."
sleep 20

# Get NodeID
echo ""
echo "=== NODE INFORMATION ==="
NODE_INFO=$(curl -s -X POST --data '{
    "jsonrpc":"2.0",
    "id":1,
    "method":"info.getNodeID"
}' -H 'content-type:application/json' http://localhost:$HTTP_PORT/ext/info 2>/dev/null)

NODE_ID=$(echo "$NODE_INFO" | jq -r '.result.nodeID' 2>/dev/null)
echo "NodeID: $NODE_ID"

# Get Node version
NODE_VERSION=$(curl -s -X POST --data '{
    "jsonrpc":"2.0",
    "id":1,
    "method":"info.getNodeVersion"
}' -H 'content-type:application/json' http://localhost:$HTTP_PORT/ext/info 2>/dev/null | jq -r '.result')
echo "Version: $NODE_VERSION"

# Check validator's addresses and balances
echo ""
echo "=== VALIDATOR ADDRESSES & BALANCES ==="

# Known test addresses to check
TEST_ADDRESSES=(
    "0x9011E888251AB053B7bD1cdB598Db4f9DEd94714"
    "0xb3d82b1367d362de99ab59a658165aff520cbd4d"
    "0x0100000000000000000000000000000000000000"
)

# P-Chain check
echo ""
echo "P-Chain Status:"
echo "---------------"
P_HEIGHT=$(curl -s -X POST --data '{
    "jsonrpc":"2.0",
    "id":1,
    "method":"platform.getHeight"
}' -H 'content-type:application/json' http://localhost:$HTTP_PORT/ext/bc/P 2>/dev/null | jq -r '.result.height' 2>/dev/null)
echo "  Height: $P_HEIGHT"

# Check P-Chain balance for test addresses
P_ADDRESS="P-lux1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqzenvla"
P_BALANCE=$(curl -s -X POST --data "{
    \"jsonrpc\":\"2.0\",
    \"id\":1,
    \"method\":\"platform.getBalance\",
    \"params\":{
        \"addresses\":[\"$P_ADDRESS\"]
    }
}" -H 'content-type:application/json' http://localhost:$HTTP_PORT/ext/bc/P 2>/dev/null)
echo "  Address: $P_ADDRESS"
echo "  Balance: $(echo "$P_BALANCE" | jq -r '.result.balance' 2>/dev/null) nLUX"

# X-Chain check
echo ""
echo "X-Chain Status:"
echo "---------------"
X_HEIGHT=$(curl -s -X POST --data '{
    "jsonrpc":"2.0",
    "id":1,
    "method":"avm.getHeight"
}' -H 'content-type:application/json' http://localhost:$HTTP_PORT/ext/bc/X 2>/dev/null | jq -r '.result.height' 2>/dev/null)
echo "  Height: $X_HEIGHT"

# Check X-Chain balance
X_ADDRESS="X-lux1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqzenvla"
X_BALANCE=$(curl -s -X POST --data "{
    \"jsonrpc\":\"2.0\",
    \"id\":1,
    \"method\":\"avm.getBalance\",
    \"params\":{
        \"address\":\"$X_ADDRESS\",
        \"assetID\":\"LUX\"
    }
}" -H 'content-type:application/json' http://localhost:$HTTP_PORT/ext/bc/X 2>/dev/null)
echo "  Address: $X_ADDRESS"
echo "  Balance: $(echo "$X_BALANCE" | jq -r '.result.balance' 2>/dev/null) nLUX"

# C-Chain check
echo ""
echo "C-Chain Status:"
echo "---------------"
C_HEIGHT=$(curl -s -X POST -H "Content-Type: application/json" \
    -d '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' \
    http://localhost:$HTTP_PORT/ext/bc/C/rpc 2>/dev/null | jq -r '.result' 2>/dev/null)

if [ ! -z "$C_HEIGHT" ] && [ "$C_HEIGHT" != "null" ]; then
    HEIGHT_DEC=$((16#${C_HEIGHT:2}))
    echo "  Height: $HEIGHT_DEC"
    
    # Get genesis block
    GENESIS=$(curl -s -X POST -H "Content-Type: application/json" \
        -d '{"jsonrpc":"2.0","method":"eth_getBlockByNumber","params":["0x0", false],"id":1}' \
        http://localhost:$HTTP_PORT/ext/bc/C/rpc 2>/dev/null | jq -r '.result.hash' 2>/dev/null)
    echo "  Genesis: $GENESIS"
    
    # Check balances for all test addresses
    echo ""
    echo "  C-Chain Balances:"
    for addr in "${TEST_ADDRESSES[@]}"; do
        BALANCE=$(curl -s -X POST -H "Content-Type: application/json" \
            -d "{\"jsonrpc\":\"2.0\",\"method\":\"eth_getBalance\",\"params\":[\"$addr\", \"latest\"],\"id\":1}" \
            http://localhost:$HTTP_PORT/ext/bc/C/rpc 2>/dev/null | jq -r '.result' 2>/dev/null)
        
        if [ ! -z "$BALANCE" ] && [ "$BALANCE" != "null" ] && [ "$BALANCE" != "0x0" ]; then
            BALANCE_DEC=$((16#${BALANCE:2}))
            BALANCE_LUX=$(echo "scale=4; $BALANCE_DEC / 1000000000000000000" | bc -l)
            echo "    $addr: $BALANCE_LUX LUX"
        fi
    done
else
    echo "  C-Chain not ready yet"
fi

# Check total supply on C-Chain
echo ""
echo "=== C-CHAIN DETAILED INFO ==="
echo "Checking latest block..."
LATEST_BLOCK=$(curl -s -X POST -H "Content-Type: application/json" \
    -d '{"jsonrpc":"2.0","method":"eth_getBlockByNumber","params":["latest", false],"id":1}' \
    http://localhost:$HTTP_PORT/ext/bc/C/rpc 2>/dev/null)

if [ ! -z "$LATEST_BLOCK" ]; then
    echo "Latest block:" 
    echo "$LATEST_BLOCK" | jq '.result | {number, hash, timestamp, gasUsed, gasLimit}'
fi

echo ""
echo "============================================"
echo "Node PID: $LUXD_PID"
echo "Data Dir: $DATA_DIR"
echo "Logs: tail -f $DATA_DIR/luxd.log"
echo "To stop: kill $LUXD_PID"
echo "============================================"