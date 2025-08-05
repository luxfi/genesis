package mainnet

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/luxfi/genesis/pkg/application"
)

// NodeInfo contains node identification information
type NodeInfo struct {
	NodeID            string `json:"nodeID"`
	BLSPublicKey      string `json:"blsPublicKey"`
	BLSProofOfPossession string `json:"blsProofOfPossession"`
}

// SimpleReplayRunner implements mainnet replay without complex dependencies
type SimpleReplayRunner struct {
	app *application.Genesis
}

func NewSimpleReplayRunner(app *application.Genesis) *SimpleReplayRunner {
	return &SimpleReplayRunner{app: app}
}

func (r *SimpleReplayRunner) Run(opts ReplayOptions) error {
	fmt.Println("=== Lux Mainnet Replay Tool ===")
	fmt.Println("Setting up mainnet with single-node consensus...")
	
	// Step 1: Prepare directories
	baseDir := opts.DataDir
	if baseDir == "" {
		baseDir = "/tmp/lux-mainnet-replay"
	}
	
	keysDir := filepath.Join(baseDir, "staking-keys")
	if err := os.MkdirAll(keysDir, 0755); err != nil {
		return fmt.Errorf("failed to create keys directory: %w", err)
	}
	
	// Step 2: Run staking keygen using genesis tool
	fmt.Println("\nStep 1: Generating staking keys with BLS...")
	keygenCmd := exec.Command(
		filepath.Join(r.app.BaseDir, "bin", "genesis"),
		"staking", "keygen",
		"--output", keysDir,
	)
	keygenCmd.Stdout = os.Stdout
	keygenCmd.Stderr = os.Stderr
	
	if err := keygenCmd.Run(); err != nil {
		return fmt.Errorf("failed to generate staking keys: %w", err)
	}
	
	// Read the generated node info
	nodeInfoPath := filepath.Join(keysDir, "genesis-staker.json")
	nodeInfoData, err := os.ReadFile(nodeInfoPath)
	if err != nil {
		return fmt.Errorf("failed to read node info: %w", err)
	}
	
	var nodeInfo map[string]interface{}
	if err := json.Unmarshal(nodeInfoData, &nodeInfo); err != nil {
		return fmt.Errorf("failed to parse node info: %w", err)
	}
	
	nodeID := nodeInfo["nodeID"].(string)
	fmt.Printf("Generated NodeID: %s\n", nodeID)
	
	// Step 3: Create genesis.json
	fmt.Println("\nStep 2: Creating genesis configuration...")
	genesisPath := filepath.Join(baseDir, "genesis.json")
	
	// Use the Python script to create genesis
	createGenesisCmd := exec.Command(
		"python3",
		filepath.Join(r.app.BaseDir, "..", "..", "node", "create-single-node-genesis.py"),
		nodeID,
		nodeInfo["signer"].(map[string]interface{})["publicKey"].(string),
		nodeInfo["signer"].(map[string]interface{})["proofOfPossession"].(string),
	)
	
	genesisFile, err := os.Create(genesisPath)
	if err != nil {
		return fmt.Errorf("failed to create genesis file: %w", err)
	}
	defer genesisFile.Close()
	
	createGenesisCmd.Stdout = genesisFile
	createGenesisCmd.Stderr = os.Stderr
	
	if err := createGenesisCmd.Run(); err != nil {
		return fmt.Errorf("failed to create genesis: %w", err)
	}
	
	fmt.Printf("Created genesis.json with NodeID: %s\n", nodeID)
	
	// Step 4: Create chain configurations
	fmt.Println("\nStep 3: Creating chain configurations...")
	chainConfigDir := filepath.Join(baseDir, "configs", "chains", "C")
	if err := os.MkdirAll(chainConfigDir, 0755); err != nil {
		return fmt.Errorf("failed to create chain config dir: %w", err)
	}
	
	cChainConfig := map[string]interface{}{
		"db-type":   opts.CChainDBType,
		"log-level": opts.LogLevel,
		"state-sync-enabled": false,
		"offline-pruning-enabled": false,
		"allow-unprotected-txs": true,
	}
	
	cChainConfigBytes, _ := json.MarshalIndent(cChainConfig, "", "  ")
	if err := os.WriteFile(filepath.Join(chainConfigDir, "config.json"), cChainConfigBytes, 0644); err != nil {
		return fmt.Errorf("failed to write C-chain config: %w", err)
	}
	
	// Step 5: Launch luxd
	if opts.SkipLaunch {
		fmt.Println("\nConfiguration complete! To launch manually:")
		fmt.Printf("cd %s\n", filepath.Dir(r.app.BaseDir))
		fmt.Printf("./node/build/luxd \\\n")
		fmt.Printf("  --network-id=%d \\\n", r.getNetworkID(opts.NetworkID))
		fmt.Printf("  --genesis-file=%s \\\n", genesisPath)
		fmt.Printf("  --data-dir=%s/data \\\n", baseDir)
		fmt.Printf("  --staking-tls-cert-file=%s \\\n", filepath.Join(keysDir, "staker.crt"))
		fmt.Printf("  --staking-tls-key-file=%s \\\n", filepath.Join(keysDir, "staker.key"))
		fmt.Printf("  --staking-signer-key-file=%s\n", filepath.Join(keysDir, "signer.key"))
		return nil
	}
	
	fmt.Println("\nStep 4: Starting luxd with single-node consensus...")
	fmt.Printf("Configuration:\n")
	fmt.Printf("  - Network ID: %d (mainnet)\n", r.getNetworkID(opts.NetworkID))
	fmt.Printf("  - NodeID: %s\n", nodeID)
	fmt.Printf("  - HTTP Port: %d\n", opts.HTTPPort)
	fmt.Printf("  - Staking Port: %d\n", opts.StakingPort)
	fmt.Printf("  - Database: %s (C-Chain), %s (P/X-Chain)\n", opts.CChainDBType, opts.DBType)
	fmt.Printf("  - Consensus: Single node (k=1, alpha=1, beta=1)\n\n")
	
	// Build the command
	luxdPath := filepath.Join(r.app.BaseDir, "..", "..", "node", "build", "luxd")
	cmd := exec.Command(luxdPath,
		fmt.Sprintf("--network-id=%d", r.getNetworkID(opts.NetworkID)),
		fmt.Sprintf("--genesis-file=%s", genesisPath),
		fmt.Sprintf("--data-dir=%s/data", baseDir),
		fmt.Sprintf("--db-type=%s", opts.DBType),
		fmt.Sprintf("--chain-config-dir=%s/configs/chains", baseDir),
		fmt.Sprintf("--staking-tls-cert-file=%s/staker.crt", keysDir),
		fmt.Sprintf("--staking-tls-key-file=%s/staker.key", keysDir),
		fmt.Sprintf("--staking-signer-key-file=%s/signer.key", keysDir),
		"--http-host=0.0.0.0",
		fmt.Sprintf("--http-port=%d", opts.HTTPPort),
		fmt.Sprintf("--staking-port=%d", opts.StakingPort),
		fmt.Sprintf("--log-level=%s", opts.LogLevel),
		"--api-admin-enabled=true",
		"--sybil-protection-enabled=true",
		"--consensus-sample-size=1",
		"--consensus-quorum-size=1",
		"--consensus-commit-threshold=1",
		"--consensus-concurrent-repolls=1",
		"--consensus-optimal-processing=1",
		"--consensus-max-processing=1",
		"--consensus-max-time-processing=2s",
		"--bootstrap-beacon-connection-timeout=10s",
		"--health-check-frequency=2s",
		"--network-max-reconnect-delay=1s",
	)
	
	// Add genesis database if specified
	if opts.GenesisDB != "" {
		cmd.Args = append(cmd.Args,
			fmt.Sprintf("--genesis-db=%s", opts.GenesisDB),
			fmt.Sprintf("--genesis-db-type=%s", opts.GenesisDBType),
		)
	}
	
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	
	return cmd.Run()
}

func (r *SimpleReplayRunner) getNetworkID(network string) uint32 {
	switch network {
	case "mainnet":
		return 96369
	case "testnet":
		return 96368
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