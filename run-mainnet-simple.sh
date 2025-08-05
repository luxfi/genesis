#!/bin/bash
# Simple mainnet runner that creates a minimal genesis to bypass BLS issues
set -e

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
NODE_DIR="${SCRIPT_DIR}/../node"
RUN_DIR="${SCRIPT_DIR}/runs/mainnet-simple"
GENESIS_DB="${SCRIPT_DIR}/state/chaindata/lux-mainnet-96369"

echo "=== Lux Mainnet Simple Runner ==="
echo "This bypasses BLS validation to get a working node"
echo ""

# Clean and create directories
rm -rf "$RUN_DIR"
mkdir -p "$RUN_DIR/genesis"

# Create a minimal genesis.json without BLS validators
cat > "$RUN_DIR/genesis/genesis.json" <<'EOF'
{
  "networkID": 96369,
  "allocations": [
    {
      "ethAddr": "0x8db97C7cEcE249c2b98bDC0226Cc4C2A57BF52FC",
      "luxAddr": "X-lux18jma8ppw3nhx5r4ap8clazz0dps7rv5u00z96u",
      "initialAmount": 300000000000000000,
      "unlockSchedule": []
    }
  ],
  "startTime": 1607148900,
  "initialStakeDuration": 31536000,
  "initialStakeDurationOffset": 5400,
  "initialStakedFunds": ["X-lux18jma8ppw3nhx5r4ap8clazz0dps7rv5u00z96u"],
  "initialStakers": [
    {
      "nodeID": "NodeID-111111111111111111116DBWJs",
      "rewardAddress": "X-lux18jma8ppw3nhx5r4ap8clazz0dps7rv5u00z96u",
      "delegationFee": 20000
    }
  ],
  "cChainGenesis": "{\"config\":{\"chainId\":96369,\"homesteadBlock\":0,\"eip150Block\":0,\"eip150Hash\":\"0x2086799aeebeae135c246c65021c82b4e15a2c451340993aacfd2751886514f0\",\"eip155Block\":0,\"eip158Block\":0,\"byzantiumBlock\":0,\"constantinopleBlock\":0,\"petersburgBlock\":0,\"istanbulBlock\":0,\"muirGlacierBlock\":0,\"apricotPhase1BlockTimestamp\":0,\"apricotPhase2BlockTimestamp\":0},\"nonce\":\"0x0\",\"timestamp\":\"0x0\",\"extraData\":\"0x00\",\"gasLimit\":\"0x5f5e100\",\"difficulty\":\"0x0\",\"mixHash\":\"0x0000000000000000000000000000000000000000000000000000000000000000\",\"coinbase\":\"0x0000000000000000000000000000000000000000\",\"alloc\":{\"8db97C7cEcE249c2b98bDC0226Cc4C2A57BF52FC\":{\"balance\":\"0x295BE96E64066972000000\"}},\"number\":\"0x0\",\"gasUsed\":\"0x0\",\"parentHash\":\"0x0000000000000000000000000000000000000000000000000000000000000000\",\"baseFeePerGas\":null}",
  "message": "Lux Mainnet Genesis - Simplified for testing"
}
EOF

# Build luxd if needed
if [ ! -f "$NODE_DIR/build/luxd" ]; then
    echo "Building luxd..."
    cd "$NODE_DIR"
    make build
fi

# Launch with the simple genesis
cd "$NODE_DIR"
echo ""
echo "Starting luxd with simplified genesis..."
echo "  - Network: 96369 (mainnet)"
echo "  - RPC: http://localhost:9630/ext/bc/C/rpc"
echo ""

exec ./build/luxd \
    --network-id=96369 \
    --data-dir="$RUN_DIR/data" \
    --genesis-file="$RUN_DIR/genesis/genesis.json" \
    --db-type=badgerdb \
    --c-chain-db-type=badgerdb \
    --p-chain-db-type=badgerdb \
    --x-chain-db-type=badgerdb \
    --http-host=0.0.0.0 \
    --http-port=9630 \
    --staking-port=9631 \
    --log-level=info \
    --api-admin-enabled=true \
    --sybil-protection-enabled=false \
    --consensus-sample-size=1 \
    --consensus-quorum-size=1 \
    --consensus-commit-threshold=1 \
    --consensus-concurrent-repolls=1 \
    --consensus-optimal-processing=1 \
    --consensus-max-processing=1 \
    --consensus-max-time-processing=2s