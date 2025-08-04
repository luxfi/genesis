package netrun

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

// NetworkManager manages multiple Lux networks in parallel
type NetworkManager struct {
	networks map[string]*Network
	mu       sync.RWMutex
	baseDir  string
}

// Network represents a managed Lux network
type Network struct {
	Name      string
	Config    NetworkConfig
	Nodes     []*Node
	Status    string
	StartTime time.Time
}

// Node represents a single node in a network
type Node struct {
	Name    string
	NodeID  string
	Process *exec.Cmd
	DataDir string
	RPC     string
	Status  string
}

// NetworkConfig defines the configuration for a network
type NetworkConfig struct {
	NetworkID    uint32
	NumNodes     int
	SingleNode   bool
	GenesisPath  string
	NodeConfigs  []NodeConfig
}

// NodeConfig defines per-node configuration
type NodeConfig struct {
	Name        string
	HTTPPort    uint16
	StakingPort uint16
	PublicIP    string
}

// NewManager creates a new NetworkManager
func NewManager(baseDir string) *NetworkManager {
	return &NetworkManager{
		networks: make(map[string]*Network),
		baseDir:  baseDir,
	}
}

// CreateNetwork creates a new network configuration
func (nm *NetworkManager) CreateNetwork(name string, config NetworkConfig) error {
	nm.mu.Lock()
	defer nm.mu.Unlock()

	if _, exists := nm.networks[name]; exists {
		return fmt.Errorf("network %s already exists", name)
	}

	// Generate node configs if not provided
	if len(config.NodeConfigs) == 0 {
		config.NodeConfigs = nm.generateNodeConfigs(config.NumNodes)
	}

	network := &Network{
		Name:   name,
		Config: config,
		Status: "created",
		Nodes:  make([]*Node, 0, len(config.NodeConfigs)),
	}

	// Create nodes
	for _, nodeConfig := range config.NodeConfigs {
		node := &Node{
			Name:    nodeConfig.Name,
			DataDir: filepath.Join(nm.baseDir, name, nodeConfig.Name),
			RPC:     fmt.Sprintf("http://localhost:%d", nodeConfig.HTTPPort),
			Status:  "initialized",
		}
		network.Nodes = append(network.Nodes, node)
	}

	nm.networks[name] = network
	return nil
}

// StartNetwork starts all nodes in a network
func (nm *NetworkManager) StartNetwork(name string) error {
	nm.mu.Lock()
	network, exists := nm.networks[name]
	nm.mu.Unlock()

	if !exists {
		return fmt.Errorf("network %s not found", name)
	}

	fmt.Printf("Starting network %s with %d nodes...\n", name, len(network.Nodes))
	
	// Start nodes in parallel
	var wg sync.WaitGroup
	errChan := make(chan error, len(network.Nodes))

	for i, node := range network.Nodes {
		wg.Add(1)
		go func(idx int, n *Node) {
			defer wg.Done()
			
			// Create node data directory
			if err := os.MkdirAll(n.DataDir, 0755); err != nil {
				errChan <- fmt.Errorf("node %s: failed to create data dir: %w", n.Name, err)
				return
			}

			// Copy genesis if provided
			if network.Config.GenesisPath != "" {
				if err := nm.copyGenesis(network.Config.GenesisPath, n.DataDir); err != nil {
					errChan <- fmt.Errorf("node %s: failed to copy genesis: %w", n.Name, err)
					return
				}
			}

			// Start the node
			nodeConfig := network.Config.NodeConfigs[idx]
			if err := nm.startNode(n, network.Config, nodeConfig); err != nil {
				errChan <- fmt.Errorf("node %s: failed to start: %w", n.Name, err)
				return
			}

			n.Status = "running"
			fmt.Printf("Node %s started on %s\n", n.Name, n.RPC)
		}(i, node)
	}

	wg.Wait()
	close(errChan)

	// Check for errors
	var errs []error
	for err := range errChan {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return fmt.Errorf("failed to start some nodes: %v", errs)
	}

	network.Status = "running"
	network.StartTime = time.Now()
	
	fmt.Printf("Network %s started successfully!\n", name)
	return nil
}

// StopNetwork stops all nodes in a network
func (nm *NetworkManager) StopNetwork(name string) error {
	nm.mu.Lock()
	network, exists := nm.networks[name]
	nm.mu.Unlock()

	if !exists {
		return fmt.Errorf("network %s not found", name)
	}

	fmt.Printf("Stopping network %s...\n", name)
	
	// Stop nodes in parallel
	var wg sync.WaitGroup
	for _, node := range network.Nodes {
		if node.Process == nil {
			continue
		}
		
		wg.Add(1)
		go func(n *Node) {
			defer wg.Done()
			
			// Send interrupt signal
			if err := n.Process.Process.Signal(os.Interrupt); err != nil {
				fmt.Printf("Failed to stop node %s gracefully: %v\n", n.Name, err)
				n.Process.Process.Kill()
			}
			
			// Wait for process to exit
			n.Process.Wait()
			n.Status = "stopped"
			fmt.Printf("Node %s stopped\n", n.Name)
		}(node)
	}

	wg.Wait()
	network.Status = "stopped"
	
	fmt.Printf("Network %s stopped\n", name)
	return nil
}

// ListNetworks returns information about all networks
func (nm *NetworkManager) ListNetworks() []NetworkInfo {
	nm.mu.RLock()
	defer nm.mu.RUnlock()

	var infos []NetworkInfo
	for _, network := range nm.networks {
		info := NetworkInfo{
			Name:      network.Name,
			NetworkID: network.Config.NetworkID,
			NumNodes:  len(network.Nodes),
			Status:    network.Status,
			StartTime: network.StartTime,
			Nodes:     make([]NodeInfo, 0, len(network.Nodes)),
		}

		for _, node := range network.Nodes {
			info.Nodes = append(info.Nodes, NodeInfo{
				Name:   node.Name,
				NodeID: node.NodeID,
				RPC:    node.RPC,
				Status: node.Status,
			})
		}

		infos = append(infos, info)
	}

	return infos
}

