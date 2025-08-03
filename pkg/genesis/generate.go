package genesis

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Network represents a network configuration
type Network struct {
	Name      string
	ID        uint32
	ChainID   uint64
	TokenName string
	Symbol    string
}

var networks = map[string]Network{
	"mainnet": {
		Name:      "mainnet",
		ID:        96369,
		ChainID:   96369,
		TokenName: "Lux",
		Symbol:    "LUX",
	},
	"testnet": {
		Name:      "testnet",
		ID:        96368,
		ChainID:   96368,
		TokenName: "Lux",
		Symbol:    "LUX",
	},
}

// GetNetwork returns network configuration by name
func GetNetwork(name string) Network {
	if network, ok := networks[name]; ok {
		return network
	}
	return networks["mainnet"]
}

// GenerateAll generates genesis files for all chains (P, C, X) for a given network
func GenerateAll(network Network, outputDir string, mnemonic string) error {
	// Create network directory
	networkDir := filepath.Join(outputDir, fmt.Sprintf("lux-%s-%d", network.Name, network.ID))
	if err := os.MkdirAll(networkDir, 0755); err != nil {
		return fmt.Errorf("failed to create network directory: %w", err)
	}

	// Generate staking configuration (111 addresses, 21 validators)
	if err := GenerateStakingConfig(mnemonic, networkDir); err != nil {
		return fmt.Errorf("failed to generate staking config: %w", err)
	}

	// Generate P-Chain genesis
	pDir := filepath.Join(networkDir, "P")
	if err := os.MkdirAll(pDir, 0755); err != nil {
		return fmt.Errorf("failed to create P-Chain directory: %w", err)
	}
	if err := GeneratePChainWithValidators(network, filepath.Join(pDir, "genesis.json"), networkDir); err != nil {
		return fmt.Errorf("failed to generate P-Chain genesis: %w", err)
	}

	// Generate C-Chain genesis
	cDir := filepath.Join(networkDir, "C")
	if err := os.MkdirAll(cDir, 0755); err != nil {
		return fmt.Errorf("failed to create C-Chain directory: %w", err)
	}
	if err := GenerateCChain(network, filepath.Join(cDir, "genesis.json")); err != nil {
		return fmt.Errorf("failed to generate C-Chain genesis: %w", err)
	}

	// Generate X-Chain genesis
	xDir := filepath.Join(networkDir, "X")
	if err := os.MkdirAll(xDir, 0755); err != nil {
		return fmt.Errorf("failed to create X-Chain directory: %w", err)
	}
	if err := GenerateXChainWithAllocations(network, filepath.Join(xDir, "genesis.json"), networkDir); err != nil {
		return fmt.Errorf("failed to generate X-Chain genesis: %w", err)
	}

	return nil
}

// GeneratePChainWithValidators generates P-Chain genesis with validators from config
func GeneratePChainWithValidators(network Network, outputPath string, configDir string) error {
	// Read validators from config
	validators, err := GetPChainValidators(configDir)
	if err != nil {
		return fmt.Errorf("failed to get validators: %w", err)
	}

	// Create P-Chain genesis structure
	genesis := map[string]interface{}{
		"networkID": network.ID,
		"allocations": []map[string]interface{}{
			{
				"ethAddr": "0x2781BDC83A612F0FE382476556C0Cc12fE602294",
				"avaxAddr": "P-lux1xlg6dzvzr9spzyutevhj5jgm9hhczfqptm9wq",
				"initialAmount": 300000000000000000,
				"unlockSchedule": []map[string]interface{}{
					{
						"amount": 10000000000000000,
						"locktime": 1633824000,
					},
				},
			},
		},
		"startTime": 1609459200,
		"initialStakeDuration": 31536000,
		"initialStakeDurationOffset": 5400,
		"initialStakedFunds": []string{
			"P-lux1xlg6dzvzr9spzyutevhj5jgm9hhczfqptm9wq",
		},
		"initialStakers": []map[string]interface{}{},
		"cChainGenesis": "{\"config\":{\"chainId\":96369}}",
		"message": "Lux genesis block",
	}

	// Add validators as initial stakers
	stakers := []map[string]interface{}{}
	for _, validator := range validators {
		staker := map[string]interface{}{
			"nodeID":        validator.NodeID,
			"rewardAddress": validator.PChainAddr,
			"delegationFee": 20000, // 2%
		}
		stakers = append(stakers, staker)
	}
	genesis["initialStakers"] = stakers

	// Write genesis file
	data, err := json.MarshalIndent(genesis, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal P-Chain genesis: %w", err)
	}

	if err := os.WriteFile(outputPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write P-Chain genesis: %w", err)
	}

	return nil
}

