package application

import (
	"path/filepath"

	"github.com/luxfi/log"
	"github.com/spf13/viper"
)

// Genesis is the main application context that holds all dependencies
type Genesis struct {
	Log     log.Logger
	BaseDir string
	Config  *viper.Viper
}

// New creates a new Genesis application instance
func New() *Genesis {
	return &Genesis{}
}

// Setup initializes the application with dependencies
func (g *Genesis) Setup(baseDir string, logger log.Logger, config *viper.Viper) {
	g.BaseDir = baseDir
	g.Log = logger
	g.Config = config
}

// GetDataDir returns the data directory path
func (g *Genesis) GetDataDir() string {
	return filepath.Join(g.BaseDir, "data")
}

// GetConfigDir returns the config directory path
func (g *Genesis) GetConfigDir() string {
	return filepath.Join(g.BaseDir, "configs")
}

// GetKeysDir returns the keys directory path
func (g *Genesis) GetKeysDir() string {
	return filepath.Join(g.BaseDir, "keys")
}

// GetNetworksDir returns the networks directory path
func (g *Genesis) GetNetworksDir() string {
	return filepath.Join(g.BaseDir, "networks")
}

// GetOutputDir returns the output directory path
func (g *Genesis) GetOutputDir() string {
	return filepath.Join(g.BaseDir, "output")
}
