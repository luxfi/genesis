// Copyright (C) 2019-2025, Lux Partners Limited. All rights reserved.
// See the file LICENSE for licensing terms.

// Command regenerate-genesis creates a new genesis file using the running node
// certificates while preserving the exact cChainGenesis string to maintain
// C-Chain genesis hash compatibility.
package main

import (
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/luxfi/crypto/bls"
	"github.com/luxfi/ids"
	luxtls "github.com/luxfi/tls"
)

// GenesisConfig represents the genesis JSON structure
type GenesisConfig struct {
	Allocations                []interface{} `json:"allocations"`
	CChainGenesis              string        `json:"cChainGenesis"`
	InitialStakeDuration       uint64        `json:"initialStakeDuration"`
	InitialStakeDurationOffset uint64        `json:"initialStakeDurationOffset"`
	InitialStakedFunds         []string      `json:"initialStakedFunds"`
	InitialStakers             []Staker      `json:"initialStakers"`
	Message                    string        `json:"message"`
	NetworkID                  uint32        `json:"networkID"`
	StartTime                  uint64        `json:"startTime"`
}

// Staker represents an initial validator
type Staker struct {
	DelegationFee uint32       `json:"delegationFee"`
	NodeID        string       `json:"nodeID"`
	RewardAddress string       `json:"rewardAddress"`
	Signer        *SignerProof `json:"signer"`
	Weight        uint64       `json:"weight"`
}

// SignerProof contains BLS signature proof
type SignerProof struct {
	ProofOfPossession string `json:"proofOfPossession"`
	PublicKey         string `json:"publicKey"`
}

func main() {
	keysDir := flag.String("keys", "", "Directory containing node keys (e.g., ~/.lux/keys)")
	origGenesis := flag.String("genesis", "", "Original genesis file to preserve cChainGenesis from")
	output := flag.String("output", "", "Output file path")
	flag.Parse()

	if *keysDir == "" || *origGenesis == "" {
		fmt.Println("Usage: regenerate-genesis -keys <keys-dir> -genesis <original-genesis.json> -output <new-genesis.json>")
		fmt.Println("\nThis tool regenerates genesis with running node NodeIDs while preserving cChainGenesis")
		os.Exit(1)
	}

	// Load original genesis
	origData, err := os.ReadFile(*origGenesis)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading original genesis: %v\n", err)
		os.Exit(1)
	}

	var genesis GenesisConfig
	if err := json.Unmarshal(origData, &genesis); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing genesis: %v\n", err)
		os.Exit(1)
	}

	// Load node keys
	stakers, err := loadNodeKeys(*keysDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading node keys: %v\n", err)
		os.Exit(1)
	}

	if len(stakers) == 0 {
		fmt.Fprintf(os.Stderr, "No node keys found in %s\n", *keysDir)
		os.Exit(1)
	}

	fmt.Printf("Found %d node keys:\n", len(stakers))
	for _, s := range stakers {
		fmt.Printf("  %s\n", s.NodeID)
	}

	// Replace stakers while keeping everything else
	genesis.InitialStakers = stakers
	genesis.InitialStakedFunds = []string{} // Clear staked funds since we're using new validators

	// Output
	newData, err := json.MarshalIndent(genesis, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error marshaling genesis: %v\n", err)
		os.Exit(1)
	}

	if *output == "" {
		fmt.Println(string(newData))
	} else {
		if err := os.WriteFile(*output, newData, 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing output: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("\nWrote regenerated genesis to: %s\n", *output)
		fmt.Println("\nIMPORTANT: The cChainGenesis field is preserved exactly, so C-Chain genesis hash is unchanged.")
		fmt.Println("The initialStakers have been replaced with the running node certificates.")
	}
}

