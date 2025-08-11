#!/bin/bash
set -e

# Reproducible Genesis Generation for LUX Mainnet
# This script ensures consistent genesis generation across CI/CD environments

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
BUILD_DIR="$PROJECT_ROOT/build"
DATA_DIR="${DATA_DIR:-$HOME/.luxd}"

echo "=== LUX Mainnet Genesis Reproducible Build ==="
echo "Project root: $PROJECT_ROOT"
echo "Data directory: $DATA_DIR"
echo ""

# Step 1: Build all genesis tools
build_tools() {
    echo "Step 1: Building genesis tools..."
    cd "$PROJECT_ROOT"
    
    # Create build directory
    mkdir -p "$BUILD_DIR"
    
    # Build main genesis tool
    echo "  Building genesis tool..."
    go build -o "$BUILD_DIR/genesis" ./cmd/genesis
    
    # Build migration verifier
    echo "  Building migration verifier..."
    cat > "$BUILD_DIR/verify_migration.go" << 'EOF'
package main

import (
    "encoding/binary"
    "encoding/hex"
    "fmt"
    "log"
    "os"
    "github.com/dgraph-io/badger/v4"
)

func main() {
    if len(os.Args) < 2 {
        fmt.Println("Usage: verify_migration <db_path>")
        os.Exit(1)
    }
    
    dbPath := os.Args[1]
    opts := badger.DefaultOptions(dbPath)
    opts.ReadOnly = true
    
    db, err := badger.Open(opts)
    if err != nil {
        log.Fatal(err)
    }
    defer db.Close()
    
    blockCount := 0
    var genesisHash string
    
    err = db.View(func(txn *badger.Txn) error {
        it := txn.NewIterator(badger.DefaultIteratorOptions)
        defer it.Close()
        
        prefix := []byte("h")
        for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
            item := it.Item()
            key := item.Key()
            if len(key) == 41 {
                blockNum := binary.BigEndian.Uint64(key[1:9])
                if blockNum == 0 {
                    genesisHash = hex.EncodeToString(key[9:41])
                }
                blockCount++
            }
        }
        return nil
    })
    
    if err != nil {
        log.Fatal(err)
    }
    
    expectedGenesis := "3f4fa2a0b0ce089f52bf0ae9199c75ffdd76ecafc987794050cb0d286f1ec61e"
    if genesisHash == expectedGenesis {
        fmt.Printf("PASS: Genesis hash correct: 0x%s\n", genesisHash)
        fmt.Printf("PASS: Total blocks: %d\n", blockCount)
        os.Exit(0)
    } else {
        fmt.Printf("FAIL: Genesis hash mismatch. Got: 0x%s, Expected: 0x%s\n", genesisHash, expectedGenesis)
        os.Exit(1)
    }
}
EOF
    go build -o "$BUILD_DIR/verify_migration" "$BUILD_DIR/verify_migration.go"
    
    echo "  Tools built successfully"
}

# Step 2: Download source data if needed
download_source() {
    echo ""
    echo "Step 2: Checking source data..."
    
    SOURCE_DB="${SOURCE_DB:-$PROJECT_ROOT/data/source.db}"
    
    if [ ! -d "$SOURCE_DB" ]; then
        echo "  Source database not found at $SOURCE_DB"
        echo "  Set SOURCE_DB environment variable to point to SubnetEVM database"
        echo "  Or download from: [URL would go here]"
        return 1
    fi
    
    echo "  Source database found: $SOURCE_DB"
}

# Step 3: Run migration
run_migration() {
    echo ""
    echo "Step 3: Running migration..."
    
    DEST_DB="$DATA_DIR/chainData/xBBY6aJcNichNCkCXgUcG5Gv2PW6FLS81LYDV8VwnPuadKGqm/ethdb"
    
    if [ -d "$DEST_DB" ]; then
        echo "  Migrated database already exists at $DEST_DB"
        echo "  Verifying..."
        "$BUILD_DIR/verify_migration" "$DEST_DB"
        if [ $? -eq 0 ]; then
            echo "  Migration verified successfully"
            return 0
        else
            echo "  Migration verification failed, re-running migration"
            rm -rf "$DEST_DB"
        fi
    fi
    
    echo "  Migrating from $SOURCE_DB to $DEST_DB..."
    mkdir -p "$(dirname "$DEST_DB")"
    
    "$BUILD_DIR/genesis" migrate \
        -source "$SOURCE_DB" \
        -dest "$DATA_DIR" \
        -network lux-mainnet \
        -verbose
    
    echo "  Migration complete, verifying..."
    "$BUILD_DIR/verify_migration" "$DEST_DB"
}

