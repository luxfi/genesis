
package main
import (
    "fmt"
    "github.com/cockroachdb/pebble"
)
func main() {
    db, _ := pebble.Open("/Users/z/work/lux/state/chaindata/lux-mainnet-96369/db/pebbledb", &pebble.Options{ReadOnly: true})
    defer db.Close()
    
    // Check specific address: 8d5081153aE1cfb41f5c932fe0b6Beb7E159cF84
    key := []byte{0x00, 0x8d, 0x50, 0x81, 0x15, 0x3a, 0xE1, 0xcf, 0xb4, 0x1f, 0x5c, 0x93, 0x2f, 0xe0, 0xb6, 0xBe, 0xb7, 0xE1, 0x59, 0xcF, 0x84}
    
    val, closer, err := db.Get(key)
    if err \!= nil {
        fmt.Printf("Address not found: %v
", err)
    } else {
        fmt.Printf("Found\! Raw value length: %d bytes
", len(val))
        closer.Close()
    }
}

