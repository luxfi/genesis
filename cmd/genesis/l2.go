package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/luxfi/genesis/pkg/core"
	"github.com/luxfi/genesis/pkg/launch"
	"github.com/spf13/cobra"
)

// getL2Cmd returns the L2 management command group
func getL2Cmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "l2",
		Short: "Manage L2 networks",
		Long:  "Create, configure, and manage Layer 2 networks on Lux",
	}

	cmd.AddCommand(getL2CreateCmd())
	cmd.AddCommand(getL2ListCmd())
	cmd.AddCommand(getL2InfoCmd())
	cmd.AddCommand(getL2DeleteCmd())

	return cmd
}

// getL2CreateCmd creates a new L2 network configuration
func getL2CreateCmd() *cobra.Command {
	var (
		name        string
		chainID     uint64
		testnetID   uint64
		symbol      string
		displayName string
		baseNetwork string
		chainData   string
	)

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new L2 network configuration",
		Long: `Create a new L2 network configuration that can be launched later.

Examples:
  # Create a new L2 for mainnet
  genesis l2 create --name=defi --chain-id=50000 --symbol=DEFI
  
  # Create with testnet variant
  genesis l2 create --name=gaming --chain-id=60000 --testnet-id=60001 --symbol=GAME
  
  # Create with existing chaindata
  genesis l2 create --name=zoo --chain-id=200200 --symbol=ZOO --chaindata=/path/to/data`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return createL2Config(name, chainID, testnetID, symbol, displayName, baseNetwork, chainData)
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Network name (required)")
	cmd.Flags().Uint64Var(&chainID, "chain-id", 0, "Mainnet chain ID (required)")
	cmd.Flags().Uint64Var(&testnetID, "testnet-id", 0, "Testnet chain ID (defaults to mainnet+1)")
	cmd.Flags().StringVar(&symbol, "symbol", "", "Token symbol (required)")
	cmd.Flags().StringVar(&displayName, "display-name", "", "Display name (defaults to capitalized name)")
	cmd.Flags().StringVar(&baseNetwork, "base", "lux", "Base network: lux or luxtest")
	cmd.Flags().StringVar(&chainData, "chaindata", "", "Path to existing chaindata to import")

	_ = cmd.MarkFlagRequired("name")
	_ = cmd.MarkFlagRequired("chain-id")
	_ = cmd.MarkFlagRequired("symbol")

	return cmd
}

// getL2ListCmd lists all configured L2 networks
func getL2ListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all L2 network configurations",
		RunE:  listL2Networks,
	}
	return cmd
}

// getL2InfoCmd shows info about a specific L2 network
func getL2InfoCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "info [name]",
		Short: "Show information about an L2 network",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return showL2Info(args[0])
		},
	}
	return cmd
}

// getL2DeleteCmd deletes an L2 network configuration
func getL2DeleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete [name]",
		Short: "Delete an L2 network configuration",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return deleteL2Config(args[0])
		},
	}
	return cmd
}

// L2Config represents a saved L2 network configuration
type L2Config struct {
	Name          string `json:"name"`
	DisplayName   string `json:"display_name"`
	ChainID       uint64 `json:"chain_id"`
	TestnetID     uint64 `json:"testnet_id"`
	Symbol        string `json:"symbol"`
	BaseNetwork   string `json:"base_network"`
	ChainDataPath string `json:"chaindata_path,omitempty"`
	CreatedAt     string `json:"created_at"`

	// Subnet configuration
	SubnetConfig map[string]interface{} `json:"subnet_config"`
}

