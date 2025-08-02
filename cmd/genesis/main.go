package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math/big"
	"os"
	"path/filepath"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	// Version information
	Version   = "1.0.0"
	BuildTime = "unknown"
	GitCommit = "unknown"

	// Flags
	configFile string
	outputDir  string
	chainType  string
	network    string
	chainID    uint64
	baseChain  string

	// Commands
	rootCmd = &cobra.Command{
		Use:   "genesis",
		Short: "Genesis configuration tool for Lux, Zoo, and Quantum chains",
		Long: `A lightweight CLI tool for managing genesis configurations 
for L1, L2, L3, and Quantum chains in the Lux ecosystem.`,
	}

	generateCmd = &cobra.Command{
		Use:   "generate",
		Short: "Generate genesis configuration",
		Long:  `Generate genesis configuration for L1, L2, L3, or Quantum chains`,
		Run:   runGenerate,
	}

	launchCmd = &cobra.Command{
		Use:   "launch",
		Short: "Launch a new chain with genesis configuration",
		Long:  `Launch a new L1, L2, L3, or Quantum chain using the specified genesis configuration`,
		Run:   runLaunch,
	}

	validateCmd = &cobra.Command{
		Use:   "validate",
		Short: "Validate a genesis configuration",
		Long:  `Validate a genesis configuration file for correctness`,
		Run:   runValidate,
	}

	versionCmd = &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("Genesis CLI v%s\n", Version)
			fmt.Printf("Build Time: %s\n", BuildTime)
			fmt.Printf("Git Commit: %s\n", GitCommit)
		},
	}
)

func init() {
	cobra.OnInitialize(initConfig)

	// Global flags
	rootCmd.PersistentFlags().StringVar(&configFile, "config", "", "config file (default is ./genesis.yaml)")
	rootCmd.PersistentFlags().StringVar(&outputDir, "output", "./configs", "output directory for genesis files")

	// Generate command flags
	generateCmd.Flags().StringVar(&chainType, "type", "l1", "chain type: l1, l2, l3, or quantum")
	generateCmd.Flags().StringVar(&network, "network", "mainnet", "network name")
	generateCmd.Flags().Uint64Var(&chainID, "chain-id", 0, "chain ID")
	generateCmd.Flags().StringVar(&baseChain, "base-chain", "", "base chain for L2/L3 (e.g., lux, zoo)")

	// Launch command flags
	launchCmd.Flags().StringVar(&chainType, "type", "l1", "chain type: l1, l2, l3, or quantum")
	launchCmd.Flags().StringVar(&configFile, "genesis", "", "genesis configuration file")

	// Add commands
	rootCmd.AddCommand(generateCmd)
	rootCmd.AddCommand(launchCmd)
	rootCmd.AddCommand(validateCmd)
	rootCmd.AddCommand(versionCmd)
}

func initConfig() {
	if configFile != "" {
		viper.SetConfigFile(configFile)
	} else {
		viper.AddConfigPath(".")
		viper.SetConfigName("genesis")
		viper.SetConfigType("yaml")
	}

	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err == nil {
		fmt.Println("Using config file:", viper.ConfigFileUsed())
	}
}

// Genesis configuration structures
type GenesisConfig struct {
	ChainID        uint64                       `json:"chainId"`
	Type           string                       `json:"type"`
	Network        string                       `json:"network"`
	Timestamp      uint64                       `json:"timestamp"`
	GasLimit       uint64                       `json:"gasLimit"`
	Difficulty     *big.Int                     `json:"difficulty"`
	Alloc          map[string]GenesisAccount    `json:"alloc"`
	Validators     []Validator                  `json:"validators,omitempty"`
	L2Config       *L2Config                    `json:"l2Config,omitempty"`
	QuantumConfig  *QuantumConfig               `json:"quantumConfig,omitempty"`
}

type GenesisAccount struct {
	Balance string            `json:"balance"`
	Code    string            `json:"code,omitempty"`
	Storage map[string]string `json:"storage,omitempty"`
	Nonce   uint64            `json:"nonce,omitempty"`
}

type Validator struct {
	Address string `json:"address"`
	Weight  uint64 `json:"weight"`
}

type L2Config struct {
	BaseChain       string `json:"baseChain"`
	SequencerURL    string `json:"sequencerUrl"`
	BatcherAddress  string `json:"batcherAddress"`
	RollupAddress   string `json:"rollupAddress"`
}

type QuantumConfig struct {
	QuantumProof    string `json:"quantumProof"`
	EntanglementKey string `json:"entanglementKey"`
	ConsensusMode   string `json:"consensusMode"`
}

