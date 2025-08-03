package genesis

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"path/filepath"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/luxfi/crypto/secp256k1"
	"github.com/luxfi/node/ids"
	"github.com/luxfi/node/utils/formatting/address"
)

type AddressInfo struct {
	Index       int    `json:"index"`
	PrivateKey  string `json:"privateKey"`
	PublicKey   string `json:"publicKey"`
	NodeID      string `json:"nodeId"`
	EthAddress  string `json:"ethAddress"`
	PChainAddr  string `json:"pChainAddress"`
	XChainAddr  string `json:"xChainAddress"`
	StakeAmount uint64 `json:"stakeAmount,omitempty"`
}

type ValidatorInfo struct {
	NodeID      string `json:"nodeId"`
	StakeAmount uint64 `json:"stakeAmount"`
	StartTime   uint64 `json:"startTime"`
	EndTime     uint64 `json:"endTime"`
	PChainAddr  string `json:"pChainAddress"`
}

type StakingConfig struct {
	Addresses   []AddressInfo   `json:"addresses"`
	Validators  []ValidatorInfo `json:"validators"`
}

// GenerateStakingConfig generates 111 addresses with 21 validators using Fibonacci distribution
func GenerateStakingConfig(mnemonic string, outputDir string) error {
	config, err := generateStakingAddresses(mnemonic, 111)
	if err != nil {
		return fmt.Errorf("failed to generate addresses: %w", err)
	}
	
	// Ensure output directory exists
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output dir: %w", err)
	}
	
	// Save addresses.json
	addressesPath := filepath.Join(outputDir, "addresses.json")
	data, err := json.MarshalIndent(config.Addresses, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal addresses: %w", err)
	}
	if err := os.WriteFile(addressesPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write addresses: %w", err)
	}
	
	// Save validators.json
	validatorsPath := filepath.Join(outputDir, "validators.json")
	data, err = json.MarshalIndent(config.Validators, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal validators: %w", err)
	}
	if err := os.WriteFile(validatorsPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write validators: %w", err)
	}
	
	// Save staking keys for local nodes
	stakingKeysPath := filepath.Join(outputDir, "staking-keys")
	if err := os.MkdirAll(stakingKeysPath, 0755); err != nil {
		return fmt.Errorf("failed to create staking keys dir: %w", err)
	}
	
	for i := 0; i < 21; i++ {
		addr := config.Addresses[i]
		keyPath := filepath.Join(stakingKeysPath, fmt.Sprintf("node%02d.key", i+1))
		if err := os.WriteFile(keyPath, []byte(addr.PrivateKey), 0600); err != nil {
			return fmt.Errorf("failed to write staking key %d: %w", i+1, err)
		}
		
		// Also create node ID file for easy reference
		nodeIDPath := filepath.Join(stakingKeysPath, fmt.Sprintf("node%02d.nodeid", i+1))
		if err := os.WriteFile(nodeIDPath, []byte(addr.NodeID), 0644); err != nil {
			return fmt.Errorf("failed to write node ID %d: %w", i+1, err)
		}
	}
	
	fmt.Printf("Generated 111 addresses to %s\n", addressesPath)
	fmt.Printf("Generated 21 validators to %s\n", validatorsPath)
	fmt.Printf("Generated 21 staking keys to %s\n", stakingKeysPath)
	
	return nil
}

