package launcher

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// Config represents the configuration for launching a Lux node
type Config struct {
	BinaryPath    string
	DataDir       string
	NetworkID     uint32
	HTTPPort      uint16
	StakingPort   uint16
	SingleNode    bool
	GenesisPath   string
	ChainDataPath string
	LogLevel      string
	PublicIP      string
	NoBootstrap   bool
	BootstrapIPs  string
	BootstrapIDs  string
	StakingKey    string
	StakingCert   string
}

// NodeLauncher manages the lifecycle of a Lux node
type NodeLauncher struct {
	config  Config
	process *exec.Cmd
	logFile *os.File
}

// New creates a new NodeLauncher
func New(config Config) *NodeLauncher {
	return &NodeLauncher{
		config: config,
	}
}

// Start launches the Lux node
func (nl *NodeLauncher) Start() error {
	// Create data directory if it doesn't exist
	if err := os.MkdirAll(nl.config.DataDir, 0755); err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}

	// Copy genesis if provided
	if nl.config.GenesisPath != "" {
		if err := nl.copyGenesis(); err != nil {
			return fmt.Errorf("failed to copy genesis: %w", err)
		}
	}

	// Copy chain data if provided
	if nl.config.ChainDataPath != "" {
		if err := nl.copyChainData(); err != nil {
			return fmt.Errorf("failed to copy chain data: %w", err)
		}
	}

	// Prepare command arguments
	args := []string{
		fmt.Sprintf("--data-dir=%s", nl.config.DataDir),
		fmt.Sprintf("--network-id=%d", nl.config.NetworkID),
		fmt.Sprintf("--http-port=%d", nl.config.HTTPPort),
		fmt.Sprintf("--staking-port=%d", nl.config.StakingPort),
		fmt.Sprintf("--public-ip=%s", nl.config.PublicIP),
		"--http-host=0.0.0.0",
		fmt.Sprintf("--log-level=%s", nl.config.LogLevel),
		"--api-admin-enabled=true",
		"--api-metrics-enabled=true",
		"--index-enabled=true",
	}

	// Handle bootstrap configuration
	if nl.config.NoBootstrap || nl.config.SingleNode {
		args = append(args, "--bootstrap-ips=")
		args = append(args, "--bootstrap-ids=")
	} else if nl.config.BootstrapIPs != "" && nl.config.BootstrapIDs != "" {
		args = append(args, fmt.Sprintf("--bootstrap-ips=%s", nl.config.BootstrapIPs))
		args = append(args, fmt.Sprintf("--bootstrap-ids=%s", nl.config.BootstrapIDs))
	}

	// Handle staking keys
	if nl.config.StakingKey != "" && nl.config.StakingCert != "" {
		args = append(args, fmt.Sprintf("--staking-tls-key-file=%s", nl.config.StakingKey))
		args = append(args, fmt.Sprintf("--staking-tls-cert-file=%s", nl.config.StakingCert))
	}

	if nl.config.SingleNode {
		// For single validator mode, we need staking enabled
		// but with k=1 consensus parameters
		args = append(args,
			"--snow-sample-size=1",
			"--snow-quorum-size=1",
			"--snow-concurrent-repolls=1",
			"--network-minimum-staking-duration=24h",
			"--network-maximum-staking-duration=8760h",
			"--network-optimal-proposer-num-pchain-blocks=1",
			"--consensus-shutdown-timeout=1s",
		)
	}

	// Create log file
	logPath := filepath.Join(nl.config.DataDir, "luxd.log")
	logFile, err := os.Create(logPath)
	if err != nil {
		return fmt.Errorf("failed to create log file: %w", err)
	}
	nl.logFile = logFile

	// Create the command
	nl.process = exec.Command(nl.config.BinaryPath, args...)
	nl.process.Stdout = logFile
	nl.process.Stderr = logFile

	// Start the process
	if err := nl.process.Start(); err != nil {
		logFile.Close()
		return fmt.Errorf("failed to start node: %w", err)
	}

	fmt.Printf("Node started with PID %d\n", nl.process.Process.Pid)
	fmt.Printf("Logs: %s\n", logPath)

	return nil
}

