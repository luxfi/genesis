package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// NetworkConfig represents the network configuration
type NetworkConfig struct {
	NumValidators     int                    `json:"num_validators"`
	NetworkID         uint32                 `json:"network_id"`
	DataDir           string                 `json:"data_dir"`
	HTTPPortStart     uint16                 `json:"http_port_start"`
	StakingPortStart  uint16                 `json:"staking_port_start"`
	ConsensusParams   ConsensusParams        `json:"consensus_params"`
	ChainConfig       map[string]interface{} `json:"chain_config"`
	MigratedDBPath    string                 `json:"migrated_db_path"`
	LuxdBinaryPath    string                 `json:"luxd_binary_path"`
}

// ConsensusParams represents consensus configuration
type ConsensusParams struct {
	K                     int           `json:"k"`
	AlphaPreference       int           `json:"alpha_preference"`
	AlphaConfidence       int           `json:"alpha_confidence"`
	Beta                  int           `json:"beta"`
	ConcurrentRepolls     int           `json:"concurrent_repolls"`
	OptimalProcessing     int           `json:"optimal_processing"`
	MaxOutstandingItems   int           `json:"max_outstanding_items"`
	MaxItemProcessingTime time.Duration `json:"max_item_processing_time"`
}

func main() {
	var (
		numValidators = flag.Int("validators", 1, "Number of validators to launch")
		networkID     = flag.Int("network-id", 96369, "Network ID")
		dataDir       = flag.String("data-dir", "/tmp/lux-validators", "Base data directory")
		httpPort      = flag.Int("http-port", 9650, "Starting HTTP port")
		stakingPort   = flag.Int("staking-port", 9651, "Starting staking port")
		luxdPath      = flag.String("luxd", "/Users/z/work/lux/node/build/luxd", "Path to luxd binary")
		migratedDB    = flag.String("db", "/tmp/lux-mainnet-final/chainData/2XpgdN3WNtM6AuzGgnXW7S6BqbH7DYY8CKwqaUiDUj67vYGvfC/db", "Path to migrated database")
		configFile    = flag.String("config", "", "Path to network config JSON file")
	)
	flag.Parse()

	// Load or create configuration
	var config NetworkConfig
	if *configFile != "" {
		data, err := ioutil.ReadFile(*configFile)
		if err != nil {
			log.Fatalf("Failed to read config file: %v", err)
		}
		if err := json.Unmarshal(data, &config); err != nil {
			log.Fatalf("Failed to parse config file: %v", err)
		}
	} else {
		// Use command-line flags
		config = NetworkConfig{
			NumValidators:    *numValidators,
			NetworkID:        uint32(*networkID),
			DataDir:          *dataDir,
			HTTPPortStart:    uint16(*httpPort),
			StakingPortStart: uint16(*stakingPort),
			LuxdBinaryPath:   *luxdPath,
			MigratedDBPath:   *migratedDB,
		}

		// Set consensus parameters based on number of validators
		config.ConsensusParams = getConsensusParams(*numValidators)
	}

	// Clean up old processes
	cleanupOldProcesses()

	// Launch network
	if err := launchNetwork(config); err != nil {
		log.Fatalf("Failed to launch network: %v", err)
	}
}

