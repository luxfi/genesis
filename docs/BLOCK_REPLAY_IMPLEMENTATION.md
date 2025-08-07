# Block Replay Implementation

This document describes the implementation of block replay functionality for migrating SubnetEVM blocks to the C-chain through consensus.

## Overview

The block replay mechanism allows historic blocks from a SubnetEVM to be replayed through the normal consensus engine of the C-chain. This ensures all blocks are properly validated and integrated into the chain rather than just copying database files.

## Two Approaches

### Approach 1: Database Conversion (Not Preferred)
- Converts SubnetEVM database format to C-chain format
- Keeps C-chain in PebbleDB (not the long-term intention)
- Direct database manipulation

### Approach 2: Consensus Replay (Preferred)
- Replays blocks through normal consensus starting with genesis
- Each finalized block is processed in order
- Creates new valid C-chain blocks from the finalized SubnetEVM blocks
- Maintains chain integrity and validation

## Implementation Details

### Key Components

1. **Migration Tool** (`pkg/migration/subnet_to_cchain.go`)
   - Converts SubnetEVM key format to C-chain format
   - Handles the 0x33 prefix and 31-byte padding removal
   - Creates canonical hash mappings

2. **Block Replayer** (`cmd/block-replayer/main.go`)
   - Reads blocks from migrated database
   - Submits blocks through RPC methods:
     - `debug_insertBlock` - Direct block insertion
     - `admin_importChain` - Import via file
     - `eth_sendRawBlock` - Raw block submission

3. **Consensus Replay** (`cmd/consensus-replay/main.go`)
   - More sophisticated replay mechanism
   - Handles genesis initialization
   - Batch processing for efficiency
   - Progress tracking and monitoring

4. **VM Integration** (`vms/cchainvm/replay.go`)
   - Native replay support in the VM
   - Uses `--genesis-db` flag
   - Automatic detection of blocks to replay
   - Integration with blockchain processor

### Database Key Formats

#### SubnetEVM Format
```
Canonical: 0x33 + padding(31) + "H" + number(8) = 41 bytes
Header:    0x33 + padding(31) + "h" + number(8) + hash(32) = 73 bytes
Body:      0x33 + padding(31) + "b" + number(8) + hash(32) = 73 bytes
```

#### C-chain Format
```
Canonical: "H" + number(8) = 9 bytes
Header:    "h" + number(8) + hash(32) = 41 bytes
Body:      "b" + number(8) + hash(32) = 41 bytes
```

### Usage

#### Running with Genesis Database
```bash
./build/luxd \
  --network-id=96369 \
  --genesis-db=/path/to/migrated/database \
  --genesis-db-type=pebbledb \
  --db-type=badgerdb \
  --data-dir=~/.luxd-mainnet-replay
```

#### Using the Replay Script
```bash
# Start luxd with replay enabled
./scripts/run-mainnet-with-replay.sh

# Test replay functionality
./genesis/scripts/test-replay.sh

# Monitor progress
./genesis/scripts/test-replay.sh --monitor
```

#### Manual Block Replay
```bash
# Check status
./bin/consensus-replay status

# Replay specific range
./bin/consensus-replay replay 1 1000

# Replay all blocks
./bin/consensus-replay replay-all
```

## Block Data

- **Total Blocks**: 1,082,781
- **Genesis Hash**: 0x3f4fa2a0b0ce089f52bf0ae9199c75ffdd76ecafc987794050cb0d286f1ec61e
- **Chain ID**: 96369
- **Database Size**: ~7.2GB

## Performance Considerations

1. **Batch Processing**: Process blocks in batches of 100-1000 for efficiency
2. **Rate Limiting**: Add delays between batches to avoid overwhelming the system
3. **Progress Tracking**: Monitor blocks/second to estimate completion time
4. **Database Type**: Use BadgerDB for new chain data (better performance)

## Troubleshooting

### Common Issues

1. **Genesis Hash Mismatch**
   - Ensure the genesis configuration matches the imported blockchain
   - Check that all genesis parameters are correctly set

2. **RPC Method Not Found**
   - Some RPC methods may not be available
   - Falls back to alternative methods automatically

3. **Database Not Found**
   - Verify the path to the migrated database
   - Check permissions on the database directory

4. **Slow Replay Speed**
   - Normal speed: 50-200 blocks/second
   - Depends on hardware and block complexity
   - Full replay may take 2-6 hours

### Verification

Check if blocks are being replayed:
```bash
# Check current height
curl -X POST -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' \
  http://localhost:9630/ext/bc/C/rpc

# Get specific block
curl -X POST -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"eth_getBlockByNumber","params":["0x1", false],"id":1}' \
  http://localhost:9630/ext/bc/C/rpc
```

## Future Improvements

1. **Parallel Processing**: Process multiple blocks in parallel where possible
2. **State Verification**: Add merkle proof verification for state transitions
3. **Checkpoint Support**: Allow resuming from checkpoints
4. **Compression**: Compress block data during replay
5. **Streaming**: Stream blocks directly from source without intermediate storage