func runGenerate(cmd *cobra.Command, args []string) {
	fmt.Printf("Generating %s genesis configuration for %s...\n", chainType, network)

	config := &GenesisConfig{
		ChainID:    chainID,
		Type:       chainType,
		Network:    network,
		Timestamp:  uint64(time.Now().Unix()),
		GasLimit:   30000000,
		Difficulty: big.NewInt(1),
		Alloc:      make(map[string]GenesisAccount),
	}

	// Set default chain IDs if not specified
	if chainID == 0 {
		switch network {
		case "lux-mainnet":
			config.ChainID = 96369
		case "lux-testnet":
			config.ChainID = 96368
		case "zoo-mainnet":
			config.ChainID = 200200
		case "zoo-testnet":
			config.ChainID = 200201
		case "quantum-mainnet":
			config.ChainID = 369369
		default:
			config.ChainID = 1337 // Default for custom networks
		}
	}

	// Configure based on chain type
	switch chainType {
	case "l1":
		configureL1(config)
	case "l2":
		configureL2(config, baseChain)
	case "l3":
		configureL3(config, baseChain)
	case "quantum":
		configureQuantum(config)
	default:
		log.Fatalf("Unknown chain type: %s", chainType)
	}

	// Add default allocations
	addDefaultAllocations(config)

	// Save configuration
	saveConfig(config)
}

func configureL1(config *GenesisConfig) {
	// L1 specific configuration
	config.Validators = []Validator{
		{Address: "0x1234567890123456789012345678901234567890", Weight: 100},
		{Address: "0x2345678901234567890123456789012345678901", Weight: 100},
		{Address: "0x3456789012345678901234567890123456789012", Weight: 100},
	}
}

func configureL2(config *GenesisConfig, baseChain string) {
	// L2 specific configuration
	if baseChain == "" {
		baseChain = "lux"
	}
	
	config.L2Config = &L2Config{
		BaseChain:      baseChain,
		SequencerURL:   fmt.Sprintf("https://sequencer.%s.network", network),
		BatcherAddress: "0x4567890123456789012345678901234567890123",
		RollupAddress:  "0x5678901234567890123456789012345678901234",
	}
}

func configureL3(config *GenesisConfig, baseChain string) {
	// L3 specific configuration
	if baseChain == "" {
		baseChain = "zoo"
	}
	
	config.L2Config = &L2Config{
		BaseChain:      baseChain,
		SequencerURL:   fmt.Sprintf("https://l3-sequencer.%s.network", network),
		BatcherAddress: "0x6789012345678901234567890123456789012345",
		RollupAddress:  "0x7890123456789012345678901234567890123456",
	}
}

func configureQuantum(config *GenesisConfig) {
	// Quantum chain specific configuration
	config.QuantumConfig = &QuantumConfig{
		QuantumProof:    "0xQUANTUM_PROOF_PLACEHOLDER",
		EntanglementKey: "0xENTANGLEMENT_KEY_PLACEHOLDER",
		ConsensusMode:   "quantum-byzantine",
	}
	
	// Quantum validators
	config.Validators = []Validator{
		{Address: "0x8901234567890123456789012345678901234567", Weight: 150},
		{Address: "0x9012345678901234567890123456789012345678", Weight: 150},
		{Address: "0x0123456789012345678901234567890123456789", Weight: 150},
	}
}

func addDefaultAllocations(config *GenesisConfig) {
	// Treasury allocation
	config.Alloc["0x1000000000000000000000000000000000000000"] = GenesisAccount{
		Balance: "1000000000000000000000000000", // 1 billion tokens
	}
	
	// Development fund
	config.Alloc["0x2000000000000000000000000000000000000000"] = GenesisAccount{
		Balance: "500000000000000000000000000", // 500 million tokens
	}
	
	// Ecosystem fund
	config.Alloc["0x3000000000000000000000000000000000000000"] = GenesisAccount{
		Balance: "300000000000000000000000000", // 300 million tokens
	}
}

func saveConfig(config *GenesisConfig) {
	// Create output directory if it doesn't exist
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		log.Fatalf("Failed to create output directory: %v", err)
	}
	
	// Generate filename
	filename := fmt.Sprintf("%s-%s-genesis.json", config.Type, config.Network)
	filepath := filepath.Join(outputDir, filename)
	
	// Marshal to JSON
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		log.Fatalf("Failed to marshal genesis config: %v", err)
	}
	
	// Write to file
	if err := ioutil.WriteFile(filepath, data, 0644); err != nil {
		log.Fatalf("Failed to write genesis file: %v", err)
	}
	
	fmt.Printf("Genesis configuration saved to: %s\n", filepath)
}

