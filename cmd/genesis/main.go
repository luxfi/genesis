// Copyright (C) 2019-2025, Lux Partners Limited. All rights reserved.
// See the file LICENSE for licensing terms.

// Command genesis generates Lux network genesis configurations.
//
// Usage:
//
//	genesis [flags]
//
// Flags:
//
//	-network        Network name: mainnet, testnet, local, custom (default: local)
//	-network-id     Network ID (overrides -network if set)
//	-keys-dir       Directory containing node keys (default: ~/.lux/keys)
//	-genesis-dir    Directory containing genesis component files
//	-output         Output file path (default: stdout or primary.json)
//	-allocation     Allocation per validator in LUX (default: 1000000000)
//	-validators     Number of validators for mnemonic-based genesis
//	-cchain         Path to existing C-Chain genesis (preserves original)
//	-format         Output format: json, pretty (default: pretty)
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/luxfi/constants"
	"github.com/luxfi/genesis/pkg/genesis"
)

func main() {
	var (
		network     = flag.String("network", "local", "Network name: mainnet, testnet, devnet, local")
		networkID   = flag.Uint("network-id", 0, "Network ID (overrides -network if set)")
		keysDir     = flag.String("keys-dir", "", "Directory containing node keys")
		genesisDir  = flag.String("genesis-dir", "", "Directory containing genesis component files")
		output      = flag.String("output", "", "Output file path (default: stdout)")
		allocation  = flag.Uint64("allocation", genesis.DefaultAllocationPerValidator, "Allocation per validator in nLUX")
		validators  = flag.Int("validators", 3, "Number of validators for mnemonic-based genesis")
		cchainPath  = flag.String("cchain", "", "Path to existing C-Chain genesis (preserves original)")
		format      = flag.String("format", "pretty", "Output format: json, pretty")
		showHelp    = flag.Bool("help", false, "Show help")
		showVersion = flag.Bool("version", false, "Show version")
	)

	flag.Parse()

	if *showHelp {
		printUsage()
		os.Exit(0)
	}

	if *showVersion {
		fmt.Println("genesis v0.1.0")
		os.Exit(0)
	}

	// Determine network ID
	netID := resolveNetworkID(*network, uint32(*networkID))
	networkIDExplicit := *networkID != 0 // User explicitly provided network ID

	// Build genesis config
	config, err := buildConfig(netID, *keysDir, *genesisDir, *allocation, *validators, *cchainPath, networkIDExplicit)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Output result
	if err := outputConfig(config, *output, *format); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing output: %v\n", err)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`genesis - Generate Lux network genesis configurations

Usage:
  genesis [flags]

Flags:
  -network string     Network name: mainnet, testnet, local, custom (default "local")
  -network-id uint    Network ID (overrides -network if set)
  -keys-dir string    Directory containing node keys (default: ~/.lux/keys)
  -genesis-dir string Directory containing genesis component files
  -output string      Output file path (default: stdout)
  -allocation uint    Allocation per validator in nLUX (default: 1B LUX)
  -validators int     Number of validators for mnemonic-based genesis (default: 3)
  -cchain string      Path to existing C-Chain genesis (preserves original)
  -format string      Output format: json, pretty (default "pretty")
  -help               Show help
  -version            Show version

Environment Variables:
  KEYS_DIR       Directory containing node keys
  MNEMONIC       BIP39 mnemonic for key derivation
  PRIVATE_KEY    Single private key (hex)
  LUX_GENESIS_DIR    Directory containing genesis component files
  LUX_NETWORK_ID     Network ID

Examples:
  # Generate local genesis from keys in ~/.lux/keys
  genesis -network local -output primary.json

  # Generate mainnet genesis preserving original C-Chain
  genesis -network mainnet -cchain mainnet/cchain.json -output primary.json

  # Generate from mnemonic with 3 validators
  MNEMONIC="your mnemonic here" genesis -validators 3 -output primary.json

  # Load from existing genesis directory
  genesis -genesis-dir ./mainnet -output primary.json`)
}

func resolveNetworkID(network string, explicitID uint32) uint32 {
	if explicitID != 0 {
		return explicitID
	}

	switch strings.ToLower(network) {
	case "mainnet", "main":
		return constants.MainnetID
	case "testnet", "test":
		return constants.TestnetID
	case "devnet", "dev":
		return constants.DevnetID
	case "local":
		return constants.LocalID
	default:
		return constants.LocalID
	}
}

func buildConfig(networkID uint32, keysDir, genesisDir string, allocation uint64, validators int, cchainPath string, networkIDExplicit bool) (*genesis.Config, error) {
	var config *genesis.Config
	var err error

	// Try loading from genesis directory first if provided
	if genesisDir != "" {
		config, err = genesis.GetConfigFromDir(genesisDir)
		if err == nil {
			// Only override networkID if explicitly specified by user
			if networkIDExplicit {
				config.NetworkID = networkID
			}
			return maybePreserveCChain(config, cchainPath)
		}
		// If genesis dir specified but failed, report error
		return nil, fmt.Errorf("failed to load from genesis dir %s: %w", genesisDir, err)
	}

	// Try environment-based config
	config, err = genesis.BuildConfigFromEnv(networkID, validators, allocation)
	if err == nil {
		return maybePreserveCChain(config, cchainPath)
	}

	// Try keys directory
	if keysDir == "" {
		home, _ := os.UserHomeDir()
		keysDir = filepath.Join(home, ".lux", "keys")
	}

	config, err = genesis.BuildConfigFromKeys(networkID, keysDir, allocation)
	if err != nil {
		return nil, fmt.Errorf("failed to build config: %w (try setting KEYS_DIR, MNEMONIC, or PRIVATE_KEY)", err)
	}

	return maybePreserveCChain(config, cchainPath)
}

func maybePreserveCChain(config *genesis.Config, cchainPath string) (*genesis.Config, error) {
	if cchainPath == "" {
		return config, nil
	}

	// Load and preserve original C-Chain genesis
	cchainData, err := os.ReadFile(cchainPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read C-Chain genesis from %s: %w", cchainPath, err)
	}

	config.CChainGenesis = string(cchainData)
	return config, nil
}

func outputConfig(config *genesis.Config, outputPath, format string) error {
	var data []byte
	var err error

	if format == "pretty" {
		data, err = json.MarshalIndent(config, "", "  ")
	} else {
		data, err = json.Marshal(config)
	}

	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Add newline
	data = append(data, '\n')

	if outputPath == "" {
		_, err = os.Stdout.Write(data)
		return err
	}

	return os.WriteFile(outputPath, data, 0644)
}