// Stop gracefully stops the node
func (nl *NodeLauncher) Stop() error {
	if nl.process == nil || nl.process.Process == nil {
		return nil
	}

	// Send interrupt signal
	if err := nl.process.Process.Signal(os.Interrupt); err != nil {
		return fmt.Errorf("failed to send interrupt signal: %w", err)
	}

	// Wait for process to exit with timeout
	done := make(chan error, 1)
	go func() {
		done <- nl.process.Wait()
	}()

	select {
	case <-time.After(30 * time.Second):
		// Force kill if doesn't exit gracefully
		_ = nl.process.Process.Kill()
		return fmt.Errorf("node did not exit gracefully, killed")
	case err := <-done:
		if nl.logFile != nil {
			nl.logFile.Close()
		}
		return err
	}
}

// Wait blocks until the node exits
func (nl *NodeLauncher) Wait() error {
	if nl.process == nil {
		return fmt.Errorf("node not started")
	}
	return nl.process.Wait()
}

// copyGenesis copies the genesis file to the appropriate location
func (nl *NodeLauncher) copyGenesis() error {
	genesisDir := filepath.Join(nl.config.DataDir, "configs", "genesis")
	if err := os.MkdirAll(genesisDir, 0755); err != nil {
		return err
	}

	src, err := os.Open(nl.config.GenesisPath)
	if err != nil {
		return err
	}
	defer src.Close()

	dst, err := os.Create(filepath.Join(genesisDir, "genesis.json"))
	if err != nil {
		return err
	}
	defer dst.Close()

	_, err = io.Copy(dst, src)
	return err
}

// copyChainData copies the C-chain data for replay
func (nl *NodeLauncher) copyChainData() error {
	cChainDir := filepath.Join(nl.config.DataDir, "db", "c-chain")
	if err := os.MkdirAll(cChainDir, 0755); err != nil {
		return err
	}

	// Use cp -r for efficient directory copy
	cmd := exec.Command("cp", "-r", nl.config.ChainDataPath, cChainDir)
	return cmd.Run()
}

// GenerateSingleValidatorGenesis creates a genesis for single validator node
func GenerateSingleValidatorGenesis(networkID uint32) (string, error) {
	// Use a known test address for single validator setup
	address := "0x9011e888251ab053b7bd1cdb598db4f9ded94714"
	nodeID := "NodeID-7Xhw2mDxuDS44j42TCB6U5579esbSt3Lg"
	
	genesis := map[string]interface{}{
		"networkID": networkID,
		"allocations": []map[string]interface{}{
			{
				"ethAddr":       address,
				"avaxAddr":      "P-lux1qsrd262r5w9dswv2wwzj0un79ncpwvdgkpqzqu",
				"initialAmount": 3000000000000000000, // 3 billion LUX
				"unlockSchedule": []map[string]interface{}{
					{
						"amount":   3000000000000000000,
						"locktime": 0,
					},
				},
			},
		},
		"startTime":                  uint64(time.Now().Unix() - 60), // 1 minute ago
		"initialStakeDuration":       365 * 24 * 60 * 60,             // 1 year
		"initialStakeDurationOffset": 5 * 60,                         // 5 minutes
		"initialStakedFunds": []string{
			"P-lux1qsrd262r5w9dswv2wwzj0un79ncpwvdgkpqzqu",
		},
		"initialStakers": []map[string]interface{}{
			{
				"nodeID":        nodeID,
				"rewardAddress": "P-lux1qsrd262r5w9dswv2wwzj0un79ncpwvdgkpqzqu",
				"delegationFee": 20000, // 2%
			},
		},
		"cChainGenesis": createCChainGenesis(networkID, address),
		"message":       fmt.Sprintf("single validator lux network %d", networkID),
	}

	// Write to temp file
	tmpFile, err := os.CreateTemp("", "genesis-single-*.json")
	if err != nil {
		return "", err
	}

	genesisJSON, _ := json.MarshalIndent(genesis, "", "  ")
	if _, err := tmpFile.Write(genesisJSON); err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		return "", err
	}
	tmpFile.Close()

	return tmpFile.Name(), nil
}

