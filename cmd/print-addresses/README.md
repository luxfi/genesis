# Print Addresses Tool

A versatile command-line tool to generate and display Lux C/P/X addresses from various inputs including mnemonic phrases, private keys, or EVM addresses.

## Features

- **Multiple Input Types**:
  - Mnemonic phrases (12-24 words)
  - Private keys (hex format)
  - EVM addresses (with limitations - see note)
  - Comma-separated lists of any of the above

- **Address Generation**:
  - Generates all chain addresses: C-Chain (EVM), P-Chain, X-Chain
  - Proper EVM address format using Keccak256
  - Supports multiple accounts from single mnemonic
  - Testnet address generation
  - JSON output format

## Usage

```bash
go run cmd/print-addresses/main.go [options] <input>
```

### Options

- `-n`: Number of accounts to generate (default: 1, only applies to mnemonic mode)
- `--json`: Output in JSON format
- `--testnet`: Also show testnet addresses

### Input Types

#### 1. Private Key
```bash
# With 0x prefix
go run cmd/print-addresses/main.go 0x0000000000000000000000000000000000000000000000000000000000000001

# Without 0x prefix
go run cmd/print-addresses/main.go 0000000000000000000000000000000000000000000000000000000000000001
```

#### 2. Mnemonic Phrase
```bash
# Generate first account
go run cmd/print-addresses/main.go "your twelve word mnemonic phrase goes here like this example phrase"

# Generate multiple accounts
go run cmd/print-addresses/main.go -n=5 "your twelve word mnemonic phrase goes here like this example phrase"
```

#### 3. EVM Address (Limited)
```bash
# Shows only the EVM address
go run cmd/print-addresses/main.go 0x742d35Cc6634C0532925a3b844Bc9e7595f88899

# Note: P-Chain and X-Chain addresses CANNOT be derived from an EVM address
# because they use different hashing algorithms from the same public key
```

#### 4. Multiple Inputs (Comma-Separated)
```bash
# Mix of private keys, addresses, and mnemonics
go run cmd/print-addresses/main.go "0x123...,0x456...,word1 word2...,0x789..."
```

## Output

For each input, the tool displays:

### From Private Key or Mnemonic:
- **Address ID**: The raw address identifier (Lux format)
- **EVM Address**: The Ethereum-compatible address (0x...) using Keccak256
- **C-Chain**: Contract chain address (same as EVM address)
- **P-Chain**: Platform chain address (for staking) using SHA256+RIPEMD160
- **X-Chain**: Exchange chain address (for transfers) using SHA256+RIPEMD160
- **Private Key**: The private key in hex format (keep this secret!)

### From EVM Address:
- **EVM Address**: The input address
- **Note**: P-Chain and X-Chain addresses cannot be derived from EVM address alone

## Examples

### Single Private Key
```bash
$ go run cmd/print-addresses/main.go 0x0000000000000000000000000000000000000000000000000000000000000001

Account #0:
  Address ID: BgGZ9tcN4rm9KBzDn7KprQz87SZ25DaKF
  EVM Address: 0x7e5f4552091a69125d5dfcb7b8c2659029395bdf
  C-Chain: C-lux1w508d6qejxtdg4y5r3zarvary0c5xw7k4xzf9k
  P-Chain: P-lux1w508d6qejxtdg4y5r3zarvary0c5xw7k4xzf9k
  X-Chain: X-lux1w508d6qejxtdg4y5r3zarvary0c5xw7k4xzf9k
  Private Key: 0x0000000000000000000000000000000000000000000000000000000000000001
```

