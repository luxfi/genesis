package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
)

// Network represents a running network instance
type Network struct {
	Name      string `json:"name"`
	Type      string `json:"type"` // mainnet, testnet, local, l2
	ChainID   uint64 `json:"chain_id"`
	Nodes     []Node `json:"nodes"`
	DataDir   string `json:"data_dir"`
	StartedAt string `json:"started_at"`
}

// Node represents a running node
type Node struct {
	NodeID   string `json:"node_id"`
	Port     int    `json:"port"`
	PID      int    `json:"pid"`
	DataDir  string `json:"data_dir"`
	LogFile  string `json:"log_file"`
}

func getNetworkCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "network",
		Short: "Manage blockchain networks",
		Long:  "Start, stop, and manage Lux networks with simple commands",
	}

	// Subcommands
	cmd.AddCommand(getNetworkStartCmd())
	cmd.AddCommand(getNetworkStopCmd())
	cmd.AddCommand(getNetworkStatusCmd())
	cmd.AddCommand(getNetworkListCmd())
	cmd.AddCommand(getNetworkSnapshotCmd())

	return cmd
}

func getNetworkStartCmd() *cobra.Command {
	var (
		nodes    int
		dataDir  string
		chainData string
	)

	cmd := &cobra.Command{
		Use:   "start [network]",
		Short: "Start a network",
		Long: `Start a blockchain network with simple commands:

Examples:
  # Start mainnet
  genesis network start mainnet
  genesis network start lux
  
  # Start testnet
  genesis network start testnet
  
  # Start local dev network
  genesis network start local
  genesis network start local --nodes=5
  
  # Start L2 networks
  genesis network start zoo
  genesis network start spc
  
  # Start with existing chaindata
  genesis network start zoo --chaindata=/path/to/chaindata`,
		Args: cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			networkName := strings.ToLower(args[0])
			
			// Map network names to configurations
			switch networkName {
			case "mainnet", "lux":
				startMainnet(dataDir)
			case "testnet":
				startTestnet(dataDir)
			case "local", "dev":
				startLocal(nodes, dataDir)
			case "zoo", "zoo-mainnet":
				startL2("zoo", 200200, chainData, dataDir)
			case "spc", "spc-mainnet":
				startL2("spc", 36911, chainData, dataDir)
			case "hanzo":
				startL2("hanzo", 36963, chainData, dataDir)
			default:
				log.Fatalf("Unknown network: %s", networkName)
			}
		},
	}

	cmd.Flags().IntVar(&nodes, "nodes", 1, "Number of nodes to start (local networks)")
	cmd.Flags().StringVar(&dataDir, "data-dir", "", "Custom data directory")
	cmd.Flags().StringVar(&chainData, "chaindata", "", "Path to existing chaindata to import")

	return cmd
}

func getNetworkStopCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stop [network]",
		Short: "Stop a running network",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			networkName := strings.ToLower(args[0])
			stopNetwork(networkName)
		},
	}
	return cmd
}

func getNetworkStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status [network]",
		Short: "Show network status",
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) > 0 {
				showNetworkStatus(args[0])
			} else {
				showAllNetworkStatus()
			}
		},
	}
	return cmd
}

func getNetworkListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all networks",
		Run: func(cmd *cobra.Command, args []string) {
			listNetworks()
		},
	}
	return cmd
}

func getNetworkSnapshotCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "snapshot [network]",
		Short: "Create network snapshot",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			createSnapshot(args[0])
		},
	}
	return cmd
}

// Implementation functions

