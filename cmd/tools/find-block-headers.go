package main

import (
    "encoding/hex"
    "fmt"
    "github.com/cockroachdb/pebble"
)

func main() {
    db, _ := pebble.Open("/home/z/work/lux/state/chaindata/lux-mainnet-96369/db/pebbledb", &pebble.Options{ReadOnly: true})
    defer db.Close()
    
    namespace := []byte{
        0x33, 0x7f, 0xb7, 0x3f, 0x9b, 0xcd, 0xac, 0x8c,
        0x31, 0xa2, 0xd5, 0xf7, 0xb8, 0x77, 0xab, 0x1e,
        0x8a, 0x2b, 0x7f, 0x2a, 0x1e, 0x9b, 0xf0, 0x2a,
        0x0a, 0x0e, 0x6c, 0x6f, 0xd1, 0x64, 0xf1, 0xd1,
    }
    
    // Try block 0 H-hash
    block0HHash, _ := hex.DecodeString("3f4fa2a0b0ce089f52bf0ae9199c75ffdd76ecafc987794050cb0d286f1ec61e")
    
    fmt.Println("Searching for block 0 header...")
    fmt.Printf("Block 0 H-hash: %x\n\n", block0HHash)
    
    // Try different key formats
    testKeys := []struct {
        name string
        key  []byte
    }{
        {"namespace + H + hash", append(append(namespace, 'H'), block0HHash...)},
        {"namespace + hash", append(namespace, block0HHash...)},
        {"H + hash", append([]byte{'H'}, block0HHash...)},
        {"h + hash", append([]byte{'h'}, block0HHash...)},
        {"just hash", block0HHash},
    }
    
    for _, test := range testKeys {
        fmt.Printf("Trying %s:\n", test.name)
        if val, closer, err := db.Get(test.key); err == nil {
            closer.Close()
            fmt.Printf("  FOUND! Value size: %d bytes\n", len(val))
            if len(val) > 0 {
                fmt.Printf("  First byte: 0x%02x\n", val[0])
                if val[0] == 0xf8 || val[0] == 0xf9 {
                    fmt.Println("  This is an RLP header!")
                }
            }
        } else {
            fmt.Println("  Not found")
        }
    }
}