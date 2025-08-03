package launch

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/luxfi/genesis/pkg/core"
	"github.com/luxfi/genesis/pkg/credentials"
)

// Launcher handles network launching
type Launcher struct {
	network  core.Network
	baseDir  string
	dryRun   bool
	validate bool
	credGen  *credentials.Generator
}

// New creates a new launcher
func New(network core.Network) *Launcher {
	return &Launcher{
		network:  network,
		validate: true,
		credGen:  credentials.NewGenerator(),
	}
}

// WithBaseDir sets the base directory
func (l *Launcher) WithBaseDir(dir string) *Launcher {
	l.baseDir = dir
	return l
}

// WithDryRun enables dry run mode
func (l *Launcher) WithDryRun(dryRun bool) *Launcher {
	l.dryRun = dryRun
	return l
}

// Launch executes the launch sequence
func (l *Launcher) Launch() error {
	// Normalize configuration
	l.network.Normalize()

	// Set default base directory
	if l.baseDir == "" {
		homeDir, _ := os.UserHomeDir()
		l.baseDir = filepath.Join(homeDir, ".luxd", l.network.Name)
	}

	// Validate
	if l.validate {
		if err := l.network.Validate(); err != nil {
			return fmt.Errorf("invalid configuration: %w", err)
		}
	}

	// Execute launch sequence
	steps := []struct {
		name string
		fn   func() error
	}{
		{"Setup directories", l.setupDirectories},
		{"Generate credentials", l.generateCredentials},
		{"Prepare genesis", l.prepareGenesis},
		{"Configure nodes", l.configureNodes},
		{"Start nodes", l.startNodes},
	}

	for _, step := range steps {
		fmt.Printf("🔧 %s...\n", step.name)
		if err := step.fn(); err != nil {
			return fmt.Errorf("%s failed: %w", step.name, err)
		}
	}

	return nil
}

func (l *Launcher) setupDirectories() error {
	if l.dryRun {
		fmt.Printf("  Would create directories in %s\n", l.baseDir)
		return nil
	}

	for i := 0; i < l.network.Nodes; i++ {
		nodeDir := l.getNodeDir(i)
		if err := os.MkdirAll(nodeDir, 0755); err != nil {
			return fmt.Errorf("failed to create node directory: %w", err)
		}
	}
	return nil
}

func (l *Launcher) generateCredentials() error {
	if l.dryRun {
		fmt.Printf("  Would generate credentials for %d nodes\n", l.network.Nodes)
		return nil
	}

	for i := 0; i < l.network.Nodes; i++ {
		nodeDir := l.getNodeDir(i)

		// Generate staking credentials
		creds, err := l.credGen.Generate()
		if err != nil {
			return fmt.Errorf("failed to generate credentials for node %d: %w", i, err)
		}

		// Save credentials
		if err := l.credGen.Save(creds, nodeDir); err != nil {
			return fmt.Errorf("failed to save credentials: %w", err)
		}

		// Save validator info
		if err := l.saveValidatorInfo(i, creds); err != nil {
			return fmt.Errorf("failed to save validator info: %w", err)
		}

		fmt.Printf("  Node %d: %s\n", i, creds.NodeID)
	}
	return nil
}

