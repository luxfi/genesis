package genesis

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Config represents genesis configuration
type Config struct {
	Network      string
	ChainID      uint64
	NetworkID    uint64
	ChainType    string
	Validators   int
	Allocations  map[string]string
	StakingStart uint64
	StakingEnd   uint64
}

// Generator handles genesis generation
type Generator struct {
	config *Config
}

// NewGenerator creates a new genesis generator
func NewGenerator(config *Config) *Generator {
	return &Generator{
		config: config,
	}
}

// Generate generates all genesis files
func (g *Generator) Generate(outputDir string) error {
	// Create output directory structure
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Generate P-Chain genesis
	if err := g.generatePChain(outputDir); err != nil {
		return fmt.Errorf("failed to generate P-Chain genesis: %w", err)
	}

	// Generate C-Chain genesis
	if err := g.generateCChain(outputDir); err != nil {
		return fmt.Errorf("failed to generate C-Chain genesis: %w", err)
	}

	// Generate X-Chain genesis
	if err := g.generateXChain(outputDir); err != nil {
		return fmt.Errorf("failed to generate X-Chain genesis: %w", err)
	}

	// Generate combined genesis.json for luxd
	if err := g.generateCombined(outputDir); err != nil {
		return fmt.Errorf("failed to generate combined genesis: %w", err)
	}

	return nil
}

func (g *Generator) generatePChain(outputDir string) error {
	pDir := filepath.Join(outputDir, "P")
	if err := os.MkdirAll(pDir, 0755); err != nil {
		return err
	}

	// P-Chain genesis structure
	pGenesis := map[string]interface{}{
		"networkID":                  g.config.NetworkID,
		"allocations":                []interface{}{},
		"startTime":                  1607133600, // Default start time
		"initialStakeDuration":       31536000,   // 1 year
		"initialStakeDurationOffset": 5400,       // 90 minutes
		"initialStakers":             []interface{}{},
		"cChainGenesis":              "", // Will be filled later
		"message":                    fmt.Sprintf("%s genesis", g.config.Network),
	}

	// Write P-Chain genesis
	pGenesisPath := filepath.Join(pDir, "genesis.json")
	data, err := json.MarshalIndent(pGenesis, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(pGenesisPath, data, 0644)
}

func (g *Generator) generateCChain(outputDir string) error {
	cDir := filepath.Join(outputDir, "C")
	if err := os.MkdirAll(cDir, 0755); err != nil {
		return err
	}

	// C-Chain genesis structure (EVM compatible)
	cGenesis := map[string]interface{}{
		"config": map[string]interface{}{
			"chainId":             g.config.ChainID,
			"homesteadBlock":      0,
			"eip150Block":         0,
			"eip150Hash":          "0x0000000000000000000000000000000000000000000000000000000000000000",
			"eip155Block":         0,
			"eip158Block":         0,
			"byzantiumBlock":      0,
			"constantinopleBlock": 0,
			"petersburgBlock":     0,
			"istanbulBlock":       0,
			"muirGlacierBlock":    0,
			"berlinBlock":         0,
			"londonBlock":         0,
		},
		"nonce":      "0x0",
		"timestamp":  "0x0",
		"extraData":  "0x00",
		"gasLimit":   "0x7A1200", // 8,000,000
		"difficulty": "0x0",
		"mixHash":    "0x0000000000000000000000000000000000000000000000000000000000000000",
		"coinbase":   "0x0000000000000000000000000000000000000000",
		"alloc":      g.config.Allocations,
		"number":     "0x0",
		"gasUsed":    "0x0",
		"parentHash": "0x0000000000000000000000000000000000000000000000000000000000000000",
	}

	// Write C-Chain genesis
	cGenesisPath := filepath.Join(cDir, "genesis.json")
	data, err := json.MarshalIndent(cGenesis, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(cGenesisPath, data, 0644)
}

func (g *Generator) generateXChain(outputDir string) error {
	xDir := filepath.Join(outputDir, "X")
	if err := os.MkdirAll(xDir, 0755); err != nil {
		return err
	}

	// X-Chain genesis structure
	xGenesis := map[string]interface{}{
		"networkID":        g.config.NetworkID,
		"allocations":      []interface{}{},
		"startTime":        1607133600,
		"initialStakeDuration": 31536000,
		"initialStakeDurationOffset": 5400,
		"initialStakedFunds": []string{},
		"initialStakers": []interface{}{},
		"cChainGenesis": "",
		"message": fmt.Sprintf("%s X-Chain genesis", g.config.Network),
	}

	// Write X-Chain genesis
	xGenesisPath := filepath.Join(xDir, "genesis.json")
	data, err := json.MarshalIndent(xGenesis, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(xGenesisPath, data, 0644)
}

func (g *Generator) generateCombined(outputDir string) error {
	// Read all chain genesis files
	pData, err := os.ReadFile(filepath.Join(outputDir, "P", "genesis.json"))
	if err != nil {
		return err
	}

	cData, err := os.ReadFile(filepath.Join(outputDir, "C", "genesis.json"))
	if err != nil {
		return err
	}

	xData, err := os.ReadFile(filepath.Join(outputDir, "X", "genesis.json"))
	if err != nil {
		return err
	}

	// Create combined genesis
	combined := map[string]json.RawMessage{
		"P": json.RawMessage(pData),
		"C": json.RawMessage(cData),
		"X": json.RawMessage(xData),
	}

	// Write combined genesis
	combinedPath := filepath.Join(outputDir, "genesis.json")
	data, err := json.MarshalIndent(combined, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(combinedPath, data, 0644)
}

// Preset configurations

// MainnetConfig returns mainnet configuration
func MainnetConfig() *Config {
	return &Config{
		Network:   "mainnet",
		ChainID:   96369,
		NetworkID: 96369,
		ChainType: "l1",
		Validators: 21,
		Allocations: map[string]string{
			// Treasury
			"0x1000000000000000000000000000000000000000": "1000000000000000000000000000",
			// Development
			"0x2000000000000000000000000000000000000000": "500000000000000000000000000",
			// Ecosystem
			"0x3000000000000000000000000000000000000000": "300000000000000000000000000",
		},
	}
}

// TestnetConfig returns testnet configuration
func TestnetConfig() *Config {
	return &Config{
		Network:   "testnet",
		ChainID:   96368,
		NetworkID: 96368,
		ChainType: "l1",
		Validators: 11,
		Allocations: map[string]string{
			// Test treasury
			"0x1000000000000000000000000000000000000000": "100000000000000000000000000",
			// Test faucet
			"0x4000000000000000000000000000000000000000": "50000000000000000000000000",
		},
	}
}

// LocalConfig returns local development configuration
func LocalConfig(validators int) *Config {
	if validators == 0 {
		validators = 5
	}
	
	return &Config{
		Network:   "local",
		ChainID:   1337,
		NetworkID: 1337,
		ChainType: "l1",
		Validators: validators,
		Allocations: map[string]string{
			// Local test account
			"0x8db97C7cEcE249c2b98bDC0226Cc4C2A57BF52FC": "100000000000000000000000",
		},
	}
}