### Mnemonic with Testnet
```bash
$ go run cmd/print-addresses/main.go --testnet "test test test test test test test test test test test junk"

Account #0:
  Address ID: 4dxGw2WvZqDLu9JZMVtFqhkKPMPszsVRH
  EVM Address: 0x5a299b0010bac9c0339b6ef600b1f2943131b1e7
  C-Chain: C-lux1yljhuvjkmtu0y5ls6kf4exsdd8gea9mpat4tes
  P-Chain: P-lux1yljhuvjkmtu0y5ls6kf4exsdd8gea9mpat4tes
  X-Chain: X-lux1yljhuvjkmtu0y5ls6kf4exsdd8gea9mpat4tes
  Testnet:
    C-Chain: C-testnet1yljhuvjkmtu0y5ls6kf4exsdd8gea9mplmgrjn
    P-Chain: P-testnet1yljhuvjkmtu0y5ls6kf4exsdd8gea9mplmgrjn
    X-Chain: X-testnet1yljhuvjkmtu0y5ls6kf4exsdd8gea9mplmgrjn
  Private Key: 0x211cdc80c23ccc8eceab5d6903312391e656366a7a553e2c501b06add1729816
```

### JSON Output
```bash
$ go run cmd/print-addresses/main.go --json -n=2 "test test test test test test test test test test test junk"

{
  "addresses": [
    {
      "addressId": "4dxGw2WvZqDLu9JZMVtFqhkKPMPszsVRH",
      "evmAddress": "0x5a299b0010bac9c0339b6ef600b1f2943131b1e7",
      "cChain": "C-lux1yljhuvjkmtu0y5ls6kf4exsdd8gea9mpat4tes",
      "pChain": "P-lux1yljhuvjkmtu0y5ls6kf4exsdd8gea9mpat4tes",
      "xChain": "X-lux1yljhuvjkmtu0y5ls6kf4exsdd8gea9mpat4tes",
      "privateKey": "0x211cdc80c23ccc8eceab5d6903312391e656366a7a553e2c501b06add1729816"
    },
    {
      "accountIndex": 1,
      "addressId": "6S99ggiA4nVcEdZRDzpfeg5CeksTq8FAd",
      "evmAddress": "0xff9bc69a6554a511e92236c019a58e5ab5f0e486",
      "cChain": "C-lux18wvaf02nxrfpxhz5fwrj5yjydhhrlef6gf8cnm",
      "pChain": "P-lux18wvaf02nxrfpxhz5fwrj5yjydhhrlef6gf8cnm",
      "xChain": "X-lux18wvaf02nxrfpxhz5fwrj5yjydhhrlef6gf8cnm",
      "privateKey": "0x9f8799874aeb19dc930f9e5d82c71ebe361bdeea343907e87cba737589dd17eb"
    }
  ]
}
```

### EVM Address Input
```bash
$ go run cmd/print-addresses/main.go 0x5a299b0010bac9c0339b6ef600b1f2943131b1e7

EVM Address: 0x5a299b0010bac9c0339b6ef600b1f2943131b1e7
Note: P-Chain and X-Chain addresses cannot be derived from an EVM address alone.
They require the original public key, which cannot be recovered from the EVM address.
```

## Technical Details

### Address Generation
- **EVM addresses**: Keccak256(publicKey)[12:] (last 20 bytes)
- **P/X-Chain addresses**: SHA256(RIPEMD160(publicKey)) encoded as bech32
- Uses BIP44 derivation path: `m/44'/9000'/0'/0/{account}`
- Coin type 9000 is the registered coin type for Lux/Avalanche

### Why EVM addresses can't map to P/X addresses
1. EVM uses Keccak256 hash of the public key
2. P/X chains use SHA256+RIPEMD160 hash of the same public key
3. You cannot reverse a hash to get the original public key
4. Therefore, you need the private key or public key to generate all addresses

### Network Support
- **Mainnet**: HRP "lux"
- **Testnet**: HRP "testnet"

## Security Warning

⚠️ **NEVER share your private keys or mnemonic phrases!**
⚠️ **This tool displays private keys in plain text - use only in secure environments**
⚠️ **When using EVM addresses, no private key can be derived**

## Building

```bash
cd /Users/z/work/lux/genesis
go build -o bin/print-addresses ./cmd/print-addresses
```

## Use Cases

1. **Wallet Recovery**: Convert mnemonic to addresses and private keys
2. **Address Verification**: Check all chain addresses for a given private key
3. **Development/Testing**: Generate test addresses quickly
4. **Batch Processing**: Process multiple addresses/keys at once
5. **Network Testing**: Generate both mainnet and testnet addresses