// GetNetworkStatus returns detailed status of a network
func (nm *NetworkManager) GetNetworkStatus(name string) (*NetworkInfo, error) {
	nm.mu.RLock()
	defer nm.mu.RUnlock()

	network, exists := nm.networks[name]
	if !exists {
		return nil, fmt.Errorf("network %s not found", name)
	}

	info := &NetworkInfo{
		Name:      network.Name,
		NetworkID: network.Config.NetworkID,
		NumNodes:  len(network.Nodes),
		Status:    network.Status,
		StartTime: network.StartTime,
		Nodes:     make([]NodeInfo, 0, len(network.Nodes)),
	}

	for _, node := range network.Nodes {
		info.Nodes = append(info.Nodes, NodeInfo{
			Name:   node.Name,
			NodeID: node.NodeID,
			RPC:    node.RPC,
			Status: node.Status,
		})
	}

	return info, nil
}

// NetworkInfo provides information about a network
type NetworkInfo struct {
	Name      string     `json:"name"`
	NetworkID uint32     `json:"networkId"`
	NumNodes  int        `json:"numNodes"`
	Status    string     `json:"status"`
	StartTime time.Time  `json:"startTime"`
	Nodes     []NodeInfo `json:"nodes"`
}

// NodeInfo provides information about a node
type NodeInfo struct {
	Name   string `json:"name"`
	NodeID string `json:"nodeId"`
	RPC    string `json:"rpc"`
	Status string `json:"status"`
}

// generateNodeConfigs creates default node configurations
func (nm *NetworkManager) generateNodeConfigs(numNodes int) []NodeConfig {
	configs := make([]NodeConfig, numNodes)
	baseHTTPPort := uint16(9630)
	baseStakingPort := uint16(9631)

	for i := 0; i < numNodes; i++ {
		configs[i] = NodeConfig{
			Name:        fmt.Sprintf("node%d", i+1),
			HTTPPort:    baseHTTPPort + uint16(i*10),
			StakingPort: baseStakingPort + uint16(i*10),
			PublicIP:    "127.0.0.1",
		}
	}

	return configs
}

// startNode starts a single node
func (nm *NetworkManager) startNode(node *Node, netConfig NetworkConfig, nodeConfig NodeConfig) error {
	// Build command arguments
	args := []string{
		fmt.Sprintf("--data-dir=%s", node.DataDir),
		fmt.Sprintf("--network-id=%d", netConfig.NetworkID),
		fmt.Sprintf("--http-port=%d", nodeConfig.HTTPPort),
		fmt.Sprintf("--staking-port=%d", nodeConfig.StakingPort),
		fmt.Sprintf("--public-ip=%s", nodeConfig.PublicIP),
		"--http-host=0.0.0.0",
		"--log-level=info",
		"--api-admin-enabled=true",
		"--api-metrics-enabled=true",
		"--index-enabled=true",
	}

	if netConfig.SingleNode {
		args = append(args,
			"--stake=false",
			"--consensus-sample-size=1",
			"--consensus-quorum-size=1",
			"--sybil-protection-disabled=true",
			"--sybil-protection-disabled-weight=100",
		)
	}

	// Create log file
	logPath := filepath.Join(node.DataDir, "luxd.log")
	logFile, err := os.Create(logPath)
	if err != nil {
		return fmt.Errorf("failed to create log file: %w", err)
	}

	// Start the process
	node.Process = exec.Command("/home/z/work/lux/node/build/luxd", args...)
	node.Process.Stdout = logFile
	node.Process.Stderr = logFile

	if err := node.Process.Start(); err != nil {
		logFile.Close()
		return fmt.Errorf("failed to start process: %w", err)
	}

	return nil
}

// copyGenesis copies genesis file to node data directory
func (nm *NetworkManager) copyGenesis(srcPath, nodeDataDir string) error {
	genesisDir := filepath.Join(nodeDataDir, "configs", "genesis")
	if err := os.MkdirAll(genesisDir, 0755); err != nil {
		return err
	}

	// Copy file
	input, err := os.ReadFile(srcPath)
	if err != nil {
		return err
	}

	dstPath := filepath.Join(genesisDir, "genesis.json")
	return os.WriteFile(dstPath, input, 0644)
}

// SaveNetworkConfig saves a network configuration to file
func (nm *NetworkManager) SaveNetworkConfig(name string) error {
	nm.mu.RLock()
	network, exists := nm.networks[name]
	nm.mu.RUnlock()

	if !exists {
		return fmt.Errorf("network %s not found", name)
	}

	// Create network directory
	networkDir := filepath.Join(nm.baseDir, name)
	if err := os.MkdirAll(networkDir, 0755); err != nil {
		return fmt.Errorf("failed to create network directory: %w", err)
	}

	configPath := filepath.Join(networkDir, "network.json")
	configData, err := json.MarshalIndent(network.Config, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(configPath, configData, 0644)
}

// LoadNetworkConfig loads a network configuration from file
func (nm *NetworkManager) LoadNetworkConfig(name string) error {
	configPath := filepath.Join(nm.baseDir, name, "network.json")
	configData, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}

	var config NetworkConfig
	if err := json.Unmarshal(configData, &config); err != nil {
		return err
	}

	return nm.CreateNetwork(name, config)
}