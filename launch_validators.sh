#!/bin/bash

# Lux Network Validator Launcher
# Supports 1 to N validators with automatic consensus configuration

set -e

# Default values
NUM_VALIDATORS=1
NETWORK_ID=96369
DATA_DIR="/tmp/lux-validators"
LUXD_PATH="/Users/z/work/lux/node/build/luxd"
MIGRATED_DB="/tmp/lux-mainnet-final/chainData/2XpgdN3WNtM6AuzGgnXW7S6BqbH7DYY8CKwqaUiDUj67vYGvfC/db"

# Parse arguments
while [[ $# -gt 0 ]]; do
  case $1 in
    -n|--nodes)
      NUM_VALIDATORS="$2"
      shift 2
      ;;
    -d|--data-dir)
      DATA_DIR="$2"
      shift 2
      ;;
    --network-id)
      NETWORK_ID="$2"
      shift 2
      ;;
    --luxd)
      LUXD_PATH="$2"
      shift 2
      ;;
    --db)
      MIGRATED_DB="$2"
      shift 2
      ;;
    -h|--help)
      echo "Usage: $0 [options]"
      echo ""
      echo "Options:"
      echo "  -n, --nodes NUM        Number of validators (default: 1)"
      echo "  -d, --data-dir DIR     Data directory (default: /tmp/lux-validators)"
      echo "  --network-id ID        Network ID (default: 96369)"
      echo "  --luxd PATH           Path to luxd binary"
      echo "  --db PATH             Path to migrated database"
      echo "  -h, --help            Show this help message"
      echo ""
      echo "Examples:"
      echo "  $0 -n 1               # Single validator (K=1)"
      echo "  $0 -n 2               # Two validators (K=2)"
      echo "  $0 -n 5               # Five validators (K=5)"
      echo "  $0 -n 11              # Testnet config (K=11)"
      echo "  $0 -n 21              # Mainnet config (K=21)"
      exit 0
      ;;
    *)
      echo "Unknown option: $1"
      exit 1
      ;;
  esac
done

# Display configuration
echo "======================================"
echo "  LUX VALIDATOR NETWORK LAUNCHER"
echo "======================================"
echo ""
echo "Configuration:"
echo "  Validators:  $NUM_VALIDATORS"
echo "  Network ID:  $NETWORK_ID"
echo "  Data Dir:    $DATA_DIR"
echo "  Luxd:        $LUXD_PATH"
echo "  Database:    $MIGRATED_DB"
echo ""

# Check prerequisites
if [ ! -f "$LUXD_PATH" ]; then
  echo "ERROR: luxd binary not found at $LUXD_PATH"
  echo "Build it with: cd /Users/z/work/lux/node && make"
  exit 1
fi

if [ ! -d "$MIGRATED_DB" ]; then
  echo "ERROR: Migrated database not found at $MIGRATED_DB"
  echo "Run the migration first with the genesis tool"
  exit 1
fi

# Clean up old processes
echo "Stopping any existing luxd processes..."
pkill -f luxd 2>/dev/null || true
sleep 2

# Build the netrunner tool if needed
NETRUNNER_TOOL="/Users/z/work/lux/genesis/cmd/netrunner/netrunner"
if [ ! -f "$NETRUNNER_TOOL" ]; then
  echo "Building netrunner tool..."
  cd /Users/z/work/lux/genesis/cmd/netrunner
  go build -o netrunner .
  cd -
fi

# Launch the network
echo "Launching $NUM_VALIDATORS validator(s)..."
$NETRUNNER_TOOL \
  -validators $NUM_VALIDATORS \
  -network-id $NETWORK_ID \
  -data-dir "$DATA_DIR" \
  -luxd "$LUXD_PATH" \
  -db "$MIGRATED_DB"

# Display next steps
echo ""
echo "======================================"
echo "  NETWORK LAUNCHED"
echo "======================================"
echo ""
echo "Monitor logs:"
echo "  tail -f $DATA_DIR/node01/node.log"
echo ""
echo "Check status:"
echo "  curl -X POST http://127.0.0.1:9650/ext/info -H 'Content-Type: application/json' -d '{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"info.peers\"}'"
echo ""
echo "Check C-Chain:"
echo "  curl -X POST http://127.0.0.1:9650/ext/bc/C/rpc -H 'Content-Type: application/json' -d '{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"eth_blockNumber\"}'"
echo ""
echo "Stop all nodes:"
echo "  pkill -f luxd"