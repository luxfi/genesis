# LUX Genesis

Migration and validation tools for LUX mainnet.

## Build
```bash
./build.sh
```

## Migrate
```bash
bin/genesis migrate <source_db> <dest_db>
```

## Verify
```bash
bin/genesis verify <db_path>
```

## Convert
```bash
bin/genesis convert <source_db> <dest_db>
```

## CI
```bash
make all
```

## Values
- Network: 96369
- Genesis: 0x3f4fa2a0b0ce089f52bf0ae9199c75ffdd76ecafc987794050cb0d286f1ec61e  
- Blocks: 1,082,781
- Validator: 0x9011E888251AB053B7bD1cdB598Db4f9DEd94714
- Min Stake: 1M LUX
- P-Chain: 1B LUX