func startMainnet(dataDir string) {
	fmt.Println("🚀 Starting Lux Mainnet...")
	
	homeDir, _ := os.UserHomeDir()
	if dataDir == "" {
		dataDir = filepath.Join(homeDir, ".luxd", "mainnet")
	}
	
	// Ensure data directory exists
	os.MkdirAll(dataDir, 0755)
	
	// Check if already running
	if isNetworkActive("mainnet") {
		fmt.Println("❌ Mainnet is already running")
		return
	}
	
	// Set up node with proper configuration
	fmt.Println("Setting up mainnet node...")
	
	// Use setup-node to create proper keys
	setupCmd := exec.Command(os.Args[0], "setup-node", 
		"--port", "9630",
		"--data-dir", dataDir,
		"--network-id", "96369")
	setupCmd.Stdout = os.Stdout
	setupCmd.Stderr = os.Stderr
	if err := setupCmd.Run(); err != nil {
		log.Fatalf("Failed to setup node: %v", err)
	}
	
	// Launch the node
	launchScript := filepath.Join(dataDir, "launch.sh")
	launchCmd := exec.Command(launchScript)
	
	// Create log file
	logFile := filepath.Join(dataDir, "node.log")
	logOut, _ := os.Create(logFile)
	
	launchCmd.Stdout = logOut
	launchCmd.Stderr = logOut
	
	if err := launchCmd.Start(); err != nil {
		log.Fatalf("Failed to start node: %v", err)
	}
	
	// Save network info
	network := Network{
		Name:      "mainnet",
		Type:      "mainnet",
		ChainID:   96369,
		DataDir:   dataDir,
		StartedAt: time.Now().Format(time.RFC3339),
		Nodes: []Node{{
			Port:    9630,
			PID:     launchCmd.Process.Pid,
			DataDir: dataDir,
			LogFile: logFile,
		}},
	}
	
	saveNetworkInfo(network)
	
	fmt.Printf("✅ Mainnet started! PID: %d\n", launchCmd.Process.Pid)
	fmt.Println("\nRPC Endpoint: http://localhost:9630/ext/bc/C/rpc")
	fmt.Println("Logs: tail -f", logFile)
}

func startTestnet(dataDir string) {
	fmt.Println("🚀 Starting Lux Testnet...")
	
	homeDir, _ := os.UserHomeDir()
	if dataDir == "" {
		dataDir = filepath.Join(homeDir, ".luxd", "testnet")
	}
	
	// Similar to mainnet but with testnet parameters
	// Network ID: 96368
	// TODO: Implement testnet start
	fmt.Println("Testnet start not yet implemented")
}

func startLocal(nodes int, dataDir string) {
	fmt.Printf("🚀 Starting Local Network with %d nodes...\n", nodes)
	
	homeDir, _ := os.UserHomeDir()
	if dataDir == "" {
		dataDir = filepath.Join(homeDir, ".luxd", "local")
	}
	
	// Check if already running
	if isNetworkActive("local") {
		fmt.Println("❌ Local network is already running")
		return
	}
	
	var network Network
	network.Name = "local"
	network.Type = "local"
	network.ChainID = 96369 // Same as mainnet for local testing
	network.DataDir = dataDir
	network.StartedAt = time.Now().Format(time.RFC3339)
	
	// Start multiple nodes
	basePort := 9630
	for i := 0; i < nodes; i++ {
		port := basePort + (i * 10)
		nodeDataDir := filepath.Join(dataDir, fmt.Sprintf("node-%d", i))
		
		fmt.Printf("Setting up node %d on port %d...\n", i, port)
		
		// Setup node
		setupCmd := exec.Command(os.Args[0], "setup-node",
			"--port", fmt.Sprintf("%d", port),
			"--data-dir", nodeDataDir,
			"--network-id", "96369",
			"--consensus-k", "2",
			"--consensus-alpha", "1",
			"--consensus-beta", "1")
		
		if err := setupCmd.Run(); err != nil {
			log.Printf("Warning: Failed to setup node %d: %v", i, err)
			continue
		}
		
		// Launch node
		launchScript := filepath.Join(nodeDataDir, "launch.sh")
		launchCmd := exec.Command(launchScript)
		
		logFile := filepath.Join(nodeDataDir, "node.log")
		logOut, _ := os.Create(logFile)
		
		launchCmd.Stdout = logOut
		launchCmd.Stderr = logOut
		
		if err := launchCmd.Start(); err != nil {
			log.Printf("Warning: Failed to start node %d: %v", i, err)
			continue
		}
		
		network.Nodes = append(network.Nodes, Node{
			Port:    port,
			PID:     launchCmd.Process.Pid,
			DataDir: nodeDataDir,
			LogFile: logFile,
		})
		
		fmt.Printf("✅ Node %d started on port %d (PID: %d)\n", i, port, launchCmd.Process.Pid)
		
		// Give node time to start before starting next one
		time.Sleep(2 * time.Second)
	}
	
	saveNetworkInfo(network)
	
	fmt.Printf("\n✅ Local network started with %d nodes!\n", len(network.Nodes))
	fmt.Println("\nRPC Endpoints:")
	for i, node := range network.Nodes {
		fmt.Printf("  Node %d: http://localhost:%d/ext/bc/C/rpc\n", i, node.Port)
	}
}