func (l *Launcher) prepareGenesis() error {
	if l.dryRun {
		fmt.Printf("  Would prepare genesis (%s)\n", l.network.Genesis.Source)
		return nil
	}

	// For mainnet, we need to generate all chain genesis files (P, C, X)
	if l.network.Name == "mainnet" && l.network.Genesis.Source == "import" {
		// First, ensure we have node credentials to include in P-Chain genesis
		if l.network.Nodes > 0 {
			// Read the first node's validator info
			infoPath := filepath.Join(l.getNodeDir(0), "validator-info.json")
			data, err := os.ReadFile(infoPath)
			if err != nil {
				return fmt.Errorf("failed to read validator info: %w", err)
			}

			var info map[string]interface{}
			if err := json.Unmarshal(data, &info); err != nil {
				return fmt.Errorf("failed to parse validator info: %w", err)
			}

			// Extract BLS info for P-Chain genesis
			nodeID, _ := info["nodeID"].(string)
			blsPubKeyHex, _ := info["blsPublicKey"].(string)
			blsPOPHex, _ := info["blsProofOfPossession"].(string)

			// Convert hex strings to bytes
			blsPubKey := []byte{}
			blsPOP := []byte{}
			if blsPubKeyHex != "" && len(blsPubKeyHex) > 2 {
				_, _ = fmt.Sscanf(blsPubKeyHex[2:], "%x", &blsPubKey)
			}
			if blsPOPHex != "" && len(blsPOPHex) > 2 {
				_, _ = fmt.Sscanf(blsPOPHex[2:], "%x", &blsPOP)
			}

			// Generate all genesis files using pchain package
			outputDir := fmt.Sprintf("/home/z/work/lux/genesis/configs/lux-mainnet-%d", l.network.NetworkID)
			nodeInfo := map[string]interface{}{
				"nodeID":               nodeID,
				"blsPublicKey":         blsPubKey,
				"blsProofOfPossession": blsPOP,
			}

			// Import pchain package functionality inline for now
			if err := l.generateMainnetGenesisFiles(outputDir, nodeInfo); err != nil {
				return fmt.Errorf("failed to generate genesis files: %w", err)
			}

			fmt.Printf("  Generated P-Chain, C-Chain, and X-Chain genesis files in %s\n", outputDir)
		}

		// Create the combined genesis.json for luxd
		return l.createCombinedGenesis()
	}

	// Original logic for non-mainnet
	genesisPath := filepath.Join(l.baseDir, "genesis.json")

	switch l.network.Genesis.Source {
	case "import":
		if l.network.Genesis.ImportPath != "" {
			// Generic import from specified path
			// TODO: Implement generic import logic
		} else {
			return fmt.Errorf("import path required for import genesis")
		}

	case "extract":
		// TODO: Implement extraction logic

	default: // "fresh" or empty
		// Create fresh genesis
		genesis := map[string]interface{}{
			"networkID":   l.network.NetworkID,
			"chainID":     l.network.ChainID,
			"allocations": l.network.Genesis.Allocations,
			"message":     l.network.Genesis.Message,
			"timestamp":   1630000000,
		}

		data, _ := json.MarshalIndent(genesis, "", "  ")
		if err := os.WriteFile(genesisPath, data, 0644); err != nil {
			return fmt.Errorf("failed to write genesis: %w", err)
		}
	}

	return nil
}

func (l *Launcher) configureNodes() error {
	if l.dryRun {
		fmt.Printf("  Would configure %d nodes\n", l.network.Nodes)
		return nil
	}

	// Collect node IDs for bootstrap configuration
	nodeIDs := make([]string, l.network.Nodes)
	for i := 0; i < l.network.Nodes; i++ {
		infoPath := filepath.Join(l.getNodeDir(i), "validator-info.json")
		data, _ := os.ReadFile(infoPath)
		var info map[string]interface{}
		_ = json.Unmarshal(data, &info)
		if nodeID, ok := info["nodeID"].(string); ok {
			nodeIDs[i] = nodeID
		}
	}

	// Configure each node
	for i := 0; i < l.network.Nodes; i++ {
		if err := l.configureNode(i, nodeIDs); err != nil {
			return fmt.Errorf("failed to configure node %d: %w", i, err)
		}
	}

	return nil
}