func loadNodeKeys(keysDir string) ([]Staker, error) {
	entries, err := os.ReadDir(keysDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read keys directory: %w", err)
	}

	var stakers []Staker
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		nodeDir := filepath.Join(keysDir, entry.Name())
		staker, err := loadStakerFromDir(nodeDir)
		if err != nil {
			fmt.Printf("Warning: skipping %s: %v\n", entry.Name(), err)
			continue
		}
		stakers = append(stakers, *staker)
	}

	// Sort by NodeID for consistent ordering
	sort.Slice(stakers, func(i, j int) bool {
		return stakers[i].NodeID < stakers[j].NodeID
	})

	return stakers, nil
}

func loadStakerFromDir(nodeDir string) (*Staker, error) {
	// Try modern path structure first
	certPath := filepath.Join(nodeDir, "staking", "staker.crt")
	keyPath := filepath.Join(nodeDir, "staking", "staker.key")
	signerPath := filepath.Join(nodeDir, "bls", "signer.key")

	// Fallback to legacy paths
	if _, err := os.Stat(certPath); os.IsNotExist(err) {
		certPath = filepath.Join(nodeDir, "staker.crt")
		keyPath = filepath.Join(nodeDir, "staker.key")
		signerPath = filepath.Join(nodeDir, "signer.key")
	}

	// Load staker certificate
	certPEM, err := os.ReadFile(certPath)
	if err != nil {
		return nil, fmt.Errorf("no staker.crt found")
	}

	keyPEM, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("no staker.key found")
	}

	// Get NodeID from certificate
	tlsCert, err := luxtls.LoadTLSCertFromBytes(keyPEM, certPEM)
	if err != nil {
		// Fallback to manual parsing
		block, _ := pem.Decode(certPEM)
		if block == nil {
			return nil, fmt.Errorf("failed to decode certificate PEM")
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("failed to parse certificate: %w", err)
		}
		stakingCert := &ids.Certificate{
			Raw:       cert.Raw,
			PublicKey: cert.PublicKey,
		}
		nodeID := ids.NodeIDFromCert(stakingCert)

		staker := &Staker{
			NodeID:        nodeID.String(),
			DelegationFee: 20000,                                          // 2%
			Weight:        1000000000000000,                               // 1 quadrillion
			RewardAddress: "X-lux1qsrd262r5w9dswv2wwzj0un79ncpwvdgkpqzqu", // placeholder
		}

		// Try to load BLS signer key
		signerData, err := os.ReadFile(signerPath)
		if err == nil && len(signerData) >= 32 {
			sk, err := bls.SecretKeyFromBytes(signerData[:32])
			if err == nil {
				pk := bls.PublicFromSecretKey(sk)
				pkBytes := bls.PublicKeyToCompressedBytes(pk)
				sig := bls.SignProofOfPossession(sk, pkBytes)
				staker.Signer = &SignerProof{
					PublicKey:         "0x" + hex.EncodeToString(pkBytes),
					ProofOfPossession: "0x" + hex.EncodeToString(bls.SignatureToBytes(sig)),
				}
			}
		}

		return staker, nil
	}

	stakingCert := &ids.Certificate{
		Raw:       tlsCert.Leaf.Raw,
		PublicKey: tlsCert.Leaf.PublicKey,
	}
	nodeID := ids.NodeIDFromCert(stakingCert)

	staker := &Staker{
		NodeID:        nodeID.String(),
		DelegationFee: 20000,
		Weight:        1000000000000000,
		RewardAddress: "X-lux1qsrd262r5w9dswv2wwzj0un79ncpwvdgkpqzqu",
	}

	// Load BLS signer key
	signerData, err := os.ReadFile(signerPath)
	if err == nil && len(signerData) >= 32 {
		sk, err := bls.SecretKeyFromBytes(signerData[:32])
		if err == nil {
			pk := bls.PublicFromSecretKey(sk)
			pkBytes := bls.PublicKeyToCompressedBytes(pk)
			sig := bls.SignProofOfPossession(sk, pkBytes)
			staker.Signer = &SignerProof{
				PublicKey:         "0x" + hex.EncodeToString(pkBytes),
				ProofOfPossession: "0x" + hex.EncodeToString(bls.SignatureToBytes(sig)),
			}
		}
	}

	return staker, nil
}
