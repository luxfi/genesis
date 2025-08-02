#!/bin/bash

# Migration script from old genesis structure to new
# This helps transition from the monolithic state repo

set -e

echo "Genesis Migration Tool"
echo "====================="

# Check if we're in the right directory
if [ ! -f "Makefile" ] || [ ! -d "cmd/genesis" ]; then
    echo "Error: Must run from genesis-new directory"
    exit 1
fi

# Parse arguments
OLD_GENESIS_PATH="${1:-../state}"
CLONE_CHAINDATA="${2:-no}"

echo "Migrating from: $OLD_GENESIS_PATH"
echo "Clone chaindata: $CLONE_CHAINDATA"
echo ""

# Create necessary directories
mkdir -p configs pkg/{config,validator,allocation,cchain}

# Check if old path exists
if [ ! -d "$OLD_GENESIS_PATH" ]; then
    echo "Error: Old genesis path not found: $OLD_GENESIS_PATH"
    exit 1
fi

# Copy useful utilities if they exist
if [ -d "$OLD_GENESIS_PATH/pkg/genesis" ]; then
    echo "Copying genesis utilities..."
    cp -r "$OLD_GENESIS_PATH/pkg/genesis/"* pkg/ 2>/dev/null || true
fi

# Copy any existing configs
if [ -d "$OLD_GENESIS_PATH/configs" ]; then
    echo "Copying configuration templates..."
    cp -r "$OLD_GENESIS_PATH/configs/"*.json configs/ 2>/dev/null || true
fi

# Clone chaindata if requested
if [ "$CLONE_CHAINDATA" = "yes" ]; then
    echo "Cloning chaindata (this may take a while)..."
    make clone-state STATE_REPO="$OLD_GENESIS_PATH"
else
    echo "Skipping chaindata clone. Run 'make clone-state' when needed."
fi

echo ""
echo "Migration complete!"
echo ""
echo "Next steps:"
echo "1. Run 'make build' to build the genesis tool"
echo "2. Run 'make help' to see available commands"
echo "3. Use 'make clone-state' to get chaindata when needed"
echo ""
echo "The new genesis tool is much lighter and only clones chaindata on-demand."