package main

import (
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"

	"github.com/luxfi/crypto/bls"
	"github.com/luxfi/crypto/staking"
)

func main() {
	outputDir := "./staking-keys"
	if len(os.Args) > 1 {
		outputDir = os.Args[1]
	}

	// Ensure output directory exists
	if err := os.MkdirAll(outputDir, 0700); err != nil {
		panic(fmt.Errorf("failed to create output directory: %w", err))
	}

	// Generate TLS certificate and key using luxfi/crypto/staking
	fmt.Println("Generating TLS certificate and key...")
	certPEM, keyPEM, err := staking.NewCertAndKeyBytes()
	if err != nil {
		panic(fmt.Errorf("failed to generate TLS cert/key: %w", err))
	}

	// Parse certificate to get details
	block, _ := pem.Decode(certPEM)
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		panic(fmt.Errorf("failed to parse certificate: %w", err))
	}

	// Write TLS certificate
	certPath := filepath.Join(outputDir, "staker.crt")
	if err := os.WriteFile(certPath, certPEM, 0644); err != nil {
		panic(fmt.Errorf("failed to write certificate: %w", err))
	}

	// Write TLS private key
	keyPath := filepath.Join(outputDir, "staker.key")
	if err := os.WriteFile(keyPath, keyPEM, 0600); err != nil {
		panic(fmt.Errorf("failed to write private key: %w", err))
	}

	// Generate BLS key
	fmt.Println("Generating BLS key with proof of possession...")
	sk, err := bls.NewSecretKey()
	if err != nil {
		panic(fmt.Errorf("failed to generate BLS secret key: %w", err))
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
		panic(fmt.Errorf("failed to write BLS key: %w", err))
	}

	// Display results
	fmt.Println("\n=== Generated Staking Keys ===")
	fmt.Printf("Output directory: %s\n", outputDir)
	fmt.Printf("TLS Certificate: %s\n", certPath)
	fmt.Printf("TLS Private Key: %s\n", keyPath)  
	fmt.Printf("BLS Private Key: %s\n", blsPath)
	fmt.Printf("\nCertificate Subject: %s\n", cert.Subject)
	fmt.Printf("Certificate Serial: %s\n", cert.SerialNumber)
	fmt.Printf("\nBLS Public Key (hex): 0x%s\n", hex.EncodeToString(pkBytes))
	fmt.Printf("BLS Proof of Possession (hex): 0x%s\n", hex.EncodeToString(sigBytes))

	// Generate genesis staker entry
	fmt.Println("\n=== Genesis Staker Entry ===")
	fmt.Printf(`{
  "nodeID": "NodeID-CHANGEME",
  "rewardAddress": "P-lux18jma8ppw3nhx5r4ap8clazz0dps7rv5u00z96u",
  "delegationFee": 20000,
  "signer": {
    "publicKey": "0x%s",
    "proofOfPossession": "0x%s"
  }
}
`, hex.EncodeToString(pkBytes), hex.EncodeToString(sigBytes))

	fmt.Println("\n✅ All staking keys generated successfully!")
	fmt.Println("\nIMPORTANT: You need to calculate the NodeID from the TLS certificate!")
}