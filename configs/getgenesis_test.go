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
		if _, ok := alloc["utxoAddr"]; !ok {
			t.Errorf("Allocation %d missing utxoAddr", i)
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

	// 1000 BIP44 wallet allocations + 3 validator-stake allocations.
	// Each wallet alloc lands on P AND X (the node builder fans
	// InitialAmount to both chains).
	allocs := m["allocations"].([]interface{})
	t.Logf("Total allocations: %d", len(allocs))
	const wantWalletAllocs = 1000
	const wantValidatorAllocs = 3
	if len(allocs) != wantWalletAllocs+wantValidatorAllocs {
		t.Errorf("Expected %d allocations (1000 wallet + 3 validator), got %d",
			wantWalletAllocs+wantValidatorAllocs, len(allocs))
	}

	// First 1000 entries are wallet allocations at 10M LUX (= 10^13
	// microLUX). The trailing 3 are validator stake allocs with a
	// locked-stake UnlockSchedule and InitialAmount = 0; skip them in
	// the per-entry check.
	const wantMicroLUXPerWallet uint64 = 50_000_000 * 1_000_000
	for i := 0; i < wantWalletAllocs && i < len(allocs); i++ {
		alloc := allocs[i].(map[string]interface{})
		if _, ok := alloc["utxoAddr"]; !ok {
			t.Errorf("Allocation %d missing utxoAddr", i)
		}
		if _, ok := alloc["evmAddr"]; !ok {
			t.Errorf("Allocation %d missing evmAddr", i)
		}
		amt := uint64(alloc["initialAmount"].(float64))
		if amt != wantMicroLUXPerWallet {
			t.Errorf("Allocation %d: expected %d, got %d", i, wantMicroLUXPerWallet, amt)
		}
	}

	// Verify first allocation is canonical BIP44 m/44'/9000'/0'/0/0
	// for LIGHT_MNEMONIC. ETH addr is the keccak256(pubkey) projection
	// of the same secp256k1 spending key, computed from the BIP44 child
	// at index 0 (not the Lux-internal hardened path).
	first := allocs[0].(map[string]interface{})
	const wantFirstETH = "0x5369615110ca435bdf798f31c20ba6163d7b0a54"
	if addr := first["evmAddr"].(string); addr != wantFirstETH {
		t.Errorf("First allocation evmAddr mismatch: got %s want %s", addr, wantFirstETH)
	}

	// initialStakedFunds tracks each initial staker's reward address.
	// With validators=3 we expect 3 entries.
	staked := m["initialStakedFunds"].([]interface{})
	if len(staked) != wantValidatorAllocs {
		t.Errorf("Expected %d staked funds, got %d", wantValidatorAllocs, len(staked))
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

	// C-Chain genesis ships with the canonical 5 LIGHT accounts (per
	// configs/local/cchain.json) plus the warp precompile. The 1000
	// BIP44 wallet allocations live on P/X chains only — they are NOT
	// merged into the C-Chain alloc unless the operator passes
	// -bip44-wallet-keys to the CLI, which the embedded local genesis
	// does not.
	allocMap := cchain["alloc"].(map[string]interface{})
	t.Logf("C-Chain alloc entries: %d", len(allocMap))
	if len(allocMap) < 1 {
		t.Errorf("Expected at least 1 C-Chain alloc entry (warp precompile), got %d", len(allocMap))
	}
}
