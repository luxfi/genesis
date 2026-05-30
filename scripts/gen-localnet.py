#!/usr/bin/env python3
"""Generate localnet genesis files with 100 LIGHT-mnemonic accounts.

Derives 100 addresses from BIP-44 path m/44'/60'/0'/0/{0..99} using the
LIGHT mnemonic and writes configs/localnet/{genesis,pchain,cchain}.json.

Usage:
    python scripts/gen-localnet.py
"""

import json
import os

from eth_account import Account

MNEMONIC = (
    "light light light light light light light light light light light energy"
)
PATH_TEMPLATE = "m/44'/60'/0'/0/{i}"
NUM_ACCOUNTS = 100

# P/X chain: 500M LUX = 500_000_000 * 1e9 nLUX = 500_000_000_000_000_000
PX_AMOUNT = 500_000_000_000_000_000

# C-chain: 500M LUX = 500_000_000 * 1e18 wei
C_BALANCE_WEI = 500_000_000 * 10**18

NETWORK_ID = 1337
CCHAIN_ID = 31337
HRP = "local"

# Existing staker config — preserved as-is
INITIAL_STAKERS = [
    {
        "nodeID": "NodeID-7Xhw2mDxuDS44j42TCB6U5579esbSt3Lg",
        "rewardAddress": "P-lux1c7wevm4667l4umtzh93r25wpxlpsadkhka6gv6",
        "delegationFee": 20000,
        "signer": {
            "publicKey": "0xb3ebbe748a1f06d19ee25d4e345ba8d6b5a426498a140c2519b518e3e6224abd7895075892f361acf24c10af968bc7de",
            "proofOfPossession": "0xa2569a137e65d3a507c4f1f7ffac74ec05916ba44dfbbb84d42c2736a2bc1e8be14038d3aeeeac7c78e19ecdde69d830051959f22559641a3f9e42377d7f64580acdc383c5c9e22f7f1114712a543c6997d6dc59c88555423497d9fff41fa79a",
        },
        "weight": 10000000000,
    },
    {
        "nodeID": "NodeID-MFrZFVCXPv5iCn6M9K6XduxGTYp891xXZ",
        "rewardAddress": "P-lux1c7wevm4667l4umtzh93r25wpxlpsadkhka6gv6",
        "delegationFee": 20000,
        "signer": {
            "publicKey": "0x8b49c4259529a801cc961c72248773b550379ff718a49f4ccc0e1f2ac338fe204432329aa1712f408f97eee12b22dd05",
            "proofOfPossession": "0xaf92674c3615d462527675da121b06023b116ddebc6e163919e562ab7cbe6adf20515cd2fc17c3e2d2d59953f285229e15f04302187bef5a4aa3ebdea1d18bf34047be654dd12491e882fb90b17b3cbde99ad43fc5cd0c26828bbe49a4b9456c",
        },
        "weight": 10000000000,
    },
    {
        "nodeID": "NodeID-NFBbbJ4qCmNaCzeW7sxErhvWqvEQMnYcN",
        "rewardAddress": "P-lux1c7wevm4667l4umtzh93r25wpxlpsadkhka6gv6",
        "delegationFee": 20000,
        "signer": {
            "publicKey": "0xb20ab07ea5cf8b77ab50f2071a6f1d2aab693c8bd89761430cd29de9fa0dbae83a32c7697d59ff1b06995b1596c16fa6",
            "proofOfPossession": "0xa113145564d7ac540fade2526938fbd33233957901111328c86c4cbe7ed25f678173d95c54b3342fd7b723c3ed813f2b04fecf4a317205ce65638bb34bb58bb6746e74c63e6cbe5cb25c103b290ed270146b756d3901c8b0f99d083a44572c79",
        },
        "weight": 10000000000,
    },
    {
        "nodeID": "NodeID-GWPcbFJZFfZreETSoWjPimr846mXEKCtu",
        "rewardAddress": "P-lux1c7wevm4667l4umtzh93r25wpxlpsadkhka6gv6",
        "delegationFee": 20000,
        "signer": {
            "publicKey": "0xb43f74196c2b9e9980f815a99288ef76166b42e93663dcbce3526f9652cdb58b31a3d68798b08fb9806024620ca1d6cd",
            "proofOfPossession": "0x8880500af123806295355eadbb36084619de729e2c4ac057f73784eb806ae19b4738550617fa8d6820cf1a352e19af6d042e140b3c967b8573e063c1e5aabce50dc2156e9caa6428b58e06a3624ac8170793ff4b4095397c1c981fbb2383dae6",
        },
        "weight": 10000000000,
    },
    {
        "nodeID": "NodeID-P7oB2McjBGgW2NXXWVYjV8JEDFoW9xDE5",
        "rewardAddress": "P-lux1c7wevm4667l4umtzh93r25wpxlpsadkhka6gv6",
        "delegationFee": 20000,
        "signer": {
            "publicKey": "0xa37e938a8e1fb71286ae1a9dc81585bae5c63d8cae3b95160ce9acce621c8ee64afa0491a2c757ba24d8be2da1052090",
            "proofOfPossession": "0x8e01fe13a6fa3ca5be4f78230b0d03ffff4741f186ddebf9a04819e8011eb9c1b81debad2e52a35ec89f4dfc517884b9075e46968d6c735f17cca3ab5c07a2cdc181233fcd8e9260d9fa7edc12572dff13972c9b3233f34258fb34d9e6b732ba",
        },
        "weight": 10000000000,
    },
]

