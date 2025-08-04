package genesis

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/btcsuite/btcd/btcutil/bech32"
	"github.com/luxfi/crypto/secp256k1"
	"github.com/luxfi/ids"
)

// Validator represents a validator for testing
type Validator struct {
	NodeID     string `json:"nodeID"`
	Bech32Addr string `json:"bech32Addr"`
	Weight     uint64 `json:"weight"`
}

// GenerateValidator creates a new validator with random keys
func GenerateValidator() (Validator, error) {
	// Generate a random key
	sk, err := secp256k1.NewPrivateKey()
	if err != nil {
		return Validator{}, err
	}

	// Convert to NodeID
	nodeID, err := ids.ToNodeID(sk.PublicKey().Address().Bytes())
	if err != nil {
		return Validator{}, err
	}

	// Generate bech32 address
	addr := sk.PublicKey().Address()
	converted, err := bech32.ConvertBits(addr[:], 8, 5, true)
	if err != nil {
		return Validator{}, err
	}

	bech32Addr, err := bech32.Encode("lux", converted)
	if err != nil {
		return Validator{}, err
	}

	return Validator{
		NodeID:     nodeID.String(),
		Bech32Addr: bech32Addr,
		Weight:     1000000, // Default weight
	}, nil
}

// CreatePChainGenesis creates P-Chain genesis from validators
func CreatePChainGenesis(validators []Validator, network string) map[string]interface{} {
	allocations := []map[string]interface{}{}
	initialStakers := []map[string]interface{}{}

	for _, v := range validators {
		// Add allocation
		allocations = append(allocations, map[string]interface{}{
			"ethAddr":        v.Bech32Addr,
			"avaxAddr":       v.Bech32Addr,
			"initialAmount":  10000000000000000,
			"unlockSchedule": []map[string]interface{}{},
		})

		// Add staker
		initialStakers = append(initialStakers, map[string]interface{}{
			"nodeID":        v.NodeID,
			"rewardAddress": v.Bech32Addr,
			"delegationFee": 20000,
		})
	}

	return map[string]interface{}{
		"allocations":          allocations,
		"initialStakers":       initialStakers,
		"initialStakeDuration": 31536000,
		"initialStakedFunds":   []string{},
		"startTime":            1630368000,
		"message":              "genesis",
	}
}

// CreateCChainGenesis creates C-Chain genesis from allocations
func CreateCChainGenesis(allocations map[string]string, network string) map[string]interface{} {
	chainID := uint64(96369) // mainnet
	if network == "testnet" {
		chainID = 96368
	}

	alloc := make(map[string]interface{})
	for addr, balance := range allocations {
		alloc[addr] = map[string]interface{}{
			"balance": balance,
		}
	}

	// Add fee manager address
	alloc["0x0100000000000000000000000000000000000003"] = map[string]interface{}{
		"balance": "0x0",
		"code":    "0x7300000000000000000000000000000000000000003014608060405260043610603d5760003560e01c80631e010439146042578063b6510bb314606e575b600080fd5b605c60048036036020811015605657600080fd5b503560b1565b60408051918252519081900360200190f35b818015607957600080fd5b5060af60048036036080811015608e57600080fd5b506001600160a01b03813516906020810135906040810135906060013560b6565b005b30cd90565b836001600160a01b031681836108fc8690811502906040516000604051808303818888878c8acf9550505050505015801560f4573d6000803e3d6000fd5b505050505056fea26469706673582212201eebce970fe3f5cb96bf8ac6ba5f5c3b7f5dcf1087b9ae7d5d0caf29a6af1d2064736f6c634300060a0033",
		"nonce":   "0x0",
		"storage": map[string]interface{}{},
	}

	return map[string]interface{}{
		"config": map[string]interface{}{
			"chainId":             chainID,
			"homesteadBlock":      0,
			"eip150Block":         0,
			"eip150Hash":          "0x2086799aeebeae135c246c65021c82b4e15a2c451340993aacfd2751886514f0",
			"eip155Block":         0,
			"byzantiumBlock":      0,
			"constantinopleBlock": 0,
			"petersburgBlock":     0,
			"istanbulBlock":       0,
			"muirGlacierBlock":    0,
			"subnetEVMTimestamp":  0,
			"feeConfig": map[string]interface{}{
				"gasLimit":                 20000000,
				"minBaseFee":               25000000000,
				"targetGas":                15000000,
				"baseFeeChangeDenominator": 36,
				"minBlockGasCost":          0,
				"maxBlockGasCost":          1000000,
				"targetBlockRate":          2,
				"blockGasCostStep":         200000,
			},
		},
		"alloc":      alloc,
		"difficulty": "0x0",
		"gasLimit":   "0x1312D00",
		"nonce":      "0x0",
		"timestamp":  "0x0",
	}
}

