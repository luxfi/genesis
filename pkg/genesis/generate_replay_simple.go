package genesis

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// GenerateReplayGenesisSimple generates genesis files for database replay without node imports
func GenerateReplayGenesisSimple(network Network, outputDir string, dbPath string, dbType string) error {
	// Create output directory
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Create subdirectories for P, C, X chains
	for _, chain := range []string{"P", "C", "X"} {
		chainDir := filepath.Join(outputDir, chain)
		if err := os.MkdirAll(chainDir, 0755); err != nil {
			return fmt.Errorf("failed to create %s directory: %w", chain, err)
		}
	}

	// Use the test validator with known working BLS signatures
	nodeIDStr := "NodeID-7Xhw2mDxuDS44j42TCB6U5579esbSt3Lg"
	
	// Test addresses
	luxAddr := "8jma8ppw3nhx5r4ap8clazz0dps7rv5u00z96u"
	ethAddrHex := "8db97C7cEcE249c2b98bDC0226Cc4C2A57BF52FC"

	// Generate P-Chain genesis
	pChainGenesis := map[string]interface{}{
		"networkID": network.ID,
		"allocations": []map[string]interface{}{
			{
				"ethAddr":       fmt.Sprintf("0x%s", ethAddrHex),
				"avaxAddr":      fmt.Sprintf("P-lux1%s", luxAddr),
				"initialAmount": 300000000000000000,
				"unlockSchedule": []map[string]interface{}{
					{
						"amount":   10000000000000000,
						"locktime": uint64(time.Now().Unix() + 86400), // 1 day from now
					},
				},
			},
		},
		"startTime":                  uint64(time.Now().Unix() - 86400), // 1 day ago
		"initialStakeDuration":       31536000,
		"initialStakeDurationOffset": 5400,
		"initialStakedFunds": []string{
			fmt.Sprintf("P-lux1%s", luxAddr),
		},
		"initialStakers": []map[string]interface{}{
			{
				"nodeID":        nodeIDStr,
				"rewardAddress": fmt.Sprintf("P-lux1%s", luxAddr),
				"delegationFee": 20000,
				"signer": map[string]interface{}{
					"publicKey": "0x900c9b119b5c82d781d4b49be78c3fc7ae65f2b435b7ed9e3a8b9a03e475edff86d8a64827fec8db23a6f236afbf127d",
					"proofOfPossession": "0x8bfd6d4d2086b2b8115d8f72f94095fefe5a6c07876b2accf51a811adf520f389e74a3d2152a6d90b521e2be58ffe468043dc5ea68b4c44410eb67f8dc24f13ed4f194000764c0e922cd254a3588a4962b1cb4db7de4bb9cda9d9d4d6b03f3d2",
				},
			},
		},
		"cChainGenesis": "", // Will be set below
		"message":       fmt.Sprintf("Database replay from %s", dbPath),
	}

	// Generate C-Chain genesis with replay marker
	cChainGenesis := map[string]interface{}{
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
		"gasLimit":   "0x5f5e100",
		"difficulty": "0x0",
		"mixHash":    "0x0000000000000000000000000000000000000000000000000000000000000000",
		"coinbase":   "0x0000000000000000000000000000000000000000",
		"alloc": map[string]interface{}{
			fmt.Sprintf("0x%s", ethAddrHex): map[string]interface{}{
				"balance": "0x21e19e0c9bab2400000", // 10000 ETH
			},
		},
		"dbPath": dbPath,
		"dbType": dbType,
		"replay": true,
	}

	// Convert C-Chain genesis to string for P-Chain
	cChainBytes, err := json.Marshal(cChainGenesis)
	if err != nil {
		return fmt.Errorf("failed to marshal C-Chain genesis: %w", err)
	}
	pChainGenesis["cChainGenesis"] = string(cChainBytes)

	// Generate X-Chain genesis
	xChainGenesis := map[string]interface{}{
		"networkID": network.ID,
		"allocations": []map[string]interface{}{
			{
				"ethAddr":       fmt.Sprintf("0x%s", ethAddrHex),
				"avaxAddr":      fmt.Sprintf("X-lux1%s", luxAddr),
				"initialAmount": 300000000000000000,
				"unlockSchedule": []map[string]interface{}{
					{
						"amount":   10000000000000000,
						"locktime": uint64(time.Now().Unix() + 86400),
					},
				},
			},
		},
		"startTime": uint64(time.Now().Unix() - 86400),
		"initialBalances": []map[string]interface{}{
			{
				"address": fmt.Sprintf("X-lux1%s", luxAddr),
				"balance": "1000000000000000000",
			},
		},
		"message": fmt.Sprintf("X-Chain genesis for replay from %s", dbPath),
	}

	// Write P-Chain genesis
	pChainPath := filepath.Join(outputDir, "P", "genesis.json")
	pChainData, err := json.MarshalIndent(pChainGenesis, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal P-Chain genesis: %w", err)
	}
	if err := os.WriteFile(pChainPath, pChainData, 0644); err != nil {
		return fmt.Errorf("failed to write P-Chain genesis: %w", err)
	}

	// Write C-Chain genesis
	cChainPath := filepath.Join(outputDir, "C", "genesis.json")
	cChainData, err := json.MarshalIndent(cChainGenesis, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal C-Chain genesis: %w", err)
	}
	if err := os.WriteFile(cChainPath, cChainData, 0644); err != nil {
		return fmt.Errorf("failed to write C-Chain genesis: %w", err)
	}

	// Write X-Chain genesis
	xChainPath := filepath.Join(outputDir, "X", "genesis.json")
	xChainData, err := json.MarshalIndent(xChainGenesis, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal X-Chain genesis: %w", err)
	}
	if err := os.WriteFile(xChainPath, xChainData, 0644); err != nil {
		return fmt.Errorf("failed to write X-Chain genesis: %w", err)
	}

	fmt.Printf("Generated replay genesis files in %s\n", outputDir)
	fmt.Printf("- P-Chain: %s\n", pChainPath)
	fmt.Printf("- C-Chain: %s\n", cChainPath)
	fmt.Printf("- X-Chain: %s\n", xChainPath)

	return nil
}