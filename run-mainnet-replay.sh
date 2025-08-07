#!/bin/bash

# Lux Mainnet Replay Script
# Runs luxd with historic chaindata replay and proper mainnet settings

set -e

echo "=== Lux Mainnet Replay Tool ==="
echo "This script launches luxd with historic chaindata replay"
echo ""

# Configuration
LUXD_BIN="/Users/z/work/lux/node/build/luxd"
GENESIS_DB="/Users/z/work/lux/state/chaindata/lux-mainnet-96369/db/pebbledb"
NETWORK_ID=96369
DATA_DIR="/tmp/lux-mainnet-replay-$$"
HTTP_PORT=9630
STAKING_PORT=9631

# Check if luxd exists
if [ ! -f "$LUXD_BIN" ]; then
    echo "Error: luxd not found at $LUXD_BIN"
    echo "Please build luxd first: cd /Users/z/work/lux/node && make"
    exit 1
fi

# Check if genesis database exists
if [ ! -d "$GENESIS_DB" ]; then
    echo "Error: Genesis database not found at $GENESIS_DB"
    exit 1
fi

# Clean up any existing luxd processes
echo "Stopping any existing luxd processes..."
pkill -f luxd || true
sleep 2

# Create data directory
echo "Creating data directory: $DATA_DIR"
mkdir -p "$DATA_DIR"
mkdir -p "$DATA_DIR/staking-keys"
mkdir -p "$DATA_DIR/configs/chains/C"

# Generate staking keys (using test keys for now)
echo "Setting up staking keys..."
cat > "$DATA_DIR/staking-keys/staker.crt" << 'EOF'
-----BEGIN CERTIFICATE-----
MIIFXTCCBEWgAwIBAgIJALxDXLLAsW9sMA0GCSqGSIb3DQEBCwUAMIGkMQswCQYD
VQQGEwJVUzELMAkGA1UECAwCQ0ExFjAUBgNVBAcMDVNhbiBGcmFuY2lzY28xGjAY
BgNVBAoMEUx1eCBJbmR1c3RyaWVzIEluYzEOMAwGA1UECwwFTm9kZXMxFDASBgNV
BAMMC2x1eC5uZXR3b3JrMS4wLAYJKoZIhvcNAQkBFh9hZG1pbkBsdXguaW5kdXN0
cmllcy5jb20uY29tLmNvbTAeFw0yNDAxMDEwMDAwMDBaFw0zNDAxMDEwMDAwMDBa
MIGkMQswCQYDVQQGEwJVUzELMAkGA1UECAwCQ0ExFjAUBgNVBAcMDVNhbiBGcmFu
Y2lzY28xGjAYBgNVBAoMEUx1eCBJbmR1c3RyaWVzIEluYzEOMAwGA1UECwwFTm9k
ZXMxFDASBgNVBAMMC2x1eC5uZXR3b3JrMS4wLAYJKoZIhvcNAQkBFh9hZG1pbkBs
dXguaW5kdXN0cmllcy5jb20uY29tLmNvbTCCASIwDQYJKoZIhvcNAQEBBQADggEP
ADCCAQoCggEBAKaOPR+Isv/2L5xGPe0RqJGKJQBN8iVlr8dE/A7sM2fD8wZWvF6k
2hEPEGnJsqQKJY3wtGkH5e0L8gw6XKCLlbEfgh9J4K9lXaGPJ5nKLZ1A8YX6MnoK
MDc1p8K4bVDqNVYmDKq+LMqVnMnnF7vkLTKrZMqCbnJzgeHlvCkDW7EnSl4X8iKc
ECcc8wqGfLpVDMKvsFg5JAYHHnVqEqpM+3yG9gL+1SH2e8bFHGLMm1xY7PNPW2RY
GBqQmEtkifGnHH3rJ2gNMHSLmqNEj0sFCEhCNB2y9mT7Zc7pFjSGcwEMB/fMEYQl
w/3/x3ryGGUdHZvtJIVlm/aqKeH6EqCxoqMCAwEAAaOCAUIwggE+MHQGA1UdIwRt
MGuAFKaOPR+Isv/2L5xGPe0RqJGKJQBNoYGqpIGnMIGkMQswCQYDVQQGEwJVUzEL
MAkGA1UECAwCQ0ExFjAUBgNVBAcMDVNhbiBGcmFuY2lzY28xGjAYBgNVBAoMEUx1
eCBJbmR1c3RyaWVzIEluYzEOMAwGA1UECwwFTm9kZXMxFDASBgNVBAMMC2x1eC5u
ZXR3b3JrMS4wLAYJKoZIhvcNAQkBFh9hZG1pbkBsdXguaW5kdXN0cmllcy5jb20u
Y29tLmNvbYIJALxDXLLAsW9sMB0GA1UdDgQWBBSmjj0fiLL/9i+cRj3tEaiRiiUA
TTAMBGA1UdEwEB/wQCMAAwCwYDVR0PBAQDAgWgMB0GA1UdJQQWMBQGCCsGAQUF
BwMBBggrBgEFBQcDAjANBgkqhkiG9w0BAQsFAAOCAQEApnhyLLlFT2p5Rb2lAvYJ
HXJQqV5sj4cJXHF7Sx4LfQMQVJXRG2gNATA9fJX8lLhJ5cAVSNWt0Mz7u5Q5Y/HS
AfBM/yqxaelKscBvyhI0KX2XzCqJgzSmbI0Iy3S7gQPOqN/A/nSpq7X5z+X/TL0q
t8RoTPvLqCJW7uH3cWEQ+Ev5z7IiIBFnKYVJb/w+VnhP0xfPqv1bpucXxJ0Rp3IU
EBVLf1prcakEqhezRZKCJ9VdyqVL+YbBPE8qjEJVsLGVCB8LqVHgEGHHUL7r6Nha
KN/VDHDdVLttTFBXkMfdVVVP9jOgB3cJ+FxMgJhG0E5Ks5PhqVV1qSGXGCMFRBoc
0w==
-----END CERTIFICATE-----
EOF

