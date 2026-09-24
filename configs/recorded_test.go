package configs

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"testing"
)

// Mainnet and testnet boot the document their validators were born from: the
// genesis.json of the luxd-genesis ConfigMap in lux-mainnet and lux-testnet
// (luxfi/universe k8s/lux-{mainnet,testnet}/luxd-genesis.yaml), copied byte for
// byte on 2026-09-23. Its C-Chain block 0 is 0x3f4fa2a0… (mainnet) and
// 0x1c5fe377… (testnet), the first block of luxfi/state's exports; its C-Chain
// blockchain ID is 2Hx3UMuW… and 5kjSQWfw…, pinned in luxfi/node where the
// builder is. A change to either document is a different network, so the
// digest is pinned here and a change has to be made on purpose.
func TestRecordedGenesisIsTheLiveDocument(t *testing.T) {
	for _, c := range []struct {
		id      uint32
		digest  string
		stakers []string
	}{
		{MainnetID, "6b72375efbf8683cec4a22e826cb707b99e2467142da13d452f42106ef11da14", []string{
			"NodeID-Mf3JfSY91oDwfBqf7rCLmhg4NDtDghw1f",
			"NodeID-2TwSZ2oyeBK2mv7JiseEQ8m74rotDj4QR",
			"NodeID-Ld9VFBQ9zGbd79z2vzaAqkQ3jHuqbtRpo",
			"NodeID-8mY2fhUehN27v3LCU84BnnKEoeRfd2weC",
			"NodeID-DwsrqSkPoE3pXWrUt9nkJ5yBycwRQ246X",
		}},
		{TestnetID, "7e71539137c048cee0a30ebcbb1941238bdcfd6ee9fd3a2dc2b119e98266a93f", []string{
			"NodeID-Jc1ACAWeKDZR4N5PgxNsPrrcKrKMTh4ct",
			"NodeID-CTyFvG1HYdcMLqdmKwqA8EFPRQdmrNat3",
			"NodeID-Hvsc6vanABE3ufcMhpMVEMVGpZ68DhoMb",
			"NodeID-44e37hMkUbjpoWsJrWcvBB4kcMK3ab8qg",
			"NodeID-HcFvgYUxXJx4gzQ7Sgsj8grYK6FDEgBrr",
		}},
	} {
		t.Run(networkNameFromID(c.id), func(t *testing.T) {
			doc, err := GetGenesis(c.id)
			if err != nil {
				t.Fatal(err)
			}
			sum := sha256.Sum256(doc)
			if got := hex.EncodeToString(sum[:]); got != c.digest {
				t.Fatalf("network %d boots a document with digest %s; the live one is %s", c.id, got, c.digest)
			}
			var g struct {
				Stakers []struct {
					NodeID string `json:"nodeID"`
				} `json:"initialStakers"`
			}
			if err := json.Unmarshal(doc, &g); err != nil {
				t.Fatal(err)
			}
			if len(g.Stakers) != len(c.stakers) {
				t.Fatalf("network %d has %d initial stakers, want %d", c.id, len(g.Stakers), len(c.stakers))
			}
			for i, s := range g.Stakers {
				if s.NodeID != c.stakers[i] {
					t.Errorf("network %d staker %d is %s, want %s", c.id, i, s.NodeID, c.stakers[i])
				}
			}
		})
	}
}

// assembledNetworks are the shipped networks built from shards, the ones with
// no history to keep. mainnet and testnet are recorded.
var assembledNetworks = []string{"devnet", "localnet"}

// Allocations handed to a recorded network would found a different one, so
// they are refused rather than applied.
func TestRecordedGenesisTakesNoAllocations(t *testing.T) {
	alloc := []byte(`{"allocations":[{"evmAddr":"0x0000000000000000000000000000000000000001","utxoAddr":"P-lux1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqq","initialAmount":1}]}`)
	for _, id := range []uint32{MainnetID, TestnetID} {
		t.Setenv("PCHAIN_ALLOCS", string(alloc))
		if _, err := GetGenesis(id); err == nil {
			t.Errorf("network %d accepted PCHAIN_ALLOCS", id)
		}
	}
}
