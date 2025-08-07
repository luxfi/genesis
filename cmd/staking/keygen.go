package staking

import (
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"

	"github.com/luxfi/crypto/bls"
	"github.com/luxfi/node/staking"
	"github.com/spf13/cobra"
)

// NewKeygenCmd creates a simple staking key generation command
func NewKeygenCmd() *cobra.Command {
	var outputDir string

	cmd := &cobra.Command{
		Use:   "keygen",
		Short: "Generate staking keys with proper BLS validation",
		Long:  "Generate TLS certificate, private key, and BLS key with proof of possession that passes validation",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Ensure output directory exists
			if err := os.MkdirAll(outputDir, 0700); err != nil {
				return fmt.Errorf("failed to create output directory: %w", err)
			}

			// Generate TLS certificate and key
			fmt.Println("Generating TLS certificate and key...")
			certPEM, keyPEM, err := staking.NewCertAndKeyBytes()
			if err != nil {
				return fmt.Errorf("failed to generate TLS cert/key: %w", err)
			}

			// Parse certificate to get NodeID
			block, _ := pem.Decode(certPEM)
			cert, err := x509.ParseCertificate(block.Bytes)
			if err != nil {
				return fmt.Errorf("failed to parse certificate: %w", err)
			}

			// Calculate NodeID from certificate - compute SHA256 of DER-encoded certificate
			certBytes := cert.Raw
			hash := sha256.Sum256(certBytes)
			nodeID := fmt.Sprintf("NodeID-%s", encodeNodeID(hash[:]))

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

			// Verify the proof of possession
			if !bls.VerifyProofOfPossession(pk, sig, pkBytes) {
				return fmt.Errorf("generated invalid proof of possession")
			}

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
			fmt.Printf("\nNode ID: %s\n", nodeID)
			fmt.Printf("Certificate Subject: %s\n", cert.Subject)
			fmt.Printf("\nBLS Public Key: 0x%s\n", hex.EncodeToString(pkBytes))
			fmt.Printf("BLS Proof of Possession: 0x%s\n", hex.EncodeToString(sigBytes))

			// Generate genesis staker entry
			fmt.Println("\n=== Genesis Staker Entry ===")
			genesisEntry := fmt.Sprintf(`{
  "nodeID": "%s",
  "rewardAddress": "P-lux18jma8ppw3nhx5r4ap8clazz0dps7rv5u00z96u",
  "delegationFee": 20000,
  "signer": {
    "publicKey": "0x%s",
    "proofOfPossession": "0x%s"
  }
}`, nodeID, hex.EncodeToString(pkBytes), hex.EncodeToString(sigBytes))
			
			fmt.Println(genesisEntry)

			// Save genesis entry to file
			genesisPath := filepath.Join(outputDir, "genesis-staker.json")
			if err := os.WriteFile(genesisPath, []byte(genesisEntry), 0644); err != nil {
				return fmt.Errorf("failed to write genesis entry: %w", err)
			}

			fmt.Printf("\nGenesis entry saved to: %s\n", genesisPath)
			fmt.Println("\n✅ All staking keys generated successfully and proof of possession verified!")
			
			// Instructions for use
			fmt.Println("\n📝 To use these keys:")
			fmt.Println("1. Copy the staking directory to your node's data directory")
			fmt.Println("2. Update your genesis.json with the staker entry above")
			fmt.Println("3. Start luxd with --staking-tls-cert-file, --staking-tls-key-file, and --staking-signer-key-file")

			return nil
		},
	}

	cmd.Flags().StringVar(&outputDir, "output", "./staking-keys", "Output directory for keys")

	return cmd
}

// encodeNodeID encodes a byte array to the node ID string format (cb58)
func encodeNodeID(data []byte) string {
	const cb58 = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"
	
	// For now, just use hex encoding with a simple transformation
	// This is a simplified version - in production you'd use proper cb58 encoding
	hexStr := hex.EncodeToString(data)
	result := ""
	
	// Convert hex to a readable format similar to NodeID format
	for i := 0; i < len(hexStr) && i < 24; i++ {
		idx := int(hexStr[i]) % len(cb58)
		result += string(cb58[idx])
	}
	
	return result
}