// GenerateStakingKeys creates a new staking key pair for validator
func GenerateStakingKeys(dataDir string) (keyPath, certPath string, err error) {
	stakingDir := filepath.Join(dataDir, "staking")
	if err := os.MkdirAll(stakingDir, 0700); err != nil {
		return "", "", fmt.Errorf("failed to create staking directory: %w", err)
	}

	keyPath = filepath.Join(stakingDir, "staker.key")
	certPath = filepath.Join(stakingDir, "staker.crt")

	// Generate using openssl or similar tool
	cmd := exec.Command("openssl", "req", "-x509", "-newkey", "rsa:4096",
		"-keyout", keyPath,
		"-out", certPath,
		"-days", "365",
		"-nodes",
		"-subj", "/C=US/ST=State/L=City/O=Organization/CN=validator")
	
	if output, err := cmd.CombinedOutput(); err != nil {
		return "", "", fmt.Errorf("failed to generate staking keys: %w\nOutput: %s", err, output)
	}

	// Set proper permissions
	if err := os.Chmod(keyPath, 0600); err != nil {
		return "", "", fmt.Errorf("failed to set key permissions: %w", err)
	}

	return keyPath, certPath, nil
}

// GenerateMinimalGenesis creates a minimal genesis for single node testing (deprecated, use GenerateSingleValidatorGenesis)
func GenerateMinimalGenesis(networkID uint32, address string) (string, error) {
	genesis := map[string]interface{}{
		"networkID": networkID,
		"allocations": []map[string]interface{}{
			{
				"ethAddr":       address,
				"avaxAddr":      "P-lux1qsrd262r5w9dswv2wwzj0un79ncpwvdgkpqzqu",
				"initialAmount": 1000000000000000000, // 1 billion
				"unlockSchedule": []map[string]interface{}{
					{
						"amount":   1000000000000000000,
						"locktime": 0,
					},
				},
			},
		},
		"startTime":                  uint64(time.Date(2022, 4, 15, 0, 0, 0, 0, time.UTC).Unix()),
		"initialStakeDuration":       365 * 24 * 60 * 60, // 1 year
		"initialStakeDurationOffset": 90 * 60,            // 90 minutes
		"initialStakedFunds": []string{
			"P-lux1qsrd262r5w9dswv2wwzj0un79ncpwvdgkpqzqu",
		},
		"initialStakers": []map[string]interface{}{
			{
				"nodeID":        "NodeID-7Xhw2mDxuDS44j42TCB6U5579esbSt3Lg",
				"rewardAddress": "P-lux1qsrd262r5w9dswv2wwzj0un79ncpwvdgkpqzqu",
				"delegationFee": 20000,
			},
		},
		"cChainGenesis": createCChainGenesis(networkID, address),
		"message":       fmt.Sprintf("lux network %d", networkID),
	}

	// Write to temp file
	tmpFile, err := os.CreateTemp("", "genesis-*.json")
	if err != nil {
		return "", err
	}

	genesisJSON, _ := json.MarshalIndent(genesis, "", "  ")
	if _, err := tmpFile.Write(genesisJSON); err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		return "", err
	}
	tmpFile.Close()

	return tmpFile.Name(), nil
}

func createCChainGenesis(chainID uint32, address string) string {
	cGenesis := map[string]interface{}{
		"config": map[string]interface{}{
			"chainId":                     chainID,
			"homesteadBlock":              0,
			"eip150Block":                 0,
			"eip155Block":                 0,
			"eip158Block":                 0,
			"byzantiumBlock":              0,
			"constantinopleBlock":         0,
			"petersburgBlock":             0,
			"istanbulBlock":               0,
			"muirGlacierBlock":            0,
			"apricotPhase1BlockTimestamp": 0,
			"apricotPhase2BlockTimestamp": 0,
			"apricotPhase3BlockTimestamp": 0,
			"apricotPhase4BlockTimestamp": 0,
			"apricotPhase5BlockTimestamp": 0,
		},
		"nonce":      "0x0",
		"timestamp":  "0x0",
		"extraData":  "0x00",
		"gasLimit":   "0x5f5e100",
		"difficulty": "0x0",
		"mixHash":    "0x0000000000000000000000000000000000000000000000000000000000000000",
		"coinbase":   "0x0000000000000000000000000000000000000000",
		"alloc": map[string]interface{}{
			address: map[string]string{
				"balance": "0x21e19e0c9bab2400000", // 10k ETH
			},
		},
		"number":     "0x0",
		"gasUsed":    "0x0",
		"parentHash": "0x0000000000000000000000000000000000000000000000000000000000000000",
	}

	cGenesisJSON, _ := json.Marshal(cGenesis)
	return string(cGenesisJSON)
}
