# LUX Mainnet Genesis - Complete CI/CD Pipeline

This directory contains the complete, reproducible pipeline for LUX mainnet genesis generation, migration, and validator deployment.

## Quick Start

```bash
# Run complete CI pipeline locally
./scripts/test_locally.sh

# Or run full pipeline with custom state
STATE_REPO_URL=https://github.com/org/state-repo ./scripts/ci_pipeline.sh

# Boot validator with mainnet data
./scripts/boot_mainnet_validator.sh
```

## Overview

The LUX mainnet launches with 1,082,781 pre-existing blocks (0-1,082,780) migrated from SubnetEVM format to Coreth format.

### Key Parameters
- **Network ID**: 96369
- **Genesis Hash**: `0x3f4fa2a0b0ce089f52bf0ae9199c75ffdd76ecafc987794050cb0d286f1ec61e`
- **Total Blocks**: 1,082,781
- **Chain ID**: `xBBY6aJcNichNCkCXgUcG5Gv2PW6FLS81LYDV8VwnPuadKGqm`

## Reproducible Workflow

### 1. Build Tools

```bash
cd /home/z/work/lux/genesis
make build
```

This creates:
- `bin/genesis` - Main migration tool
- `bin/verify_migration` - Database verification tool  
- `bin/check_balance` - Account balance checker

### 2. Run Migration

```bash
# Set source database path
export SOURCE_DB=/path/to/subnet/evm/database

# Run migration
make migrate
```

The migration:
1. Reads SubnetEVM database (PebbleDB format)
2. Converts to Coreth format (BadgerDB)
3. Transforms key mappings to standard geth format
4. Preserves all 1,082,781 blocks with correct hashes

### 3. Verify Migration

```bash
make verify
```

Verification checks:
- Genesis hash matches expected value
- All blocks from 0 to 1,082,780 are present
- No gaps in blockchain
- Account allocations are correct

### 4. Create Distribution Package

```bash
make package
```

Creates `lux-mainnet-genesis-YYYYMMDD.tar.gz` containing:
- Converted database
- Genesis configuration files
- Verification tools
- SHA256 checksums

## CI/CD Pipeline

### Complete Workflow

The CI pipeline (`scripts/ci_pipeline.sh`) performs:

1. **Clone State Repository** - Gets SubnetEVM state data
2. **Build Tools** - Compiles migration and verification tools
3. **Migrate to Coreth** - Converts SubnetEVM → Coreth format  
4. **Convert Database** - Transforms to standard Geth format
5. **Verify Migration** - Validates all 1,082,781 blocks
6. **Run Node** - Starts validator and checks balance
7. **Package Artifacts** - Creates distribution archive

### GitHub Actions

```yaml
on:
  push:
    branches: [main]
  workflow_dispatch:
    inputs:
      state_repo_url:
        description: 'State repository URL'

jobs:
  build-genesis:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - run: ./genesis/scripts/ci_pipeline.sh
```

### Docker Build

```bash
# Build Docker image with genesis
make docker

# Run in container
docker run -v $(pwd)/data:/data luxfi/genesis:latest
```

## Verification Process

### Manual Verification

```bash
# Check specific account balance
./bin/check_balance /path/to/database 0x9011E888251AB053B7bD1cdB598Db4f9DEd94714

# Full database verification
./bin/verify_migration /path/to/database
```

### Expected Output

```
✓ Block 0 canonical hash: 3f4fa2a0b0ce089f52bf0ae9199c75ffdd76ecafc987794050cb0d286f1ec61e
✓ CORRECT GENESIS HASH!
✓ Block 1082780 canonical hash: 32dede1fc8e0f11ecde12fb42aef7933fc6c5fcf863bc277b5eac08ae4d461f0
✓ Total blocks: 1082781
✓ No gaps found
```

## Account Information

### Validator Account
- **Address**: `0x9011E888251AB053B7bD1cdB598Db4f9DEd94714`
- **Initial Allocation**: 500 LUX
- **Minimum Stake**: 2000 LUX
- **LUX Address**: `X-lux1hfhf94tjcufccczxwfgp8kh9qnphxp5ycvzqzd`

## Development

### Running Tests

```bash
# Run all tests
make test

# Run benchmarks
make benchmark

# Test with local luxd
make dev-server
```

### Project Structure

```
genesis/
├── Makefile                    # Build system
├── scripts/
│   ├── reproducible_genesis.sh # Main workflow script
│   └── test_with_luxd.sh      # Integration tests
├── cmd/
│   └── genesis/                # Main tool source
├── tools/                      # Verification tools
├── data/                       # Source databases
└── build/                      # Build artifacts
```

## Troubleshooting

### Common Issues

1. **Database not found**
   ```bash
   export SOURCE_DB=/correct/path/to/database
   ```

2. **Wrong genesis hash**
   - Ensure using correct source database
   - Check migration completed successfully
   - Verify no data corruption

3. **Missing blocks**
   - Re-run migration with verbose output
   - Check source database integrity
   - Verify sufficient disk space

## Security

- All builds are reproducible and deterministic
- SHA256 checksums provided for all artifacts
- Database integrity verified at each step
- No private keys or sensitive data included

## Support

For issues or questions:
- GitHub Issues: [github.com/luxfi/node/issues](https://github.com/luxfi/node/issues)
- Documentation: [docs.lux.network](https://docs.lux.network)

---

Built with ❤️ for the LUX Network