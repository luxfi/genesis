package commands

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/big"
	"os"

	"github.com/ethereum/go-ethereum/common"
	"github.com/spf13/cobra"
)

// ValidateOptions contains options for the validate command
type ValidateOptions struct {
	FilePath string
}

// NewValidateCommand creates the validate command
func NewValidateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "validate [file]",
		Short: "Validate a genesis configuration",
		Long:  "Validate a genesis configuration file for correctness",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts := &ValidateOptions{
				FilePath: args[0],
			}
			return RunValidate(opts)
		},
	}
	
	return cmd
}

// GenesisConfig structure for validation
type GenesisConfig struct {
	ChainID        uint64                       `json:"chainId"`
	Type           string                       `json:"type"`
	Network        string                       `json:"network"`
	Timestamp      uint64                       `json:"timestamp"`
	GasLimit       uint64                       `json:"gasLimit"`
	Difficulty     *big.Int                     `json:"difficulty"`
	Alloc          map[string]GenesisAccount    `json:"alloc"`
	Validators     []Validator                  `json:"validators,omitempty"`
	L2Config       *L2GenesisConfig             `json:"l2Config,omitempty"`
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

type L2GenesisConfig struct {
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

func RunValidate(opts *ValidateOptions) error {
	fmt.Printf("Validating genesis file: %s\n", opts.FilePath)
	
	// Read file
	data, err := ioutil.ReadFile(opts.FilePath)
	if err != nil {
		return fmt.Errorf("failed to read genesis file: %w", err)
	}
	
	// Parse JSON
	var config GenesisConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("invalid JSON format: %w", err)
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
	return nil
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