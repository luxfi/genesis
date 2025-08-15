package main
import (
	"encoding/binary"
	"github.com/cockroachdb/pebble"
	"github.com/luxfi/geth/common"
)
func main() {
	db, _ := pebble.Open("/Users/z/.luxd/network-96369/chains/EWi9aPkGe6EfJ3SobCAmSUXRPLa4brF3cThwPwmHTrD1y13jy/vm", &pebble.Options{})
	defer db.Close()
	db.Set([]byte("initialized"), []byte{1}, pebble.Sync)
	db.Set([]byte("lastAccepted"), common.HexToHash("0x32dede1fc8e0f11ecde12fb42aef7933fc6c5fcf863bc277b5eac08ae4d461f0").Bytes(), pebble.Sync)
	height := make([]byte, 8)
	binary.BigEndian.PutUint64(height, 1082780)
	db.Set([]byte("lastAcceptedHeight"), height, pebble.Sync)
}
