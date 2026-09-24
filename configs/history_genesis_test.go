package configs

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
)

// cchain is the C-Chain genesis inside a document: the document itself, or the
// cChainGenesis a network genesis carries, as an object or as a string.
func cchain(t *testing.T, raw []byte) map[string]any {
	t.Helper()
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	switch c := doc["cChainGenesis"].(type) {
	case string:
		doc = nil
		if err := json.Unmarshal([]byte(c), &doc); err != nil {
			t.Fatal(err)
		}
	case map[string]any:
		doc = c
	}
	// An address is the same account with or without 0x, in any case.
	if alloc, ok := doc["alloc"].(map[string]any); ok {
		norm := map[string]any{}
		for k, v := range alloc {
			norm[strings.ToLower(strings.TrimPrefix(k, "0x"))] = v
		}
		doc["alloc"] = norm
	}
	return doc
}

// Mainnet and testnet boot the genesis their history starts from. luxfi/state's
// exports open with the block these files make (mainnet 0x3f4fa2a0…, testnet
// 0x1c5fe377…), and block 1 of each names that block as its parent, so a node
// booted from any other allocation refuses the export at block 1.
// cchain.canonical.json and node-genesis.json are the history's own copies;
// what GetGenesis hands a node must agree with them on every field the genesis
// block commits to.
func TestBootedGenesisIsTheHistorys(t *testing.T) {
	for _, c := range []struct {
		id      uint32
		history string
	}{
		{1, "mainnet/cchain.canonical.json"},
		{2, "testnet/node-genesis.json"},
	} {
		t.Run(c.history, func(t *testing.T) {
			raw, err := GetGenesis(c.id)
			if err != nil {
				t.Fatal(err)
			}
			booted := cchain(t, raw)
			want, err := os.ReadFile(c.history)
			if err != nil {
				t.Fatal(err)
			}
			history := cchain(t, want)
			for _, f := range []string{
				"alloc", "coinbase", "difficulty", "extraData", "gasLimit", "gasUsed",
				"mixHash", "nonce", "number", "parentHash", "timestamp", "baseFeePerGas",
			} {
				if !reflect.DeepEqual(booted[f], history[f]) {
					t.Errorf("network %d boots %s = %v; its history has %v", c.id, f, booted[f], history[f])
				}
			}
		})
	}
}
