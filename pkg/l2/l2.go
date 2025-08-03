package l2

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/luxfi/genesis/pkg/application"
)

// Manager handles L2 network operations
type Manager struct {
	app *application.Genesis
}

// L2Config represents an L2 network configuration
type L2Config struct {
	Name          string    `json:"name"`
	ChainID       uint64    `json:"chainId"`
	TestnetID     uint64    `json:"testnetId,omitempty"`
	Symbol        string    `json:"symbol"`
	DisplayName   string    `json:"displayName"`
	BaseNetwork   string    `json:"baseNetwork"`
	ChainDataPath string    `json:"chainDataPath,omitempty"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

// New creates a new L2 Manager
func New(app *application.Genesis) *Manager {
	return &Manager{app: app}
}

// Create creates a new L2 network configuration
func (m *Manager) Create(config L2Config) error {
	// Validate configuration
	if config.Name == "" {
		return fmt.Errorf("L2 name is required")
	}
	if config.ChainID == 0 {
		return fmt.Errorf("chain ID is required")
	}
	if config.Symbol == "" {
		return fmt.Errorf("token symbol is required")
	}

	// Set timestamps
	config.CreatedAt = time.Now()
	config.UpdatedAt = config.CreatedAt

	// Create L2 directory
	l2Dir := filepath.Join(m.app.BaseDir, "l2", config.Name)
	if err := os.MkdirAll(l2Dir, 0755); err != nil {
		return fmt.Errorf("failed to create L2 directory: %w", err)
	}

	// Save configuration
	configPath := filepath.Join(l2Dir, "config.json")
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	// Generate genesis configuration
	genesis := m.generateGenesisConfig(config)
	genesisPath := filepath.Join(l2Dir, "genesis.json")
	genesisData, err := json.MarshalIndent(genesis, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal genesis: %w", err)
	}

	if err := os.WriteFile(genesisPath, genesisData, 0644); err != nil {
		return fmt.Errorf("failed to save genesis: %w", err)
	}

	m.app.Log.Info("L2 network created",
		"name", config.Name,
		"chainId", config.ChainID,
		"symbol", config.Symbol,
		"path", l2Dir)

	return nil
}

// List returns all configured L2 networks
func (m *Manager) List() ([]L2Config, error) {
	l2Dir := filepath.Join(m.app.BaseDir, "l2")
	if _, err := os.Stat(l2Dir); os.IsNotExist(err) {
		return []L2Config{}, nil
	}

	entries, err := os.ReadDir(l2Dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read L2 directory: %w", err)
	}

	var configs []L2Config
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		configPath := filepath.Join(l2Dir, entry.Name(), "config.json")
		data, err := os.ReadFile(configPath)
		if err != nil {
			m.app.Log.Warn("Failed to read L2 config", "name", entry.Name(), "error", err)
			continue
		}

		var config L2Config
		if err := json.Unmarshal(data, &config); err != nil {
			m.app.Log.Warn("Failed to parse L2 config", "name", entry.Name(), "error", err)
			continue
		}

		configs = append(configs, config)
	}

	return configs, nil
}

// Get retrieves a specific L2 configuration
func (m *Manager) Get(name string) (*L2Config, error) {
	configPath := filepath.Join(m.app.BaseDir, "l2", name, "config.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("L2 network '%s' not found", name)
		}
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	var config L2Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	return &config, nil
}

// Delete removes an L2 network configuration
func (m *Manager) Delete(name string) error {
	l2Path := filepath.Join(m.app.BaseDir, "l2", name)
	if _, err := os.Stat(l2Path); os.IsNotExist(err) {
		return fmt.Errorf("L2 network '%s' not found", name)
	}

	if err := os.RemoveAll(l2Path); err != nil {
		return fmt.Errorf("failed to delete L2 network: %w", err)
	}

	m.app.Log.Info("L2 network deleted", "name", name)
	return nil
}

// generateGenesisConfig creates a genesis configuration for the L2
func (m *Manager) generateGenesisConfig(config L2Config) map[string]interface{} {
	// Base configuration similar to SubnetEVM
	return map[string]interface{}{
		"config": map[string]interface{}{
			"chainId":             config.ChainID,
			"homesteadBlock":      0,
			"eip150Block":         0,
			"eip155Block":         0,
			"eip158Block":         0,
			"byzantiumBlock":      0,
			"constantinopleBlock": 0,
			"petersburgBlock":     0,
			"istanbulBlock":       0,
			"muirGlacierBlock":    0,
			"berlinBlock":         0,
			"londonBlock":         0,
			"subnetEVMTimestamp":  0,
			"feeConfig": map[string]interface{}{
				"gasLimit":                 8000000,
				"minBaseFee":               25000000000,
				"targetGas":                15000000,
				"baseFeeChangeDenominator": 36,
				"minBlockGasCost":          0,
				"maxBlockGasCost":          1000000,
				"targetBlockRate":          2,
				"blockGasCostStep":         200000,
			},
		},
		"nonce":      "0x0",
		"timestamp":  "0x0",
		"gasLimit":   "0x7A1200",
		"difficulty": "0x0",
		"mixHash":    "0x0000000000000000000000000000000000000000000000000000000000000000",
		"coinbase":   "0x0000000000000000000000000000000000000000",
		"number":     "0x0",
		"gasUsed":    "0x0",
		"parentHash": "0x0000000000000000000000000000000000000000000000000000000000000000",
		"alloc":      map[string]interface{}{},
	}
}
