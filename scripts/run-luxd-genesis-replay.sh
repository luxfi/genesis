#!/bin/bash

echo "============================================================="
echo "       LUX C-Chain Genesis Database Replay                  "
echo "============================================================="
echo ""

# Kill any existing luxd
pkill -f luxd
sleep 2

LUXD_BIN="/home/z/work/lux/node/build/luxd"
GENESIS_DB="/home/z/work/lux/state/chaindata/lux-mainnet-96369/db/pebbledb"
DATA_DIR="/tmp/luxd-genesis-replay"
NETWORK_ID=96369

# Clean data directory for fresh start
rm -rf $DATA_DIR
mkdir -p $DATA_DIR

echo "Configuration:"
echo "  Binary:      $LUXD_BIN"
echo "  Genesis DB:  $GENESIS_DB"
echo "  Data Dir:    $DATA_DIR"
echo "  Network ID:  $NETWORK_ID"
echo ""

echo "Starting luxd with genesis database replay..."
echo "============================================="

# Start luxd with genesis-db flag pointing to our subnet-evm database
$LUXD_BIN \
    --genesis-db="$GENESIS_DB" \
    --genesis-db-type=pebbledb \
    --network-id=$NETWORK_ID \
    --data-dir=$DATA_DIR \
    --http-host=0.0.0.0 \
    --http-port=9630 \
    --staking-port=9631 \
    --sybil-protection-enabled=false \
    --sybil-protection-disabled-weight=100 \
    --log-level=info &

PID=$!
echo "luxd started with PID $PID"
echo ""

# Wait for startup
echo "Waiting for luxd to initialize (15 seconds)..."
sleep 15

echo ""
echo "Checking C-Chain block height..."
echo "================================="

HEIGHT=$(curl -s -X POST -H "Content-Type: application/json" \
    -d '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' \
    http://localhost:9630/ext/bc/C/rpc 2>/dev/null | jq -r '.result' 2>/dev/null)

if [ -z "$HEIGHT" ]; then
    echo "❌ Failed to get block height - C-chain may not be ready yet"
else
    # Convert hex to decimal
    HEIGHT_DEC=$((16#${HEIGHT:2}))
    echo "✅ C-Chain block height: $HEIGHT_DEC"
    
    if [ $HEIGHT_DEC -gt 0 ]; then
        echo ""
        echo "🎉 SUCCESS! Genesis database replay worked!"
        echo "   C-Chain has $HEIGHT_DEC blocks loaded from historic data!"
        
        # Try to get a specific block
        echo ""
        echo "Testing block retrieval (block 0)..."
        curl -s -X POST -H "Content-Type: application/json" \
            -d '{"jsonrpc":"2.0","method":"eth_getBlockByNumber","params":["0x0", false],"id":1}' \
            http://localhost:9630/ext/bc/C/rpc | jq '.result | {number, hash, timestamp}'
    else
        echo ""
        echo "⚠️  Chain height is 0 - genesis replay may not have worked"
        echo "   Check logs at: tail -f $DATA_DIR/logs/main.log"
    fi
fi

echo ""
echo "============================================================="
echo "luxd is running with PID $PID"
echo "To stop: kill $PID"
echo "To view logs: tail -f $DATA_DIR/logs/main.log"
echo "============================================================="