# 5. Operator Runbook

This runbook provides practical procedures for operators responsible for deploying, maintaining, and monitoring a production Lux Network.

## Production Deployment

While the [Getting Started](./2_getting_started.md) guide covers local Docker-based deployment, a production deployment requires a more robust, multi-node setup. The recommended approach is to use a configuration management tool (like Ansible, Terraform, or Salt) to provision multiple nodes and distribute the necessary configurations and data.

### Core Deployment Workflow

1.  **Provision Nodes**: Set up multiple virtual or physical machines that meet the recommended system requirements (see below).
2.  **Generate Validator Keys**: Securely generate validator keys as described in the Getting Started guide. **Each node must have its own unique keys.**
3.  **Distribute Keys**: Securely copy the keys for each validator to its respective node (e.g., `validator-keys/validator-1/*` goes to node 1).
4.  **Distribute Genesis File**: Copy the final `genesis.json` file to each node.
5.  **Distribute Node Binary**: Ensure the correct `luxd` binary is on each node.
6.  **Start Nodes**: Start the `luxd` process on each node, pointing to its unique keys and the shared genesis file. The nodes will use the bootstrap nodes defined in the genesis to find each other and form a network.

### System Requirements (Per Node)

-   **CPU**: 16 cores
-   **RAM**: 32GB
-   **Disk**: 1TB NVMe SSD (for mainnet)
-   **Network**: 1Gbps

---

## Chain Data Import and Backup

For a new network, you start from a genesis file. But if you are restoring a node or adding a new one to an existing network, you will need to import chain data.

### The Import -> Monitor -> Backup Workflow

This is the recommended, battle-tested workflow for safely importing data.

**Step 1: Import Chain Data**

Use the `genesis import chain-data` command to bootstrap a node with existing data from a trusted source (e.g., another one of your nodes or a snapshot).

```bash
# This command handles the entire import process, including starting,
# monitoring, and restarting the node in normal mode.
./bin/genesis import chain-data /path/to/source/chaindata \
  --data-dir=$HOME/.luxd \
  --network-id=96369
```

**Step 2: Monitor for Stability (48 Hours)**

After the import is complete, the node needs to be monitored to ensure it's stable and syncing correctly with the rest of the network.

```bash
# The monitor command checks health, block height, and peer count.
# It should be run in the background (e.g., using nohup or screen).
./bin/genesis import monitor \
  --rpc-url=http://localhost:9630 \
  --duration=48h
```

**Step 3: Create a Backup**

Once the node has been stable for 48 hours, create a compressed backup of its database.
This backup is your primary recovery tool in case of failure.

```bash
./bin/genesis export backup \
  --data-dir=$HOME/.luxd \
  --backup-dir=/backups \
  --compress=true
```

---

## Auditing and Verification

Verifying the integrity of the chain, especially the treasury balance, is a critical task for operators and community auditors.

### How to Verify the Treasury Balance

This procedure allows you to check the balance of the treasury account (`0x9011E888251AB053B7bD1cdB598Db4f9DEd94714`) directly from a chain database.

**1. Extract State from the Database**

First, use the `extract state` command to process the database.

```bash
./bin/genesis extract state \
  /path/to/mainnet/db/pebbledb \
  /tmp/extracted-mainnet-state \
  --network 96369 \
  --state
```

**2. Analyze the Extracted State**

Next, use the `analyze balance` command to query the treasury address within the extracted data.

```bash
./bin/genesis analyze balance \
  --db /tmp/extracted-mainnet-state/pebbledb \
  --account 0x9011E888251AB053B7bD1cdB598Db4f9DEd94714
```

**3. Compare the Result**

The command will output the balance. You can compare this against the expected value (e.g., ~1.995T LUX for a running mainnet).

---

## Emergency Procedures

### Node Crash

1.  **Check System Resources**: Look for memory exhaustion (`free -h`), disk space issues (`df -h`), or CPU spikes (`top`).
2.  **Check Node Logs**: The logs are the most important diagnostic tool. Look for `FATAL` or `ERROR` messages in `~/.luxd/logs/main.log`.
3.  **Restart**: Attempt a simple restart of the `luxd` process.
4.  **Restore from Backup**: If the database is corrupted and will not start, your fastest recovery path is to stop the node, delete the corrupted data directory, and restore from your latest backup. After restoring, you will need to let the node sync to catch up with the network.

### Rollback Procedure

If a network-wide issue occurs after an upgrade, a coordinated rollback may be necessary.

1.  **Stop All Validators**: Halt the `luxd` process on all validator nodes.
2.  **Restore from Pre-Upgrade Backups**: On each node, restore the database from the backup created just before the upgrade.
3.  **Restart Network**: Restart all validator nodes with the previous `luxd` binary and configuration.
4.  **Investigate**: Perform a post-mortem to understand what went wrong before attempting the upgrade again.
