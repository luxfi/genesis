#!/bin/bash

# Simplified luxd build script
# This attempts to build luxd with minimal features

echo "🔨 Building luxd with simplified configuration..."
echo "================================================"

cd ~/work/lux/node

# Clean any previous build attempts
rm -rf build/

# Try building with minimal tags and CGO disabled
echo "Attempting CGO_ENABLED=0 build..."
CGO_ENABLED=0 go build \
    -tags "badgerdb" \
    -ldflags "-X main.version=v1.13.4-custom" \
    -o build/luxd \
    ./main 2>&1 | tee build.log

if [ -f build/luxd ]; then
    echo "✅ Build successful!"
    echo "Binary location: $(pwd)/build/luxd"
    ./build/luxd --version
else
    echo "❌ Build failed. Checking errors..."
    
    # Count errors
    ERROR_COUNT=$(grep -c "error" build.log)
    echo "Found $ERROR_COUNT errors"
    
    # Show first few errors
    echo ""
    echo "First errors:"
    grep "error" build.log | head -10
    
    echo ""
    echo "The node repository has compilation issues that need to be fixed."
    echo "This appears to be due to:"
    echo "1. Duplicate import statements in generated protobuf files"
    echo "2. Context API migration from custom to standard library"
    echo "3. Import path issues in various packages"
    
    echo ""
    echo "Suggested fixes:"
    echo "1. Regenerate protobuf files with correct import settings"
    echo "2. Complete the context migration consistently"
    echo "3. Fix import statements across all packages"
fi