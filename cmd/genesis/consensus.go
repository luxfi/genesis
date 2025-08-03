package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"
)

// getMinimalConsensusCmd creates a command for running minimal consensus networks
func getMinimalConsensusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "minimal",
		Short: "Launch network with minimal consensus (1-2 nodes)",
		Long: `Launch a network with minimal consensus parameters optimized for 1-2 nodes.

Avalanche's DAG-based consensus allows flexible configurations:
- With K=1: Only need 1 node to make progress (single node network)
- With K=2, alpha=1: Can tolerate 1 of 2 nodes being offline
- Network can split and reconverge thanks to DAG structure

This is perfect for development, testing, and running mainnet replay locally.`,
		RunE: runMinimalConsensus,
	}

	cmd.Flags().Int("nodes", 2, "Number of nodes (1 or 2)")
	cmd.Flags().Bool("split-test", false, "Enable split/rejoin testing mode")

	return cmd
}

func runMinimalConsensus(cmd *cobra.Command, args []string) error {
	nodes, _ := cmd.Flags().GetInt("nodes")
	splitTest, _ := cmd.Flags().GetBool("split-test")

	if nodes < 1 || nodes > 2 {
		return fmt.Errorf("minimal consensus only supports 1 or 2 nodes")
	}

	fmt.Printf("🚀 Launching Minimal Consensus Network (%d node%s)\n", nodes, plural(nodes))
	
	// Explain consensus parameters
	fmt.Println("\n📊 Consensus Configuration:")
	if nodes == 1 {
		fmt.Println("  K = 1                  (sample size - only query self)")
		fmt.Println("  Alpha = 1              (confidence threshold - immediate)")
		fmt.Println("  Beta = 1               (finalization threshold - immediate)")
		fmt.Println("  → Single node can finalize blocks independently")
	} else {
		fmt.Println("  K = 2                  (sample size - query both nodes)")
		fmt.Println("  Alpha = 1              (confidence threshold - need 1 response)")
		fmt.Println("  Beta = 1               (finalization threshold - 1 round)")
		fmt.Println("  → Can operate with 1 of 2 nodes (50% availability)")
		fmt.Println("  → Network can split and reconverge")
	}

	// Step 1: Set up nodes with proper staking keys
	fmt.Println("\n🔑 Setting up nodes with staking keys and PoP...")
	
	nodeConfigs := []NodeConfig{}
	basePort := 9630

	for i := 0; i < nodes; i++ {
		port := basePort + (i * 10)
		nodeDir := filepath.Join(os.Getenv("HOME"), ".luxd", fmt.Sprintf("minimal-%d", port))
		
		fmt.Printf("\nNode %d setup:\n", i)
		
		// Use setup-node command to create proper keys
		setupCmd := exec.Command(os.Args[0], "setup-node",
			"--port", fmt.Sprintf("%d", port),
			"--data-dir", nodeDir,
			"--network-id", "96369",
			"--consensus-k", fmt.Sprintf("%d", nodes),
			"--consensus-alpha", "1",
			"--consensus-beta", "1")
		
		setupCmd.Stdout = os.Stdout
		setupCmd.Stderr = os.Stderr
		
		if err := setupCmd.Run(); err != nil {
			return fmt.Errorf("failed to setup node %d: %w", i, err)
		}
		
		// Load validator info to get node details
		validatorInfoPath := filepath.Join(nodeDir, "validator-info.json")
		data, _ := os.ReadFile(validatorInfoPath)
		var info map[string]interface{}
		json.Unmarshal(data, &info)
		
		nodeConfigs = append(nodeConfigs, NodeConfig{
			Index:    i,
			Port:     port,
			DataDir:  nodeDir,
			NodeID:   info["nodeID"].(string),
		})
	}

	// Step 2: Configure bootstrap nodes for multi-node setup
	if nodes > 1 {
		fmt.Println("\n🔗 Configuring node connectivity...")
		
		// Each node needs to know about the others
		for i, node := range nodeConfigs {
			configPath := filepath.Join(node.DataDir, "node-config.json")
			
			// Read existing config
			data, _ := os.ReadFile(configPath)
			var config map[string]interface{}
			json.Unmarshal(data, &config)
			
			// Add bootstrap nodes (all other nodes)
			bootstrapIPs := []string{}
			bootstrapIDs := []string{}
			
			for j, other := range nodeConfigs {
				if i != j {
					bootstrapIPs = append(bootstrapIPs, fmt.Sprintf("127.0.0.1:%d", other.Port+1))
					bootstrapIDs = append(bootstrapIDs, other.NodeID)
				}
			}
			
			config["bootstrap-ips"] = bootstrapIPs
			config["bootstrap-ids"] = bootstrapIDs
			
			// For minimal consensus, adjust timeouts
			config["network-peer-list-gossip-frequency"] = "250ms"
			config["network-max-reconnect-delay"] = "1s"
			config["consensus-gossip-frequency"] = "250ms"
			
			// Save updated config
			updatedData, _ := json.MarshalIndent(config, "", "  ")
			os.WriteFile(configPath, updatedData, 0644)
		}
	}

	// Step 3: Create launch scripts
	fmt.Println("\n🚀 Creating launch scripts...")
	
	// Main launch script
	mainScript := filepath.Join(os.Getenv("HOME"), ".luxd", "minimal-launch.sh")
	scriptContent := `#!/bin/bash
# Minimal Consensus Network Launcher

echo "🚀 Starting Minimal Consensus Network..."
`

	for i, node := range nodeConfigs {
		scriptContent += fmt.Sprintf(`
echo "Starting Node %d (Port %d)..."
%s/launch.sh > %s/node.log 2>&1 &
echo $! > %s/node.pid
sleep 2
`, i, node.Port, node.DataDir, node.DataDir, node.DataDir)
	}

	scriptContent += `
echo "✅ All nodes started!"
echo ""
echo "📊 Network Status:"
`

	for i, node := range nodeConfigs {
		scriptContent += fmt.Sprintf(`echo "  Node %d: http://localhost:%d (PID: $(cat %s/node.pid))"\n`, 
			i, node.Port, node.DataDir)
	}

	scriptContent += `
echo ""
echo "To check consensus health:"
echo "  curl http://localhost:9630/ext/health"
echo ""
echo "To stop all nodes:"
echo "  ./minimal-stop.sh"
`

	os.WriteFile(mainScript, []byte(scriptContent), 0755)

	// Stop script
	stopScript := filepath.Join(os.Getenv("HOME"), ".luxd", "minimal-stop.sh")
	stopContent := `#!/bin/bash
echo "🛑 Stopping Minimal Consensus Network..."
`

	for _, node := range nodeConfigs {
		stopContent += fmt.Sprintf(`
if [ -f %s/node.pid ]; then
    PID=$(cat %s/node.pid)
    echo "Stopping Node on port %d (PID: $PID)..."
    kill $PID 2>/dev/null
    rm %s/node.pid
fi
`, node.DataDir, node.DataDir, node.Port, node.DataDir)
	}

	stopContent += `echo "✅ All nodes stopped!"`
	os.WriteFile(stopScript, []byte(stopContent), 0755)

	// Step 4: Explain split-brain handling
	if splitTest && nodes == 2 {
		fmt.Println("\n🔀 Split-Brain Test Mode Enabled!")
		fmt.Println("\nWith K=2, alpha=1, the network can handle splits:")
		fmt.Println("1. Both nodes running → Normal consensus")
		fmt.Println("2. Kill one node → Remaining node continues (1/2 = 50%)")
		fmt.Println("3. Restart killed node → Automatic reconvergence")
		fmt.Println("\nThe DAG structure allows both sides of a split to progress")
		fmt.Println("and merge when connectivity is restored!")
		
		// Create split test script
		splitTestScript := filepath.Join(os.Getenv("HOME"), ".luxd", "minimal-split-test.sh")
		testContent := fmt.Sprintf(`#!/bin/bash
echo "🔀 Split-Brain Test Script"
echo ""
echo "1. Starting with both nodes..."
sleep 5

echo "2. Killing Node 1 to simulate network split..."
kill $(cat %s/node.pid)
echo "   Node 0 is now running alone (1/2 nodes)"
echo "   It can still make progress with alpha=1!"
sleep 10

echo "3. Restarting Node 1..."
%s/launch.sh > %s/node.log 2>&1 &
echo $! > %s/node.pid
echo "   Nodes will now reconverge..."
sleep 5

echo "✅ Split test complete! Check logs to see reconvergence."
`, nodeConfigs[1].DataDir, nodeConfigs[1].DataDir, nodeConfigs[1].DataDir, nodeConfigs[1].DataDir)
		
		os.WriteFile(splitTestScript, []byte(testContent), 0755)
		fmt.Printf("\nSplit test script created: %s\n", splitTestScript)
	}

	// Step 5: Launch the network
	fmt.Println("\n🎯 Launching network...")
	launchCmd := exec.Command(mainScript)
	launchCmd.Stdout = os.Stdout
	launchCmd.Stderr = os.Stderr
	
	if err := launchCmd.Run(); err != nil {
		return fmt.Errorf("failed to launch network: %w", err)
	}

	fmt.Println("\n✅ Minimal Consensus Network is running!")
	fmt.Println("\n📝 Key Points:")
	fmt.Printf("  • Running with %d node%s\n", nodes, plural(nodes))
	if nodes == 1 {
		fmt.Println("  • Single node can finalize blocks independently")
		fmt.Println("  • Perfect for local development and testing")
	} else {
		fmt.Println("  • Can tolerate 1 node failure (50% availability)")
		fmt.Println("  • Network can split and reconverge")
		fmt.Println("  • Ideal for testing consensus behavior")
	}
	fmt.Println("\n🔧 Management Commands:")
	fmt.Printf("  • Start: %s\n", mainScript)
	fmt.Printf("  • Stop:  %s\n", stopScript)
	if splitTest && nodes == 2 {
		fmt.Printf("  • Test:  %s\n", filepath.Join(os.Getenv("HOME"), ".luxd", "minimal-split-test.sh"))
	}

	return nil
}

// Helper types and functions

type NodeConfig struct {
	Index   int
	Port    int
	DataDir string
	NodeID  string
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

// Add this command to main.go
func init() {
	// This would be added to the root command in main.go
	// rootCmd.AddCommand(getMinimalConsensusCmd())
}