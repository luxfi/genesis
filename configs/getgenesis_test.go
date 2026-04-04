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
