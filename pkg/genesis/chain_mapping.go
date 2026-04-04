// Copyright (C) 2019-2025, Lux Partners Limited. All rights reserved.
// See the file LICENSE for licensing terms.

package genesis

import (
	"encoding/json"
	"fmt"

	"github.com/luxfi/ids"
)

// ChainRole represents the role/purpose of a chain in the network
type ChainRole string

const (
	RolePlatform  ChainRole = "P" // Platform/staking chain
	RoleExchange  ChainRole = "X" // Exchange/asset transfer chain
	RoleContract  ChainRole = "C" // Contract/EVM chain
	RoleQuantum   ChainRole = "Q" // Quantum-resistant chain
	RoleAttest    ChainRole = "A" // Attestation chain
	RoleBridge    ChainRole = "B" // Bridge chain
	RoleThreshold ChainRole = "T" // Threshold/MPC chain
	RoleZK        ChainRole = "Z" // Zero-knowledge chain
	RoleGraph     ChainRole = "G" // Graph/data chain (future)
	RoleIdentity  ChainRole = "I" // Identity chain (future)
	RoleKMS       ChainRole = "K" // KMS chain (future)
)

// ChainConfig represents a single chain's configuration
type ChainConfig struct {
	// ChainID is the current active chain ID (can change via migration)
	ChainID ids.ID `json:"chainID"`

	// VMID identifies which VM runs this chain
	VMID ids.ID `json:"vmID"`

	// Role indicates the chain's purpose (P/X/C/Q/etc.)
	Role ChainRole `json:"role"`

	// Name is the human-readable chain name
	Name string `json:"name"`

	// Aliases for RPC/API routing
	Aliases []string `json:"aliases,omitempty"`

	// EVMChainID is the EVM chain ID (only for EVM chains)
	EVMChainID uint64 `json:"evmChainID,omitempty"`

	// Genesis is the chain-specific genesis data
	Genesis json.RawMessage `json:"genesis,omitempty"`

	// PreviousChainIDs tracks historical chain IDs for migration
	PreviousChainIDs []ChainIDHistory `json:"previousChainIDs,omitempty"`
}

// ChainIDHistory records a historical chain ID and when it was active
type ChainIDHistory struct {
	ChainID   ids.ID `json:"chainID"`
	StartTime uint64 `json:"startTime"`
	EndTime   uint64 `json:"endTime"`
}

// ChainMapping maps chain roles to their configurations
// This allows dynamic chain ID assignment and migration
type ChainMapping struct {
	// Version of the chain mapping schema
	Version uint32 `json:"version"`

	// Chains maps role -> chain config
	Chains map[ChainRole]*ChainConfig `json:"chains"`

	// MigrationPending indicates if a migration is in progress
	MigrationPending *ChainMigration `json:"migrationPending,omitempty"`
}

// ChainMigration represents a pending chain ID migration
type ChainMigration struct {
	// Role being migrated
	Role ChainRole `json:"role"`

	// OldChainID is the current chain ID
	OldChainID ids.ID `json:"oldChainID"`

	// NewChainID is the target chain ID
	NewChainID ids.ID `json:"newChainID"`

	// ActivationHeight is when the migration becomes active
	ActivationHeight uint64 `json:"activationHeight"`

	// ProposalID links to governance proposal
	ProposalID ids.ID `json:"proposalID,omitempty"`
}

// NewChainMapping creates a default chain mapping for a network
func NewChainMapping(networkID uint32) *ChainMapping {
	return &ChainMapping{
		Version: 1,
		Chains:  make(map[ChainRole]*ChainConfig),
	}
}

// GetChainByRole returns the chain config for a given role
func (m *ChainMapping) GetChainByRole(role ChainRole) (*ChainConfig, error) {
	if chain, ok := m.Chains[role]; ok {
		return chain, nil
	}
	return nil, fmt.Errorf("chain role %s not found", role)
}

// GetChainByID returns the chain config with the given chain ID
func (m *ChainMapping) GetChainByID(chainID ids.ID) (*ChainConfig, ChainRole, error) {
	for role, chain := range m.Chains {
		if chain.ChainID == chainID {
			return chain, role, nil
		}
	}
	return nil, "", fmt.Errorf("chain ID %s not found", chainID)
}

// RegisterChain adds or updates a chain in the mapping
func (m *ChainMapping) RegisterChain(role ChainRole, config *ChainConfig) error {
	if config.Role != role {
		return fmt.Errorf("chain config role %s doesn't match registration role %s", config.Role, role)
	}
	m.Chains[role] = config
	return nil
}

