package staking

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"time"
)

// GenerateStakingKeys generates TLS and BLS keys for a validator
func GenerateStakingKeys(outputDir, nodeID string) error {
	// Create output directory
	if err := os.MkdirAll(outputDir, 0700); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
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
			Organization: []string{"Luxfi"},
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

	// Generate BLS signer key (32 bytes)
	blsKey := make([]byte, 32)
	if _, err := rand.Read(blsKey); err != nil {
		return fmt.Errorf("failed to generate BLS key: %w", err)
	}

	// Write BLS key
	blsPath := filepath.Join(outputDir, "signer.key")
	if err := os.WriteFile(blsPath, blsKey, 0600); err != nil {
		return fmt.Errorf("failed to write BLS key: %w", err)
	}

	fmt.Printf("Generated staking keys in %s\n", outputDir)
	fmt.Printf("NodeID: %s\n", nodeID)
	fmt.Printf("BLS Key: %x\n", blsKey)

	return nil
}

// ComputeProofOfPossession computes the proof of possession for a BLS key
func ComputeProofOfPossession(blsKeyHex string) (string, string, error) {
	// Decode the BLS key
	blsKey, err := hex.DecodeString(blsKeyHex)
	if err != nil {
		return "", "", fmt.Errorf("failed to decode BLS key: %w", err)
	}

	// For a simple implementation, we'll compute:
	// publicKey = SHA256(blsKey || "public")
	// proofOfPossession = SHA256(blsKey || "proof")

	// Compute public key
	pubKeyData := append(blsKey, []byte("public")...)
	pubKeyHash := sha256.Sum256(pubKeyData)
	publicKey := hex.EncodeToString(pubKeyHash[:])

	// Compute proof of possession
	popData := append(blsKey, []byte("proof")...)
	popHash := sha256.Sum256(popData)
	proofOfPossession := hex.EncodeToString(popHash[:])

	return publicKey, proofOfPossession, nil
}

// GenerateNodeIDFromCert generates a node ID from a certificate file
func GenerateNodeIDFromCert(certPath string) (string, error) {
	// Read certificate
	certPEM, err := os.ReadFile(certPath)
	if err != nil {
		return "", fmt.Errorf("failed to read certificate: %w", err)
	}

	// Parse PEM
	block, _ := pem.Decode(certPEM)
	if block == nil {
		return "", fmt.Errorf("failed to parse PEM block")
	}

	// Parse certificate
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return "", fmt.Errorf("failed to parse certificate: %w", err)
	}

	// Compute hash of public key
	pubKeyDER, err := x509.MarshalPKIXPublicKey(cert.PublicKey)
	if err != nil {
		return "", fmt.Errorf("failed to marshal public key: %w", err)
	}

	hash := sha256.Sum256(pubKeyDER)

	// Convert to node ID format
	// This is a simplified version - the actual implementation would use proper base58 encoding
	nodeID := fmt.Sprintf("NodeID-%x", hash[:6])

	return nodeID, nil
}
