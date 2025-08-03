package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/luxfi/genesis/pkg/staking"
	"github.com/spf13/cobra"
)

var stakingCmd = &cobra.Command{
	Use:   "staking",
	Short: "Staking key management commands",
	Long:  `Commands for generating and managing validator staking keys and proof of possession`,
}

var generateKeysCmd = &cobra.Command{
	Use:   "generate-keys",
	Short: "Generate new staking keys (TLS and BLS)",
	Long:  `Generate a new set of staking keys including TLS certificate/key and BLS signer key`,
	RunE: func(cmd *cobra.Command, args []string) error {
		outputDir, _ := cmd.Flags().GetString("output")
		nodeID, _ := cmd.Flags().GetString("node-id")
		
		if outputDir == "" {
			outputDir = "./staking-keys"
		}
		
		return staking.GenerateStakingKeys(outputDir, nodeID)
	},
}

var computePOPCmd = &cobra.Command{
	Use:   "compute-pop",
	Short: "Compute proof of possession from BLS key",
	Long:  `Compute the public key and proof of possession from a BLS private key`,
	RunE: func(cmd *cobra.Command, args []string) error {
		keyFile, _ := cmd.Flags().GetString("key-file")
		keyHex, _ := cmd.Flags().GetString("key-hex")
		
		if keyFile != "" {
			// Read key from file
			keyBytes, err := os.ReadFile(keyFile)
			if err != nil {
				return fmt.Errorf("failed to read key file: %w", err)
			}
			keyHex = fmt.Sprintf("%x", keyBytes)
		}
		
		if keyHex == "" {
			return fmt.Errorf("either --key-file or --key-hex must be provided")
		}
		
		publicKey, proofOfPossession, err := staking.ComputeProofOfPossession(keyHex)
		if err != nil {
			return err
		}
		
		fmt.Printf("Public Key:         0x%s\n", publicKey)
		fmt.Printf("Proof of Possession: 0x%s\n", proofOfPossession)
		
		return nil
	},
}

var nodeIDCmd = &cobra.Command{
	Use:   "node-id",
	Short: "Get node ID from certificate",
	Long:  `Extract the node ID from a staking certificate`,
	RunE: func(cmd *cobra.Command, args []string) error {
		certFile, _ := cmd.Flags().GetString("cert-file")
		
		if certFile == "" {
			return fmt.Errorf("--cert-file is required")
		}
		
		nodeID, err := staking.GenerateNodeIDFromCert(certFile)
		if err != nil {
			return err
		}
		
		fmt.Printf("NodeID: %s\n", nodeID)
		return nil
	},
}

func init() {
	// Add flags to generate-keys
	generateKeysCmd.Flags().String("output", "", "Output directory for keys")
	generateKeysCmd.Flags().String("node-id", "", "Node ID (optional)")
	
	// Add flags to compute-pop
	computePOPCmd.Flags().String("key-file", "", "Path to BLS key file")
	computePOPCmd.Flags().String("key-hex", "", "BLS key in hex format")
	
	// Add flags to node-id
	nodeIDCmd.Flags().String("cert-file", "", "Path to certificate file")
	
	// Add subcommands
	stakingCmd.AddCommand(generateKeysCmd)
	stakingCmd.AddCommand(computePOPCmd)
	stakingCmd.AddCommand(nodeIDCmd)
	
	rootCmd.AddCommand(stakingCmd)
}