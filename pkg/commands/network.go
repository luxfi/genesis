package commands

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

// NetworkOptions contains options for network commands
type NetworkOptions struct {
	Name    string
	BaseDir string
}

// NewNetworkCommand creates the network command
func NewNetworkCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "network",
		Short: "Manage networks",
		Long:  "Start, stop, and manage test networks",
	}

	cmd.AddCommand(
		NewNetworkStartCommand(),
		NewNetworkStopCommand(),
		NewNetworkStatusCommand(),
		NewNetworkListCommand(),
		NewNetworkSnapshotCommand(),
	)

	return cmd
}

// NewNetworkStartCommand creates the network start command
func NewNetworkStartCommand() *cobra.Command {
	opts := &NetworkOptions{}
	
	cmd := &cobra.Command{
		Use:   "start [name]",
		Short: "Start a network",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Name = args[0]
			return RunNetworkStart(opts)
		},
	}
	
	cmd.Flags().StringVar(&opts.BaseDir, "base-dir", "", "Base directory for network data")
	
	return cmd
}

// NewNetworkStopCommand creates the network stop command
func NewNetworkStopCommand() *cobra.Command {
	opts := &NetworkOptions{}
	
	cmd := &cobra.Command{
		Use:   "stop [name]",
		Short: "Stop a network",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Name = args[0]
			return RunNetworkStop(opts)
		},
	}
	
	cmd.Flags().StringVar(&opts.BaseDir, "base-dir", "", "Base directory for network data")
	
	return cmd
}

// NewNetworkStatusCommand creates the network status command
func NewNetworkStatusCommand() *cobra.Command {
	opts := &NetworkOptions{}
	
	cmd := &cobra.Command{
		Use:   "status [name]",
		Short: "Show network status",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				opts.Name = args[0]
			}
			return RunNetworkStatus(opts)
		},
	}
	
	cmd.Flags().StringVar(&opts.BaseDir, "base-dir", "", "Base directory for network data")
	
	return cmd
}

// NewNetworkListCommand creates the network list command
func NewNetworkListCommand() *cobra.Command {
	opts := &NetworkOptions{}
	
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all networks",
		RunE: func(cmd *cobra.Command, args []string) error {
			return RunNetworkList(opts)
		},
	}
	
	cmd.Flags().StringVar(&opts.BaseDir, "base-dir", "", "Base directory for network data")
	
	return cmd
}

// NewNetworkSnapshotCommand creates the network snapshot command
func NewNetworkSnapshotCommand() *cobra.Command {
	opts := &NetworkOptions{}
	
	cmd := &cobra.Command{
		Use:   "snapshot [name]",
		Short: "Create network snapshot",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Name = args[0]
			return RunNetworkSnapshot(opts)
		},
	}
	
	cmd.Flags().StringVar(&opts.BaseDir, "base-dir", "", "Base directory for network data")
	
	return cmd
}

// Implementation functions

func RunNetworkStart(opts *NetworkOptions) error {
	baseDir := opts.BaseDir
	if baseDir == "" {
		baseDir = filepath.Join(os.Getenv("HOME"), ".genesis-networks")
	}
	
	networkDir := filepath.Join(baseDir, opts.Name)
	if _, err := os.Stat(networkDir); os.IsNotExist(err) {
		return fmt.Errorf("network %s does not exist. Run 'genesis launch' first", opts.Name)
	}
	
	// Start luxd process
	luxdPath := filepath.Join(networkDir, "luxd")
	if _, err := os.Stat(luxdPath); os.IsNotExist(err) {
		luxdPath = "luxd" // Try system path
	}
	
	configPath := filepath.Join(networkDir, "config.json")
	cmd := exec.Command(luxdPath, 
		"--config-file", configPath,
		"--data-dir", filepath.Join(networkDir, "data"),
	)
	
	// Create log file
	logFile, err := os.Create(filepath.Join(networkDir, "luxd.log"))
	if err != nil {
		return fmt.Errorf("failed to create log file: %w", err)
	}
	defer logFile.Close()
	
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start network: %w", err)
	}
	
	// Save PID
	pidFile := filepath.Join(networkDir, "luxd.pid")
	if err := os.WriteFile(pidFile, []byte(fmt.Sprintf("%d", cmd.Process.Pid)), 0644); err != nil {
		return fmt.Errorf("failed to save PID: %w", err)
	}
	
	fmt.Printf("✅ Network %s started (PID: %d)\n", opts.Name, cmd.Process.Pid)
	return nil
}