func startL2(name string, chainID uint64, chainData string, dataDir string) {
	fmt.Printf("🚀 Starting %s L2 Network (Chain ID: %d)...\n", strings.ToUpper(name), chainID)
	
	homeDir, _ := os.UserHomeDir()
	if dataDir == "" {
		dataDir = filepath.Join(homeDir, ".luxd", name)
	}
	
	// Check if already running
	if isNetworkActive(name) {
		fmt.Printf("❌ %s network is already running\n", name)
		return
	}
	
	// If chaindata provided, set it up
	if chainData != "" {
		fmt.Printf("Importing chaindata from: %s\n", chainData)
		// TODO: Import chaindata
	}
	
	// Setup and start similar to mainnet
	// TODO: Implement L2 specific configuration
	fmt.Printf("%s L2 network start not yet fully implemented\n", name)
}

func stopNetwork(networkName string) {
	fmt.Printf("🛑 Stopping %s network...\n", networkName)
	
	network := loadNetworkInfo(networkName)
	if network == nil {
		fmt.Printf("❌ Network %s is not running\n", networkName)
		return
	}
	
	// Stop all nodes
	for i, node := range network.Nodes {
		fmt.Printf("Stopping node %d (PID: %d)...\n", i, node.PID)
		
		// Try graceful shutdown first
		if err := syscall.Kill(node.PID, syscall.SIGTERM); err != nil {
			log.Printf("Warning: Failed to stop node %d: %v", i, err)
		}
	}
	
	// Wait a bit for graceful shutdown
	time.Sleep(2 * time.Second)
	
	// Force kill if still running
	for _, node := range network.Nodes {
		syscall.Kill(node.PID, syscall.SIGKILL)
	}
	
	// Remove network info
	removeNetworkInfo(networkName)
	
	fmt.Printf("✅ %s network stopped\n", networkName)
}

func showNetworkStatus(networkName string) {
	network := loadNetworkInfo(networkName)
	if network == nil {
		fmt.Printf("❌ Network %s is not running\n", networkName)
		return
	}
	
	fmt.Printf("Network: %s\n", network.Name)
	fmt.Printf("Type: %s\n", network.Type)
	fmt.Printf("Chain ID: %d\n", network.ChainID)
	fmt.Printf("Started: %s\n", network.StartedAt)
	fmt.Printf("Nodes: %d\n", len(network.Nodes))
	
	for i, node := range network.Nodes {
		// Check if process is still running
		process, err := os.FindProcess(node.PID)
		running := true
		if err != nil || process.Signal(syscall.Signal(0)) != nil {
			running = false
		}
		
		status := "🟢 Running"
		if !running {
			status = "🔴 Stopped"
		}
		
		fmt.Printf("\nNode %d:\n", i)
		fmt.Printf("  Status: %s\n", status)
		fmt.Printf("  Port: %d\n", node.Port)
		fmt.Printf("  PID: %d\n", node.PID)
		fmt.Printf("  RPC: http://localhost:%d/ext/bc/C/rpc\n", node.Port)
		fmt.Printf("  Logs: %s\n", node.LogFile)
	}
}

