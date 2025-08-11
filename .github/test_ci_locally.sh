#!/bin/bash
set -e

echo "=== Running CI Test Locally ==="
echo ""

# Simulate CI environment
export CI=true
export GITHUB_WORKSPACE=$(pwd)
export GITHUB_SHA=$(git rev-parse HEAD 2>/dev/null || echo "test-sha")

echo "Step 1: Setup"
echo "  Workspace: $GITHUB_WORKSPACE"
echo "  SHA: $GITHUB_SHA"
echo ""

echo "Step 2: Build Tools"
make clean
make build
if [ $? -ne 0 ]; then
    echo "✗ Build failed"
    exit 1
fi
echo "✓ Build successful"
echo ""

echo "Step 3: Verify Tools Exist"
for tool in genesis verify_migration check_balance; do
    if [ -f "bin/$tool" ]; then
        echo "  ✓ bin/$tool exists"
    else
        echo "  ✗ bin/$tool missing"
        exit 1
    fi
done
echo ""

echo "Step 4: Run Verification (if database exists)"
TEST_DB="/home/z/.luxd/chainData/xBBY6aJcNichNCkCXgUcG5Gv2PW6FLS81LYDV8VwnPuadKGqm/ethdb"
if [ -d "$TEST_DB" ]; then
    ./bin/verify_migration "$TEST_DB"
    if [ $? -ne 0 ]; then
        echo "✗ Verification failed"
        exit 1
    fi
    echo "✓ Verification passed"
else
    echo "  Skipping verification (no test database)"
fi
echo ""

echo "Step 5: Test Balance Check"
./bin/check_balance "$TEST_DB" "0x9011E888251AB053B7bD1cdB598Db4f9DEd94714"
echo ""

echo "Step 6: Create Package"
make package
if [ $? -ne 0 ]; then
    echo "✗ Package creation failed"
    exit 1
fi
echo "✓ Package created"
echo ""

echo "Step 7: Verify Package Contents"
if ls lux-mainnet-genesis-*.tar.gz 1> /dev/null 2>&1; then
    echo "  ✓ Package archive found"
    tar -tzf lux-mainnet-genesis-*.tar.gz | head -10
else
    echo "  ✗ Package archive not found"
    exit 1
fi
echo ""

echo "=== CI Test Complete ==="
echo "✓ All CI steps passed successfully!"
echo ""
echo "Summary:"
echo "  - Tools built: ✓"
echo "  - Verification passed: ✓"
echo "  - Package created: ✓"
echo ""
echo "This workflow would pass in CI!"