package mainnet

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/luxfi/genesis/pkg/application"
)

// NodeInfo contains node identification information
type NodeInfo struct {
	NodeID               string `json:"nodeID"`
	BLSPublicKey        string `json:"blsPublicKey"`
	BLSProofOfPossession string `json:"blsProofOfPossession"`
}

// SimpleReplayRunner implements mainnet replay with proper consensus settings
type SimpleReplayRunner struct {
	app *application.Genesis
}

func NewSimpleReplayRunner(app *application.Genesis) *SimpleReplayRunner {
	return &SimpleReplayRunner{app: app}
}

func (r *SimpleReplayRunner) Run(opts ReplayOptions) error {
	fmt.Println("=== Lux Mainnet Replay Tool ===")
	fmt.Printf("Network: %s (ID: %d)\n", opts.NetworkID, r.getNetworkID(opts.NetworkID))
	fmt.Printf("Genesis DB: %s\n", opts.GenesisDB)
	
	// Verify genesis database exists
	if opts.GenesisDB != "" {
		if _, err := os.Stat(opts.GenesisDB); os.IsNotExist(err) {
			return fmt.Errorf("genesis database not found at %s", opts.GenesisDB)
		}
		fmt.Printf("✓ Genesis database found: %s\n", opts.GenesisDB)
	}

	// Step 1: Prepare directories
	baseDir := opts.DataDir
	if baseDir == "" {
		baseDir = filepath.Join("/tmp", fmt.Sprintf("lux-mainnet-replay-%d", time.Now().Unix()))
	}

	// Create necessary directories
	dirs := []string{
		baseDir,
		filepath.Join(baseDir, "staking-keys"),
		filepath.Join(baseDir, "configs", "chains", "C"),
		filepath.Join(baseDir, "configs", "chains", "X"),
		filepath.Join(baseDir, "configs", "chains", "P"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	keysDir := filepath.Join(baseDir, "staking-keys")

	// Step 2: Generate or use existing staking keys
	fmt.Println("\n=== Step 1: Staking Keys ===")
	
	// Check if we should use existing keys
	if opts.KeysDir != "" {
		fmt.Printf("Using existing keys from: %s\n", opts.KeysDir)
		// Copy existing keys
		if err := r.copyKeys(opts.KeysDir, keysDir); err != nil {
			return fmt.Errorf("failed to copy keys: %w", err)
		}
	} else if opts.GenerateKeys {
		fmt.Println("Generating new staking keys with BLS...")
		if err := r.generateKeys(keysDir); err != nil {
			return fmt.Errorf("failed to generate keys: %w", err)
		}
	}

	// Read node information
	nodeInfo, err := r.readNodeInfo(keysDir)
	if err != nil {
		return fmt.Errorf("failed to read node info: %w", err)
	}

	fmt.Printf("NodeID: %s\n", nodeInfo["nodeID"])

	// Step 3: Create genesis configuration
	fmt.Println("\n=== Step 2: Genesis Configuration ===")
	genesisPath := filepath.Join(baseDir, "genesis.json")
	
	if err := r.createGenesis(genesisPath, nodeInfo, opts); err != nil {
		return fmt.Errorf("failed to create genesis: %w", err)
	}

	fmt.Printf("Created genesis.json at: %s\n", genesisPath)

	// Step 4: Create chain configurations
	fmt.Println("\n=== Step 3: Chain Configurations ===")
	
	// C-Chain config with database settings
	cChainConfig := map[string]interface{}{
		"db-type":                   opts.CChainDBType,
		"log-level":                 opts.LogLevel,
		"state-sync-enabled":        false,
		"offline-pruning-enabled":   false,
		"allow-unprotected-txs":     true,
		"continuous-profiler-dir":   "",
		"continuous-profiler-frequency": 900000000000,
		"continuous-profiler-max-files": 5,
	}

	// Add genesis database configuration if provided
	if opts.GenesisDB != "" {
		cChainConfig["genesis-db"] = opts.GenesisDB
		cChainConfig["genesis-db-type"] = opts.GenesisDBType
	}

	cChainConfigBytes, _ := json.MarshalIndent(cChainConfig, "", "  ")
	cChainConfigPath := filepath.Join(baseDir, "configs", "chains", "C", "config.json")
	if err := os.WriteFile(cChainConfigPath, cChainConfigBytes, 0644); err != nil {
		return fmt.Errorf("failed to write C-chain config: %w", err)
	}

	// P-Chain config
	pChainConfig := map[string]interface{}{
		"db-type":   opts.DBType,
		"log-level": opts.LogLevel,
	}
	pChainConfigBytes, _ := json.MarshalIndent(pChainConfig, "", "  ")
	pChainConfigPath := filepath.Join(baseDir, "configs", "chains", "P", "config.json")
	if err := os.WriteFile(pChainConfigPath, pChainConfigBytes, 0644); err != nil {
		return fmt.Errorf("failed to write P-chain config: %w", err)
	}

	// X-Chain config
	xChainConfig := map[string]interface{}{
		"db-type":   opts.DBType,
		"log-level": opts.LogLevel,
	}
	xChainConfigBytes, _ := json.MarshalIndent(xChainConfig, "", "  ")
	xChainConfigPath := filepath.Join(baseDir, "configs", "chains", "X", "config.json")
	if err := os.WriteFile(xChainConfigPath, xChainConfigBytes, 0644); err != nil {
		return fmt.Errorf("failed to write X-chain config: %w", err)
	}

	fmt.Println("✓ Created chain configurations")

	// Step 5: Launch luxd
	if opts.SkipLaunch {
		fmt.Println("\n=== Configuration Complete ===")
		fmt.Println("To launch luxd manually, run:")
		fmt.Printf("\nluxd \\\n")
		fmt.Printf("  --network-id=%d \\\n", r.getNetworkID(opts.NetworkID))
		fmt.Printf("  --genesis-file=%s \\\n", genesisPath)
		fmt.Printf("  --data-dir=%s/data \\\n", baseDir)
		fmt.Printf("  --chain-config-dir=%s/configs/chains \\\n", baseDir)
		if opts.GenesisDB != "" {
			fmt.Printf("  --genesis-db=%s \\\n", opts.GenesisDB)
			fmt.Printf("  --genesis-db-type=%s \\\n", opts.GenesisDBType)
		}
		if opts.EnableStaking {
			fmt.Printf("  --staking-tls-cert-file=%s/staker.crt \\\n", keysDir)
			fmt.Printf("  --staking-tls-key-file=%s/staker.key \\\n", keysDir)
			fmt.Printf("  --staking-signer-key-file=%s/signer.key \\\n", keysDir)
		}
		fmt.Printf("  --http-port=%d \\\n", opts.HTTPPort)
		fmt.Printf("  --staking-port=%d\n", opts.StakingPort)
		return nil
	}

	fmt.Println("\n=== Step 4: Launching luxd ===")
	return r.launchNode(baseDir, keysDir, genesisPath, nodeInfo, opts)
}

func (r *SimpleReplayRunner) generateKeys(keysDir string) error {
	// Use the genesis staking keygen command
	cmd := exec.Command("genesis", "staking", "keygen", "--output", keysDir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (r *SimpleReplayRunner) copyKeys(sourceDir, destDir string) error {
	files := []string{"staker.crt", "staker.key", "signer.key", "genesis-staker.json"}
	for _, file := range files {
		src := filepath.Join(sourceDir, file)
		dst := filepath.Join(destDir, file)
		
		data, err := os.ReadFile(src)
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", file, err)
		}
		
		if err := os.WriteFile(dst, data, 0600); err != nil {
			return fmt.Errorf("failed to write %s: %w", file, err)
		}
	}
	return nil
}

func (r *SimpleReplayRunner) readNodeInfo(keysDir string) (map[string]interface{}, error) {
	nodeInfoPath := filepath.Join(keysDir, "genesis-staker.json")
	data, err := os.ReadFile(nodeInfoPath)
	if err != nil {
		return nil, err
	}

	var nodeInfo map[string]interface{}
	if err := json.Unmarshal(data, &nodeInfo); err != nil {
		return nil, err
	}

	return nodeInfo, nil
}

func (r *SimpleReplayRunner) createGenesis(genesisPath string, nodeInfo map[string]interface{}, opts ReplayOptions) error {
	// Create proper mainnet genesis with consensus parameters
	genesis := r.buildGenesisConfig(nodeInfo, opts)
	
	genesisBytes, err := json.MarshalIndent(genesis, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(genesisPath, genesisBytes, 0644)
}

func (r *SimpleReplayRunner) buildGenesisConfig(nodeInfo map[string]interface{}, opts ReplayOptions) map[string]interface{} {
	// Build proper genesis configuration
	networkID := r.getNetworkID(opts.NetworkID)
	
	// Get BLS info from nodeInfo
	signerInfo := nodeInfo["signer"].(map[string]interface{})
	
	// Initial staker for the validator
	initialStaker := map[string]interface{}{
		"nodeID":             nodeInfo["nodeID"],
		"rewardAddress":      "lux1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqzenvla",
		"delegationFee":      20000,
		"signer": map[string]interface{}{
			"publicKey":         signerInfo["publicKey"],
			"proofOfPossession": signerInfo["proofOfPossession"],
		},
	}

	// Build full genesis structure
	genesis := map[string]interface{}{
		"networkID": networkID,
		"allocations": []interface{}{
			map[string]interface{}{
				"ethAddr":        "0xb3d82b1367d362de99ab59a658165aff520cbd4d",
				"luxAddr":        "lux1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqzenvla",
				"initialAmount":  333333333333333333,
				"unlockSchedule": []interface{}{},
			},
		},
		"startTime":                  1630987200,
		"initialStakeDuration":       31536000,
		"initialStakeDurationOffset": 5400,
		"initialStakedFunds": []interface{}{
			"lux1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqzenvla",
		},
		"initialStakers": []interface{}{initialStaker},
		"cChainGenesis": "{\"config\":{\"chainId\":96369,\"homesteadBlock\":0,\"eip150Block\":0,\"eip150Hash\":\"0x2086799aeebeae135c246c65021c82b4e15a2c451340993aacfd2751886514f0\",\"eip155Block\":0,\"eip158Block\":0,\"byzantiumBlock\":0,\"constantinopleBlock\":0,\"petersburgBlock\":0,\"istanbulBlock\":0,\"muirGlacierBlock\":0,\"subnetEVMTimestamp\":0},\"nonce\":\"0x0\",\"timestamp\":\"0x0\",\"extraData\":\"0x00\",\"gasLimit\":\"0x7A1200\",\"difficulty\":\"0x0\",\"mixHash\":\"0x0000000000000000000000000000000000000000000000000000000000000000\",\"coinbase\":\"0x0000000000000000000000000000000000000000\",\"alloc\":{\"0100000000000000000000000000000000000000\":{\"code\":\"0x7300000000000000000000000000000000000000003014608060405260043610603d5760003560e01c80631e010439146042578063b6510bb314606e575b600080fd5b605c60048036036020811015605657600080fd5b503560b1565b60408051918252519081900360200190f35b818015607957600080fd5b5060af60048036036080811015608e57600080fd5b506001600160a01b03813516906020810135906040810135906060013560b6565b005b30cd90565b836001600160a01b031681836108fc8690811502906040516000604051808303818888878c8acf9550505050505015801560f4573d6000803e3d6000fd5b505050505056fea26469706673582212201eebce970fe3f5cb96bf8ac6ba5f5c133fc2908ae3dcd51082cfee8f583429d064736f6c634300060a0033\",\"balance\":\"0x0\"}},\"number\":\"0x0\",\"gasUsed\":\"0x0\",\"parentHash\":\"0x0000000000000000000000000000000000000000000000000000000000000000\"}",
		"message": "genesis mainnet",
	}

	// Add consensus parameters if not single node
	if !opts.SingleNode {
		genesis["consensusParameters"] = map[string]interface{}{
			"k":                     opts.K,
			"alphaPreference":       opts.AlphaPreference,
			"alphaConfidence":       opts.AlphaConfidence,
			"beta":                  opts.Beta,
			"maxItemProcessingTime": fmt.Sprintf("%dms", 9630), // 9.63s for mainnet
		}
	}

	return genesis
}

func (r *SimpleReplayRunner) launchNode(baseDir, keysDir, genesisPath string, nodeInfo map[string]interface{}, opts ReplayOptions) error {
	// Find luxd binary
	luxdPath := r.findLuxdBinary()
	if luxdPath == "" {
		return fmt.Errorf("luxd binary not found")
	}

	fmt.Printf("Using luxd: %s\n", luxdPath)
	fmt.Printf("Configuration:\n")
	fmt.Printf("  - Network ID: %d\n", r.getNetworkID(opts.NetworkID))
	fmt.Printf("  - NodeID: %s\n", nodeInfo["nodeID"])
	fmt.Printf("  - HTTP Port: %d\n", opts.HTTPPort)
	fmt.Printf("  - Staking Port: %d\n", opts.StakingPort)
	fmt.Printf("  - Database: %s (C-Chain), %s (P/X-Chain)\n", opts.CChainDBType, opts.DBType)
	
	if opts.SingleNode {
		fmt.Printf("  - Consensus: Single node (k=1)\n")
	} else {
		fmt.Printf("  - Consensus: Mainnet (k=%d, alpha=%d/%d, beta=%d)\n", 
			opts.K, opts.AlphaPreference, opts.AlphaConfidence, opts.Beta)
	}
	
	if opts.GenesisDB != "" {
		fmt.Printf("  - Genesis DB: %s (%s)\n", opts.GenesisDB, opts.GenesisDBType)
	}

	// Build command arguments
	args := []string{
		fmt.Sprintf("--network-id=%d", r.getNetworkID(opts.NetworkID)),
		fmt.Sprintf("--genesis-file=%s", genesisPath),
		fmt.Sprintf("--data-dir=%s/data", baseDir),
		fmt.Sprintf("--db-type=%s", opts.DBType),
		fmt.Sprintf("--chain-config-dir=%s/configs/chains", baseDir),
		"--http-host=0.0.0.0",
		fmt.Sprintf("--http-port=%d", opts.HTTPPort),
		fmt.Sprintf("--staking-port=%d", opts.StakingPort),
		fmt.Sprintf("--log-level=%s", opts.LogLevel),
		"--api-admin-enabled=true",
		"--api-keystore-enabled=true",
		"--api-metrics-enabled=true",
		"--health-check-frequency=2s",
		"--network-max-reconnect-delay=1s",
	}

	// Add staking configuration if enabled
	if opts.EnableStaking {
		args = append(args,
			fmt.Sprintf("--staking-tls-cert-file=%s/staker.crt", keysDir),
			fmt.Sprintf("--staking-tls-key-file=%s/staker.key", keysDir),
			fmt.Sprintf("--staking-signer-key-file=%s/signer.key", keysDir),
			"--sybil-protection-enabled=true",
		)
	} else {
		args = append(args,
			"--sybil-protection-enabled=false",
			"--sybil-protection-disabled-weight=100",
		)
	}

	// Add genesis database if specified
	if opts.GenesisDB != "" {
		args = append(args,
			fmt.Sprintf("--genesis-db=%s", opts.GenesisDB),
			fmt.Sprintf("--genesis-db-type=%s", opts.GenesisDBType),
		)
	}

	// Add consensus parameters
	if opts.SingleNode {
		args = append(args,
			"--consensus-sample-size=1",
			"--consensus-quorum-size=1",
			"--consensus-commit-threshold=1",
			"--consensus-concurrent-repolls=1",
			"--consensus-optimal-processing=1",
			"--consensus-max-processing=1",
			"--consensus-max-time-processing=2s",
		)
	} else {
		// Use proper mainnet consensus parameters
		args = append(args,
			fmt.Sprintf("--snow-sample-size=%d", opts.K),
			fmt.Sprintf("--snow-quorum-size=%d", opts.SnowQuorumSize),
			fmt.Sprintf("--snow-virtuous-commit-threshold=%d", opts.AlphaPreference),
			fmt.Sprintf("--snow-rogue-commit-threshold=%d", opts.AlphaConfidence),
			fmt.Sprintf("--snow-concurrent-repolls=%d", opts.Beta),
			"--snow-optimal-processing=10",
			"--snow-max-processing=1000",
			"--snow-max-time-processing=2m",
			"--bootstrap-beacon-connection-timeout=1m",
		)
	}

	fmt.Println("\nStarting luxd...")
	cmd := exec.Command(luxdPath, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	return cmd.Run()
}

func (r *SimpleReplayRunner) findLuxdBinary() string {
	// Check common locations for luxd
	paths := []string{
		filepath.Join(r.app.BaseDir, "..", "..", "node", "build", "luxd"),
		filepath.Join(r.app.BaseDir, "..", "node", "build", "luxd"),
		"/Users/z/work/lux/node/build/luxd",
		"./build/luxd",
		"luxd",
	}

	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			// Resolve to absolute path
			abs, _ := filepath.Abs(path)
			return abs
		}
	}

	// Try to find in PATH
	if path, err := exec.LookPath("luxd"); err == nil {
		return path
	}

	return ""
}

func (r *SimpleReplayRunner) getNetworkID(network string) uint32 {
	switch network {
	case "mainnet":
		return 96369
	case "testnet":
		return 96368
	case "local":
		return 1337
	default:
		// Try to parse as number
		var id uint32
		fmt.Sscanf(network, "%d", &id)
		if id > 0 {
			return id
		}
		return 96369 // Default to mainnet
	}
}