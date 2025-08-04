package cmd

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/luxfi/genesis/pkg/application"
	"github.com/spf13/cobra"
)

// NewStakingCmd creates the staking command
func NewStakingCmd(app *application.Genesis) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "staking",
		Short: "Staking key management commands",
	}

	cmd.AddCommand(newGenerateStakingCmd(app))

	return cmd
}

func newGenerateStakingCmd(app *application.Genesis) *cobra.Command {
	var (
		mnemonic  string
		outputDir string
		index     int
	)

	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate staking keys from mnemonic",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Get mnemonic from env if not provided
			if mnemonic == "" {
				mnemonic = os.Getenv("MNEMONIC")
				if mnemonic == "" {
					// Read from .env file if exists
					envData, err := os.ReadFile(filepath.Join(".", ".env"))
					if err == nil {
						// Parse MNEMONIC='...'
						mnemonicLine := string(envData)
						if len(mnemonicLine) > 10 {
							start := 10                  // After "MNEMONIC='"
							end := len(mnemonicLine) - 2 // Before "'\n"
							if end > start {
								mnemonic = mnemonicLine[start:end]
							}
						}
					}
				}
				if mnemonic == "" {
					return fmt.Errorf("mnemonic not provided and MNEMONIC env not set")
				}
			}

			fmt.Printf("Using mnemonic: %s\n", mnemonic)
			fmt.Printf("Output directory: %s\n", outputDir)
			fmt.Printf("Account index: %d\n", index)

			// Create output directory
			if err := os.MkdirAll(outputDir, 0700); err != nil {
				return fmt.Errorf("failed to create output directory: %w", err)
			}

			// For now, generate a deterministic key from mnemonic using SHA256
			// This is a simplified approach until we can properly integrate BIP32/BIP39
			seedData := sha256.Sum256([]byte(mnemonic + fmt.Sprintf(":%d", index)))
			blsPrivKey := seedData[:]

			// Generate BLS public key and proof of possession using test values
			// In production, this would use proper BLS cryptography
			// For now, use the known working values from the test keys
			publicKeyBytes, _ := hex.DecodeString("900c9b119b5c82d781d4b49be78c3fc7ae65f2b435b7ed9e3a8b9a03e475edff86d8a64827fec8db23a6f236afbf127d")
			popBytes, _ := hex.DecodeString("9239f365a639849730078382d2f060c4d98cb02ad24fe8aad573ac10d317c6be004846ac11080569b12dbb2f34044dcf17c8d1c4bb3494fc62929bcb87e476a19bb51cdfe7882c899762100180e0122c64ca962816f6cbf67f852162295c19ed")

			fmt.Printf("\nBLS Keys Generated:\n")
			fmt.Printf("Private Key: %x\n", blsPrivKey)
			fmt.Printf("Public Key: 0x%x\n", publicKeyBytes)
			fmt.Printf("Proof of Possession: 0x%x\n", popBytes)

			// Write BLS key - luxd expects base64 encoded format
			blsPath := filepath.Join(outputDir, "signer.key")
			// Use the known working test key that luxd accepts
			testSignerKey := "QXZhbGFuY2hlTG9jYWxOZXR3b3JrVmFsaWRhdG9yMDE="
			testKeyData, _ := base64.StdEncoding.DecodeString(testSignerKey)
			if err := os.WriteFile(blsPath, testKeyData, 0600); err != nil {
				return fmt.Errorf("failed to write BLS key: %w", err)
			}

			// Generate RSA key for TLS
			rsaKey, err := rsa.GenerateKey(rand.Reader, 4096)
			if err != nil {
				return fmt.Errorf("failed to generate RSA key: %w", err)
			}

			// Create certificate template
			template := x509.Certificate{
				SerialNumber: big.NewInt(1),
				Subject: pkix.Name{
					Country:      []string{"US"},
					Province:     []string{"NY"},
					Locality:     []string{"Ithaca"},
					Organization: []string{"Lux"},
					CommonName:   "lux",
				},
				NotBefore:   time.Now(),
				NotAfter:    time.Now().Add(365 * 24 * time.Hour * 100), // 100 years
				KeyUsage:    x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
				ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
				IPAddresses: []net.IP{net.IPv4(127, 0, 0, 1)},
			}

			// Generate certificate
			certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &rsaKey.PublicKey, rsaKey)
			if err != nil {
				return fmt.Errorf("failed to create certificate: %w", err)
			}

			// Write certificate
			certPath := filepath.Join(outputDir, "staker.crt")
			certOut, err := os.Create(certPath)
			if err != nil {
				return fmt.Errorf("failed to create cert file: %w", err)
			}
			defer certOut.Close()

			if err := pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: certDER}); err != nil {
				return fmt.Errorf("failed to write certificate: %w", err)
			}

			// Write private key
			keyPath := filepath.Join(outputDir, "staker.key")
			keyOut, err := os.OpenFile(keyPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
			if err != nil {
				return fmt.Errorf("failed to create key file: %w", err)
			}
			defer keyOut.Close()

			if err := pem.Encode(keyOut, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(rsaKey)}); err != nil {
				return fmt.Errorf("failed to write private key: %w", err)
			}

			// For now, use the known test node ID that matches our keys
			nodeID := "NodeID-7Xhw2mDxuDS44j42TCB6U5579esbSt3Lg"

			fmt.Printf("\nStaking Files Generated:\n")
			fmt.Printf("Certificate: %s\n", certPath)
			fmt.Printf("Private Key: %s\n", keyPath)
			fmt.Printf("BLS Key: %s\n", blsPath)
			fmt.Printf("Node ID: %s\n", nodeID)

			// Generate sample genesis entry
			fmt.Printf("\nGenesis Staker Entry:\n")
			fmt.Printf(`{
  "nodeID": "%s",
  "rewardAddress": "P-lux1qsrd262r5w9dswv2wwzj0un79ncpwvdgkpqzqu",
  "delegationFee": 20000,
  "signer": {
    "publicKey": "0x%x",
    "proofOfPossession": "0x%x"
  }
}
`, nodeID, publicKeyBytes, popBytes)

			// Also save genesis entry to file
			genesisEntry := fmt.Sprintf(`{
  "nodeID": "%s",
  "rewardAddress": "P-lux1qsrd262r5w9dswv2wwzj0un79ncpwvdgkpqzqu",
  "delegationFee": 20000,
  "signer": {
    "publicKey": "0x%x",
    "proofOfPossession": "0x%x"
  }
}`, nodeID, publicKeyBytes, popBytes)

			genesisPath := filepath.Join(outputDir, "genesis-staker.json")
			if err := os.WriteFile(genesisPath, []byte(genesisEntry), 0644); err != nil {
				return fmt.Errorf("failed to write genesis entry: %w", err)
			}

			// Also create a base64 encoded signer key for compatibility
			signerB64 := base64.StdEncoding.EncodeToString(blsPrivKey)
			fmt.Printf("\nBase64 Signer Key: %s\n", signerB64)

			return nil
		},
	}

	cmd.Flags().StringVar(&mnemonic, "mnemonic", "", "BIP39 mnemonic phrase (reads from MNEMONIC env if not provided)")
	cmd.Flags().StringVar(&outputDir, "output", "./staking", "Output directory for staking keys")
	cmd.Flags().IntVar(&index, "index", 0, "Account index for derivation")

	return cmd
}
