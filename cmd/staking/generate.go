package staking

import (
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"

	"github.com/luxfi/crypto/bls"
	"github.com/luxfi/crypto/staking"
	"github.com/spf13/cobra"
)

// NewGenerateCmd creates the staking key generation command
func NewGenerateCmd() *cobra.Command {
	var outputDir string

	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate complete staking keys (TLS cert/key and BLS key)",
		Long:  "Generate TLS certificate, private key, and BLS key with proof of possession for staking",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Ensure output directory exists
			if err := os.MkdirAll(outputDir, 0700); err != nil {
				return fmt.Errorf("failed to create output directory: %w", err)
			}

			// Generate TLS certificate and key using luxfi/crypto/staking
			fmt.Println("Generating TLS certificate and key...")
			certPEM, keyPEM, err := staking.NewCertAndKeyBytes()
			if err != nil {
				return fmt.Errorf("failed to generate TLS cert/key: %w", err)
			}

			// Parse certificate to get details
			block, _ := pem.Decode(certPEM)
			cert, err := x509.ParseCertificate(block.Bytes)
			if err != nil {
				return fmt.Errorf("failed to parse certificate: %w", err)
			}

			// Update the certificate subject to use Lux Industries
			// Note: We need to regenerate with proper subject
			// For now, let's save what we have and show how to update it
			
			// Write TLS certificate
			certPath := filepath.Join(outputDir, "staker.crt")
			if err := os.WriteFile(certPath, certPEM, 0644); err != nil {
				return fmt.Errorf("failed to write certificate: %w", err)
			}

			// Write TLS private key
			keyPath := filepath.Join(outputDir, "staker.key")
			if err := os.WriteFile(keyPath, keyPEM, 0600); err != nil {
				return fmt.Errorf("failed to write private key: %w", err)
			}

			// Generate BLS key
			fmt.Println("Generating BLS key with proof of possession...")
			sk, err := bls.NewSecretKey()
			if err != nil {
				return fmt.Errorf("failed to generate BLS secret key: %w", err)
			}

			// Get public key
			pk := sk.PublicKey()
			pkBytes := bls.PublicKeyToCompressedBytes(pk)

			// Sign the public key as proof of possession
			sig := sk.SignProofOfPossession(pkBytes)
			sigBytes := bls.SignatureToBytes(sig)

			// Write BLS secret key
			skBytes := bls.SecretKeyToBytes(sk)
			blsPath := filepath.Join(outputDir, "signer.key")
			if err := os.WriteFile(blsPath, skBytes, 0600); err != nil {
				return fmt.Errorf("failed to write BLS key: %w", err)
			}

			// Display results
			fmt.Println("\n=== Generated Staking Keys ===")
			fmt.Printf("Output directory: %s\n", outputDir)
			fmt.Printf("TLS Certificate: %s\n", certPath)
			fmt.Printf("TLS Private Key: %s\n", keyPath)
			fmt.Printf("BLS Private Key: %s\n", blsPath)
			fmt.Printf("\nCertificate Subject: %s\n", cert.Subject)
			fmt.Printf("Certificate Serial: %s\n", cert.SerialNumber)
			fmt.Printf("\nBLS Public Key (hex): %s\n", hex.EncodeToString(pkBytes))
			fmt.Printf("BLS Proof of Possession (hex): %s\n", hex.EncodeToString(sigBytes))

			// Save the BLS values for updating genesis
			fmt.Println("\n=== For Updating node/genesis/genesis.go FromDatabase() ===")
			fmt.Println("Replace the Signer field with:")
			fmt.Println("Signer: &signer.ProofOfPossession{")
			
			// PublicKey field
			fmt.Print("\tPublicKey: [48]byte{")
			for i, b := range pkBytes {
				if i%12 == 0 {
					fmt.Printf("\n\t\t")
				}
				fmt.Printf("0x%02x", b)
				if i < len(pkBytes)-1 {
					fmt.Print(", ")
				}
			}
			fmt.Println(",\n\t},")
			
			// ProofOfPossession field
			fmt.Print("\tProofOfPossession: [96]byte{")
			for i, b := range sigBytes {
				if i%12 == 0 {
					fmt.Printf("\n\t\t")
				}
				fmt.Printf("0x%02x", b)
				if i < len(sigBytes)-1 {
					fmt.Print(", ")
				}
			}
			fmt.Println(",\n\t},")
			fmt.Println("},")

			fmt.Println("\n✅ All staking keys generated successfully!")
			return nil
		},
	}

	cmd.Flags().StringVar(&outputDir, "output", "runs/lux-mainnet-replay/staking", "Output directory for keys")

	return cmd
}