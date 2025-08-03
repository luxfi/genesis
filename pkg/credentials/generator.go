package credentials

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"time"

	"github.com/luxfi/crypto/bls"
	"github.com/luxfi/genesis/pkg/core"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/staking"
	"github.com/ava-labs/avalanchego/utils/perms"
)

// Generator creates staking credentials
type Generator struct {
	// Can add options here if needed
}

// NewGenerator creates a new credential generator
func NewGenerator() *Generator {
	return &Generator{}
}

// Generate creates new staking credentials
func (g *Generator) Generate() (*core.StakingCredentials, error) {
	// Generate TLS certificate
	cert, key, err := g.generateTLSCert()
	if err != nil {
		return nil, fmt.Errorf("failed to generate TLS cert: %w", err)
	}

	// Parse certificate to get NodeID
	tlsCert, err := x509.ParseCertificate(cert)
	if err != nil {
		return nil, fmt.Errorf("failed to parse certificate: %w", err)
	}

	// Create staking certificate
	stakingCert := &staking.Certificate{
		Raw:       cert,
		PublicKey: tlsCert.PublicKey,
	}

	nodeID := ids.NodeIDFromCert(stakingCert)

	// Generate BLS key
	blsKey, err := bls.NewSecretKey()
	if err != nil {
		return nil, fmt.Errorf("failed to generate BLS key: %w", err)
	}

	blsPubKey := bls.PublicFromSecretKey(blsKey)
	blsPubKeyBytes := bls.PublicKeyToCompressedBytes(blsPubKey)
	blsSecretKey := bls.SecretKeyToBytes(blsKey)

	// Create proof of possession - sign the compressed public key bytes
	pop := bls.Sign(blsKey, blsPubKeyBytes)
	popBytes := bls.SignatureToBytes(pop)

	return &core.StakingCredentials{
		NodeID:            nodeID.String(),
		Certificate:       cert,
		PrivateKey:        key,
		BLSSecretKey:      blsSecretKey,
		BLSPublicKey:      blsPubKeyBytes,
		ProofOfPossession: popBytes,
	}, nil
}

// Save writes credentials to disk
func (g *Generator) Save(creds *core.StakingCredentials, baseDir string) error {
	stakingDir := filepath.Join(baseDir, "staking")
	if err := os.MkdirAll(stakingDir, perms.ReadWriteExecute); err != nil {
		return fmt.Errorf("failed to create staking directory: %w", err)
	}

	// Save TLS certificate
	certPath := filepath.Join(stakingDir, "staker.crt")
	certPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: creds.Certificate,
	})
	if err := os.WriteFile(certPath, certPEM, perms.ReadOnly); err != nil {
		return fmt.Errorf("failed to write certificate: %w", err)
	}

	// Save TLS private key
	keyPath := filepath.Join(stakingDir, "staker.key")
	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: creds.PrivateKey,
	})
	if err := os.WriteFile(keyPath, keyPEM, perms.ReadOnly); err != nil {
		return fmt.Errorf("failed to write private key: %w", err)
	}

	// Save BLS key
	signerPath := filepath.Join(stakingDir, "signer.key")
	if err := os.WriteFile(signerPath, creds.BLSSecretKey, perms.ReadOnly); err != nil {
		return fmt.Errorf("failed to write BLS key: %w", err)
	}

	return nil
}

func (g *Generator) generateTLSCert() (cert, key []byte, err error) {
	// Generate RSA key
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}

	// Create certificate template
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"Lux Labs"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
	}

	// Generate certificate
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return nil, nil, err
	}

	// Encode private key
	keyDER, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return nil, nil, err
	}

	return certDER, keyDER, nil
}
