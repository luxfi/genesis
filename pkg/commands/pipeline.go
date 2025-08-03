package commands

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/luxfi/genesis/pkg/ancient"
	"github.com/luxfi/genesis/pkg/consensus"
	"github.com/ethereum/go-ethereum/common"
	"github.com/spf13/cobra"
)

// PipelineOptions contains options for pipeline commands
type PipelineOptions struct {
	Network string
}

// NewPipelineCommand creates the pipeline command
func NewPipelineCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pipeline",
		Short: "Run full genesis pipeline",
		Long:  "Execute the complete pipeline from cloning state to importing into node",
	}

	cmd.AddCommand(
		NewPipelineFullCommand(),
		NewPipelineCloneCommand(),
		NewPipelineProcessCommand(),
		NewPipelineImportCommand(),
	)

	return cmd
}

// NewPipelineFullCommand creates the full pipeline command
func NewPipelineFullCommand() *cobra.Command {
	opts := &PipelineOptions{}
	
	cmd := &cobra.Command{
		Use:   "full [network]",
		Short: "Run full pipeline for a network",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Network = args[0]
			return RunFullPipeline(opts)
		},
	}
	
	return cmd
}

// NewPipelineCloneCommand creates the clone command
func NewPipelineCloneCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "clone",
		Short: "Clone state repository",
		RunE: func(cmd *cobra.Command, args []string) error {
			return RunCloneState()
		},
	}
	
	return cmd
}

// NewPipelineProcessCommand creates the process command
func NewPipelineProcessCommand() *cobra.Command {
	opts := &PipelineOptions{}
	
	cmd := &cobra.Command{
		Use:   "process [network]",
		Short: "Process chaindata into ancient store format",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Network = args[0]
			return RunProcessChaindata(opts)
		},
	}
	
	return cmd
}

// NewPipelineImportCommand creates the import command
func NewPipelineImportCommand() *cobra.Command {
	opts := &PipelineOptions{}
	
	cmd := &cobra.Command{
		Use:   "import [network]",
		Short: "Import processed data into node",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Network = args[0]
			return RunImportToNode(opts)
		},
	}
	
	return cmd
}

// State management commands

// NewStateCommand creates the state command
func NewStateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "state",
		Short: "Manage state repository",
		Long:  "Clone, update, and manage historic chaindata from state repository",
	}

	cmd.AddCommand(
		NewStateCloneCommand(),
		NewStateUpdateCommand(),
		NewStateCleanCommand(),
	)

	return cmd
}

// NewStateCloneCommand creates the state clone command
func NewStateCloneCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "clone",
		Short: "Clone state repository",
		RunE: func(cmd *cobra.Command, args []string) error {
			return RunCloneState()
		},
	}
	
	return cmd
}

// NewStateUpdateCommand creates the state update command
func NewStateUpdateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update cloned state repository",
		RunE: func(cmd *cobra.Command, args []string) error {
			return RunUpdateState()
		},
	}
	
	return cmd
}

// NewStateCleanCommand creates the state clean command
func NewStateCleanCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "clean",
		Short: "Remove cloned state data",
		RunE: func(cmd *cobra.Command, args []string) error {
			return RunCleanState()
		},
	}
	
	return cmd
}

// Implementation functions

func RunFullPipeline(opts *PipelineOptions) error {
	fmt.Printf("Running full genesis pipeline for %s...\n", opts.Network)
	
	// Step 1: Clone state if needed
	if _, err := os.Stat("state"); os.IsNotExist(err) {
		fmt.Println("Step 1: Cloning state repository...")
		if err := RunCloneState(); err != nil {
			return err
		}
	} else {
		fmt.Println("Step 1: State repository already exists")
	}
	
	// Step 2: Process chaindata
	fmt.Println("Step 2: Processing chaindata...")
	if err := RunProcessChaindata(opts); err != nil {
		return err
	}
	
	// Step 3: Generate genesis
	fmt.Println("Step 3: Generating genesis configuration...")
	genOpts := &GenerateOptions{
		Network: opts.Network,
		ChainType: "unified",
		OutputDir: "./configs",
	}
	
	// Get chain info to set proper chain ID
	if info, exists := consensus.GetChainInfo(opts.Network); exists {
		genOpts.ChainID = info.ChainID
	}
	
	if err := RunGenerate(genOpts); err != nil {
		return err
	}
	
	// Step 4: Import to node
	fmt.Println("Step 4: Importing to node...")
	if err := RunImportToNode(opts); err != nil {
		return err
	}
	
	fmt.Println("✅ Pipeline completed successfully!")
	return nil
}

