package commands

import (
	"github.com/spf13/cobra"
)

// RegisterAllCommands registers all commands to the root command
func RegisterAllCommands(rootCmd *cobra.Command) {
	// Core commands
	rootCmd.AddCommand(NewGenerateCommand())
	rootCmd.AddCommand(NewLaunchCommand())
	rootCmd.AddCommand(NewValidateCommand())
	
	// Network management
	rootCmd.AddCommand(NewNetworkCommand())
	
	// Consensus management
	rootCmd.AddCommand(NewConsensusCommand())
	rootCmd.AddCommand(NewMinimalConsensusCommand()) // Backward compatibility
	
	// Pipeline operations
	rootCmd.AddCommand(NewPipelineCommand())
	rootCmd.AddCommand(NewStateCommand())
	
	// Backward compatibility commands
	rootCmd.AddCommand(NewSetupNodeCommand())
	rootCmd.AddCommand(NewSingleNodeCommand())
	
	// Version command
	rootCmd.AddCommand(NewVersionCommand())
}

// NewVersionCommand creates the version command
func NewVersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, args []string) {
			// These will be set by ldflags during build
			version := "1.0.0"
			buildTime := "unknown"
			gitCommit := "unknown"
			
			cmd.Printf("Genesis CLI v%s\n", version)
			cmd.Printf("Build Time: %s\n", buildTime)
			cmd.Printf("Git Commit: %s\n", gitCommit)
		},
	}
}