#!/usr/bin/env python3
"""Derive 100 Ethereum accounts from a BIP39 mnemonic for Lux C-chain genesis allocation.

Usage:
    python derive_accounts.py > accounts_100.json
"""

import json
import sys

from eth_account import Account


MNEMONIC = "REDACTED_USE_KMS"
PATH_TEMPLATE = "m/44'/60'/0'/0/{i}"
NUM_ACCOUNTS = 100

# 2 trillion LUX in wei (2e30) for account 0
BALANCE_ACCOUNT_0 = "0x193e5939a08ce9dbd480000000"

# 1 billion LUX in wei (1e27) for accounts 1-99
BALANCE_DEFAULT = "0x33b2e3c9fd0803ce8000000"


def derive_accounts(mnemonic: str, count: int) -> list[dict]:
    Account.enable_unaudited_hdwallet_features()
    accounts = []
    for i in range(count):
        path = PATH_TEMPLATE.format(i=i)
        acct = Account.from_mnemonic(mnemonic, account_path=path)
        accounts.append({
            "index": i,
            "address": acct.address,
            "private_key_hex": acct.key.hex(),
        })
    return accounts


def build_alloc(accounts: list[dict]) -> dict:
    alloc = {}
    for entry in accounts:
        addr = entry["address"]
        balance = BALANCE_ACCOUNT_0 if entry["index"] == 0 else BALANCE_DEFAULT
        alloc[addr] = {"balance": balance}
    return alloc


def main():
    accounts = derive_accounts(MNEMONIC, NUM_ACCOUNTS)
    alloc = build_alloc(accounts)

    output = {
        "mnemonic": MNEMONIC,
        "derivation_path_template": PATH_TEMPLATE,
        "num_accounts": NUM_ACCOUNTS,
        "account_0_balance": BALANCE_ACCOUNT_0,
        "default_balance": BALANCE_DEFAULT,
        "accounts": accounts,
        "alloc": alloc,
    }

    json.dump(output, sys.stdout, indent=2)
    sys.stdout.write("\n")


if __name__ == "__main__":
    main()