# Bech32 encoding for Lux addresses
BECH32_CHARSET = "qpzry9x8gf2tvdw0s3jn54khce6mua7l"


def bech32_polymod(values: list[int]) -> int:
    gen = [0x3B6A57B2, 0x26508E6D, 0x1EA119FA, 0x3D4233DD, 0x2A1462B3]
    chk = 1
    for v in values:
        b = chk >> 25
        chk = ((chk & 0x1FFFFFF) << 5) ^ v
        for i in range(5):
            chk ^= gen[i] if ((b >> i) & 1) else 0
    return chk


def bech32_hrp_expand(hrp: str) -> list[int]:
    return [ord(x) >> 5 for x in hrp] + [0] + [ord(x) & 31 for x in hrp]


def bech32_create_checksum(hrp: str, data: list[int]) -> list[int]:
    values = bech32_hrp_expand(hrp) + data
    polymod = bech32_polymod(values + [0, 0, 0, 0, 0, 0]) ^ 1
    return [(polymod >> 5 * (5 - i)) & 31 for i in range(6)]


def convertbits(data: bytes, frombits: int, tobits: int, pad: bool = True) -> list[int]:
    acc = 0
    bits = 0
    ret = []
    maxv = (1 << tobits) - 1
    for value in data:
        acc = (acc << frombits) | value
        bits += frombits
        while bits >= tobits:
            bits -= tobits
            ret.append((acc >> bits) & maxv)
    if pad:
        if bits:
            ret.append((acc << (tobits - bits)) & maxv)
    elif bits >= frombits or ((acc << (tobits - bits)) & maxv):
        return []
    return ret


def bech32_encode(hrp: str, data: bytes) -> str:
    """Encode raw 20-byte address to bech32."""
    data5 = convertbits(data, 8, 5)
    checksum = bech32_create_checksum(hrp, data5)
    return hrp + "1" + "".join(BECH32_CHARSET[d] for d in data5 + checksum)


def eth_addr_to_bech32(eth_addr: str, hrp: str) -> str:
    """Convert 0x-prefixed Ethereum address to Lux bech32 format."""
    raw = bytes.fromhex(eth_addr[2:])
    return bech32_encode(hrp, raw)