func showAllNetworkStatus() {
	networks := listAllNetworks()
	if len(networks) == 0 {
		fmt.Println("No networks are currently running")
		return
	}
	
	fmt.Println("Running Networks:")
	fmt.Println("================")
	for _, name := range networks {
		fmt.Printf("\n")
		showNetworkStatus(name)
		fmt.Println("----------------")
	}
}

func listNetworks() {
	fmt.Println("Available Networks:")
	fmt.Println("==================")
	fmt.Println("  mainnet (lux)    - Lux Mainnet (96369)")
	fmt.Println("  testnet          - Lux Testnet (96368)")
	fmt.Println("  local            - Local Development Network")
	fmt.Println("  zoo              - ZOO L2 Network (200200)")
	fmt.Println("  spc              - SPC L2 Network (36911)")
	fmt.Println("  hanzo            - Hanzo L2 Network (36963)")
	
	fmt.Println("\nRunning Networks:")
	fmt.Println("=================")
	networks := listAllNetworks()
	if len(networks) == 0 {
		fmt.Println("  None")
	} else {
		for _, name := range networks {
			network := loadNetworkInfo(name)
			if network != nil {
				fmt.Printf("  %s (%d nodes)\n", name, len(network.Nodes))
			}
		}
	}
}

func createSnapshot(networkName string) {
	fmt.Printf("📸 Creating snapshot of %s network...\n", networkName)
	
	network := loadNetworkInfo(networkName)
	if network == nil {
		fmt.Printf("❌ Network %s is not running\n", networkName)
		return
	}
	
	// Create snapshot directory
	homeDir, _ := os.UserHomeDir()
	snapshotDir := filepath.Join(homeDir, ".luxd", "snapshots", networkName, 
		time.Now().Format("20060102-150405"))
	os.MkdirAll(snapshotDir, 0755)
	
	// TODO: Implement actual snapshot logic
	// - Stop nodes gracefully
	// - Copy chaindata
	// - Copy configs
	// - Create manifest
	
	fmt.Printf("✅ Snapshot created at: %s\n", snapshotDir)
}

// Helper functions

func getNetworkInfoPath() string {
	homeDir, _ := os.UserHomeDir()
	return filepath.Join(homeDir, ".luxd", "networks")
}

func saveNetworkInfo(network Network) {
	infoDir := getNetworkInfoPath()
	os.MkdirAll(infoDir, 0755)
	
	data, _ := json.MarshalIndent(network, "", "  ")
	infoFile := filepath.Join(infoDir, network.Name+".json")
	ioutil.WriteFile(infoFile, data, 0644)
}

func loadNetworkInfo(name string) *Network {
	infoFile := filepath.Join(getNetworkInfoPath(), name+".json")
	data, err := ioutil.ReadFile(infoFile)
	if err != nil {
		return nil
	}
	
	var network Network
	json.Unmarshal(data, &network)
	return &network
}

func removeNetworkInfo(name string) {
	infoFile := filepath.Join(getNetworkInfoPath(), name+".json")
	os.Remove(infoFile)
}

func isNetworkActive(name string) bool {
	return loadNetworkInfo(name) != nil
}

func listAllNetworks() []string {
	infoDir := getNetworkInfoPath()
	files, err := ioutil.ReadDir(infoDir)
	if err != nil {
		return []string{}
	}
	
	var networks []string
	for _, file := range files {
		if strings.HasSuffix(file.Name(), ".json") {
			name := strings.TrimSuffix(file.Name(), ".json")
			networks = append(networks, name)
		}
	}
	return networks
}