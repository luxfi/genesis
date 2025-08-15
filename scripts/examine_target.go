package main

import (
    "bytes"
    "encoding/hex"
    "fmt"
    "log"
    "math/big"
    
    "github.com/cockroachdb/pebble"
    "github.com/ethereum/go-ethereum/common"
    "github.com/ethereum/go-ethereum/crypto"
    "github.com/ethereum/go-ethereum/rlp"
)

// SubnetEVM namespace prefix (32 bytes)
var subnetNamespace = []byte{
    0x33, 0x7f, 0xb7, 0x3f, 0x9b, 0xcd, 0xac, 0x8c,
    0x31, 0xa2, 0xd5, 0xf7, 0xb8, 0x77, 0xab, 0x1e,
    0x8a, 0x2b, 0x7f, 0x2a, 0x1e, 0x9b, 0xf0, 0x2a,
    0x0a, 0x0e, 0x6c, 0x6f, 0xd1, 0x64, 0xf1, 0xd1,
}

// Account represents an Ethereum account
type Account struct {
    Nonce    uint64
    Balance  *big.Int
    Root     common.Hash // merkle root of the storage trie
    CodeHash []byte
}

func main() {
    sourcePath := "/Users/z/work/lux/state/chaindata/lux-mainnet-96369/db/pebbledb"
    
    fmt.Println("Examining Target Address Entries")
    fmt.Println("================================")
    
    // Open source PebbleDB
    db, err := pebble.Open(sourcePath, &pebble.Options{
        ReadOnly: true,
    })
    if err != nil {
        log.Fatal("Failed to open database:", err)
    }
    defer db.Close()
    
    // Target address - luxdefi.eth
    targetAddr := common.HexToAddress("0x9011E888251AB053B7bD1cdB598Db4f9DEd94714")
    fmt.Printf("Target address: %s (luxdefi.eth)\n", targetAddr.Hex())
    
    // Calculate the account hash (keccak256 of address)
    accountHash := crypto.Keccak256(targetAddr.Bytes())
    fmt.Printf("Account hash: %x\n\n", accountHash)
    
    // Keys found from previous scan that contain the address
    keysToCheck := []string{
        "337fb73f9bcdac8c31a2d5f7b877ab1e8a2b7f2a1e9bf02a0a0e6c6fd164f1d100a7a8e9c052fb6d5fa84066e49e283562318170a19d83b71322ce0bc6cc3ce7",
        "337fb73f9bcdac8c31a2d5f7b877ab1e8a2b7f2a1e9bf02a0a0e6c6fd164f1d10491139590c6bdb4fc1bf76b283d6898cd518f3a30c7208c3a40f23b422610ee",
        "337fb73f9bcdac8c31a2d5f7b877ab1e8a2b7f2a1e9bf02a0a0e6c6fd164f1d105f208655f209d3d68a0213e7073aa4d6f814d24110fd33e5524261dc797136a",
        "337fb73f9bcdac8c31a2d5f7b877ab1e8a2b7f2a1e9bf02a0a0e6c6fd164f1d10911a6828198c27b13d8811b37bd4eeef50349d0eb473196e6d24d42ade83197",
    }
    
    for i, keyHex := range keysToCheck {
        key, err := hex.DecodeString(keyHex)
        if err != nil {
            fmt.Printf("Error decoding key %d: %v\n", i+1, err)
            continue
        }
        
        val, closer, err := db.Get(key)
        if err != nil {
            fmt.Printf("Error getting value for key %d: %v\n", i+1, err)
            continue
        }
        defer closer.Close()
        
        fmt.Printf("Entry %d:\n", i+1)
        fmt.Printf("  Key (after namespace): %x\n", key[32:])
        fmt.Printf("  Value length: %d bytes\n", len(val))
        fmt.Printf("  Value: %x\n", val)
        
        // Try to decode as account
        var acc Account
        if err := rlp.DecodeBytes(val, &acc); err == nil {
            fmt.Printf("  ✓ Decoded as account:\n")
            fmt.Printf("    Balance: %s wei\n", acc.Balance.String())
            fmt.Printf("    Balance: %s LUX\n", formatBalance(acc.Balance))
            fmt.Printf("    Nonce: %d\n", acc.Nonce)
        } else {
            fmt.Printf("  ✗ Failed to decode as account: %v\n", err)
            
            // Check if value contains the address directly
            if bytes.Contains(val, targetAddr.Bytes()) {
                fmt.Printf("  Note: Value contains target address bytes\n")
                // Find position
                pos := bytes.Index(val, targetAddr.Bytes())
                fmt.Printf("  Address found at position %d\n", pos)
            }
        }
        fmt.Println()
    }
    
    // Also try to look up the account directly using the account hash
    fmt.Println("Trying direct account lookup with hash...")
    
    // Try with namespace + account hash
    accountKey := make([]byte, 64)
    copy(accountKey[:32], subnetNamespace)
    copy(accountKey[32:], accountHash)
    
    val, closer, err := db.Get(accountKey)
    if err == nil {
        defer closer.Close()
        fmt.Printf("Found account with hash key!\n")
        fmt.Printf("  Value length: %d bytes\n", len(val))
        
        var acc Account
        if err := rlp.DecodeBytes(val, &acc); err == nil {
            fmt.Printf("  ✓ Balance: %s LUX\n", formatBalance(acc.Balance))
            fmt.Printf("  Nonce: %d\n", acc.Nonce)
        }
    } else {
        fmt.Printf("Account not found with hash key: %v\n", err)
    }
}

func formatBalance(balance *big.Int) string {
    if balance == nil {
        return "0"
    }
    
    // Convert from wei to LUX (18 decimals)
    ether := new(big.Float).SetInt(balance)
    divisor := new(big.Float).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil))
    ether.Quo(ether, divisor)
    
    return fmt.Sprintf("%s", ether.Text('f', 6))
}