func RunCloneState() error {
	fmt.Println("Cloning state repository...")
	
	// Use make command
	makeCmd := exec.Command("make", "clone-state")
	makeCmd.Stdout = os.Stdout
	makeCmd.Stderr = os.Stderr
	
	if err := makeCmd.Run(); err != nil {
		return fmt.Errorf("failed to clone state: %w", err)
	}
	
	return nil
}

func RunUpdateState() error {
	fmt.Println("Updating state repository...")
	
	makeCmd := exec.Command("make", "update-state")
	makeCmd.Stdout = os.Stdout
	makeCmd.Stderr = os.Stderr
	
	if err := makeCmd.Run(); err != nil {
		return fmt.Errorf("failed to update state: %w", err)
	}
	
	return nil
}

func RunCleanState() error {
	fmt.Println("Cleaning state data...")
	
	if err := os.RemoveAll("state"); err != nil {
		return fmt.Errorf("failed to clean state: %w", err)
	}
	
	fmt.Println("✅ State data removed")
	return nil
}

func RunProcessChaindata(opts *PipelineOptions) error {
	fmt.Printf("Processing chaindata for %s...\n", opts.Network)
	
	// Get chain info
	info, exists := consensus.GetChainInfo(opts.Network)
	if !exists {
		return fmt.Errorf("unknown network: %s", opts.Network)
	}
	
	// Determine paths
	chainDataPath := fmt.Sprintf("state/chaindata/%s-%d", opts.Network, info.ChainID)
	if _, err := os.Stat(chainDataPath); os.IsNotExist(err) {
		// Try alternate path
		chainDataPath = fmt.Sprintf("state/chaindata/%s", opts.Network)
		if _, err := os.Stat(chainDataPath); os.IsNotExist(err) {
			return fmt.Errorf("chaindata not found for %s. Run 'make clone-state' first", opts.Network)
		}
	}
	
	// Create ancient data config
	ancientConfig := &ancient.CChainAncientData{
		ChainID:      info.ChainID,
		GenesisHash:  common.HexToHash("0x0"), // Will be updated
		StartBlock:   0,
		EndBlock:     1000000, // Process first 1M blocks
		DataPath:     chainDataPath,
		CompactedDir: fmt.Sprintf("output/ancient-%s", opts.Network),
	}
	
	// Build ancient store
	builder, err := ancient.NewBuilder(ancientConfig)
	if err != nil {
		return fmt.Errorf("failed to create ancient builder: %w", err)
	}
	defer builder.Close()
	
	// Compact data
	if err := builder.CompactAncientData(); err != nil {
		return fmt.Errorf("failed to compact ancient data: %w", err)
	}
	
	// Export for genesis
	outputPath := fmt.Sprintf("output/genesis-%s", opts.Network)
	if err := builder.ExportToGenesis(outputPath); err != nil {
		return fmt.Errorf("failed to export genesis data: %w", err)
	}
	
	fmt.Printf("✅ Chaindata processed and exported to: %s\n", outputPath)
	return nil
}

func RunImportToNode(opts *PipelineOptions) error {
	fmt.Printf("Importing genesis data to node for %s...\n", opts.Network)
	
	// Get paths
	genesisPath := fmt.Sprintf("output/genesis-%s", opts.Network)
	nodePath := os.Getenv("LUXD_PATH")
	if nodePath == "" {
		nodePath = filepath.Join(os.Getenv("HOME"), ".luxd")
	}
	
	// Import using ancient package
	targetPath := filepath.Join(nodePath, "chains", "C")
	if err := ancient.ImportFromGenesis(genesisPath, targetPath); err != nil {
		return fmt.Errorf("failed to import genesis data: %w", err)
	}
	
	fmt.Println("✅ Genesis data imported successfully!")
	fmt.Printf("Node data directory: %s\n", targetPath)
	return nil
}