// ProposeMigration initiates a chain ID migration
func (m *ChainMapping) ProposeMigration(role ChainRole, newChainID ids.ID, activationHeight uint64, proposalID ids.ID) error {
	chain, err := m.GetChainByRole(role)
	if err != nil {
		return err
	}

	if m.MigrationPending != nil {
		return fmt.Errorf("migration already pending for role %s", m.MigrationPending.Role)
	}

	m.MigrationPending = &ChainMigration{
		Role:             role,
		OldChainID:       chain.ChainID,
		NewChainID:       newChainID,
		ActivationHeight: activationHeight,
		ProposalID:       proposalID,
	}
	return nil
}

// ApplyMigration applies a pending migration at the given height
func (m *ChainMapping) ApplyMigration(currentHeight uint64) error {
	if m.MigrationPending == nil {
		return nil // No pending migration
	}

	if currentHeight < m.MigrationPending.ActivationHeight {
		return nil // Not yet time to migrate
	}

	chain, err := m.GetChainByRole(m.MigrationPending.Role)
	if err != nil {
		return err
	}

	// Record the old chain ID in history
	history := ChainIDHistory{
		ChainID:   chain.ChainID,
		StartTime: 0, // Would need to track from chain state
		EndTime:   currentHeight,
	}
	chain.PreviousChainIDs = append(chain.PreviousChainIDs, history)

	// Apply the new chain ID
	chain.ChainID = m.MigrationPending.NewChainID

	// Clear pending migration
	m.MigrationPending = nil

	return nil
}

// DefaultMainnetMapping returns the default chain mapping for mainnet
func DefaultMainnetMapping() *ChainMapping {
	return &ChainMapping{
		Version: 1,
		Chains: map[ChainRole]*ChainConfig{
			RolePlatform: {
				ChainID: ids.PChainID,
				VMID:    ids.Empty, // Platform VM ID
				Role:    RolePlatform,
				Name:    "Platform Chain",
				Aliases: []string{"P", "platform"},
			},
			RoleExchange: {
				ChainID: ids.XChainID,
				VMID:    ids.Empty, // Exchange VM ID
				Role:    RoleExchange,
				Name:    "Exchange Chain",
				Aliases: []string{"X", "exchange"},
			},
			RoleContract: {
				ChainID:    ids.CChainID,
				VMID:       ids.Empty, // EVM VM ID
				Role:       RoleContract,
				Name:       "Contract Chain",
				Aliases:    []string{"C", "contract", "evm"},
				EVMChainID: 96369, // Mainnet EVM chain ID
			},
			RoleQuantum: {
				ChainID: ids.QChainID,
				VMID:    ids.Empty, // Quantum VM ID
				Role:    RoleQuantum,
				Name:    "Quantum Chain",
				Aliases: []string{"Q", "quantum", "pq"},
			},
			RoleAttest: {
				ChainID: ids.AChainID,
				VMID:    ids.Empty, // AI/Attestation VM ID
				Role:    RoleAttest,
				Name:    "Attestation Chain",
				Aliases: []string{"A", "attest", "ai"},
			},
			RoleBridge: {
				ChainID: ids.BChainID,
				VMID:    ids.Empty, // Bridge VM ID
				Role:    RoleBridge,
				Name:    "Bridge Chain",
				Aliases: []string{"B", "bridge"},
			},
			RoleThreshold: {
				ChainID: ids.TChainID,
				VMID:    ids.Empty, // Threshold VM ID
				Role:    RoleThreshold,
				Name:    "Threshold Chain",
				Aliases: []string{"T", "threshold", "mpc"},
			},
			RoleZK: {
				ChainID: ids.ZChainID,
				VMID:    ids.Empty, // ZK VM ID
				Role:    RoleZK,
				Name:    "Zero-Knowledge Chain",
				Aliases: []string{"Z", "zk", "zkvm"},
			},
			RoleGraph: {
				ChainID: ids.GChainID,
				VMID:    ids.Empty, // Graph VM ID
				Role:    RoleGraph,
				Name:    "Graph Chain",
				Aliases: []string{"G", "graph", "dgraph"},
			},
			RoleKMS: {
				ChainID: ids.KChainID,
				VMID:    ids.Empty, // KMS VM ID
				Role:    RoleKMS,
				Name:    "KMS Chain",
				Aliases: []string{"K", "kms"},
			},
		},
	}
}

// MarshalJSON implements custom JSON marshaling
func (m *ChainMapping) MarshalJSON() ([]byte, error) {
	type Alias ChainMapping
	return json.Marshal(&struct {
		*Alias
	}{
		Alias: (*Alias)(m),
	})
}

// UnmarshalJSON implements custom JSON unmarshaling
func (m *ChainMapping) UnmarshalJSON(data []byte) error {
	type Alias ChainMapping
	aux := &struct {
		*Alias
	}{
		Alias: (*Alias)(m),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	return nil
}
