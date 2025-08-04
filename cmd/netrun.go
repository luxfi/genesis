package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/luxfi/genesis/pkg/application"
	"github.com/luxfi/genesis/pkg/netrun"
	"github.com/spf13/cobra"
)

// NewNetrunCmd creates the netrun command for launching networks with netrunner
func NewNetrunCmd(app *application.Genesis) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "netrun",
		Short: "Launch and manage multiple Lux networks in parallel",
		Long: `Netrun provides tools to manage multiple Lux networks simultaneously.
You can create, start, stop, and monitor multiple networks with different configurations.`,
	}

	cmd.AddCommand(newNetrunCreateCmd(app))
	cmd.AddCommand(newNetrunStartCmd(app))
	cmd.AddCommand(newNetrunStopCmd(app))
	cmd.AddCommand(newNetrunListCmd(app))
	cmd.AddCommand(newNetrunStatusCmd(app))
	cmd.AddCommand(newNetrunServerCmd(app))

	return cmd
}

func newNetrunServerCmd(app *application.Genesis) *cobra.Command {
	var (
		port        string
		grpcGateway string
		logLevel    string
	)

	cmd := &cobra.Command{
		Use:   "server",
		Short: "Start netrunner RPC server",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf("Starting netrunner server...\n")
			fmt.Printf("Port: %s\n", port)
			fmt.Printf("gRPC Gateway: %s\n", grpcGateway)
			fmt.Printf("Log Level: %s\n", logLevel)

			// Note: This would normally launch the netrunner server binary
			// For now, we'll just print instructions
			fmt.Println("\nTo start the server manually, run:")
			fmt.Printf("netrunner server --log-level %s --port=%s --grpc-gateway-port=%s\n", logLevel, port, grpcGateway)

			return nil
		},
	}

	cmd.Flags().StringVar(&port, "port", ":8080", "RPC server port")
	cmd.Flags().StringVar(&grpcGateway, "grpc-gateway-port", ":8081", "gRPC gateway port")
	cmd.Flags().StringVar(&logLevel, "log-level", "info", "Log level")

	return cmd
}

func newNetrunStartCmd(app *application.Genesis) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start [name]",
		Short: "Start a network",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			// Create network manager
			homeDir, _ := os.UserHomeDir()
			baseDir := filepath.Join(homeDir, ".lux-networks")
			manager := netrun.NewManager(baseDir)

			// Load saved network
			if err := manager.LoadNetworkConfig(name); err != nil {
				return fmt.Errorf("failed to load network: %w", err)
			}

			// Start the network
			if err := manager.StartNetwork(name); err != nil {
				return fmt.Errorf("failed to start network: %w", err)
			}

			// Get network status
			info, err := manager.GetNetworkStatus(name)
			if err != nil {
				return err
			}

			// Print connection info
			fmt.Println("\nNetwork endpoints:")
			for _, node := range info.Nodes {
				fmt.Printf("  %s: %s/ext/bc/C/rpc\n", node.Name, node.RPC)
			}

			fmt.Println("\nUse 'genesis netrun status " + name + "' to check network status")
			fmt.Println("Use 'genesis netrun stop " + name + "' to stop the network")

			return nil
		},
	}

	return cmd
}




