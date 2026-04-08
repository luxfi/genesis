package configs

import (
	"encoding/json"
	"testing"
)

func TestGetGenesisMainnet(t *testing.T) {
	data, err := GetGenesis(1) // mainnet
	if err != nil {
		t.Fatalf("Error: %v", err)
	}

	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	allocs := m["allocations"].([]interface{})
	t.Logf("Total allocations: %d", len(allocs))

	if len(allocs) == 0 {
		t.Error("No allocations found in mainnet genesis")
	}

	// Verify each allocation has required fields
	for i, a := range allocs {
		alloc := a.(map[string]interface{})
		if _, ok := alloc["luxAddr"]; !ok {
			t.Errorf("Allocation %d missing luxAddr", i)
		}
	}

	// Verify C-Chain genesis exists with treasury
	cchainIface, ok := m["cChainGenesis"]
	if !ok {
		t.Fatal("cChainGenesis not found in genesis")
	}

	// cChainGenesis can be a string or object
	var cchain map[string]interface{}
	switch v := cchainIface.(type) {
	case string:
		if err := json.Unmarshal([]byte(v), &cchain); err != nil {
			t.Fatalf("Failed to parse cChainGenesis string: %v", err)
		}
	case map[string]interface{}:
		cchain = v
	default:
		t.Fatalf("Unexpected cChainGenesis type: %T", v)
	}

	// Check that alloc section exists in C-Chain
	if alloc, ok := cchain["alloc"]; ok {
		allocMap := alloc.(map[string]interface{})
		t.Logf("C-Chain alloc entries: %d", len(allocMap))

		// Check for treasury address (without 0x prefix, lowercase)
		treasuryAddr := "9011e888251ab053b7bd1cdb598db4f9ded94714"
		if _, found := allocMap[treasuryAddr]; found {
			t.Logf("Treasury found in C-Chain alloc: 0x%s", treasuryAddr)
		}
	}
}

func TestGetGenesisLocalnet(t *testing.T) {
	data, err := GetGenesis(1337) // localnet
	if err != nil {
		t.Fatalf("Error: %v", err)
	}

	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	// Verify networkID
	if nid := uint32(m["networkID"].(float64)); nid != 1337 {
		t.Fatalf("Expected networkID 1337, got %d", nid)
	}

	// 100 accounts x 2 chains (X + P) = 200 allocations
	allocs := m["allocations"].([]interface{})
	t.Logf("Total allocations: %d", len(allocs))
	if len(allocs) != 200 {
		t.Errorf("Expected 200 allocations (100 accounts x X+P), got %d", len(allocs))
	}

	// Verify each allocation has required fields and correct amount
	for i, a := range allocs {
		alloc := a.(map[string]interface{})
		if _, ok := alloc["luxAddr"]; !ok {
			t.Errorf("Allocation %d missing luxAddr", i)
		}
		if _, ok := alloc["ethAddr"]; !ok {
			t.Errorf("Allocation %d missing ethAddr", i)
		}
		amt := uint64(alloc["initialAmount"].(float64))
		if amt != 500_000_000_000_000_000 {
			t.Errorf("Allocation %d: expected 500000000000000000, got %d", i, amt)
		}
	}

	// Verify first allocation is LIGHT mnemonic account 0
	first := allocs[0].(map[string]interface{})
	if addr := first["ethAddr"].(string); addr != "0x35d64ff3f618f7a17df34dcb21be375a4686a8de" {
		t.Errorf("First allocation ethAddr mismatch: %s", addr)
	}

	// Verify initialStakedFunds has 5 entries
	staked := m["initialStakedFunds"].([]interface{})
	if len(staked) != 5 {
		t.Errorf("Expected 5 staked funds, got %d", len(staked))
	}

	// Verify C-Chain genesis
	cchainIface, ok := m["cChainGenesis"]
	if !ok {
		t.Fatal("cChainGenesis not found")
	}

	var cchain map[string]interface{}
	switch v := cchainIface.(type) {
	case string:
		if err := json.Unmarshal([]byte(v), &cchain); err != nil {
			t.Fatalf("Failed to parse cChainGenesis: %v", err)
		}
	case map[string]interface{}:
		cchain = v
	default:
		t.Fatalf("Unexpected cChainGenesis type: %T", v)
	}

	// Verify chainId
	config := cchain["config"].(map[string]interface{})
	if chainID := uint32(config["chainId"].(float64)); chainID != 31337 {
		t.Fatalf("Expected C-Chain chainId 31337, got %d", chainID)
	}

	// 100 LIGHT accounts + warp precompile = 101 entries
	allocMap := cchain["alloc"].(map[string]interface{})
	t.Logf("C-Chain alloc entries: %d", len(allocMap))
	if len(allocMap) != 101 {
		t.Errorf("Expected 101 C-Chain alloc entries (100 accounts + warp precompile), got %d", len(allocMap))
	}
}