func (l *Launcher) configureNode(index int, allNodeIDs []string) error {
	nodeDir := l.getNodeDir(index)
	// Use 9630 for mainnet, 9650 for others
	port := 9650 + (index * 10)
	if l.network.Name == "mainnet" {
		port = 9630 + (index * 10)
	}

	// Build bootstrap lists (exclude self)
	bootstrapIPs := []string{}
	bootstrapIDs := []string{}
	for j := 0; j < l.network.Nodes; j++ {
		if index != j {
			bootstrapIPs = append(bootstrapIPs, fmt.Sprintf("127.0.0.1:%d", 9651+(j*10)))
			if allNodeIDs[j] != "" {
				bootstrapIDs = append(bootstrapIDs, allNodeIDs[j])
			}
		}
	}

	// Node configuration
	config := map[string]interface{}{
		"network-id":                     l.network.NetworkID,
		"http-port":                      port,
		"staking-port":                   port + 1,
		"data-dir":                       nodeDir,
		"snow-sample-size":               l.network.Consensus.K,
		"snow-quorum-size":               l.network.Consensus.Alpha,
		"snow-virtuous-commit-threshold": l.network.Consensus.Alpha,
		"snow-rogue-commit-threshold":    l.network.Consensus.Beta,
		"bootstrap-ips":                  bootstrapIPs,
		"bootstrap-ids":                  bootstrapIDs,
		"staking-tls-cert-file":          filepath.Join(nodeDir, "staking", "staker.crt"),
		"staking-tls-key-file":           filepath.Join(nodeDir, "staking", "staker.key"),
		"staking-signer-key-file":        filepath.Join(nodeDir, "staking", "signer.key"),
	}

	// Only add genesis-file for non-mainnet networks
	// Mainnet uses built-in genesis or GENESIS_LUX=1 handles it
	if l.network.Name != "mainnet" {
		config["genesis-file"] = filepath.Join(l.baseDir, "genesis.json")
	}

	// Add mainnet-specific configuration for POA mode
	if l.network.Name == "mainnet" && l.network.Nodes == 1 {
		config["staking-enabled"] = false
		config["sybil-protection-enabled"] = false
		config["api-admin-enabled"] = true
		config["api-debug-enabled"] = true
		config["api-eth-enabled"] = true
		config["api-web3-enabled"] = true
		config["api-personal-enabled"] = true
		config["api-txpool-enabled"] = true
		config["log-level"] = "info"
		// Disable BLS validation for single-node mainnet replay
		config["staking-disable-bls"] = true
		// Use ephemeral BLS keys for mainnet replay
		config["staking-ephemeral-signer-enabled"] = true
	}

	// Add any network-specific metadata
	for k, v := range l.network.Metadata {
		config[k] = v
	}

	configPath := filepath.Join(nodeDir, "node-config.json")
	configData, _ := json.MarshalIndent(config, "", "  ")
	if err := os.WriteFile(configPath, configData, 0644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	// Create launch script
	if err := l.createLaunchScript(index, port, configPath); err != nil {
		return fmt.Errorf("failed to create launch script: %w", err)
	}

	return nil
}

func (l *Launcher) createLaunchScript(index, port int, configPath string) error {
	// Check if this is mainnet with import genesis (replay mode)
	envPrefix := ""
	if l.network.Name == "mainnet" && l.network.Genesis.Source == "import" {
		envPrefix = "GENESIS_LUX=1 "
	}

	launchScript := fmt.Sprintf(`#!/bin/bash
echo "Starting node %d on port %d..."
%sexec /home/z/work/lux/node/build/luxd --config-file=%s "$@"
`, index, port, envPrefix, configPath)

	launchPath := filepath.Join(l.getNodeDir(index), "launch.sh")
	return os.WriteFile(launchPath, []byte(launchScript), 0755)
}

func (l *Launcher) startNodes() error {
	if l.dryRun {
		fmt.Printf("  Would start %d nodes\n", l.network.Nodes)
		return nil
	}

	fmt.Printf("\n✅ Network '%s' configured successfully!\n", l.network.Name)
	fmt.Printf("\nTo start the network:\n")
	for i := 0; i < l.network.Nodes; i++ {
		fmt.Printf("  %s\n", filepath.Join(l.getNodeDir(i), "launch.sh"))
	}

	return nil
}

func (l *Launcher) getNodeDir(index int) string {
	if l.network.Nodes == 1 {
		return l.baseDir
	}
	return filepath.Join(l.baseDir, fmt.Sprintf("node-%d", index))
}

func (l *Launcher) saveValidatorInfo(index int, creds *core.StakingCredentials) error {
	// Use correct ports based on network
	basePort := 9650
	if l.network.Name == "mainnet" {
		basePort = 9630
	}

	info := map[string]interface{}{
		"nodeID":               creds.NodeID,
		"nodePort":             basePort + (index * 10),
		"stakingPort":          basePort + 1 + (index * 10),
		"networkID":            l.network.NetworkID,
		"blsPublicKey":         fmt.Sprintf("0x%x", creds.BLSPublicKey),
		"blsProofOfPossession": fmt.Sprintf("0x%x", creds.ProofOfPossession),
		"dataDirectory":        l.getNodeDir(index),
		"consensusParameters": map[string]int{
			"k":     l.network.Consensus.K,
			"alpha": l.network.Consensus.Alpha,
			"beta":  l.network.Consensus.Beta,
		},
		"timestamp": time.Now().Unix(),
	}

	infoPath := filepath.Join(l.getNodeDir(index), "validator-info.json")
	data, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(infoPath, data, 0644)
}

func (l *Launcher) generateMainnetGenesisFiles(outputDir string, nodeInfo map[string]interface{}) error {
	// Create directory structure
	pDir := filepath.Join(outputDir, "P")
	cDir := filepath.Join(outputDir, "C")
	xDir := filepath.Join(outputDir, "X")

	for _, dir := range []string{pDir, cDir, xDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	// Ensure C-Chain genesis exists
	cGenesisPath := filepath.Join(cDir, "genesis.json")
	if _, err := os.Stat(cGenesisPath); err != nil {
		return fmt.Errorf("C-Chain genesis not found at %s", cGenesisPath)
	}

	// Read C-Chain genesis to include in P-Chain config
	cGenesisData, err := os.ReadFile(cGenesisPath)
	if err != nil {
		return fmt.Errorf("failed to read C-Chain genesis: %w", err)
	}

	// Load validators configuration (21 validators for mainnet)
	validatorsPath := filepath.Join(outputDir, "validators.json")
	var validators []map[string]interface{}

	if validatorsData, err := os.ReadFile(validatorsPath); err == nil {
		// Use existing validators file if available
		if err := json.Unmarshal(validatorsData, &validators); err != nil {
			return fmt.Errorf("failed to parse validators.json: %w", err)
		}
		fmt.Printf("  Loaded %d validators from validators.json\n", len(validators))
	} else {
		// Fall back to single validator for development
		nodeID, _ := nodeInfo["nodeID"].(string)
		blsPubKey, _ := nodeInfo["blsPublicKey"].([]byte)
		blsPOP, _ := nodeInfo["blsProofOfPossession"].([]byte)

		validators = []map[string]interface{}{
			{
				"nodeID":        nodeID,
				"rewardAddress": "X-lux1w6ajywx2t9wfqej7ddxk9v0ej3qtxs5p6f7q9",
				"delegationFee": 20000, // 2%
				"signer": map[string]string{
					"publicKey":         fmt.Sprintf("0x%x", blsPubKey),
					"proofOfPossession": fmt.Sprintf("0x%x", blsPOP),
				},
			},
		}
	}

	// Build allocations from validators
	allocations := []map[string]interface{}{}
	stakedFunds := []string{}

	// For mainnet with 21 validators, distribute initial supply
	totalSupply := uint64(500000000000000000) // 500M LUX
	perValidatorAmount := totalSupply / uint64(len(validators))

	for i, validator := range validators {
		ethAddr := fmt.Sprintf("0x%040x", i)     // Simple eth addresses
		luxAddr := fmt.Sprintf("X-lux1%039x", i) // Simple lux addresses

		// Update validator with addresses if not present
		if _, ok := validator["ethAddress"]; ok {
			ethAddr = validator["ethAddress"].(string)
		}
		if _, ok := validator["rewardAddress"]; ok {
			luxAddr = validator["rewardAddress"].(string)
		} else {
			validator["rewardAddress"] = luxAddr
		}

		allocations = append(allocations, map[string]interface{}{
			"ethAddr":       ethAddr,
			"luxAddr":       luxAddr,
			"initialAmount": perValidatorAmount,
			"unlockSchedule": []map[string]interface{}{
				{
					"amount":   perValidatorAmount,
					"locktime": 0,
				},
			},
		})

		stakedFunds = append(stakedFunds, luxAddr)
	}

	// Create P-Chain genesis configuration
	pConfig := map[string]interface{}{
		"networkID":                  l.network.NetworkID,
		"startTime":                  1750460293, // Match C-Chain timestamp
		"initialStakeDuration":       31536000,   // 1 year
		"initialStakeDurationOffset": 5400,       // 90 minutes between validators
		"message":                    "lux mainnet genesis",
		"cChainGenesis":              string(cGenesisData),
		"allocations":                allocations,
		"initialStakedFunds":         stakedFunds,
		"initialStakers":             validators,
	}

	// Write P-Chain genesis
	pGenesisPath := filepath.Join(pDir, "genesis.json")
	pGenesisData, err := json.MarshalIndent(pConfig, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal P-Chain genesis: %w", err)
	}
	if err := os.WriteFile(pGenesisPath, pGenesisData, 0644); err != nil {
		return fmt.Errorf("failed to write P-Chain genesis: %w", err)
	}

	// Create X-Chain genesis with cross-chain airdrop allocations
	xAllocations := []map[string]interface{}{}

	// Add validator allocations to X-Chain
	for _, alloc := range allocations {
		if luxAddr, ok := alloc["luxAddr"].(string); ok {
			// Initial X-Chain allocation for validators
			xAllocations = append(xAllocations, map[string]interface{}{
				"address": luxAddr,
				"balance": alloc["initialAmount"],
			})
		}
	}

	// TODO: Add BSC/ETH cross-chain airdrop allocations here
	// This would include:
	// - BSC ZOO token burns (1:1 mapping)
	// - EGG NFT holders (4.2M ZOO per egg)
	// - Ethereum Lux Genesis NFT holders

	xConfig := map[string]interface{}{
		"networkID":     l.network.NetworkID,
		"initialSupply": totalSupply,
		"allocations":   xAllocations,
	}

	xGenesisPath := filepath.Join(xDir, "genesis.json")
	xGenesisData, err := json.MarshalIndent(xConfig, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal X-Chain genesis: %w", err)
	}
	if err := os.WriteFile(xGenesisPath, xGenesisData, 0644); err != nil {
		return fmt.Errorf("failed to write X-Chain genesis: %w", err)
	}

	return nil
}

func (l *Launcher) createCombinedGenesis() error {
	// For mainnet with GENESIS_LUX=1, luxd expects specific genesis format
	// This creates a combined genesis.json in the node directory
	genesisPath := filepath.Join(l.baseDir, "genesis.json")

	// Read all three genesis files
	baseDir := fmt.Sprintf("/home/z/work/lux/genesis/configs/lux-mainnet-%d", l.network.NetworkID)

	pGenesis, err := os.ReadFile(filepath.Join(baseDir, "P", "genesis.json"))
	if err != nil {
		return fmt.Errorf("failed to read P-Chain genesis: %w", err)
	}

	cGenesis, err := os.ReadFile(filepath.Join(baseDir, "C", "genesis.json"))
	if err != nil {
		return fmt.Errorf("failed to read C-Chain genesis: %w", err)
	}

	xGenesis, err := os.ReadFile(filepath.Join(baseDir, "X", "genesis.json"))
	if err != nil {
		return fmt.Errorf("failed to read X-Chain genesis: %w", err)
	}

	// Parse P-Chain genesis to extract the main config
	var pConfig map[string]interface{}
	if err := json.Unmarshal(pGenesis, &pConfig); err != nil {
		return fmt.Errorf("failed to parse P-Chain genesis: %w", err)
	}

	// The combined genesis format expected by luxd
	combined := map[string]interface{}{
		"networkID":                  l.network.NetworkID,
		"allocations":                pConfig["allocations"],
		"startTime":                  pConfig["startTime"],
		"initialStakeDuration":       pConfig["initialStakeDuration"],
		"initialStakeDurationOffset": pConfig["initialStakeDurationOffset"],
		"initialStakedFunds":         pConfig["initialStakedFunds"],
		"initialStakers":             pConfig["initialStakers"],
		"cChainGenesis":              string(cGenesis),
		"xChainGenesis":              string(xGenesis),
		"message":                    pConfig["message"],
	}

	// Write combined genesis
	data, err := json.MarshalIndent(combined, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal combined genesis: %w", err)
	}

	if err := os.WriteFile(genesisPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write combined genesis: %w", err)
	}

	fmt.Printf("  Created combined genesis at %s\n", genesisPath)
	return nil
}
