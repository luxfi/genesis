package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/luxfi/genesis/pkg/application"
	"github.com/spf13/cobra"
)

func newLaunchBFTCmd(app *application.Genesis) *cobra.Command {
	var (
		dataDir     string
		httpPort    int
		stakingPort int
		logLevel    string
		simple      bool
	)

	cmd := &cobra.Command{
		Use:   "bft",
		Short: "Launch a BFT network",
		Long:  "Launch a Lux node with Byzantine Fault Tolerant consensus using a single validator.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if simple {
				return launchBFTSimple(app, dataDir, httpPort, stakingPort, logLevel)
			}
			return launchBFT(app, dataDir, httpPort, stakingPort, logLevel)
		},
	}

	cmd.Flags().StringVar(&dataDir, "data-dir", "", "Data directory (auto-generated if empty)")
	cmd.Flags().IntVar(&httpPort, "http-port", 9630, "HTTP API port")
	cmd.Flags().IntVar(&stakingPort, "staking-port", 9631, "Staking port")
	cmd.Flags().StringVar(&logLevel, "log-level", "info", "Log level")
	cmd.Flags().BoolVar(&simple, "simple", false, "Use simplified genesis and launch")

	return cmd
}

func launchBFT(app *application.Genesis, dataDir string, httpPort, stakingPort int, logLevel string) error {
	// Setup directories
	if dataDir == "" {
		dataDir = filepath.Join("runs", fmt.Sprintf("bft-%d", time.Now().Unix()))
	}
	stakingDir := filepath.Join(dataDir, "staking")

	// Create directories
	if err := os.MkdirAll(stakingDir, 0755); err != nil {
		return fmt.Errorf("failed to create directories: %w", err)
	}

	fmt.Println("=== Lux BFT Node Launcher ===")
	fmt.Println("Launching with Byzantine Fault Tolerant consensus (k=1)")
	fmt.Println()

	// Step 1: Generate BLS keys
	fmt.Println("Step 1: Generating BLS keys...")
	keygenCmd := exec.Command("./bin/genesis", "staking", "keygen", "--output", stakingDir)
	keygenOutput, err := keygenCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to generate keys: %w\n%s", err, keygenOutput)
	}
	fmt.Print(string(keygenOutput))

	// Read staker info
	stakerData, err := os.ReadFile(filepath.Join(stakingDir, "genesis-staker.json"))
	if err != nil {
		return fmt.Errorf("failed to read staker info: %w", err)
	}

	var staker map[string]interface{}
	if err := json.Unmarshal(stakerData, &staker); err != nil {
		return fmt.Errorf("failed to parse staker info: %w", err)
	}

	// Step 2: Create genesis
	fmt.Println("\nStep 2: Creating genesis configuration...")
	genesis := map[string]interface{}{
		"networkID": 96369,
		"allocations": []map[string]interface{}{
			{
				"ethAddr":        "0x8db97C7cEcE249c2b98bDC0226Cc4C2A57BF52FC",
				"luxAddr":        "X-lux13kuhcl8vufyu9wvtmspzdnzv9ftm75huejyuac",
				"initialAmount":  300000000000000000,
				"unlockSchedule": []interface{}{},
			},
		},
		"startTime":                  time.Now().Unix(),
		"initialStakeDuration":       31536000,
		"initialStakeDurationOffset": 5400,
		"initialStakedFunds":         []string{"P-lux13kuhcl8vufyu9wvtmspzdnzv9ftm75huejyuac"},
		"initialStakers":             []interface{}{staker},
		"cChainGenesis": `{"config":{"chainId":96369,"homesteadBlock":0,"eip150Block":0,"eip150Hash":"0x2086799aeebeae135c246c65021c82b4e15a2c451340993aacfd2751886514f0","eip155Block":0,"eip158Block":0,"byzantiumBlock":0,"constantinopleBlock":0,"petersburgBlock":0,"istanbulBlock":0,"muirGlacierBlock":0,"berlinBlock":0,"londonBlock":0},"nonce":"0x0","timestamp":"0x0","extraData":"0x00","gasLimit":"0x5f5e100","difficulty":"0x0","mixHash":"0x0000000000000000000000000000000000000000000000000000000000000000","coinbase":"0x0000000000000000000000000000000000000000","alloc":{"8db97C7cEcE249c2b98bDC0226Cc4C2A57BF52FC":{"balance":"0x295BE96E64066972000000"}},"number":"0x0","gasUsed":"0x0,"parentHash":"0x0000000000000000000000000000000000000000000000000000000000000000"}`,
		"message": "Lux BFT Network",
	}

	genesisPath := filepath.Join(dataDir, "genesis.json")
	genesisData, err := json.MarshalIndent(genesis, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal genesis: %w", err)
	}

	if err := os.WriteFile(genesisPath, genesisData, 0644); err != nil {
		return fmt.Errorf("failed to write genesis: %w", err)
	}

	fmt.Printf("Genesis created at: %s\n", genesisPath)

	// Step 3: Validate consensus parameters
	fmt.Println("\nStep 3: Validating consensus parameters...")
	validateCmd := exec.Command("./bin/genesis", "consensus", "validate",
		"--k", "1",
		"--alpha-preference", "1",
		"--alpha-confidence", "1",
		"--beta", "20",
		"--concurrent-repolls", "1",
	)
	validateOutput, err := validateCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("consensus validation failed: %w\n%s", err, validateOutput)
	}
	fmt.Print(string(validateOutput))

	// Check if luxd exists
	luxdPath := filepath.Join("..", "node", "build", "luxd")
	if _, err := os.Stat(luxdPath); err != nil {
		fmt.Println("\n⚠️  Building luxd...")
		buildCmd := exec.Command("make", "build")
		buildCmd.Dir = "../node"
		if err := buildCmd.Run(); err != nil {
			return fmt.Errorf("failed to build luxd: %w", err)
		}
	}

	// Step 4: Launch node
	fmt.Println("\nStep 4: Launching BFT node...")
	fmt.Println("Configuration:")
	fmt.Printf("  Network ID: %s\n", "96369")
	fmt.Printf("  Data Directory: %s\n", dataDir)
	fmt.Printf("  HTTP Port: %d\n", httpPort)
	fmt.Printf("  Staking Port: %d\n", stakingPort)
	fmt.Println("  Consensus: k=1 (BFT with BLS)")
	fmt.Printf("  RPC Endpoint: http://localhost:%d/ext/bc/C/rpc\n", httpPort)
	fmt.Println()

	luxdArgs := []string{
		fmt.Sprintf("--network-id=%s", "96369"),
		fmt.Sprintf("--data-dir=%s", dataDir),
		// Don't override built-in genesis
		// fmt.Sprintf("--genesis-file=%s", genesisPath),
		fmt.Sprintf("--staking-tls-cert-file=%s", filepath.Join(stakingDir, "staker.crt")),
		fmt.Sprintf("--staking-tls-key-file=%s", filepath.Join(stakingDir, "staker.key")),
		fmt.Sprintf("--staking-signer-key-file=%s", filepath.Join(stakingDir, "signer.key")),
		"--db-type=badgerdb",
		"--http-host=0.0.0.0",
		fmt.Sprintf("--http-port=%d", httpPort),
		fmt.Sprintf("--staking-port=%d", stakingPort),
		fmt.Sprintf("--log-level=%s", logLevel),
		"--api-admin-enabled=true",
		"--consensus-sample-size=1",
		"--consensus-quorum-size=1",
		"--consensus-commit-threshold=1",
		"--consensus-concurrent-repolls=1",
		"--consensus-optimal-processing=1",
		"--consensus-max-processing=1",
		"--consensus-max-time-processing=2s",
	}

	luxdCmd := exec.Command(luxdPath, luxdArgs...)
	luxdCmd.Stdout = os.Stdout
	luxdCmd.Stderr = os.Stderr
	luxdCmd.Stdin = os.Stdin

	fmt.Println("Starting luxd with BFT consensus...")
	return luxdCmd.Run()
}

