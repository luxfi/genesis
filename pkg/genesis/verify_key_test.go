package genesis_test

import (
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/luxfi/genesis/pkg/genesis"
)

func TestFee0Key(t *testing.T) {
	privKeyHex := "abd51d463510cb17d7ba09e535828383d9c2c817aa386024aacce1660a1ee625"

	keyInfo, err := genesis.ComputeValidatorKeyInfo(privKeyHex)
	if err != nil {
		t.Fatal(err)
	}

	fmt.Printf("ETH address: %s\n", keyInfo.EVMAddr)
	fmt.Printf("ShortID (hex): %s\n", hex.EncodeToString(keyInfo.ShortID[:]))

	pAddr, err := genesis.FormatChainAddress("P", "dev", keyInfo.ShortID)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Printf("P-chain address: %s\n", pAddr)
	fmt.Printf("\nExpected: P-dev1e44zjaddy52vjqa40ws90uwu9c2ryp7egufeqg\n")
}