func createL2Config(name string, chainID uint64, testnetID uint64, symbol string, displayName string, baseNetwork string, chainData string) error {
	// Validate name
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		return fmt.Errorf("name cannot be empty")
	}

	// Check if already exists
	configPath := getL2ConfigPath(name)
	if _, err := os.Stat(configPath); err == nil {
		return fmt.Errorf("L2 network '%s' already exists. Use 'genesis l2 delete %s' first", name, name)
	}

	// Set defaults
	if displayName == "" {
		displayName = strings.Title(name) + " Network"
	}
	if testnetID == 0 {
		testnetID = chainID + 1
	}

	// Validate chaindata if provided
	if chainData != "" {
		if _, err := os.Stat(chainData); err != nil {
			return fmt.Errorf("chaindata path does not exist: %s", chainData)
		}
		// Convert to absolute path
		chainData, _ = filepath.Abs(chainData)
	}

	// Create subnet configuration
	subnetConfig := map[string]interface{}{
		"chainId": chainID,
		"nativeMinterPrecompile": map[string]interface{}{
			"blockTimestamp": 0,
			"adminAddresses": []string{},
		},
		"contractDeployerAllowListConfig": map[string]interface{}{
			"blockTimestamp": 0,
			"adminAddresses": []string{},
		},
		"txAllowListConfig": map[string]interface{}{
			"blockTimestamp": 0,
			"adminAddresses": []string{},
		},
		"feeConfig": map[string]interface{}{
			"gasLimit":                 20000000,
			"targetBlockRate":          2,
			"minBaseFee":               25000000000,
			"targetGas":                100000000,
			"baseFeeChangeDenominator": 36,
			"minBlockGasCost":          0,
			"maxBlockGasCost":          10000000,
			"blockGasCostStep":         500000,
		},
		"warpConfig": map[string]interface{}{
			"blockTimestamp":  0,
			"quorumNumerator": 67,
		},
	}

	// Create L2 configuration
	config := L2Config{
		Name:          name,
		DisplayName:   displayName,
		ChainID:       chainID,
		TestnetID:     testnetID,
		Symbol:        symbol,
		BaseNetwork:   baseNetwork,
		ChainDataPath: chainData,
		CreatedAt:     time.Now().Format(time.RFC3339),
		SubnetConfig:  subnetConfig,
	}

	// Save configuration
	configDir := filepath.Dir(configPath)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	// Print success message
	fmt.Printf("✅ Created L2 network configuration: %s\n", name)
	fmt.Printf("\nNetwork Details:\n")
	fmt.Printf("  Name:         %s\n", name)
	fmt.Printf("  Display:      %s\n", displayName)
	fmt.Printf("  Chain ID:     %d (mainnet) / %d (testnet)\n", chainID, testnetID)
	fmt.Printf("  Symbol:       %s\n", symbol)
	fmt.Printf("  Base Network: %s\n", baseNetwork)
	if chainData != "" {
		fmt.Printf("  Chain Data:   %s\n", chainData)
	}

	fmt.Printf("\nTo launch this network:\n")
	fmt.Printf("  genesis launch %s      # Launch on %s\n", name+"net", baseNetwork)
	fmt.Printf("  genesis launch %s     # Launch on %stest\n", name+"test", baseNetwork)

	// Also register in our predefined networks for easy launch
	registerL2Network(name, config)

	return nil
}

func listL2Networks(cmd *cobra.Command, args []string) error {
	configDir := getL2ConfigDir()

	// List predefined networks
	fmt.Println("Predefined L2 Networks:")
	fmt.Println("======================")
	fmt.Println("  zoonet    - ZOO Network (200200)")
	fmt.Println("  zootest   - ZOO Testnet (200201)")
	fmt.Println("  spcnet    - SPC Network (36911)")
	fmt.Println("  spctest   - SPC Testnet (36912)")
	fmt.Println("  hanzonet  - Hanzo Network (36963)")
	fmt.Println("  hanzotest - Hanzo Testnet (36962)")

	// List custom networks
	fmt.Println("\nCustom L2 Networks:")
	fmt.Println("===================")

	files, err := os.ReadDir(configDir)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("  None")
			return nil
		}
		return err
	}

	found := false
	for _, file := range files {
		if strings.HasSuffix(file.Name(), ".json") {
			name := strings.TrimSuffix(file.Name(), ".json")

			// Load config to show details
			config, err := loadL2Config(name)
			if err != nil {
				continue
			}

			fmt.Printf("  %-12s - %s (%d)\n", name, config.DisplayName, config.ChainID)
			found = true
		}
	}

	if !found {
		fmt.Println("  None")
	}

	return nil
}