# Step 4: Generate genesis files
generate_genesis() {
    echo ""
    echo "Step 4: Generating genesis files..."
    
    GENESIS_DIR="$DATA_DIR/genesis"
    mkdir -p "$GENESIS_DIR"
    
    # Extract genesis from migrated data
    echo "  Extracting C-Chain genesis..."
    "$BUILD_DIR/genesis" generate-genesis \
        -dest "$DATA_DIR" \
        -network lux-mainnet
    
    # Copy genesis files to standard location
    cp "$PROJECT_ROOT/genesis/genesis_96369.json" "$GENESIS_DIR/network_96369.json"
    
    echo "  Genesis files generated in $GENESIS_DIR"
}

# Step 5: Create verification report
create_report() {
    echo ""
    echo "Step 5: Creating verification report..."
    
    REPORT_FILE="$BUILD_DIR/genesis_report.txt"
    
    cat > "$REPORT_FILE" << EOF
LUX Mainnet Genesis Verification Report
========================================
Generated: $(date -u +"%Y-%m-%d %H:%M:%S UTC")

Network ID: 96369
Chain ID: xBBY6aJcNichNCkCXgUcG5Gv2PW6FLS81LYDV8VwnPuadKGqm

Expected Genesis Hash: 0x3f4fa2a0b0ce089f52bf0ae9199c75ffdd76ecafc987794050cb0d286f1ec61e
Expected Block Count: 1082781 (blocks 0-1082780)

Migration Status:
EOF
    
    "$BUILD_DIR/verify_migration" "$DATA_DIR/chainData/xBBY6aJcNichNCkCXgUcG5Gv2PW6FLS81LYDV8VwnPuadKGqm/ethdb" >> "$REPORT_FILE" 2>&1
    
    echo "" >> "$REPORT_FILE"
    echo "Files Generated:" >> "$REPORT_FILE"
    ls -la "$DATA_DIR/genesis/" >> "$REPORT_FILE" 2>&1
    
    echo "  Report saved to $REPORT_FILE"
    cat "$REPORT_FILE"
}

# Step 6: Package for distribution
package_genesis() {
    echo ""
    echo "Step 6: Creating distribution package..."
    
    DIST_DIR="$BUILD_DIR/dist"
    ARCHIVE_NAME="lux-mainnet-genesis-$(date +%Y%m%d).tar.gz"
    
    mkdir -p "$DIST_DIR"
    
    # Copy essential files
    cp -r "$DATA_DIR/genesis" "$DIST_DIR/"
    cp "$BUILD_DIR/genesis_report.txt" "$DIST_DIR/"
    cp "$BUILD_DIR/genesis" "$DIST_DIR/"
    cp "$BUILD_DIR/verify_migration" "$DIST_DIR/"
    
    # Create archive
    cd "$BUILD_DIR"
    tar czf "$ARCHIVE_NAME" dist/
    
    echo "  Distribution package created: $BUILD_DIR/$ARCHIVE_NAME"
    echo "  SHA256: $(sha256sum "$ARCHIVE_NAME" | cut -d' ' -f1)"
}

# Main execution
main() {
    # Parse arguments
    case "${1:-all}" in
        build)
            build_tools
            ;;
        migrate)
            build_tools
            download_source
            run_migration
            ;;
        verify)
            build_tools
            "$BUILD_DIR/verify_migration" "${2:-$DATA_DIR/chainData/xBBY6aJcNichNCkCXgUcG5Gv2PW6FLS81LYDV8VwnPuadKGqm/ethdb}"
            ;;
        package)
            package_genesis
            ;;
        all)
            build_tools
            download_source
            run_migration
            generate_genesis
            create_report
            package_genesis
            ;;
        *)
            echo "Usage: $0 [build|migrate|verify|package|all]"
            echo ""
            echo "Commands:"
            echo "  build    - Build genesis tools only"
            echo "  migrate  - Run database migration"
            echo "  verify   - Verify migrated database"
            echo "  package  - Create distribution package"
            echo "  all      - Run complete workflow (default)"
            echo ""
            echo "Environment variables:"
            echo "  SOURCE_DB - Path to source SubnetEVM database"
            echo "  DATA_DIR  - Output directory (default: ~/.luxd)"
            exit 1
            ;;
    esac
}

main "$@"