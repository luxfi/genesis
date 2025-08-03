#!/bin/bash

# Replace all go-ethereum imports with luxfi/geth
echo "Replacing go-ethereum imports with luxfi/geth..."

# Find all Go files and replace imports
find /home/z/work/lux/genesis -name "*.go" -type f -print0 | \
xargs -0 sed -i 's|github.com/ethereum/go-ethereum|github.com/luxfi/geth|g'

# Also replace any hdwallet imports that depend on go-ethereum
find /home/z/work/lux/genesis -name "*.go" -type f -print0 | \
xargs -0 sed -i 's|github.com/miguelmota/go-ethereum-hdwallet|github.com/luxfi/hdwallet|g'

echo "Import replacement complete!"