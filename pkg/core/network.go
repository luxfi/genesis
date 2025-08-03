package core

// Network represents ANY network configuration - no special cases
type Network struct {
	Name      string                 `json:"name"`
	NetworkID uint64                 `json:"network_id"`
	ChainID   uint64                 `json:"chain_id"`
	Nodes     int                    `json:"nodes"`
	Genesis   GenesisConfig          `json:"genesis"`
	Consensus ConsensusConfig        `json:"consensus"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// GenesisConfig specifies how genesis is created
type GenesisConfig struct {
	Source      string            `json:"source"`      // "fresh", "import", "extract"
	ImportPath  string            `json:"import_path,omitempty"`
	Allocations map[string]uint64 `json:"allocations,omitempty"`
	Message     string            `json:"message,omitempty"`
}

// ConsensusConfig specifies consensus parameters
type ConsensusConfig struct {
	K     int `json:"k"`     // Sample size
	Alpha int `json:"alpha"` // Quorum
	Beta  int `json:"beta"`  // Decision threshold
}

// NodeInfo contains information about a network node
type NodeInfo struct {
	ID          string
	Port        int
	StakingPort int
	DataDir     string
	Credentials *StakingCredentials
}

// StakingCredentials contains all staking-related credentials
type StakingCredentials struct {
	NodeID            string
	Certificate       []byte
	PrivateKey        []byte
	BLSSecretKey      []byte
	BLSPublicKey      []byte
	ProofOfPossession []byte
}

// Validate ensures the network configuration is valid
func (n *Network) Validate() error {
	if n.Name == "" {
		return ErrInvalidConfig("network name required")
	}
	if n.NetworkID == 0 {
		return ErrInvalidConfig("network ID required")
	}
	if n.ChainID == 0 {
		return ErrInvalidConfig("chain ID required")
	}
	return nil
}

// Normalize applies defaults and derives values
func (n *Network) Normalize() {
	if n.Nodes == 0 {
		n.Nodes = 1
	}
	
	// Default consensus for node count
	if n.Consensus.K == 0 {
		if n.Nodes == 1 {
			n.Consensus = ConsensusConfig{K: 1, Alpha: 1, Beta: 1}
		} else {
			k := (n.Nodes + 1) / 2
			n.Consensus = ConsensusConfig{K: k, Alpha: k, Beta: n.Nodes}
		}
	}
	
	// Default genesis message
	if n.Genesis.Message == "" {
		n.Genesis.Message = n.Name + " Network"
	}
}