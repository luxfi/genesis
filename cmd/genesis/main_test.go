package main

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

func TestGenerateL1Genesis(t *testing.T) {
	// Create temp directory
	tmpDir, err := ioutil.TempDir("", "genesis-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Set output directory
	outputDir = tmpDir

	// Test L1 generation
	chainType = "l1"
	network = "test-network"
	chainID = 12345

	// Run generate
	runGenerate(nil, nil)

	// Check if file was created
	expectedFile := filepath.Join(tmpDir, "l1-test-network-genesis.json")
	if _, err := os.Stat(expectedFile); os.IsNotExist(err) {
		t.Errorf("Genesis file not created: %s", expectedFile)
	}

	// Read and validate content
	data, err := ioutil.ReadFile(expectedFile)
	if err != nil {
		t.Fatal(err)
	}

	var config GenesisConfig
	if err := json.Unmarshal(data, &config); err != nil {
		t.Fatal(err)
	}

	// Validate fields
	if config.ChainID != 12345 {
		t.Errorf("Expected chainID 12345, got %d", config.ChainID)
	}
	if config.Type != "l1" {
		t.Errorf("Expected type l1, got %s", config.Type)
	}
	if len(config.Validators) != 3 {
		t.Errorf("Expected 3 validators, got %d", len(config.Validators))
	}
}

func TestGenerateL2Genesis(t *testing.T) {
	// Create temp directory
	tmpDir, err := ioutil.TempDir("", "genesis-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	outputDir = tmpDir
	chainType = "l2"
	network = "test-l2"
	chainID = 23456
	baseChain = "lux"

	runGenerate(nil, nil)

	// Check file
	expectedFile := filepath.Join(tmpDir, "l2-test-l2-genesis.json")
	data, err := ioutil.ReadFile(expectedFile)
	if err != nil {
		t.Fatal(err)
	}

	var config GenesisConfig
	if err := json.Unmarshal(data, &config); err != nil {
		t.Fatal(err)
	}

	// Validate L2 specific fields
	if config.L2Config == nil {
		t.Error("L2Config is nil")
	} else if config.L2Config.BaseChain != "lux" {
		t.Errorf("Expected base chain lux, got %s", config.L2Config.BaseChain)
	}
}

func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name   string
		config GenesisConfig
		errors int
	}{
		{
			name: "valid config",
			config: GenesisConfig{
				ChainID:  1,
				Type:     "l1",
				GasLimit: 30000000,
				Alloc: map[string]GenesisAccount{
					"0x1234567890123456789012345678901234567890": {
						Balance: "1000000000000000000",
					},
				},
			},
			errors: 0,
		},
		{
			name: "missing chain id",
			config: GenesisConfig{
				Type:     "l1",
				GasLimit: 30000000,
			},
			errors: 1,
		},
		{
			name: "invalid address",
			config: GenesisConfig{
				ChainID:  1,
				Type:     "l1",
				GasLimit: 30000000,
				Alloc: map[string]GenesisAccount{
					"invalid-address": {
						Balance: "1000",
					},
				},
			},
			errors: 1,
		},
		{
			name: "missing L2 config",
			config: GenesisConfig{
				ChainID:  1,
				Type:     "l2",
				GasLimit: 30000000,
			},
			errors: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := validateConfig(&tt.config)
			if len(errors) != tt.errors {
				t.Errorf("Expected %d errors, got %d: %v", tt.errors, len(errors), errors)
			}
		})
	}
}

func TestQuantumGenesis(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "genesis-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	outputDir = tmpDir
	chainType = "quantum"
	network = "quantum-test"
	chainID = 369369

	runGenerate(nil, nil)

	// Read generated file
	expectedFile := filepath.Join(tmpDir, "quantum-quantum-test-genesis.json")
	data, err := ioutil.ReadFile(expectedFile)
	if err != nil {
		t.Fatal(err)
	}

	var config GenesisConfig
	if err := json.Unmarshal(data, &config); err != nil {
		t.Fatal(err)
	}

	// Validate quantum specific fields
	if config.QuantumConfig == nil {
		t.Error("QuantumConfig is nil")
	} else {
		if config.QuantumConfig.ConsensusMode != "quantum-byzantine" {
			t.Errorf("Expected quantum-byzantine consensus, got %s", config.QuantumConfig.ConsensusMode)
		}
	}
}

func TestAllocationBalances(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "genesis-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	outputDir = tmpDir
	chainType = "l1"
	network = "test"
	chainID = 1

	runGenerate(nil, nil)

	// Read file
	expectedFile := filepath.Join(tmpDir, "l1-test-genesis.json")
	data, err := ioutil.ReadFile(expectedFile)
	if err != nil {
		t.Fatal(err)
	}

	var config GenesisConfig
	if err := json.Unmarshal(data, &config); err != nil {
		t.Fatal(err)
	}

	// Check default allocations exist
	expectedAddresses := []string{
		"0x1000000000000000000000000000000000000000", // Treasury
		"0x2000000000000000000000000000000000000000", // Development
		"0x3000000000000000000000000000000000000000", // Ecosystem
	}

	for _, addr := range expectedAddresses {
		if _, exists := config.Alloc[addr]; !exists {
			t.Errorf("Expected allocation for %s not found", addr)
		}
	}
}