cat > "$DATA_DIR/staking-keys/staker.key" << 'EOF'
-----BEGIN RSA PRIVATE KEY-----
MIIEowIBAAKCAQEApo49H4iy//YvnEY97RGokYolAE3yJWWvx0T8DuwzZ8PzBla8
XqTaEQ8Qacmy pAoljfC0aQfl7QvyDDpcoIuVsR+CH0ngr2VdoY8nmcotnUDxhfoy
egowNzWnwrhtUOo1ViYMqr4sypWcyecXu+QtMqtkyoJucnOB4eW8KQNbsSdKXhfy
IpwQJxzzCoZ8ulUMwq+wWDkkBgcedWoSqkz7fIb2Av7VIfZ7xsUcYsybXFjs809b
ZFgYGpCYS2SJ8accfesnaA0wdIuao0SPSwUISEI0HbL2ZPtlzukWNIZzAQwH98wR
hCXD/f/HevIYZR0dm+0khWWb9qop4foSoLGiowIDAQABAoIBAAv4tTp6WZA8MNgC
2IDRw37XS0vVVDvEjtNJQvApMU7MNeVXjF0xw1qX5drgMj1kVzHDEfSGm4LR8xKC
2Y9sHJAVEdPJ+julWCVsLGPHaVyRByGGDM5K7gV+3+cLQC8bE2u2lAQI2ZCVnbRP
IzOy3aFMD8iYKpF5f8kNGnPsSncVTmJZFaGG9YiB0sTTi4gM3kV0a8xV1C0QuPB0
vgysMh7cFKsh5inH5AsplKHTNNhbBll2JjMBuXWI/8cKjDLBvFngAZ0zLIFVcdcp
HfkPpQBXDcVJXCOI2sGhB4oPExdGY3gPKK2OhHkVlpRQlBCbCJFqGuj72Z8daPJC
zPWPHfECgYEA10f5Y55xLaC1NzVA1wYLLThLjI2KcsEFhKbPCkPU9pCVHvxnLnaD
PusHUBvBHVTS/8m9LRSrQYRqW+KG0F7Yte+eSNpC5pLHBNlTj7Y3z9SQp70kEWGP
Y7OVTdQNs+6Cqeb5ydQS8hzqTXl2shFaGBSJxWjD0vCPNEUFMZTdL38CgYEAxiMb
0sjQ/2iZ4I0xITqkGvFVPksMn9cqxJGMDmPNsMCbSLwxKU0sPwhZVzLvXQqqUQyn
qPuI3J4y/U7VVLLPBDoHSLOQlXaVQQxPU5c5fmOab8U/xFT9SVR7dubPLNTjzBAG
cF8TDyUgKKx2IlBf7QKIBYV6jMQRUNkK7CaDP00CgYB5SdsKdFg5cgvEWg8jFNV3
+fqCqyLCXRjR3cD/DNLM6EQx+idLZqLjGDPMGG+LDVDWBLFdvLrwCKHFfnsLHhGq
P0EfSGlrDHWBvGP4DIjlk0+Ckb0FGSLa4qX5rCYR0LfXYp4yrQcrD9xVPryqOSNE
V/wSVPXO4F7uGpn5qPlvXQKBgGtLXJhKSPcOYTxuB3lFObbmvYHpBhLWO4r1gUBH
B7MLpoFp7vvzwR3bF3F9D5uYgEhF0TwxJGSqGGXq6BxFNl7crHIUuNkpEkxA9LcH
3fPsIEPpGiLokc5roGJ/VLvNn0xBGemLVJpz1ovBVmGR2gPAI1pxWVMZI6mQ0pRO
7vBhAoGBAKrYJZkDotLGqSjJJPMlQqpJG9m4KqYMfMZjBGpSXx/dVfNYYUvX9nNR
w9xIclkiKXtvM0rFD7dYNKKKL5fVVBqHSKkAcps7z0u/4fs5SUKz8LqxjZLg7T9Z
i5P1XJJqR4GzJCZlxR4d4jpWB9KBgPk4AcPcQNPJvF5gX3MLZBxC
-----END RSA PRIVATE KEY-----
EOF