func launchBFTSimple(app *application.Genesis, dataDir string, httpPort, stakingPort int, logLevel string) error {
	// Setup directories
	if dataDir == "" {
		dataDir = filepath.Join("runs", fmt.Sprintf("bft-simple-%d", time.Now().Unix()))
	}
	stakingDir := filepath.Join(dataDir, "staking")

	// Create directories
	if err := os.MkdirAll(stakingDir, 0755); err != nil {
		return fmt.Errorf("failed to create directories: %w", err)
	}

	fmt.Println("=== Lux BFT Node Launcher (Simplified) ===")
	fmt.Println("Launching with Byzantine Fault Tolerant consensus (k=1)")
	fmt.Println()

	// Step 1: Generate BLS keys
	fmt.Println("Step 1: Generating BLS keys...")
	keygenCmd := exec.Command("./bin/genesis", "staking", "keygen", "--output", stakingDir)
	keygenOutput, err := keygenCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to generate keys: %w\n%s", err, keygenOutput)
	}
	fmt.Print(string(keygenOutput))

	// Step 2: Create simplified genesis using local network format
	fmt.Println("\nStep 2: Creating simplified genesis configuration...")

	// Use the local network genesis format which works for custom networks
	genesis := map[string]interface{}{
		"networkID": 96369,
		"allocations": []map[string]interface{}{
			{
				"ethAddr":        "0x8db97C7cEcE249c2b98bDC0226Cc4C2A57BF52FC",
				"luxAddr":        "X-local18jma8ppw3nhx5r4ap8clazz0dps7rv5u00z96u",
				"initialAmount":  300000000000000000,
				"unlockSchedule": []interface{}{},
			},
		},
		"startTime":                  time.Now().Unix(),
		"initialStakeDuration":       31536000,
		"initialStakeDurationOffset": 5400,
		"initialStakedFunds":         []string{"X-local18jma8ppw3nhx5r4ap8clazz0dps7rv5u00z96u"},
		"initialStakers": []map[string]interface{}{
			{
				"nodeID":        "NodeID-7Xhw2mDxuDS44j42TCB6U5579esbSt3Lg",
				"rewardAddress": "X-local18jma8ppw3nhx5r4ap8clazz0dps7rv5u00z96u",
				"delegationFee": 20000,
				"weight":        20000000000000,
			},
		},
		"cChainGenesis": `{"config":{"chainId":96369,"homesteadBlock":0,"eip150Block":0,"eip150Hash":"0x2086799aeebeae135c246c65021c82b4e15a2c451340993aacfd2751886514f0","eip155Block":0,"eip158Block":0,"byzantiumBlock":0,"constantinopleBlock":0,"petersburgBlock":0,"istanbulBlock":0,"muirGlacierBlock":0,"berlinBlock":0,"londonBlock":0},"nonce":"0x0","timestamp":"0x0","extraData":"0x00","gasLimit":"0x5f5e100","difficulty":"0x0","mixHash":"0x0000000000000000000000000000000000000000000000000000000000000000","coinbase":"0x0000000000000000000000000000000000000000","alloc":{"8db97C7cEcE249c2b98bDC0226Cc4C2A57BF52FC":{"balance":"0x295BE96E64066972000000"}},"number":"0x0","gasUsed":"0x0,"parentHash":"0x0000000000000000000000000000000000000000000000000000000000000000"}`,
		"message": "Lux BFT Network (Simplified)",
	}

	genesisPath := filepath.Join(dataDir, "genesis.json")
	genesisData, err := json.MarshalIndent(genesis, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal genesis: %w", err)
	}

	if err := os.WriteFile(genesisPath, genesisData, 0644); err != nil {
		return fmt.Errorf("failed to write genesis: %w", err)
	}

	fmt.Printf("Genesis created at: %s\n", genesisPath)

	// Check if luxd exists
	luxdPath := filepath.Join("..", "node", "build", "luxd")
	if _, err := os.Stat(luxdPath); err != nil {
		fmt.Println("\n⚠️  Building luxd...")
		buildCmd := exec.Command("make", "build")
		buildCmd.Dir = "../node"
		if err := buildCmd.Run(); err != nil {
			return fmt.Errorf("failed to build luxd: %w", err)
		}
	}

	// Step 3: Launch node without BLS
	fmt.Println("\nStep 3: Launching BFT node (simplified mode)...")
	fmt.Println("Configuration:")
	fmt.Printf("  Network ID: 96369\n")
	fmt.Printf("  Data Directory: %s\n", dataDir)
	fmt.Printf("  HTTP Port: %d\n", httpPort)
	fmt.Printf("  Staking Port: %d\n", stakingPort)
	fmt.Println("  Consensus: k=1 (simplified)")
	fmt.Printf("  RPC Endpoint: http://localhost:%d/ext/bc/C/rpc\n", httpPort)
	fmt.Println()

	luxdArgs := []string{
		"--dev",
		"--network-id=96369",
		fmt.Sprintf("--data-dir=%s", dataDir),
		fmt.Sprintf("--genesis-file=%s", genesisPath),
		fmt.Sprintf("--staking-tls-cert-file=%s", filepath.Join(stakingDir, "staker.crt")),
		fmt.Sprintf("--staking-tls-key-file=%s", filepath.Join(stakingDir, "staker.key")),
		"--db-type=badgerdb",
		"--http-host=0.0.0.0",
		fmt.Sprintf("--http-port=%d", httpPort),
		fmt.Sprintf("--staking-port=%d", stakingPort),
		fmt.Sprintf("--log-level=%s", logLevel),
		"--api-admin-enabled=true",
	}

	luxdCmd := exec.Command(luxdPath, luxdArgs...)
	luxdCmd.Stdout = os.Stdout
	luxdCmd.Stderr = os.Stderr
	luxdCmd.Stdin = os.Stdin

	fmt.Println("Starting luxd with simplified BFT consensus...")
	return luxdCmd.Run()
}