// getConsensusParams returns appropriate consensus parameters for the number of validators
func getConsensusParams(numValidators int) ConsensusParams {
	switch numValidators {
	case 1:
		// Single validator configuration
		return ConsensusParams{
			K:                     1,
			AlphaPreference:       1,
			AlphaConfidence:       1,
			Beta:                  1,
			ConcurrentRepolls:     1,
			OptimalProcessing:     10,
			MaxOutstandingItems:   256,
			MaxItemProcessingTime: 30 * time.Second,
		}
	case 2:
		// Two validator configuration
		return ConsensusParams{
			K:                     2,
			AlphaPreference:       2,
			AlphaConfidence:       2,
			Beta:                  2,
			ConcurrentRepolls:     2,
			OptimalProcessing:     10,
			MaxOutstandingItems:   256,
			MaxItemProcessingTime: 30 * time.Second,
		}
	case 3, 4, 5:
		// Small network configuration (3-5 validators)
		k := numValidators
		alpha := (k*2 + 2) / 3 // ~67%
		return ConsensusParams{
			K:                     k,
			AlphaPreference:       alpha,
			AlphaConfidence:       alpha,
			Beta:                  k / 2,
			ConcurrentRepolls:     4,
			OptimalProcessing:     10,
			MaxOutstandingItems:   256,
			MaxItemProcessingTime: 5 * time.Second,
		}
	case 11:
		// Testnet configuration (11 validators)
		return ConsensusParams{
			K:                     11,
			AlphaPreference:       7,
			AlphaConfidence:       9,
			Beta:                  6,
			ConcurrentRepolls:     4,
			OptimalProcessing:     10,
			MaxOutstandingItems:   256,
			MaxItemProcessingTime: 6300 * time.Millisecond,
		}
	case 21:
		// Mainnet configuration (21 validators)
		return ConsensusParams{
			K:                     21,
			AlphaPreference:       13,
			AlphaConfidence:       18,
			Beta:                  8,
			ConcurrentRepolls:     4,
			OptimalProcessing:     10,
			MaxOutstandingItems:   256,
			MaxItemProcessingTime: 9630 * time.Millisecond,
		}
	default:
		// Dynamic calculation for other sizes
		k := numValidators
		alpha := (k*2 + 2) / 3 // ~67%
		beta := k / 3
		if beta < 1 {
			beta = 1
		}
		return ConsensusParams{
			K:                     k,
			AlphaPreference:       alpha,
			AlphaConfidence:       alpha,
			Beta:                  beta,
			ConcurrentRepolls:     4,
			OptimalProcessing:     10,
			MaxOutstandingItems:   256,
			MaxItemProcessingTime: time.Duration(k*300) * time.Millisecond,
		}
	}
}

// cleanupOldProcesses stops any existing luxd processes
func cleanupOldProcesses() {
	fmt.Println("Stopping any existing luxd processes...")
	exec.Command("pkill", "-f", "luxd").Run()
	time.Sleep(2 * time.Second)
}

// launchNetwork launches the validator network
func launchNetwork(config NetworkConfig) error {
	fmt.Printf("===================================\n")
	fmt.Printf("  LUX NETWORK LAUNCHER\n")
	fmt.Printf("  Validators: %d\n", config.NumValidators)
	fmt.Printf("  Network ID: %d\n", config.NetworkID)
	fmt.Printf("===================================\n\n")

	// Clean and create base directory
	os.RemoveAll(config.DataDir)
	os.MkdirAll(config.DataDir, 0755)

	// Track node info
	var nodes []NodeInfo
	var bootstrapIP string
	var bootstrapID string

	// Launch each validator
	for i := 1; i <= config.NumValidators; i++ {
		nodeInfo, err := launchValidator(i, config, bootstrapIP, bootstrapID)
		if err != nil {
			return fmt.Errorf("failed to launch validator %d: %w", i, err)
		}
		nodes = append(nodes, nodeInfo)

		// First node becomes bootstrap
		if i == 1 {
			bootstrapIP = fmt.Sprintf("127.0.0.1:%d", config.StakingPortStart)
			bootstrapID = nodeInfo.NodeID
			fmt.Printf("Bootstrap node: %s\n", bootstrapID)
		}
	}

	// Wait for network to stabilize
	fmt.Println("\nWaiting for network to stabilize...")
	time.Sleep(10 * time.Second)

	// Check network status
	checkNetworkStatus(nodes)

	// Save network info
	saveNetworkInfo(config, nodes)

	fmt.Printf("\n===================================\n")
	fmt.Printf("  NETWORK LAUNCHED SUCCESSFULLY\n")
	fmt.Printf("===================================\n")
	for _, node := range nodes {
		fmt.Printf("Node %d: PID=%d, API=http://127.0.0.1:%d\n", 
			node.Index, node.PID, node.HTTPPort)
	}
	fmt.Printf("\nLogs: tail -f %s/node*/node.log\n", config.DataDir)
	fmt.Printf("Stop all: pkill -f luxd\n")

	return nil
}

// NodeInfo tracks information about a launched node
type NodeInfo struct {
	Index       int
	NodeID      string
	PID         int
	HTTPPort    uint16
	StakingPort uint16
	DataDir     string
}