# Create a simple BLS signer key (test key)
echo "QXZhbGFuY2hlTG9jYWxOZXR3b3JrVmFsaWRhdG9yMDE=" | base64 -d > "$DATA_DIR/staking-keys/signer.key"

# Create C-chain config
cat > "$DATA_DIR/configs/chains/C/config.json" << EOF
{
  "db-type": "badgerdb",
  "log-level": "info",
  "state-sync-enabled": false,
  "offline-pruning-enabled": false,
  "allow-unprotected-txs": true
}
EOF

# Create genesis.json for single-node operation
cat > "$DATA_DIR/genesis.json" << 'EOF'
{
  "networkID": 96369,
  "allocations": [
    {
      "ethAddr": "0xb3d82b1367d362de99ab59a658165aff520cbd4d",
      "luxAddr": "lux1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqzenvla",
      "initialAmount": 333333333333333333,
      "unlockSchedule": []
    }
  ],
  "startTime": 1630987200,
  "initialStakeDuration": 31536000,
  "initialStakeDurationOffset": 5400,
  "initialStakedFunds": [
    "lux1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqzenvla"
  ],
  "initialStakers": [
    {
      "nodeID": "NodeID-7Xhw2mDxuDS44j42TCB6U5579esbSt3Lg",
      "rewardAddress": "lux1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqzenvla",
      "delegationFee": 20000,
      "signer": {
        "publicKey": "0x900c9b119b5c82d781d4b49be78c3fc7ae65f2b435b7ed9e3a8b9a03e475edff86d8a64827fec8db23a6f236afbf127d",
        "proofOfPossession": "0x9239f365a639849730078382d2f060c4d98cb02ad24fe8aad573ac10d317c6be004846ac11080569b12dbb2f34044dcf17c8d1c4bb3494fc62929bcb87e476a19bb51cdfe7882c899762100180e0122c64ca962816f6cbf67f852162295c19ed"
      }
    }
  ],
  "cChainGenesis": "{\"config\":{\"chainId\":96369,\"homesteadBlock\":0,\"eip150Block\":0,\"eip150Hash\":\"0x2086799aeebeae135c246c65021c82b4e15a2c451340993aacfd2751886514f0\",\"eip155Block\":0,\"eip158Block\":0,\"byzantiumBlock\":0,\"constantinopleBlock\":0,\"petersburgBlock\":0,\"istanbulBlock\":0,\"muirGlacierBlock\":0,\"subnetEVMTimestamp\":0},\"nonce\":\"0x0\",\"timestamp\":\"0x0\",\"extraData\":\"0x00\",\"gasLimit\":\"0x7A1200\",\"difficulty\":\"0x0\",\"mixHash\":\"0x0000000000000000000000000000000000000000000000000000000000000000\",\"coinbase\":\"0x0000000000000000000000000000000000000000\",\"alloc\":{\"0100000000000000000000000000000000000000\":{\"code\":\"0x7300000000000000000000000000000000000000003014608060405260043610603d5760003560e01c80631e010439146042578063b6510bb314606e575b600080fd5b605c60048036036020811015605657600080fd5b503560b1565b60408051918252519081900360200190f35b818015607957600080fd5b5060af60048036036080811015608e57600080fd5b506001600160a01b03813516906020810135906040810135906060013560b6565b005b30cd90565b836001600160a01b031681836108fc8690811502906040516000604051808303818888878c8acf9550505050505015801560f4573d6000803e3d6000fd5b505050505056fea26469706673582212201eebce970fe3f5cb96bf8ac6ba5f5c133fc2908ae3dcd51082cfee8f583429d064736f6c634300060a0033\",\"balance\":\"0x0\"}},\"number\":\"0x0\",\"gasUsed\":\"0x0\",\"parentHash\":\"0x0000000000000000000000000000000000000000000000000000000000000000\"}",
  "message": "mainnet genesis with replay"
}
EOF

