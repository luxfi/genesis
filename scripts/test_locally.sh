#!/bin/bash
set -e

# Test the complete pipeline locally
# Simulates what happens in CI

echo "==================================="
echo "  LOCAL CI TEST"
echo "==================================="
echo ""

# Use existing migrated data for testing
export SOURCE_DB="/home/z/.luxd/chainData/xBBY6aJcNichNCkCXgUcG5Gv2PW6FLS81LYDV8VwnPuadKGqm/ethdb"
export DATA_DIR="/tmp/test_luxd"

# Clean test directory
rm -rf "$DATA_DIR"
mkdir -p "$DATA_DIR"

echo "Running CI pipeline with local data..."
echo "Source: $SOURCE_DB"
echo "Destination: $DATA_DIR"
echo ""

# Run the pipeline
"$(dirname "$0")/ci_pipeline.sh"

echo ""
echo "Test complete! Check $DATA_DIR for results."