def derive_accounts(mnemonic: str, count: int) -> list[dict]:
    Account.enable_unaudited_hdwallet_features()
    accounts = []
    for i in range(count):
        path = PATH_TEMPLATE.format(i=i)
        acct = Account.from_mnemonic(mnemonic, account_path=path)
        accounts.append({"index": i, "address": acct.address})
    return accounts


def build_px_allocations(accounts: list[dict]) -> tuple[list[dict], list[str]]:
    """Build P/X chain allocations. Returns (allocations, initialStakedFunds)."""
    allocations = []
    staked_funds = []

    for acct in accounts:
        eth_addr = acct["address"].lower()
        bech32_addr = eth_addr_to_bech32(eth_addr, HRP)

        # X-chain allocation
        allocations.append({
            "evmAddr": eth_addr,
            "utxoAddr": f"X-{bech32_addr}",
            "initialAmount": PX_AMOUNT,
            "unlockSchedule": [],
        })

        # P-chain allocation
        allocations.append({
            "evmAddr": eth_addr,
            "utxoAddr": f"P-{bech32_addr}",
            "initialAmount": PX_AMOUNT,
            "unlockSchedule": [],
        })

        # First 5 accounts stake
        if acct["index"] < 5:
            staked_funds.append(f"P-{bech32_addr}")

    return allocations, staked_funds


def build_cchain_alloc(accounts: list[dict]) -> dict:
    """Build C-chain alloc map: address -> {balance: hex}."""
    balance_hex = hex(C_BALANCE_WEI)
    alloc = {}
    for acct in accounts:
        # Strip 0x prefix for cchain alloc keys (matching existing convention)
        addr = acct["address"][2:]
        alloc[addr] = {"balance": balance_hex}
    return alloc


def build_cchain_genesis(alloc: dict) -> dict:
    """Build standalone cchain.json content."""
    return {
        "config": {
            "chainId": CCHAIN_ID,
            "homesteadBlock": 0,
            "eip150Block": 0,
            "eip155Block": 0,
            "eip158Block": 0,
            "byzantiumBlock": 0,
            "constantinopleBlock": 0,
            "petersburgBlock": 0,
            "istanbulBlock": 0,
            "muirGlacierBlock": 0,
            "berlinBlock": 0,
            "londonBlock": 0,
            "arrowGlacierBlock": 0,
            "grayGlacierBlock": 0,
            "mergeNetsplitBlock": 0,
            "shanghaiTime": 0,
            "cancunTime": 0,
            "terminalTotalDifficulty": 0,
            "chainEVMTimestamp": 0,
            "durangoTimestamp": 0,
            "etnaTimestamp": 0,
            "fortunaTimestamp": 0,
            "graniteTimestamp": 0,
            "blobSchedule": {
                "cancun": {
                    "target": 3,
                    "max": 6,
                    "baseFeeUpdateFraction": 3338477,
                }
            },
            "feeConfig": {
                "gasLimit": 15000000,
                "targetBlockRate": 1,
                "minBaseFee": 1000000000,
                "targetGas": 60000000,
                "baseFeeChangeDenominator": 36,
                "minBlockGasCost": 0,
                "maxBlockGasCost": 1000000,
                "blockGasCostStep": 200000,
            },
            "warpConfig": {
                "blockTimestamp": 0,
                "quorumNumerator": 67,
                "requirePrimaryNetworkSigners": False,
            },
        },
        "nonce": "0x0",
        "timestamp": "0x0",
        "extraData": "0x",
        "gasLimit": "0xe4e1c0",
        "difficulty": "0x0",
        "mixHash": "0x0000000000000000000000000000000000000000000000000000000000000000",
        "coinbase": "0x0000000000000000000000000000000000000000",
        "alloc": {
            # Warp precompile
            "0200000000000000000000000000000000000005": {
                "balance": "0x0",
                "code": "0x01",
                "nonce": "0x1",
            },
            **alloc,
        },
        "number": "0x0",
        "gasUsed": "0x0",
        "parentHash": "0x0000000000000000000000000000000000000000000000000000000000000000",
        "baseFeePerGas": "0x3b9aca00",
    }


