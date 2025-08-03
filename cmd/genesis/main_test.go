package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRootCommand(t *testing.T) {
	// Test that root command can be created
	assert.NotNil(t, rootCmd)
	assert.Equal(t, "genesis", rootCmd.Use)
	assert.Contains(t, rootCmd.Short, "Genesis configuration tool")
}

func TestVersionCommand(t *testing.T) {
	// Test version command
	assert.NotNil(t, versionCmd)
	assert.Equal(t, "version", versionCmd.Use)
	assert.Contains(t, versionCmd.Short, "Print version information")
}

func TestCommandRegistration(t *testing.T) {
	// Test that all expected commands are registered
	commands := rootCmd.Commands()

	// Create a map of command names for easy checking
	cmdMap := make(map[string]bool)
	for _, cmd := range commands {
		cmdMap[cmd.Use] = true
	}

	// Check that expected commands exist
	expectedCommands := []string{
		"version",
		"replay",
		"compact-ancient",
		"convert",
		"database",
		"extract",
		"import-blockchain",
		"inspect",
		"debug-keys",
		"l2",
		"setup-chain-state",
	}

	for _, expected := range expectedCommands {
		// Commands might have arguments in their Use field, so we check if it starts with the command name
		found := false
		for _, cmd := range commands {
			if cmd.Use == expected || (len(cmd.Use) > len(expected) && cmd.Use[:len(expected)] == expected) {
				found = true
				break
			}
		}
		require.True(t, found, "Expected command %s to be registered", expected)
	}
}

func TestInitConfig(t *testing.T) {
	// Test that initConfig doesn't panic
	assert.NotPanics(t, func() {
		initConfig()
	})
}
