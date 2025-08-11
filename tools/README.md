# Genesis Tools Documentation

This directory contains all tools necessary for LUX mainnet genesis migration, analysis, and validation.

## Tool Categories

### 1. Analysis Tools (`/analysis`)

Tools for analyzing blockchain databases and verifying data integrity.

#### block_analyzer.go
- **Purpose**: Analyzes blocks in both BadgerDB and PebbleDB formats
- **Usage**: `./block_analyzer <db_path> [format]`
- **Features**:
  - Auto-detects database format (BadgerDB/PebbleDB)
  - Verifies all 1,082,781 blocks
  - Checks genesis hash
  - Detects gaps in block sequence

#### final_block_analyzer.go
- **Purpose**: Comprehensive analysis of the final migrated database
- **Usage**: `./final_block_analyzer <db_path>`
- **Features**:
  - Analyzes key structure and patterns
  - Verifies block continuity
  - Reports database statistics

#### canonical_checker.go
- **Purpose**: Verifies canonical block mappings
- **Usage**: `./canonical_checker <db_path>`
- **Features**:
  - Checks canonical chain integrity
  - Verifies block number to hash mappings
  - Ensures proper chain structure

#### converted_verifier.go
- **Purpose**: Verifies database after format conversion
- **Usage**: `./converted_verifier <db_path>`
- **Features**:
  - Validates converted database format
  - Checks all blocks are accessible
  - Verifies genesis hash matches expected value

### 2. Migration Tools (`/migration`)

Tools for migrating data between different blockchain formats.

#### subnet_to_cchain_migrator.go
- **Purpose**: Migrates SubnetEVM data to C-Chain format
- **Usage**: `./subnet_to_cchain_migrator <source_db> <dest_db>`
- **Features**:
  - Converts SubnetEVM database to Coreth format
  - Preserves all account states and balances
  - Maintains transaction history

#### cchain_complete_migrator.go
- **Purpose**: Complete C-Chain migration with all data
- **Usage**: `./cchain_complete_migrator <source_db> <dest_db>`
- **Features**:
  - Full migration including receipts and logs
  - Preserves contract storage
  - Maintains all blockchain metadata

#### db_format_converter.go (if present)
- **Purpose**: Converts between different database key formats
- **Usage**: `./db_format_converter <source_db> <dest_db>`
- **Features**:
  - Converts migrated format (41-byte keys) to standard geth format
  - Handles 'h' prefix to 'H' prefix conversion
  - Preserves all 1,082,781 blocks

### 3. Inspection Tools (`/inspection`)

Tools for inspecting and debugging blockchain databases.

#### header_inspector.go
- **Purpose**: Inspects block headers in the database
- **Usage**: `./header_inspector <db_path>`
- **Features**:
  - Displays block header information
  - Shows parent hash relationships
  - Useful for debugging chain structure

#### key_inspector.go
- **Purpose**: Inspects database key structure
- **Usage**: `./key_inspector <db_path>`
- **Features**:
  - Analyzes key patterns and prefixes
  - Identifies key format (41-byte vs standard)
  - Useful for understanding database structure

### 4. Core Tools (root directory)

#### verify_migration.go
- **Purpose**: Comprehensive migration verification
- **Usage**: `./verify_migration <db_path>`
- **Expected Output**:
  ```
  Genesis Hash: 0x3f4fa2a0b0ce089f52bf0ae9199c75ffdd76ecafc987794050cb0d286f1ec61e
  Total Blocks: 1,082,781
  Status: PASS
  ```

#### check_balance.go
- **Purpose**: Check account balances via RPC
- **Usage**: `./check_balance <address>`
- **Default Address**: `0x9011E888251AB053B7bD1cdB598Db4f9DEd94714`
- **Features**:
  - Queries balance via eth_getBalance
  - Shows transaction count
  - Displays balance in Wei and LUX

#### convert_database.go
- **Purpose**: Main database format converter
- **Usage**: `./convert_database <source_db> <dest_db>`
- **Features**:
  - Converts migrated format to standard geth format
  - Handles all key mappings
  - Preserves complete blockchain state

## Migration Pipeline

The complete migration pipeline follows these steps:

1. **Source Data**: SubnetEVM database from state repository
2. **Migration**: Convert to Coreth format using migration tools
3. **Format Conversion**: Convert to standard geth format
4. **Verification**: Verify all blocks and genesis hash
5. **Launch**: Start luxd with migrated data
6. **Validation**: Check balances and validator status

## Key Database Formats

### Migrated Format (41-byte keys)
- Key: `'h' + blockNum(8 bytes) + hash(32 bytes)`
- Total: 41 bytes
- Used by migrated SubnetEVM data

### Standard Geth Format
- Key: `'H' + blockNum(8 bytes)`
- Value: `hash(32 bytes)`
- Canonical mapping format

## Expected Values

| Parameter | Value |
|-----------|-------|
| Network ID | 96369 |
| Genesis Hash | `0x3f4fa2a0b0ce089f52bf0ae9199c75ffdd76ecafc987794050cb0d286f1ec61e` |
| Total Blocks | 1,082,781 |
| Validator Address | `0x9011E888251AB053B7bD1cdB598Db4f9DEd94714` |
| P-Chain Staked | 1,000,000,000 LUX (1B LUX) |
| Min Validator Stake | 1,000,000 LUX (1M LUX) |

## Building Tools

All tools can be built using the Makefile in the genesis directory:

```bash
cd /home/z/work/lux/genesis
make build-tools
```

Or build individually:

```bash
go build -o bin/block_analyzer tools/analysis/block_analyzer.go
go build -o bin/verify_migration tools/verify_migration.go
```

## CI Integration

These tools are automatically built and used in the CI pipeline:
- See `scripts/ci_pipeline.sh` for complete workflow
- GitHub Actions workflow in `.github/workflows/genesis_ci.yml`
- All tools are reproducible and deterministic

## Troubleshooting

### Database Not Found
- Ensure source database path is correct
- Check if migration has been completed
- Verify database format matches tool expectations

### Wrong Genesis Hash
- Database may not be fully migrated
- Check if using correct network ID (96369)
- Verify source data is from mainnet

### Missing Blocks
- Check for gaps using block_analyzer
- Ensure complete migration from source
- Verify database wasn't corrupted during transfer

## Support

For issues or questions:
- Check CI pipeline logs
- Run verification tools
- Inspect database with inspection tools
- Review migration logs for errors