package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/luxfi/genesis/pkg/application"
	"github.com/luxfi/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	// Version information (set by ldflags)
	Version   = "1.0.0"
	BuildTime = "unknown"
	GitCommit = "unknown"

	// Global flags
	configFile string
	logLevel   string
	baseDir    string

	// Application context
	app *application.Genesis
)

// NewRootCmd creates the root command
func NewRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:     "genesis",
		Short:   "Genesis configuration tool for Lux ecosystem",
		Long:    `A unified CLI tool for managing genesis configurations and blockchain operations in the Lux ecosystem.`,
		Version: fmt.Sprintf("%s (built %s, commit %s)", Version, BuildTime, GitCommit),
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// Initialize application context
			return initializeApp()
		},
	}

	// Global flags
	rootCmd.PersistentFlags().StringVar(&configFile, "config", "", "config file (default is ./genesis.yaml)")
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "info", "log level (debug, info, warn, error)")
	rootCmd.PersistentFlags().StringVar(&baseDir, "base-dir", "", "base directory for genesis data")

	// Initialize config
	cobra.OnInitialize(initConfig)

	// Add commands
	rootCmd.AddCommand(NewGenerateCmd(app))
	rootCmd.AddCommand(NewAddressCmd(app))
	rootCmd.AddCommand(NewImportCmd(app))
	rootCmd.AddCommand(NewExtractCmd(app))
	rootCmd.AddCommand(NewInspectCmd(app))
	rootCmd.AddCommand(NewLaunchCmd(app))
	rootCmd.AddCommand(NewConvertCmd(app))
	rootCmd.AddCommand(NewDatabaseCmd(app))
	rootCmd.AddCommand(NewL2Cmd(app))
	rootCmd.AddCommand(NewReplayCmd(app))
	rootCmd.AddCommand(NewSubnetBlockReplayCmd(app))
	rootCmd.AddCommand(NewValidatorsCmd(app))
	rootCmd.AddCommand(NewToolsCmd(app))
	rootCmd.AddCommand(NewSetupCmd(app))

	return rootCmd
}

func initConfig() {
	if configFile != "" {
		viper.SetConfigFile(configFile)
	} else {
		viper.AddConfigPath(".")
		viper.SetConfigName("genesis")
		viper.SetConfigType("yaml")
	}

	viper.SetEnvPrefix("GENESIS")
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err == nil {
		// Config file found and loaded
	}
}

func initializeApp() error {
	// Set up base directory
	if baseDir == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		baseDir = filepath.Join(homeDir, ".genesis")
	}

	// Create base directory if it doesn't exist
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return fmt.Errorf("failed to create base directory: %w", err)
	}

	// Initialize logger
	logger := log.NewLogger("genesis")

	// Create application context
	app = application.New()
	app.Setup(baseDir, logger, viper.GetViper())

	return nil
}