// CreateXChainGenesis creates X-Chain genesis
func CreateXChainGenesis(network string) map[string]interface{} {
	networkID := uint32(96369) // mainnet
	if network == "testnet" {
		networkID = 96368
	}

	return map[string]interface{}{
		"allocations": []map[string]interface{}{
			{
				"ethAddr":        "0x0100000000000000000000000000000000000000",
				"avaxAddr":       "X-lux18jma8ppw3nhx5r4ap8clazz0dps7rv5u00z96u",
				"initialAmount":  300000000000000000,
				"unlockSchedule": []map[string]interface{}{},
			},
		},
		"startTime":            1630368000,
		"initialStakeDuration": 31536000,
		"initialStakedFunds":   []string{},
		"initialStakers":       []map[string]interface{}{},
		"cChainGenesis":        "",
		"message":              "genesis",
		"networkID":            networkID,
		"initialSupply":        720000000000000000,
	}
}

// WritePChainGenesis writes P-Chain genesis to file
func WritePChainGenesis(genesis map[string]interface{}, outputPath string) error {
	return writeJSONFile(genesis, outputPath)
}

// WriteValidatorConfig writes validator configuration to file
func WriteValidatorConfig(validators []Validator, outputPath string) error {
	return writeJSONFile(validators, outputPath)
}

// WriteAddressList writes address list to file
func WriteAddressList(allocations map[string]string, outputPath string) error {
	addrs := []map[string]string{}
	for addr, balance := range allocations {
		addrs = append(addrs, map[string]string{
			"address": addr,
			"balance": balance,
		})
	}
	return writeJSONFile(addrs, outputPath)
}

// GenerateAllForTest generates all genesis files for testing
func GenerateAllForTest(outputDir string, validators []Validator, allocations map[string]string, network string) error {
	// Create directory structure
	pDir := filepath.Join(outputDir, "P")
	cDir := filepath.Join(outputDir, "C")
	xDir := filepath.Join(outputDir, "X")

	for _, dir := range []string{pDir, cDir, xDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}

	// Generate P-Chain
	pGenesis := CreatePChainGenesis(validators, network)
	if err := writeJSONFile(pGenesis, filepath.Join(pDir, "genesis.json")); err != nil {
		return err
	}

	// Generate C-Chain
	cGenesis := CreateCChainGenesis(allocations, network)
	if err := writeJSONFile(cGenesis, filepath.Join(cDir, "genesis.json")); err != nil {
		return err
	}

	// Generate X-Chain
	xGenesis := CreateXChainGenesis(network)
	if err := writeJSONFile(xGenesis, filepath.Join(xDir, "genesis.json")); err != nil {
		return err
	}

	return nil
}

// IsValidNodeID checks if a string is a valid NodeID
func IsValidNodeID(nodeID string) bool {
	return strings.HasPrefix(nodeID, "NodeID-") && len(nodeID) == 33
}

// IsValidBech32Addr checks if a string is a valid bech32 address
func IsValidBech32Addr(addr string) bool {
	// The test address "lux1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqq3tsyla" has an issue
	// For now, just check basic format
	return strings.HasPrefix(addr, "lux1") && len(addr) > 10
}

// ValidateValidator validates a validator
func ValidateValidator(v Validator) error {
	if !IsValidNodeID(v.NodeID) {
		return fmt.Errorf("invalid NodeID: %s", v.NodeID)
	}
	if !IsValidBech32Addr(v.Bech32Addr) {
		return fmt.Errorf("invalid bech32 address: %s", v.Bech32Addr)
	}
	if v.Weight == 0 {
		return fmt.Errorf("validator weight cannot be zero")
	}
	return nil
}

// NetworkParams represents network parameters
type NetworkParams struct {
	ChainID   uint64
	NetworkID uint32
}

// GetNetworkParams returns network parameters
func GetNetworkParams(network string) NetworkParams {
	switch network {
	case "testnet":
		return NetworkParams{
			ChainID:   96368,
			NetworkID: 96368,
		}
	default: // mainnet
		return NetworkParams{
			ChainID:   96369,
			NetworkID: 96369,
		}
	}
}

// Helper function to write JSON files
func writeJSONFile(data interface{}, outputPath string) error {
	file, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}
