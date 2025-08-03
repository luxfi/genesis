package main

import (
	"fmt"
	"log"
	"os"

	"github.com/luxfi/genesis/pkg/core"
	"github.com/luxfi/genesis/pkg/credentials"
	"github.com/luxfi/genesis/pkg/launch"
)

// Example: Launch a custom test network with 5 nodes
func main() {
	// Create network configuration
	network := &core.Network{
		Name:      "test-network",
		NetworkID: 99999,
		ChainID:   99999,
		Nodes:     5,
		Genesis: core.GenesisConfig{
			Source:  "fresh",
			Message: "Test Network Genesis",
			Allocations: map[string]uint64{
				// Treasury
				"0x1000000000000000000000000000000000000000": 1000000000000000000,
				// Test account
				"0x8db97C7cEcE249c2b98bDC0226Cc4C2A57BF52FC": 100000000000000000,
			},
		},
		// Consensus will be auto-configured for 5 nodes
	}

	// Validate configuration
	if err := network.Validate(); err != nil {
		log.Fatalf("Invalid network configuration: %v", err)
	}

	// Apply defaults (sets consensus parameters based on node count)
	network.Normalize()

	fmt.Printf("Network Configuration:\n")
	fmt.Printf("  Name: %s\n", network.Name)
	fmt.Printf("  Network ID: %d\n", network.NetworkID)
	fmt.Printf("  Chain ID: %d\n", network.ChainID)
	fmt.Printf("  Nodes: %d\n", network.Nodes)
	fmt.Printf("  Consensus K: %d\n", network.Consensus.K)
	fmt.Printf("  Consensus Alpha: %d\n", network.Consensus.Alpha)
	fmt.Printf("  Consensus Beta: %d\n", network.Consensus.Beta)

	// Generate credentials for each node
	gen := credentials.NewGenerator()
	for i := 0; i < network.Nodes; i++ {
		nodeDir := fmt.Sprintf("./nodes/node%d", i)
		
		creds, err := gen.Generate()
		if err != nil {
			log.Fatalf("Failed to generate credentials for node %d: %v", i, err)
		}

		if err := gen.Save(creds, nodeDir); err != nil {
			log.Fatalf("Failed to save credentials for node %d: %v", i, err)
		}

		fmt.Printf("\nNode %d:\n", i)
		fmt.Printf("  NodeID: %s\n", creds.NodeID)
		fmt.Printf("  Credentials saved to: %s/staking/\n", nodeDir)
	}

	// Create launcher
	launcher := launch.NewLauncher()

	// Set up launch options
	launcher.SetDataDir("./nodes")
	launcher.SetLogLevel("info")

	// Launch the network
	fmt.Println("\nLaunching network...")
	if err := launcher.LaunchNetwork(network); err != nil {
		log.Fatalf("Failed to launch network: %v", err)
	}

	fmt.Println("\nNetwork launched successfully!")
	fmt.Println("Press Ctrl+C to stop the network")

	// Wait for interrupt
	select {}
}