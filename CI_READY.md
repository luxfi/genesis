# CI Pipeline

## Structure
```
genesis/
├── scripts/
│   ├── ci_pipeline.sh            # Main CI entry
│   ├── boot_mainnet_validator.sh # Boot validator
│   └── test_locally.sh           # Local test
├── tools/
│   ├── verify_migration.go       # Verify blocks
│   ├── check_balance.go          # Check balance
│   └── convert_database.go       # Convert DB
├── .github/
│   └── workflows/
│       └── genesis_ci.yml        # GitHub Actions
└── Makefile                      # Build automation
```

## Pipeline
1. Clone state repo
2. Build tools  
3. Migrate to Coreth
4. Convert format
5. Verify genesis
6. Run node
7. Check balance
8. Package artifacts

## Values
- Network: 96369
- Genesis: 0x3f4fa2a0b0ce089f52bf0ae9199c75ffdd76ecafc987794050cb0d286f1ec61e
- Blocks: 1,082,781
- Validator: 0x9011E888251AB053B7bD1cdB598Db4f9DEd94714
- Min Stake: 1M LUX
- P-Chain: 1B LUX

## Run
```bash
# GitHub Actions
gh workflow run genesis_ci.yml

# Local
./scripts/test_locally.sh

# Custom state
STATE_REPO_URL=https://github.com/org/state ./scripts/ci_pipeline.sh
```