func showL2Info(name string) error {
	// Check presets first
	if network, ok := launch.Presets[name]; ok {
		fmt.Printf("Network: %s (preset)\n", name)
		fmt.Printf("Network ID: %d\n", network.NetworkID)
		fmt.Printf("Chain ID: %d\n", network.ChainID)
		if tokenSymbol, ok := network.Metadata["token-symbol"].(string); ok {
			fmt.Printf("Symbol: %s\n", tokenSymbol)
		}
		if base, ok := network.Metadata["l2-base-chain"].(string); ok {
			fmt.Printf("Base: %s\n", base)
		}
		return nil
	}

	// Load custom config
	config, err := loadL2Config(name)
	if err != nil {
		return fmt.Errorf("L2 network '%s' not found", name)
	}

	fmt.Printf("Network: %s\n", config.Name)
	fmt.Printf("Display: %s\n", config.DisplayName)
	fmt.Printf("Chain ID: %d (mainnet) / %d (testnet)\n", config.ChainID, config.TestnetID)
	fmt.Printf("Symbol: %s\n", config.Symbol)
	fmt.Printf("Base: %s\n", config.BaseNetwork)
	fmt.Printf("Created: %s\n", config.CreatedAt)

	if config.ChainDataPath != "" {
		fmt.Printf("Chain Data: %s\n", config.ChainDataPath)
	}

	fmt.Printf("\nSubnet Configuration:\n")
	configJSON, _ := json.MarshalIndent(config.SubnetConfig, "  ", "  ")
	fmt.Printf("%s\n", configJSON)

	return nil
}

func deleteL2Config(name string) error {
	configPath := getL2ConfigPath(name)

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return fmt.Errorf("L2 network '%s' not found", name)
	}

	// Confirm deletion
	fmt.Printf("Are you sure you want to delete L2 network '%s'? [y/N]: ", name)
	var response string
	_, _ = fmt.Scanln(&response)

	if strings.ToLower(response) != "y" {
		fmt.Println("Deletion cancelled")
		return nil
	}

	if err := os.Remove(configPath); err != nil {
		return fmt.Errorf("failed to delete config: %w", err)
	}

	fmt.Printf("✅ Deleted L2 network configuration: %s\n", name)
	return nil
}

// Helper functions

func getL2ConfigDir() string {
	homeDir, _ := os.UserHomeDir()
	return filepath.Join(homeDir, ".luxd", "l2-configs")
}

func getL2ConfigPath(name string) string {
	return filepath.Join(getL2ConfigDir(), name+".json")
}

func loadL2Config(name string) (*L2Config, error) {
	configPath := getL2ConfigPath(name)

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	var config L2Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

func registerL2Network(name string, config L2Config) {
	// Create mainnet preset
	mainnetNetwork := core.Network{
		Name:      name + "net",
		NetworkID: config.ChainID,
		ChainID:   config.ChainID,
		Nodes:     1,
		Genesis:   core.GenesisConfig{Source: "fresh"},
		Consensus: core.ConsensusConfig{K: 3, Alpha: 3, Beta: 5},
		Metadata: map[string]interface{}{
			"l2-base-chain": "lux",
			"token-symbol":  config.Symbol,
			"display-name":  config.DisplayName,
		},
	}

	// Create testnet preset
	testnetNetwork := core.Network{
		Name:      name + "test",
		NetworkID: config.TestnetID,
		ChainID:   config.TestnetID,
		Nodes:     1,
		Genesis:   core.GenesisConfig{Source: "fresh"},
		Consensus: core.ConsensusConfig{K: 3, Alpha: 3, Beta: 5},
		Metadata: map[string]interface{}{
			"l2-base-chain": "luxtest",
			"token-symbol":  config.Symbol,
			"display-name":  config.DisplayName + " Testnet",
			"is-testnet":    true,
		},
	}

	// TODO: Add to presets dynamically
	_ = mainnetNetwork
	_ = testnetNetwork
}
