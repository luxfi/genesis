package pchain

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/luxfi/node/ids"
)

// GenesisConfig represents P-Chain genesis configuration
type GenesisConfig struct {
	NetworkID     uint32                 `json:"networkID"`
	Allocations   []Allocation          `json:"allocations"`
	StartTime     uint64                `json:"startTime"`
	InitialStakedFunds []string         `json:"initialStakedFunds"`
	InitialStakers []Staker             `json:"initialStakers"`
	CChainGenesis string                `json:"cChainGenesis"`
	InitialStakeDuration uint64        `json:"initialStakeDuration"`
	InitialStakeDurationOffset uint64   `json:"initialStakeDurationOffset"`
	Message       string                `json:"message"`
}

// Allocation represents an initial token allocation
type Allocation struct {
	ETHAddr        string           `json:"ethAddr"`
	LUXAddr        string           `json:"luxAddr"`
	InitialAmount  uint64           `json:"initialAmount"`
	UnlockSchedule []LockedAmount   `json:"unlockSchedule"`
}

// LockedAmount represents tokens locked until a specific time
type LockedAmount struct {
	Amount   uint64 `json:"amount"`
	Locktime uint64 `json:"locktime"`
}

// Staker represents an initial validator
type Staker struct {
	NodeID         string                 `json:"nodeID"`
	RewardAddress  string                 `json:"rewardAddress"`
	DelegationFee  uint32                 `json:"delegationFee"`
	Signer         *Signer               `json:"signer,omitempty"`
}

// Signer contains BLS signing information
type Signer struct {
	PublicKey         string `json:"publicKey"`
	ProofOfPossession string `json:"proofOfPossession"`
}

// GenerateMainnetGenesis creates the P-Chain genesis for mainnet
func GenerateMainnetGenesis(outputDir string, nodeID string, blsPubKey, blsPOP []byte) error {
	// Create directory structure
	pDir := filepath.Join(outputDir, "P")
	cDir := filepath.Join(outputDir, "C") 
	xDir := filepath.Join(outputDir, "X")
	
	for _, dir := range []string{pDir, cDir, xDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	// Load C-Chain genesis
	cGenesisPath := filepath.Join(cDir, "genesis.json")
	cGenesisData, err := os.ReadFile(cGenesisPath)
	if err != nil {
		return fmt.Errorf("failed to read C-Chain genesis: %w", err)
	}

	// Create minimal P-Chain genesis for mainnet replay
	// This matches the expected format from node/genesis/genesis.go
	config := &GenesisConfig{
		NetworkID:     96369,
		StartTime:     1750460293, // Match C-Chain timestamp
		InitialStakeDuration: 31536000, // 1 year in seconds
		InitialStakeDurationOffset: 0,
		Message:       "lux mainnet genesis",
		CChainGenesis: string(cGenesisData),
		Allocations: []Allocation{
			{
				ETHAddr: "0x9011E888251AB053B7bD1cdB598Db4f9DEd94714",
				LUXAddr: "X-lux1w6ajywx2t9wfqej7ddxk9v0ej3qtxs5p6f7q9",
				InitialAmount: 500000000000000000, // 500M LUX
				UnlockSchedule: []LockedAmount{
					{
						Amount:   500000000000000000,
						Locktime: 0,
					},
				},
			},
		},
		InitialStakedFunds: []string{
			"X-lux1w6ajywx2t9wfqej7ddxk9v0ej3qtxs5p6f7q9",
		},
		InitialStakers: []Staker{
			{
				NodeID:        nodeID,
				RewardAddress: "X-lux1w6ajywx2t9wfqej7ddxk9v0ej3qtxs5p6f7q9",
				DelegationFee: 20000, // 2%
				Signer: &Signer{
					PublicKey:         fmt.Sprintf("0x%x", blsPubKey),
					ProofOfPossession: fmt.Sprintf("0x%x", blsPOP),
				},
			},
		},
	}

	// Write P-Chain genesis
	pGenesisPath := filepath.Join(pDir, "genesis.json")
	pGenesisData, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal P-Chain genesis: %w", err)
	}
	if err := os.WriteFile(pGenesisPath, pGenesisData, 0644); err != nil {
		return fmt.Errorf("failed to write P-Chain genesis: %w", err)
	}

	// Create minimal X-Chain genesis
	xGenesis := map[string]interface{}{
		"networkID": 96369,
		"initialSupply": 500000000000000000,
		"allocations": []map[string]interface{}{
			{
				"address": "X-lux1w6ajywx2t9wfqej7ddxk9v0ej3qtxs5p6f7q9",
				"balance": 500000000000000000,
			},
		},
	}
	
	xGenesisPath := filepath.Join(xDir, "genesis.json")
	xGenesisData, err := json.MarshalIndent(xGenesis, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal X-Chain genesis: %w", err)
	}
	if err := os.WriteFile(xGenesisPath, xGenesisData, 0644); err != nil {
		return fmt.Errorf("failed to write X-Chain genesis: %w", err)
	}

	return nil
}

// GenerateGenesisFiles creates all genesis files for a network
func GenerateGenesisFiles(networkID uint32, outputDir string, nodeInfo map[string]interface{}) error {
	switch networkID {
	case 96369: // Mainnet
		nodeID, _ := nodeInfo["nodeID"].(string)
		blsPubKey, _ := nodeInfo["blsPublicKey"].([]byte)
		blsPOP, _ := nodeInfo["blsProofOfPossession"].([]byte)
		return GenerateMainnetGenesis(outputDir, nodeID, blsPubKey, blsPOP)
	default:
		return fmt.Errorf("unsupported network ID: %d", networkID)
	}
}