func generateStakingAddresses(mnemonic string, count int) (*StakingConfig, error) {
	// For simplicity, we'll use a deterministic approach based on the mnemonic
	// In production, you'd use proper BIP32/39 derivation
	
	config := &StakingConfig{
		Addresses:  make([]AddressInfo, count),
		Validators: make([]ValidatorInfo, 0),
	}
	
	// Generate Fibonacci sequence for staking amounts
	fib := generateFibonacciStakes(21, 2000000000000000) // 2M LUX base (in nLUX)
	
	for i := 0; i < count; i++ {
		// Generate deterministic private key based on mnemonic and index
		// In production, use proper HD wallet derivation
		seedStr := fmt.Sprintf("%s-%d", mnemonic, i)
		privKey, err := crypto.HexToECDSA(fmt.Sprintf("%064x", crypto.Keccak256([]byte(seedStr))))
		if err != nil {
			return nil, err
		}
		
		privKeyBytes := crypto.FromECDSA(privKey)
		pubKeyBytes := crypto.FromECDSAPub(&privKey.PublicKey)
		
		// Create secp256k1 key for Lux
		sk, err := secp256k1.ToPrivateKey(privKeyBytes)
		if err != nil {
			return nil, err
		}
		
		// Get addresses
		nodeID := ids.NodeIDFromCert(sk.PublicKey())
		pAddr, err := address.Format("P", "lux", sk.PublicKey().Address().Bytes())
		if err != nil {
			return nil, err
		}
		xAddr, err := address.Format("X", "lux", sk.PublicKey().Address().Bytes())
		if err != nil {
			return nil, err
		}
		
		// EVM address
		ethAddr := crypto.PubkeyToAddress(privKey.PublicKey).Hex()
		
		info := AddressInfo{
			Index:      i,
			PrivateKey: hex.EncodeToString(privKeyBytes),
			PublicKey:  hex.EncodeToString(pubKeyBytes),
			NodeID:     nodeID.String(),
			EthAddress: ethAddr,
			PChainAddr: pAddr,
			XChainAddr: xAddr,
		}
		
		// First 21 are validators with Fibonacci stakes
		if i < 21 {
			info.StakeAmount = fib[i]
			
			validator := ValidatorInfo{
				NodeID:      nodeID.String(),
				StakeAmount: fib[i],
				StartTime:   1609459200, // 2021-01-01
				EndTime:     1924992000, // 2031-01-01
				PChainAddr:  pAddr,
			}
			config.Validators = append(config.Validators, validator)
		}
		
		config.Addresses[i] = info
	}
	
	return config, nil
}


func generateFibonacciStakes(count int, base uint64) []uint64 {
	stakes := make([]uint64, count)
	fib := []int{1, 1}
	
	// Generate Fibonacci sequence
	for len(fib) < count {
		next := fib[len(fib)-1] + fib[len(fib)-2]
		fib = append(fib, next)
	}
	
	// Reverse so largest stake goes to validator 21
	for i := 0; i < count; i++ {
		stakes[i] = base * uint64(fib[count-1-i])
	}
	
	return stakes
}

// GetXChainAllocations reads addresses.json and returns X-Chain allocations
func GetXChainAllocations(configDir string) (map[string]*big.Int, error) {
	addressesPath := filepath.Join(configDir, "addresses.json")
	data, err := os.ReadFile(addressesPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read addresses.json: %w", err)
	}
	
	var addresses []AddressInfo
	if err := json.Unmarshal(data, &addresses); err != nil {
		return nil, fmt.Errorf("failed to unmarshal addresses: %w", err)
	}
	
	allocations := make(map[string]*big.Int)
	
	// Give each address 10,000 LUX on X-Chain
	baseAmount := new(big.Int).Mul(big.NewInt(10000), big.NewInt(1e9)) // 10,000 LUX in nLUX
	
	for _, addr := range addresses {
		allocations[addr.XChainAddr] = new(big.Int).Set(baseAmount)
	}
	
	return allocations, nil
}

// GetPChainValidators reads validators.json and returns P-Chain validators
func GetPChainValidators(configDir string) ([]ValidatorInfo, error) {
	validatorsPath := filepath.Join(configDir, "validators.json")
	data, err := os.ReadFile(validatorsPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read validators.json: %w", err)
	}
	
	var validators []ValidatorInfo
	if err := json.Unmarshal(data, &validators); err != nil {
		return nil, fmt.Errorf("failed to unmarshal validators: %w", err)
	}
	
	return validators, nil
}