// launchValidator launches a single validator node
func launchValidator(index int, config NetworkConfig, bootstrapIP, bootstrapID string) (NodeInfo, error) {
	nodeDir := filepath.Join(config.DataDir, fmt.Sprintf("node%02d", index))
	httpPort := config.HTTPPortStart + uint16(index-1)*2
	stakingPort := config.StakingPortStart + uint16(index-1)*2

	// Create node directories
	os.MkdirAll(filepath.Join(nodeDir, "staking"), 0755)
	os.MkdirAll(filepath.Join(nodeDir, "db"), 0755)
	os.MkdirAll(filepath.Join(nodeDir, "logs"), 0755)
	os.MkdirAll(filepath.Join(nodeDir, "chainData"), 0755)

	// Copy migrated database
	chainID := "2XpgdN3WNtM6AuzGgnXW7S6BqbH7DYY8CKwqaUiDUj67vYGvfC"
	chainDataPath := filepath.Join(nodeDir, "chainData", chainID)
	os.MkdirAll(chainDataPath, 0755)
	
	copyCmd := exec.Command("cp", "-r", config.MigratedDBPath, filepath.Join(chainDataPath, "db"))
	if err := copyCmd.Run(); err != nil {
		return NodeInfo{}, fmt.Errorf("failed to copy database: %w", err)
	}

	// Generate staking keys
	if err := generateStakingKeys(nodeDir, index); err != nil {
		return NodeInfo{}, fmt.Errorf("failed to generate staking keys: %w", err)
	}

	// Create node configuration with proper cancun fork
	nodeConfig := createNodeConfig(config, index)
	configPath := filepath.Join(nodeDir, "config.json")
	
	configData, err := json.MarshalIndent(nodeConfig, "", "  ")
	if err != nil {
		return NodeInfo{}, fmt.Errorf("failed to marshal config: %w", err)
	}
	
	if err := ioutil.WriteFile(configPath, configData, 0644); err != nil {
		return NodeInfo{}, fmt.Errorf("failed to write config: %w", err)
	}

	// Build launch command
	args := []string{
		"--config-file=" + configPath,
		"--data-dir=" + nodeDir,
		"--db-dir=" + filepath.Join(nodeDir, "db"),
		"--chain-data-dir=" + filepath.Join(nodeDir, "chainData"),
		"--log-dir=" + filepath.Join(nodeDir, "logs"),
		"--http-host=0.0.0.0",
		fmt.Sprintf("--http-port=%d", httpPort),
		fmt.Sprintf("--staking-port=%d", stakingPort),
		"--staking-tls-cert-file=" + filepath.Join(nodeDir, "staking", "staker.crt"),
		"--staking-tls-key-file=" + filepath.Join(nodeDir, "staking", "staker.key"),
		"--log-level=info",
	}

	// Add bootstrap info for non-first nodes
	if index > 1 && bootstrapIP != "" && bootstrapID != "" {
		args = append(args, "--bootstrap-ips="+bootstrapIP)
		args = append(args, "--bootstrap-ids="+bootstrapID)
	} else {
		args = append(args, "--bootstrap-ips=")
		args = append(args, "--bootstrap-ids=")
	}

	// Launch the node
	cmd := exec.Command(config.LuxdBinaryPath, args...)
	
	// Create log file
	logFile, err := os.Create(filepath.Join(nodeDir, "node.log"))
	if err != nil {
		return NodeInfo{}, fmt.Errorf("failed to create log file: %w", err)
	}
	defer logFile.Close()
	
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	
	if err := cmd.Start(); err != nil {
		return NodeInfo{}, fmt.Errorf("failed to start node: %w", err)
	}

	fmt.Printf("Launched validator %d (PID: %d, HTTP: %d, Staking: %d)\n", 
		index, cmd.Process.Pid, httpPort, stakingPort)

	// Wait for node to initialize
	time.Sleep(5 * time.Second)

	// Get node ID
	nodeID := getNodeID(httpPort)

	return NodeInfo{
		Index:       index,
		NodeID:      nodeID,
		PID:         cmd.Process.Pid,
		HTTPPort:    httpPort,
		StakingPort: stakingPort,
		DataDir:     nodeDir,
	}, nil
}

// generateStakingKeys generates TLS certificates for staking
func generateStakingKeys(nodeDir string, index int) error {
	stakingDir := filepath.Join(nodeDir, "staking")
	keyPath := filepath.Join(stakingDir, "staker.key")
	certPath := filepath.Join(stakingDir, "staker.crt")

	// Generate private key
	keyCmd := exec.Command("openssl", "genrsa", "-out", keyPath, "4096")
	if err := keyCmd.Run(); err != nil {
		return err
	}

	// Generate certificate
	subject := fmt.Sprintf("/C=US/ST=State/L=City/O=Lux/CN=validator%02d", index)
	certCmd := exec.Command("openssl", "req", "-new", "-x509", 
		"-key", keyPath, "-out", certPath, "-days", "365", "-subj", subject)
	return certCmd.Run()
}