func newNetrunCreateCmd(app *application.Genesis) *cobra.Command {
	var (
		networkID   uint32
		numNodes    int
		singleNode  bool
		genesisPath string
	)

	cmd := &cobra.Command{
		Use:   "create [name]",
		Short: "Create a new network configuration",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			// Create network manager
			homeDir, _ := os.UserHomeDir()
			baseDir := filepath.Join(homeDir, ".lux-networks")
			manager := netrun.NewManager(baseDir)

			// Create network config
			config := netrun.NetworkConfig{
				NetworkID:   networkID,
				NumNodes:    numNodes,
				SingleNode:  singleNode,
				GenesisPath: genesisPath,
			}

			if err := manager.CreateNetwork(name, config); err != nil {
				return fmt.Errorf("failed to create network: %w", err)
			}

			// Save configuration
			if err := manager.SaveNetworkConfig(name); err != nil {
				return fmt.Errorf("failed to save network config: %w", err)
			}

			fmt.Printf("Network '%s' created successfully!\n", name)
			fmt.Printf("Network ID: %d\n", networkID)
			fmt.Printf("Number of nodes: %d\n", numNodes)
			if singleNode {
				fmt.Println("Mode: Single node (k=1)")
			}

			return nil
		},
	}

	cmd.Flags().Uint32Var(&networkID, "network-id", 96369, "Network ID")
	cmd.Flags().IntVar(&numNodes, "num-nodes", 5, "Number of nodes")
	cmd.Flags().BoolVar(&singleNode, "single-node", false, "Run in single node mode")
	cmd.Flags().StringVar(&genesisPath, "genesis", "", "Path to genesis file")

	return cmd
}

func newNetrunStopCmd(app *application.Genesis) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stop [name]",
		Short: "Stop a running network",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			// Create network manager
			homeDir, _ := os.UserHomeDir()
			baseDir := filepath.Join(homeDir, ".lux-networks")
			manager := netrun.NewManager(baseDir)

			// Load saved network
			if err := manager.LoadNetworkConfig(name); err != nil {
				return fmt.Errorf("failed to load network: %w", err)
			}

			// Stop the network
			if err := manager.StopNetwork(name); err != nil {
				return fmt.Errorf("failed to stop network: %w", err)
			}

			return nil
		},
	}

	return cmd
}

func newNetrunListCmd(app *application.Genesis) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all networks",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Create network manager
			homeDir, _ := os.UserHomeDir()
			baseDir := filepath.Join(homeDir, ".lux-networks")
			manager := netrun.NewManager(baseDir)

			// List networks
			networks := manager.ListNetworks()

			if len(networks) == 0 {
				fmt.Println("No networks found.")
				return nil
			}

			fmt.Println("NETWORK NAME    NETWORK ID    NODES    STATUS    UPTIME")
			fmt.Println("────────────────────────────────────────────────────────")

			for _, net := range networks {
				uptime := ""
				if net.Status == "running" && !net.StartTime.IsZero() {
					uptime = time.Since(net.StartTime).Round(time.Second).String()
				}

				fmt.Printf("%-15s %-13d %-8d %-9s %s\n",
					net.Name,
					net.NetworkID,
					net.NumNodes,
					net.Status,
					uptime,
				)
			}

			return nil
		},
	}

	return cmd
}

func newNetrunStatusCmd(app *application.Genesis) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status [name]",
		Short: "Get detailed status of a network",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			// Create network manager
			homeDir, _ := os.UserHomeDir()
			baseDir := filepath.Join(homeDir, ".lux-networks")
			manager := netrun.NewManager(baseDir)

			// Get network status
			info, err := manager.GetNetworkStatus(name)
			if err != nil {
				return err
			}

			// Print network info
			fmt.Printf("Network: %s\n", info.Name)
			fmt.Printf("Network ID: %d\n", info.NetworkID)
			fmt.Printf("Status: %s\n", info.Status)
			if info.Status == "running" && !info.StartTime.IsZero() {
				fmt.Printf("Uptime: %s\n", time.Since(info.StartTime).Round(time.Second))
			}
			fmt.Printf("\nNodes (%d):\n", len(info.Nodes))
			fmt.Println("────────────────────────────────────────────────────")

			for _, node := range info.Nodes {
				fmt.Printf("  %s:\n", node.Name)
				fmt.Printf("    Status: %s\n", node.Status)
				fmt.Printf("    RPC: %s\n", node.RPC)
				if node.NodeID != "" {
					fmt.Printf("    Node ID: %s\n", node.NodeID)
				}
			}

			return nil
		},
	}

	return cmd
}
