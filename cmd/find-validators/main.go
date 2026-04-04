package main

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/luxfi/ids"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: find-validators <dir> [nodeID-to-find...]")
		os.Exit(1)
	}

	baseDir := os.Args[1]
	targetNodeIDs := map[string]bool{}
	for i := 2; i < len(os.Args); i++ {
		targetNodeIDs[os.Args[i]] = true
	}

	// Find all staker.crt files
	matches := []string{}
	err := filepath.Walk(baseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.Name() == "staker.crt" {
			matches = append(matches, path)
		}
		return nil
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error walking directory: %v\n", err)
		os.Exit(1)
	}

	sort.Strings(matches)

	fmt.Printf("Found %d staker.crt files\n\n", len(matches))

	for _, certPath := range matches {
		nodeID, err := getNodeIDFromCert(certPath)
		if err != nil {
			fmt.Printf("%-60s ERROR: %v\n", certPath, err)
			continue
		}

		nodeIDStr := nodeID.String()
		if len(targetNodeIDs) == 0 {
			// Print all
			fmt.Printf("%-60s %s\n", certPath, nodeIDStr)
		} else if targetNodeIDs[nodeIDStr] {
			// Only print matches
			fmt.Printf("MATCH: %s\n  Path: %s\n\n", nodeIDStr, certPath)
		}
	}
}

func getNodeIDFromCert(certPath string) (ids.NodeID, error) {
	certPEM, err := os.ReadFile(certPath)
	if err != nil {
		return ids.NodeID{}, err
	}

	block, _ := pem.Decode(certPEM)
	if block == nil {
		return ids.NodeID{}, fmt.Errorf("no PEM block found")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return ids.NodeID{}, err
	}

	stakingCert := &ids.Certificate{
		Raw:       cert.Raw,
		PublicKey: cert.PublicKey,
	}

	return ids.NodeIDFromCert(stakingCert), nil
}