// createNodeConfig creates the node configuration with proper fork settings
func createNodeConfig(config NetworkConfig, index int) map[string]interface{} {
	return map[string]interface{}{
		"network-id":                      config.NetworkID,
		"health-check-frequency":          "2s",
		"network-max-reconnect-delay":     "1s",
		"network-allow-private-ips":       true,
		"consensus-shutdown-timeout":      "10s",
		"consensus-gossip-frequency":      "250ms",
		"min-stake-duration":              "336h",
		"max-stake-duration":              "8760h",
		"stake-minting-period":            "8760h",
		"stake-max-consumption-rate":      120000,
		"stake-min-consumption-rate":      100000,
		"stake-supply-cap":                720000000000000000,
		"snow-sample-size":                config.ConsensusParams.K,
		"snow-quorum-size":                config.ConsensusParams.AlphaPreference,
		"snow-virtuous-commit-threshold":  5,
		"snow-rogue-commit-threshold":     10,
		"p-chain-config": map[string]interface{}{
			"K":     config.ConsensusParams.K,
			"alpha": config.ConsensusParams.AlphaPreference,
			"beta":  config.ConsensusParams.Beta,
		},
		"c-chain-config": map[string]interface{}{
			"K":     config.ConsensusParams.K,
			"alpha": config.ConsensusParams.AlphaPreference,
			"beta":  config.ConsensusParams.Beta,
			"fork-config": map[string]interface{}{
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
				"arrowGlacierBlock":   0,
				"grayGlacierBlock":    0,
				"mergeNetsplitBlock":  0,
				"shanghaiTime":        0,
				"cancunTime":          0,
				"pragueTime":          nil,
				"verkleTime":          nil,
			},
			"blobSchedule": map[string]interface{}{
				"cancun": 131072,
			},
		},
	}
}

// getNodeID retrieves the node ID via RPC
func getNodeID(httpPort uint16) string {
	cmd := exec.Command("curl", "-s", "-X", "POST",
		fmt.Sprintf("http://127.0.0.1:%d/ext/info", httpPort),
		"-H", "Content-Type: application/json",
		"-d", `{"jsonrpc":"2.0","id":1,"method":"info.getNodeID"}`)
	
	output, err := cmd.Output()
	if err != nil {
		return "Unknown"
	}

	// Simple JSON parsing for NodeID
	var result map[string]interface{}
	if err := json.Unmarshal(output, &result); err != nil {
		return "Unknown"
	}
	
	if res, ok := result["result"].(map[string]interface{}); ok {
		if nodeID, ok := res["nodeID"].(string); ok {
			return nodeID
		}
	}
	
	return "Unknown"
}

// checkNetworkStatus checks the status of all nodes
func checkNetworkStatus(nodes []NodeInfo) {
	fmt.Println("\n=== Network Status ===")
	for _, node := range nodes {
		fmt.Printf("Node %d (ID: %s):\n", node.Index, node.NodeID)
		
		// Check if node is running
		checkCmd := exec.Command("ps", "-p", fmt.Sprintf("%d", node.PID))
		if err := checkCmd.Run(); err != nil {
			fmt.Printf("  Status: NOT RUNNING\n")
			continue
		}
		
		fmt.Printf("  Status: RUNNING\n")
		
		// Check C-Chain block height
		cmd := exec.Command("curl", "-s", "-X", "POST",
			fmt.Sprintf("http://127.0.0.1:%d/ext/bc/C/rpc", node.HTTPPort),
			"-H", "Content-Type: application/json",
			"-d", `{"jsonrpc":"2.0","id":1,"method":"eth_blockNumber"}`)
		
		if output, err := cmd.Output(); err == nil {
			var result map[string]interface{}
			if json.Unmarshal(output, &result) == nil {
				if res, ok := result["result"].(string); ok {
					fmt.Printf("  C-Chain Height: %s\n", res)
				}
			}
		}
	}
}

// saveNetworkInfo saves network information to a file
func saveNetworkInfo(config NetworkConfig, nodes []NodeInfo) {
	info := map[string]interface{}{
		"network_id":    config.NetworkID,
		"num_validators": config.NumValidators,
		"data_dir":      config.DataDir,
		"consensus":     config.ConsensusParams,
		"nodes":         nodes,
		"timestamp":     time.Now(),
	}

	data, _ := json.MarshalIndent(info, "", "  ")
	ioutil.WriteFile(filepath.Join(config.DataDir, "network-info.json"), data, 0644)
}