func RunNetworkStop(opts *NetworkOptions) error {
	baseDir := opts.BaseDir
	if baseDir == "" {
		baseDir = filepath.Join(os.Getenv("HOME"), ".genesis-networks")
	}
	
	networkDir := filepath.Join(baseDir, opts.Name)
	pidFile := filepath.Join(networkDir, "luxd.pid")
	
	pidData, err := os.ReadFile(pidFile)
	if err != nil {
		return fmt.Errorf("network %s is not running", opts.Name)
	}
	
	var pid int
	if _, err := fmt.Sscanf(string(pidData), "%d", &pid); err != nil {
		return fmt.Errorf("invalid PID file: %w", err)
	}
	
	// Kill process
	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("process not found: %w", err)
	}
	
	if err := process.Kill(); err != nil {
		return fmt.Errorf("failed to stop network: %w", err)
	}
	
	// Remove PID file
	os.Remove(pidFile)
	
	fmt.Printf("✅ Network %s stopped\n", opts.Name)
	return nil
}

func RunNetworkStatus(opts *NetworkOptions) error {
	baseDir := opts.BaseDir
	if baseDir == "" {
		baseDir = filepath.Join(os.Getenv("HOME"), ".genesis-networks")
	}
	
	if opts.Name != "" {
		// Check specific network
		return checkNetworkStatus(baseDir, opts.Name)
	}
	
	// List all networks with status
	entries, err := os.ReadDir(baseDir)
	if err != nil {
		return fmt.Errorf("failed to read networks directory: %w", err)
	}
	
	fmt.Println("Networks:")
	fmt.Println("=========")
	for _, entry := range entries {
		if entry.IsDir() {
			checkNetworkStatus(baseDir, entry.Name())
		}
	}
	
	return nil
}

func checkNetworkStatus(baseDir, name string) error {
	networkDir := filepath.Join(baseDir, name)
	pidFile := filepath.Join(networkDir, "luxd.pid")
	
	status := "stopped"
	if pidData, err := os.ReadFile(pidFile); err == nil {
		var pid int
		if _, err := fmt.Sscanf(string(pidData), "%d", &pid); err == nil {
			// Check if process is running
			if process, err := os.FindProcess(pid); err == nil {
				if err := process.Signal(os.Signal(nil)); err == nil {
					status = fmt.Sprintf("running (PID: %d)", pid)
				}
			}
		}
	}
	
	fmt.Printf("%-20s %s\n", name, status)
	return nil
}

func RunNetworkList(opts *NetworkOptions) error {
	baseDir := opts.BaseDir
	if baseDir == "" {
		baseDir = filepath.Join(os.Getenv("HOME"), ".genesis-networks")
	}
	
	entries, err := os.ReadDir(baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("No networks found")
			return nil
		}
		return fmt.Errorf("failed to read networks directory: %w", err)
	}
	
	fmt.Println("Available networks:")
	for _, entry := range entries {
		if entry.IsDir() {
			fmt.Printf("  - %s\n", entry.Name())
		}
	}
	
	return nil
}

func RunNetworkSnapshot(opts *NetworkOptions) error {
	baseDir := opts.BaseDir
	if baseDir == "" {
		baseDir = filepath.Join(os.Getenv("HOME"), ".genesis-networks")
	}
	
	networkDir := filepath.Join(baseDir, opts.Name)
	if _, err := os.Stat(networkDir); os.IsNotExist(err) {
		return fmt.Errorf("network %s does not exist", opts.Name)
	}
	
	// Create snapshot directory
	snapshotDir := filepath.Join(baseDir, "snapshots", opts.Name)
	if err := os.MkdirAll(snapshotDir, 0755); err != nil {
		return fmt.Errorf("failed to create snapshot directory: %w", err)
	}
	
	// Copy network data
	timestamp := strings.ReplaceAll(strings.ReplaceAll(
		strings.Split(fmt.Sprintf("%v", os.Getpid()), " ")[0], ":", "-"), ".", "-")
	snapshotPath := filepath.Join(snapshotDir, fmt.Sprintf("snapshot-%s.tar.gz", timestamp))
	
	cmd := exec.Command("tar", "-czf", snapshotPath, "-C", baseDir, opts.Name)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to create snapshot: %w\n%s", err, output)
	}
	
	fmt.Printf("✅ Snapshot created: %s\n", snapshotPath)
	return nil
}