def build_pchain(allocations: list[dict], staked_funds: list[str]) -> dict:
    """Build pchain.json (split file for Go embed)."""
    return {
        "allocations": allocations,
        "initialStakeDuration": 31536000,
        "initialStakeDurationOffset": 5400,
        "initialStakedFunds": staked_funds,
        "initialStakers": INITIAL_STAKERS,
    }


def build_genesis(
    allocations: list[dict],
    staked_funds: list[str],
    cchain_genesis: dict,
) -> dict:
    """Build combined genesis.json."""
    return {
        "allocations": allocations,
        "cChainGenesis": json.dumps(cchain_genesis, separators=(",", ":")),
        "initialStakeDuration": 31536000,
        "initialStakeDurationOffset": 5400,
        "initialStakedFunds": staked_funds,
        "initialStakers": INITIAL_STAKERS,
        "message": "Lux et Libertas",
        "networkID": NETWORK_ID,
        "startTime": 1766708400,
        "dChainGenesis": '{"version":1,"message":"Lux Chain Genesis"}',
        "qChainGenesis": '{"version":1,"message":"Lux Chain Genesis"}',
        "aChainGenesis": '{"version":1,"message":"Lux Chain Genesis"}',
        "bChainGenesis": '{"version":1,"message":"Lux Chain Genesis"}',
        "tChainGenesis": '{"version":1,"message":"Lux Chain Genesis"}',
        "zChainGenesis": '{"version":1,"message":"Lux Chain Genesis"}',
        "gChainGenesis": '{"version":1,"message":"Lux Chain Genesis"}',
        "kChainGenesis": '{"version":1,"message":"Lux Chain Genesis"}',
    }


def main():
    repo_root = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
    out_dir = os.path.join(repo_root, "configs", "localnet")

    print(f"Deriving {NUM_ACCOUNTS} accounts from LIGHT mnemonic...")
    accounts = derive_accounts(MNEMONIC, NUM_ACCOUNTS)

    print(f"Account 0: {accounts[0]['address']}")
    print(f"Account 99: {accounts[99]['address']}")

    # Build P/X allocations
    allocations, staked_funds = build_px_allocations(accounts)
    print(f"P/X allocations: {len(allocations)} entries ({len(allocations)//2} accounts x 2 chains)")
    print(f"Initial staked funds: {len(staked_funds)} addresses")

    # Build C-chain
    cchain_alloc = build_cchain_alloc(accounts)
    cchain_genesis = build_cchain_genesis(cchain_alloc)
    print(f"C-chain alloc: {len(cchain_alloc)} addresses")

    # Build pchain (split file for Go embed)
    pchain = build_pchain(allocations, staked_funds)

    # Build combined genesis
    genesis = build_genesis(allocations, staked_funds, cchain_genesis)

    # Write pchain.json (split file, read by Go embed)
    pchain_path = os.path.join(out_dir, "pchain.json")
    with open(pchain_path, "w") as f:
        json.dump(pchain, f, indent=2)
        f.write("\n")
    print(f"Wrote {pchain_path}")

    # Write cchain.json (standalone, readable object)
    cchain_path = os.path.join(out_dir, "cchain.json")
    with open(cchain_path, "w") as f:
        json.dump(cchain_genesis, f, indent=2)
        f.write("\n")
    print(f"Wrote {cchain_path}")

    # Write genesis.json (combined, with cChainGenesis as string)
    genesis_path = os.path.join(out_dir, "genesis.json")
    with open(genesis_path, "w") as f:
        json.dump(genesis, f, indent=2)
        f.write("\n")
    print(f"Wrote {genesis_path}")

    # Summary
    print(f"\nDone. {NUM_ACCOUNTS} LIGHT accounts, 500M LUX each on P/X/C chains.")


if __name__ == "__main__":
    main()