func runLaunch(cmd *cobra.Command, args []string) {
	fmt.Printf("Launching %s chain with configuration: %s\n", chainType, configFile)
	
	// Read genesis configuration
	data, err := ioutil.ReadFile(configFile)
	if err != nil {
		log.Fatalf("Failed to read genesis file: %v", err)
	}
	
	var config GenesisConfig
	if err := json.Unmarshal(data, &config); err != nil {
		log.Fatalf("Failed to parse genesis file: %v", err)
	}
	
	// Launch based on chain type
	switch config.Type {
	case "l1":
		launchL1(&config)
	case "l2":
		launchL2(&config)
	case "l3":
		launchL3(&config)
	case "quantum":
		launchQuantum(&config)
	default:
		log.Fatalf("Unknown chain type: %s", config.Type)
	}
}

func launchL1(config *GenesisConfig) {
	fmt.Println("Launching L1 chain...")
	fmt.Printf("Chain ID: %d\n", config.ChainID)
	fmt.Printf("Network: %s\n", config.Network)
	fmt.Printf("Validators: %d\n", len(config.Validators))
	
	// TODO: Integrate with actual L1 launch process
	fmt.Println("L1 launch process initiated. Run 'lux network start' to complete.")
}

func launchL2(config *GenesisConfig) {
	fmt.Println("Launching L2 chain...")
	fmt.Printf("Chain ID: %d\n", config.ChainID)
	fmt.Printf("Base Chain: %s\n", config.L2Config.BaseChain)
	fmt.Printf("Sequencer: %s\n", config.L2Config.SequencerURL)
	
	// TODO: Integrate with L2 deployment process
	fmt.Println("L2 launch process initiated. Deploy rollup contracts to complete.")
}

func launchL3(config *GenesisConfig) {
	fmt.Println("Launching L3 app chain...")
	fmt.Printf("Chain ID: %d\n", config.ChainID)
	fmt.Printf("Base Chain: %s\n", config.L2Config.BaseChain)
	
	// TODO: Integrate with L3 deployment process
	fmt.Println("L3 launch process initiated. Deploy app chain contracts to complete.")
}

func launchQuantum(config *GenesisConfig) {
	fmt.Println("Launching Quantum chain...")
	fmt.Printf("Chain ID: %d\n", config.ChainID)
	fmt.Printf("Consensus Mode: %s\n", config.QuantumConfig.ConsensusMode)
	fmt.Printf("Validators: %d\n", len(config.Validators))
	
	// TODO: Integrate with quantum chain launch process
	fmt.Println("Quantum chain launch process initiated.")
}

func runValidate(cmd *cobra.Command, args []string) {
	if len(args) < 1 {
		log.Fatal("Please provide a genesis file to validate")
	}
	
	filepath := args[0]
	fmt.Printf("Validating genesis file: %s\n", filepath)
	
	// Read file
	data, err := ioutil.ReadFile(filepath)
	if err != nil {
		log.Fatalf("Failed to read genesis file: %v", err)
	}
	
	// Parse JSON
	var config GenesisConfig
	if err := json.Unmarshal(data, &config); err != nil {
		log.Fatalf("Invalid JSON format: %v", err)
	}
	
	// Validate configuration
	errors := validateConfig(&config)
	if len(errors) > 0 {
		fmt.Println("Validation errors:")
		for _, err := range errors {
			fmt.Printf("  - %s\n", err)
		}
		os.Exit(1)
	}
	
	fmt.Println("✓ Genesis configuration is valid")
}

func validateConfig(config *GenesisConfig) []string {
	var errors []string
	
	// Basic validation
	if config.ChainID == 0 {
		errors = append(errors, "Chain ID must be greater than 0")
	}
	
	if config.Type == "" {
		errors = append(errors, "Chain type must be specified")
	}
	
	if config.GasLimit == 0 {
		errors = append(errors, "Gas limit must be greater than 0")
	}
	
	// Type-specific validation
	switch config.Type {
	case "l2", "l3":
		if config.L2Config == nil {
			errors = append(errors, "L2/L3 configuration is required")
		} else {
			if config.L2Config.BaseChain == "" {
				errors = append(errors, "Base chain must be specified for L2/L3")
			}
		}
	case "quantum":
		if config.QuantumConfig == nil {
			errors = append(errors, "Quantum configuration is required")
		}
	}
	
	// Validate allocations
	for addr, account := range config.Alloc {
		if !common.IsHexAddress(addr) {
			errors = append(errors, fmt.Sprintf("Invalid address: %s", addr))
		}
		
		if _, ok := new(big.Int).SetString(account.Balance, 10); !ok {
			errors = append(errors, fmt.Sprintf("Invalid balance for %s", addr))
		}
	}
	
	return errors
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}