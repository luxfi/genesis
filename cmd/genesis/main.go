package main

import (
	"os"

	"github.com/luxfi/genesis/pkg/commands"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	// Flags
	configFile string

	// Commands
	rootCmd = &cobra.Command{
		Use:   "genesis",
		Short: "Genesis configuration tool for Lux ecosystem",
		Long: `A unified CLI tool for managing genesis configurations 
and launching networks in the Lux ecosystem.

Everything is composable and configuration-driven.`,
	}
)

func init() {
	cobra.OnInitialize(initConfig)

	// Global flags
	rootCmd.PersistentFlags().StringVar(&configFile, "config", "", "config file (default is ./genesis.yaml)")

	// Register all commands using the unified registry
	commands.RegisterAllCommands(rootCmd)
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
		os.Exit(1)
	}
}