echo ""
echo "Configuration:"
echo "  Binary: $LUXD_BIN"
echo "  Genesis DB: $GENESIS_DB"
echo "  Network ID: $NETWORK_ID"
echo "  Data Dir: $DATA_DIR"
echo "  HTTP Port: $HTTP_PORT"
echo "  Staking Port: $STAKING_PORT"
echo ""

# Launch luxd with genesis database replay
echo "Starting luxd with genesis database replay..."
echo "============================================="

$LUXD_BIN \
    --network-id=$NETWORK_ID \
    --genesis-file="$DATA_DIR/genesis.json" \
    --genesis-db="$GENESIS_DB" \
    --genesis-db-type=pebbledb \
    --data-dir="$DATA_DIR/data" \
    --db-type=badgerdb \
    --chain-config-dir="$DATA_DIR/configs/chains" \
    --staking-tls-cert-file="$DATA_DIR/staking-keys/staker.crt" \
    --staking-tls-key-file="$DATA_DIR/staking-keys/staker.key" \
    --staking-signer-key-file="$DATA_DIR/staking-keys/signer.key" \
    --http-host=0.0.0.0 \
    --http-port=$HTTP_PORT \
    --staking-port=$STAKING_PORT \
    --log-level=info \
    --api-admin-enabled=true \
    --api-keystore-enabled=true \
    --api-metrics-enabled=true \
    --sybil-protection-enabled=false \
    --sybil-protection-disabled-weight=100 \
    --health-check-frequency=2s \
    --network-max-reconnect-delay=1s \
    --consensus-sample-size=1 \
    --consensus-quorum-size=1 \
    --consensus-commit-threshold=1 \
    --consensus-concurrent-repolls=1 \
    --consensus-optimal-processing=1 \
    --consensus-max-processing=1 \
    --consensus-max-time-processing=2s &

PID=$!
echo ""
echo "luxd started with PID $PID"
echo ""

# Wait for startup
echo "Waiting for luxd to initialize (20 seconds)..."
sleep 20

echo ""
echo "Checking C-Chain status..."
echo "================================="

# Check C-Chain block height
HEIGHT=$(curl -s -X POST -H "Content-Type: application/json" \
    -d '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' \
    http://localhost:$HTTP_PORT/ext/bc/C/rpc 2>/dev/null | jq -r '.result' 2>/dev/null)

if [ -z "$HEIGHT" ] || [ "$HEIGHT" = "null" ]; then
    echo "❌ Failed to get block height - C-chain may not be ready yet"
    echo "   Check logs at: tail -f $DATA_DIR/data/logs/main.log"
else
    # Convert hex to decimal
    HEIGHT_DEC=$((16#${HEIGHT:2}))
    echo "✅ C-Chain block height: $HEIGHT_DEC"

    if [ $HEIGHT_DEC -gt 0 ]; then
        echo ""
        echo "🎉 SUCCESS! Genesis database replay worked!"
        echo "   C-Chain has $HEIGHT_DEC blocks loaded from historic data!"
        
        # Get genesis block
        echo ""
        echo "Genesis block info:"
        curl -s -X POST -H "Content-Type: application/json" \
            -d '{"jsonrpc":"2.0","method":"eth_getBlockByNumber","params":["0x0", false],"id":1}' \
            http://localhost:$HTTP_PORT/ext/bc/C/rpc | jq '.result | {number, hash, timestamp}'
    else
        echo ""
        echo "⚠️  Chain height is 0 - genesis replay may not have worked"
    fi
fi

echo ""
echo "============================================================="
echo "luxd is running with PID $PID"
echo "To stop: kill $PID"
echo "To view logs: tail -f $DATA_DIR/data/logs/main.log"
echo "Data directory: $DATA_DIR"
echo "============================================================="