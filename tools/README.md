# Genesis Tools

## Analysis (`/analysis`)

### block_analyzer
Analyzes blocks in BadgerDB/PebbleDB formats.
```bash
./block_analyzer <db_path> [format]
```

### block_analyzer_full
Full database analysis with key structure and statistics.
```bash
./block_analyzer_full <db_path>
```

### canonical_checker
Verifies canonical block mappings.
```bash
./canonical_checker <db_path>
```

### converted_verifier
Validates converted database format.
```bash
./converted_verifier <db_path>
```

## Migration (`/migration`)

### subnet_to_cchain_migrator
Migrates SubnetEVM to C-Chain format.
```bash
./subnet_to_cchain_migrator <source_db> <dest_db>
```

### cchain_migrator
Migrates C-Chain with receipts and storage.
```bash
./cchain_migrator <source_db> <dest_db>
```

### db_format_converter
Converts 41-byte to standard key format.
```bash
./db_format_converter <source_db> <dest_db>
```

## Inspection (`/inspection`)

### header_inspector
Inspects block headers.
```bash
./header_inspector <db_path>
```

### key_inspector
Analyzes database keys.
```bash
./key_inspector <db_path>
```

## Core Tools

### verify_migration
Verifies migration (1,082,781 blocks).
```bash
./verify_migration <db_path>
```

### check_balance
Checks account balance via RPC.
```bash
./check_balance <address>
```

### convert_database
Converts database format.
```bash
./convert_database <source_db> <dest_db>
```

## Build

```bash
make build-tools
```

## Values

- Network: 96369
- Genesis: 0x3f4fa2a0b0ce089f52bf0ae9199c75ffdd76ecafc987794050cb0d286f1ec61e
- Blocks: 1,082,781
- Validator: 0x9011E888251AB053B7bD1cdB598Db4f9DEd94714
- Min Stake: 1M LUX
- P-Chain: 1B LUX