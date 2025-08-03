# 1. Core Concepts

This document explains the core concepts of the Lux Network's architecture and genesis process. Understanding these concepts is essential for effectively using the tools in this project.

## L1 vs. L2 Architecture

The Lux ecosystem is composed of a primary Level 1 (L1) network and multiple Level 2 (L2) networks.

-   **L1 (Primary Network)**: This is the main Lux blockchain. It consists of three distinct chains: the P-Chain, C-Chain, and X-Chain. It provides the ultimate security and finality for the entire ecosystem.
-   **L2 (Subnets)**: These are independent blockchains that are validated by a subset of the L1 validators. They offer custom execution environments and can be tailored for specific applications (e.g., DeFi, gaming, AI). The ZOO, SPC, and Hanzo networks are L2s.

## The Three Chains of the L1

The Lux Primary Network is a "heterogeneous network of blockchains." This means it's composed of multiple specialized chains.

### P-Chain (Platform Chain)

-   **Purpose**: The P-Chain is the metadata blockchain of the Lux Network.
-   **Responsibilities**:
    -   Coordinating validators.
    -   Tracking active subnets (L2s).
    -   Enabling the creation of new subnets.
-   You interact with the P-Chain to stake LUX, become a validator, or create a new L2 network.

### X-Chain (Exchange Chain)

-   **Purpose**: The X-Chain is a decentralized platform for creating and trading digital smart assets.
-   **Architecture**: It's an instance of the Avalanche Virtual Machine (AVM).
-   **Key Feature**: It uses a UTXO (Unspent Transaction Output) model, similar to Bitcoin, which is highly parallelizable and excellent for asset transfers. The native token, LUX, is traded on the X-Chain.

### C-Chain (Contract Chain)

-   **Purpose**: The C-Chain is the smart contract platform of the Lux Network.
-   **Architecture**: It's an instance of the Ethereum Virtual Machine (EVM). It allows developers to deploy and run Ethereum-compatible smart contracts.
-   **Key Feature**: It uses an account-based model, just like Ethereum. This is where most user-facing applications, like DeFi protocols, are built. The `genesis` tools in this project are heavily focused on configuring and migrating data for the C-Chain and EVM-based L2s.

## What is a Genesis File?

A `genesis.json` file is the "birth certificate" of a blockchain. It's a configuration file that defines the initial state of the network. It specifies critical parameters, including:

-   **Network ID & Chain ID**: Identifiers that distinguish this network from others.
-   **Initial Allocations**: Which accounts or addresses will have a balance of tokens at the very start of the chain.
-   **Validator Set**: The initial group of validators who will secure the network.
-   **Staking Parameters**: Rules for staking, such as minimum stake amounts and durations.
-   **EVM Configuration**: Settings for the C-Chain, such as which Ethereum hard forks (upgrades) are active from the beginning.

## The Importance of Data Migration

For a new blockchain, the genesis file is all that's needed. However, the Lux Network already has a rich history from previous versions and related projects (e.g., tokens on other chains like BSC). Data migration is the process of carrying this history forward into the new network.

This project's tools are designed to handle several complex migration scenarios:

1.  **Preserving C-Chain History**: The current Lux C-Chain (ID 96369) was once a subnet. The migration tools can take the entire history of that subnet (all blocks, transactions, and account states) and make it the history of the new C-Chain, ensuring no data is lost.
2.  **Cross-Chain Token Migration**: The ZOO token existed on the Binance Smart Chain (BSC). The migration tools can scan the BSC for events like token burns and NFT ownership, and then credit the corresponding users with tokens in the new ZOO L2 genesis file.
3.  **State-Only Migration**: Sometimes, a full transaction history isn't available, but a "snapshot" of all account balances is. The tools can handle this "state-only" data, creating a genesis file that preserves everyone's assets, even if the transaction history starts from block 0.

## Network Reference

This table contains the key identifiers for the networks in the Lux ecosystem.

| Network         | Type       | Mainnet Chain ID | Testnet Chain ID |
| --------------- | ---------- | ---------------- | ---------------- |
| **LUX**         | L1 Primary | `96369`          | `96368`          |
| **ZOO**         | L2 Subnet  | `200200`         | `200201`         |
| **SPC**         | L2 Subnet  | `36911`          | `36912`          |
| **Hanzo**       | L2 Subnet  | `36963`          | `36962`          |

### Important Addresses

-   **Default Treasury Address**: `0x9011E888251AB053B7bD1cdB598Db4f9DEd94714`
-   **ZOO BSC Token**: `0x0a6045b79151d0a54dbd5227082445750a023af2`
-   **EGG NFT on BSC**: `0x5bb68cf06289d54efde25155c88003be685356a8`
-   **Lux Genesis NFT (ETH)**: `0x31e0f919c67cedd2bc3e294340dc900735810311`