// GenerateCChain generates C-Chain genesis
func GenerateCChain(network Network, outputPath string) error {
	// C-Chain genesis with proper configuration
	genesis := map[string]interface{}{
		"config": map[string]interface{}{
			"chainId":             network.ChainID,
			"homesteadBlock":      0,
			"daoForkBlock":        0,
			"daoForkSupport":      true,
			"eip150Block":         0,
			"eip150Hash":          "0x2086799aeebeae135c246c65021c82b4e15a2c451340993aacfd2751886514f0",
			"eip155Block":         0,
			"eip158Block":         0,
			"byzantiumBlock":      0,
			"constantinopleBlock": 0,
			"petersburgBlock":     0,
			"istanbulBlock":       0,
			"muirGlacierBlock":    0,
			"subnetEVMTimestamp":  0,
		},
		"nonce":      "0x0",
		"timestamp":  "0x0",
		"extraData":  "0x00",
		"gasLimit":   "0x7A1200",
		"difficulty": "0x0",
		"mixHash":    "0x0000000000000000000000000000000000000000000000000000000000000000",
		"coinbase":   "0x0000000000000000000000000000000000000000",
		"alloc": map[string]interface{}{
			"0x2781BDC83A612F0FE382476556C0Cc12fE602294": map[string]string{
				"balance": "0x33b2e3c9fd0803ce8000000", // 1,000,000 LUX
			},
		},
		"number":     "0x0",
		"gasUsed":    "0x0",
		"parentHash": "0x0000000000000000000000000000000000000000000000000000000000000000",
	}

	// Write genesis file
	data, err := json.MarshalIndent(genesis, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal C-Chain genesis: %w", err)
	}

	if err := os.WriteFile(outputPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write C-Chain genesis: %w", err)
	}

	// Also create config.json
	configPath := filepath.Join(filepath.Dir(outputPath), "config.json")
	config := map[string]interface{}{
		"snowman-api-enabled": false,
		"coreth-admin-api-enabled": false,
		"eth-apis": []string{
			"eth",
			"eth-filter",
			"net",
			"web3",
			"internal-eth",
			"internal-blockchain",
			"internal-transaction",
		},
		"rpc-gas-cap": 50000000,
		"rpc-tx-fee-cap": 100,
		"pruning-enabled": true,
		"local-txs-enabled": true,
		"api-max-duration": 30000000000,
		"ws-cpu-refill-rate": 0,
		"ws-cpu-max-stored": 0,
		"api-max-blocks-per-request": 30,
		"allow-unfinalized-queries": false,
		"allow-unprotected-txs": false,
		"keystore-directory": "",
		"keystore-external-signer": "",
		"keystore-insecure-unlock-allowed": false,
		"remote-tx-gossip-only-enabled": false,
		"tx-regossip-frequency": 60000000000,
		"tx-regossip-max-size": 15,
		"log-level": "info",
		"offline-pruning-enabled": false,
		"offline-pruning-bloom-filter-size": 512,
		"offline-pruning-data-directory": "",
		"max-outbound-active-requests": 16,
		"state-sync-enabled": false,
	}

	configData, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal C-Chain config: %w", err)
	}

	if err := os.WriteFile(configPath, configData, 0644); err != nil {
		return fmt.Errorf("failed to write C-Chain config: %w", err)
	}

	return nil
}

// GenerateXChainWithAllocations generates X-Chain genesis with allocations from config
func GenerateXChainWithAllocations(network Network, outputPath string, configDir string) error {
	// Read allocations from config
	allocations, err := GetXChainAllocations(configDir)
	if err != nil {
		return fmt.Errorf("failed to get allocations: %w", err)
	}

	// Create X-Chain genesis structure
	initialBalances := []map[string]interface{}{}
	for addr, amount := range allocations {
		balance := map[string]interface{}{
			"address": addr,
			"balance": amount.String(),
		}
		initialBalances = append(initialBalances, balance)
	}

	genesis := map[string]interface{}{
		"networkID": network.ID,
		"allocations": []map[string]interface{}{
			{
				"ethAddr": "0x2781BDC83A612F0FE382476556C0Cc12fE602294",
				"avaxAddr": "X-lux1xlg6dzvzr9spzyutevhj5jgm9hhczfqptm9wq",
				"initialAmount": 300000000000000000,
				"unlockSchedule": []map[string]interface{}{
					{
						"amount": 10000000000000000,
						"locktime": 1633824000,
					},
				},
			},
		},
		"startTime": 1609459200,
		"initialBalances": initialBalances,
		"message": "Lux X-Chain genesis",
	}

	// Write genesis file
	data, err := json.MarshalIndent(genesis, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal X-Chain genesis: %w", err)
	}

	if err := os.WriteFile(outputPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write X-Chain genesis: %w", err)
	}

	return nil
}