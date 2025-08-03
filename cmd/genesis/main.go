package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	// Version information
	Version   = "1.0.0"
	BuildTime = "unknown"
	GitCommit = "unknown"

	// Flags
	configFile string

	// Commands
	rootCmd = &cobra.Command{
		Use:   "genesis",
		Short: "Genesis configuration tool for Lux ecosystem",
		Long: `A unified CLI tool for managing genesis configurations 
and launching networks in the Lux ecosystem.`,
	}

	versionCmd = &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("Genesis CLI v%s\n", Version)
			fmt.Printf("Build Time: %s\n", BuildTime)
			fmt.Printf("Git Commit: %s\n", GitCommit)
		},
	}
)

func init() {
	cobra.OnInitialize(initConfig)

	// Global flags
	rootCmd.PersistentFlags().StringVar(&configFile, "config", "", "config file (default is ./genesis.yaml)")

	// Add commands
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(getReplayerCmd())
	rootCmd.AddCommand(getCompactAncientCmd())
	rootCmd.AddCommand(getConvertCmd())
	rootCmd.AddCommand(getDatabaseCmd())
	rootCmd.AddCommand(getExtractCmd())
	rootCmd.AddCommand(getImportBlockchainCmd())
	rootCmd.AddCommand(getInspectCmd())
	rootCmd.AddCommand(getDebugKeysCmd())
	rootCmd.AddCommand(getL2Cmd())
	rootCmd.AddCommand(getSetupChainStateCmd())
}

func initConfig() {
	if configFile != "" {
		viper.SetConfigFile(configFile)
	} else {
		viper.AddConfigPath(".")
		viper.SetConfigName("genesis")
		viper.SetConfigType("yaml")
	}

	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err == nil {
		// Silent - don't print config file used
	}
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
