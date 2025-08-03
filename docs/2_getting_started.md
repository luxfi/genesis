# 2. Getting Started: A Hands-On Tutorial

This guide provides a step-by-step tutorial for setting up a local Lux development network. We will generate validator keys, create a genesis file, and launch a network using the recommended Docker-based approach.

## Prerequisites

-   **Go**: Version 1.24.5 or later.
-   **Git**: For cloning the required repositories.
-   **Make**: For using the simplified build commands.
-   **Docker**: For running the network in a containerized environment.
-   **A secure 12-word mnemonic phrase**: For deterministically generating validator keys. **DO NOT USE A MNEMONIC FROM A REAL WALLET.** Generate a new one for testing purposes.

## Step 1: Build All Tools and Dependencies

The `Makefile` is configured to build the `genesis` CLI, `luxd` (the node binary), and all other required tools.

```bash
# This command will build everything needed for the next steps.
make all
```

## Step 2: Generate Validator Keys

Validators are the nodes that secure the network. Each validator needs a set of cryptographic keys. We will generate keys for 11 validators using a mnemonic phrase.

**Important**: For this tutorial, you can use the example mnemonic. For any real network, you must use your own, securely generated mnemonic.

```bash
# Set your mnemonic as an environment variable
export MNEMONIC="spirit level garage dot typical page asset abstract embark primary vendor right"

# Run the make target to generate keys for 11 validators
make validators-generate
```

This command will:
1.  Create a `validator-keys/` directory (which is ignored by git).
2.  Inside, it will create a directory for each validator (`validator-1`, `validator-2`, etc.) containing its private keys (`bls.key`, `staker.key`) and public certificate (`staker.crt`).
3.  Create a `configs/mainnet-validators.json` file, which contains the public information about all the validators.

## Step 3: Generate the Genesis File

With the validator set defined, we can now create the `genesis.json` file. This file will define the initial state of our local network.

```bash
# This target reads the validator config and creates the genesis file
make genesis-generate
```

This will create `genesis_mainnet.json` in the root directory. This file tells the Lux node everything it needs to know to start the network from block 0.

## Step 4: Launch the Network with Docker

The easiest and most reliable way to launch a local network that mirrors a production setup is by using Docker. The project includes a `docker-compose.yml` file that configures the entire network stack.

```bash
# This command builds the Docker images and starts all services.
# The -d flag runs it in detached mode (in the background).
make launch-docker
```

This command starts:
-   `lux-primary`: The main Lux network node.
-   `subnet-deployer`: A service that waits for the primary node to be healthy and then deploys the ZOO, SPC, and Hanzo subnets (L2s).

## Step 5: Verify the Network is Running

After a minute or two, the network should be up and running. You can verify this by sending a request to the C-Chain's RPC endpoint.

```bash
# Ask for the current block number
curl -X POST -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"eth_blockNumber","params":[]}' \
  http://localhost:9630/ext/bc/C/rpc
```

If the network is running, you should receive a JSON response with a block number (e.g., `"result":"0x0"` or higher).

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": "0x0"
}
```

## Interacting with the Network

You can also use the `Makefile`'s dynamic network targeting to interact with specific L2 networks.

```bash
# Get information about the ZOO network
make network-info NETWORK=zoo

# Deploy a specific network (if not using the full Docker launch)
make deploy NETWORK=spc
```

## Stopping the Network

To stop all the Docker containers and clean up the network:

```bash
make kill-docker
```

Congratulations! You have successfully generated a genesis file and launched a complete, local Lux network stack. You can now proceed to the other guides to learn more about the `genesis` CLI tool or how to perform advanced data migrations.
