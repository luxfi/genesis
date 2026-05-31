// Copyright (C) 2019-2025, Lux Partners Limited. All rights reserved.
// See the file LICENSE for licensing terms.

package genesis

import (
	"encoding/json"
	"fmt"

	"github.com/luxfi/constants"
	"github.com/luxfi/ids"
)

// Config is the top-level genesis configuration
type Config struct {
	NetworkID                  uint32        `json:"networkID"`
	Allocations                []Allocation  `json:"allocations"`
	StartTime                  uint64        `json:"startTime"`
	InitialStakeDuration       uint64        `json:"initialStakeDuration"`
	InitialStakeDurationOffset uint64        `json:"initialStakeDurationOffset"`
	InitialStakedFunds         []ids.ShortID `json:"initialStakedFunds"`
	InitialStakers             []Staker      `json:"initialStakers"`
	XChainGenesis              string        `json:"xChainGenesis,omitempty"` // X-Chain: UTXO Exchange VM (small JSON: {"symbol":"LUX","name":"Lux","denomination":9}); empty → skip X-Chain
	CChainGenesis              string        `json:"cChainGenesis"`
	DChainGenesis              string        `json:"dChainGenesis,omitempty"` // D-Chain: DEX VM genesis
	QChainGenesis              string        `json:"qChainGenesis,omitempty"` // Q-Chain: Quantum VM genesis
	AChainGenesis              string        `json:"aChainGenesis,omitempty"` // A-Chain: Attestation/AI VM genesis
	BChainGenesis              string        `json:"bChainGenesis,omitempty"` // B-Chain: Bridge VM genesis
	TChainGenesis              string        `json:"tChainGenesis,omitempty"` // T-Chain: Threshold VM genesis
	ZChainGenesis              string        `json:"zChainGenesis,omitempty"` // Z-Chain: ZK VM genesis
	GChainGenesis              string        `json:"gChainGenesis,omitempty"` // G-Chain: Graph VM genesis
	KChainGenesis              string        `json:"kChainGenesis,omitempty"` // K-Chain: KMS VM genesis
	Message                    string        `json:"message"`

	// Chains defines additional chains to include in genesis beyond the
	// built-in alphabet chains (C, D, Q, B, T, Z, G, K). Each entry becomes
	// a CreateChainTx in the P-Chain genesis. This allows chains to be
	// embedded directly for automatic bootstrap.
	Chains []ChainEntry `json:"chains,omitempty"`

	// ChainMapping provides dynamic chain ID configuration per network.
	// This allows chains (P/X/C/Q/etc.) to have network-specific IDs that
	// can be migrated/upgraded over time via governance.
	ChainMapping *ChainMapping `json:"chainMapping,omitempty"`

	// SecurityProfile is the genesis-level pin for this chain's locked
	// ChainSecurityProfile (consensus/config). Carries the ProfileID
	// byte plus the SHA3-384 content hash of the canonical profile.
	// luxfi/node loads this at startup and refuses to boot if the live
	// canonical profile content doesn't match the pinned hash.
	//
	// Optional in JSON so legacy genesis files keep parsing; node
	// bootstrap warns + refuses strict-PQ features when absent.
	// Closes red-team finding F102.
	SecurityProfile *SecurityProfile `json:"securityProfile,omitempty"`
}

// Allocation represents a genesis allocation.
//
// EVMAddr is the 20-byte H160 EVM address (C-Chain and other EVM chains).
// UTXOAddr is the 20-byte ShortID used by both P-Chain and X-Chain UTXOs
//   (same bytes; bech32 prefix differentiates the chain).
type Allocation struct {
	EVMAddr        ids.ShortID    `json:"evmAddr"`
	UTXOAddr       ids.ShortID    `json:"utxoAddr"`
	InitialAmount  uint64         `json:"initialAmount"`
	UnlockSchedule []LockedAmount `json:"unlockSchedule"`
}

// LockedAmount represents a time-locked amount
type LockedAmount struct {
	Amount   uint64 `json:"amount"`
	Locktime uint64 `json:"locktime"`
}

// Staker represents an initial validator
type Staker struct {
	NodeID        ids.NodeID         `json:"nodeID"`
	RewardAddress ids.ShortID        `json:"rewardAddress"`
	DelegationFee uint32             `json:"delegationFee"`
	Signer        *ProofOfPossession `json:"signer,omitempty"`
	PQIdentity    *PQIdentity        `json:"pqIdentity,omitempty"`
	// Weight is the explicit validator stake weight (optional, derived from allocations if not set)
	Weight uint64 `json:"weight,omitempty"`
	// StartTime is the Unix timestamp when staking begins (optional)
	StartTime uint64 `json:"startTime,omitempty"`
	// EndTime is the Unix timestamp when staking ends (optional)
	EndTime uint64 `json:"endTime,omitempty"`
}

// ProofOfPossession contains BLS signature data
type ProofOfPossession struct {
	PublicKey         string `json:"publicKey"`
	ProofOfPossession string `json:"proofOfPossession"`
}

// PQIdentity binds post-quantum keys to a validator's BLS consensus identity.
// ML-DSA certificates prove BLS/Corona key ownership — if BLS is broken by
// a quantum computer, the ML-DSA certificates still prove validator identity.
type PQIdentity struct {
	// ML-DSA public key (FIPS 204, Dilithium — 1952 bytes hex-encoded)
	MLDSAPublicKey string `json:"mldsaPublicKey"`
	// ML-DSA signature over the BLS public key — proves BLS key belongs to this ML-DSA identity
	BLSCertificate string `json:"blsCertificate"`
	// Corona public key for ring signatures (33 bytes hex-encoded)
	CoronaPublicKey string `json:"coronaPublicKey,omitempty"`
	// ML-DSA signature over the Corona public key
	CoronaCertificate string `json:"coronaCertificate,omitempty"`
}

// Bootstrapper represents a bootstrap node
type Bootstrapper struct {
	ID string `json:"id"`
	IP string `json:"ip"`
}

// StakingConfig contains staking parameters
type StakingConfig struct {
	UptimeRequirement float64      `json:"uptimeRequirement"`
	MinValidatorStake uint64       `json:"minValidatorStake"`
	MaxValidatorStake uint64       `json:"maxValidatorStake"`
	MinDelegatorStake uint64       `json:"minDelegatorStake"`
	MinDelegationFee  uint32       `json:"minDelegationFee"`
	MinStakeDuration  uint64       `json:"minStakeDuration"` // seconds
	MaxStakeDuration  uint64       `json:"maxStakeDuration"` // seconds
	RewardConfig      RewardConfig `json:"rewardConfig"`
}

// RewardConfig contains reward parameters
type RewardConfig struct {
	MaxConsumptionRate uint64 `json:"maxConsumptionRate"`
	MinConsumptionRate uint64 `json:"minConsumptionRate"`
	MintingPeriod      uint64 `json:"mintingPeriod"` // seconds
	SupplyCap          uint64 `json:"supplyCap"`
}

// TxFeeConfig contains transaction fee parameters
type TxFeeConfig struct {
	TxFee            uint64 `json:"txFee"`
	CreateAssetTxFee uint64 `json:"createAssetTxFee"`
}

// NetworkConfig contains network-level configuration (network.json)
type NetworkConfig struct {
	NetworkID uint32 `json:"networkID"`
	StartTime uint64 `json:"startTime"`
	Message   string `json:"message"`
}

// PChainConfig contains P-Chain specific configuration (pchain.json)
type PChainConfig struct {
	Allocations                []AllocationJSON `json:"allocations"`
	InitialStakeDuration       uint64           `json:"initialStakeDuration"`
	InitialStakeDurationOffset uint64           `json:"initialStakeDurationOffset"`
	InitialStakedFunds         []string         `json:"initialStakedFunds"`
	InitialStakers             []StakerJSON     `json:"initialStakers"`
}

// AllocationJSON is the JSON representation of an allocation.
//
// Address fields use the canonical UTXO/EVM split:
//   - utxoAddr (bech32, P-/X- prefix interchangeable) — P-Chain + X-Chain UTXO owner
//   - evmAddr (0x H160) — C-Chain and other EVM chain owner
//
// One name pair, one shape. luxd consumers must be on a build that reads
// these canonical names; no legacy luxAddr/ethAddr alias is emitted or
// accepted.
type AllocationJSON struct {
	EVMAddr        string         `json:"evmAddr"`
	UTXOAddr       string         `json:"utxoAddr"`
	InitialAmount  uint64         `json:"initialAmount"`
	UnlockSchedule []LockedAmount `json:"unlockSchedule"`
}

// StakerJSON is the JSON representation of a staker
type StakerJSON struct {
	NodeID        string             `json:"nodeID"`
	RewardAddress string             `json:"rewardAddress"`
	DelegationFee uint32             `json:"delegationFee"`
	Signer        *ProofOfPossession `json:"signer,omitempty"`
	PQIdentity    *PQIdentity        `json:"pqIdentity,omitempty"`
	// Weight is the explicit validator stake weight (optional)
	Weight uint64 `json:"weight,omitempty"`
	// StartTime is the Unix timestamp when staking begins (optional)
	StartTime uint64 `json:"startTime,omitempty"`
	// EndTime is the Unix timestamp when staking ends (optional)
	EndTime uint64 `json:"endTime,omitempty"`
}


// CChainConfig is the C-Chain genesis configuration
type CChainConfig struct {
	Config     CChainParams       `json:"config"`
	Nonce      string             `json:"nonce"`
	Timestamp  string             `json:"timestamp"`
	ExtraData  string             `json:"extraData"`
	GasLimit   string             `json:"gasLimit"`
	Difficulty string             `json:"difficulty"`
	MixHash    string             `json:"mixHash"`
	Coinbase   string             `json:"coinbase"`
	Alloc      map[string]Balance `json:"alloc"`
	Number     string             `json:"number"`
	GasUsed    string             `json:"gasUsed"`
	ParentHash string             `json:"parentHash"`
}

// CChainParams contains C-Chain config parameters
type CChainParams struct {
	ChainID                 uint64      `json:"chainId"`
	HomesteadBlock          uint64      `json:"homesteadBlock"`
	EIP150Block             uint64      `json:"eip150Block"`
	EIP155Block             uint64      `json:"eip155Block"`
	EIP158Block             uint64      `json:"eip158Block"`
	ByzantiumBlock          uint64      `json:"byzantiumBlock"`
	ConstantinopleBlock     uint64      `json:"constantinopleBlock"`
	PetersburgBlock         uint64      `json:"petersburgBlock"`
	IstanbulBlock           uint64      `json:"istanbulBlock"`
	MuirGlacierBlock        uint64      `json:"muirGlacierBlock"`
	BerlinBlock             uint64      `json:"berlinBlock"`
	LondonBlock             uint64      `json:"londonBlock"`
	ArrowGlacierBlock       uint64      `json:"arrowGlacierBlock,omitempty"`
	GrayGlacierBlock        uint64      `json:"grayGlacierBlock,omitempty"`
	MergeNetsplitBlock      uint64      `json:"mergeNetsplitBlock,omitempty"`
	ShanghaiTime            uint64      `json:"shanghaiTime,omitempty"`
	CancunTime              uint64      `json:"cancunTime,omitempty"`
	TerminalTotalDifficulty uint64      `json:"terminalTotalDifficulty,omitempty"`
	EVMTimestamp            uint64      `json:"evmTimestamp,omitempty"`
	DurangoTimestamp        uint64      `json:"durangoTimestamp,omitempty"`
	EtnaTimestamp           uint64      `json:"etnaTimestamp,omitempty"`
	FeeConfig               FeeConfig   `json:"feeConfig"`
	WarpConfig              *WarpConfig `json:"warpConfig,omitempty"`
}

// FeeConfig contains EVM fee parameters
type FeeConfig struct {
	GasLimit                 uint64 `json:"gasLimit"`
	TargetBlockRate          uint64 `json:"targetBlockRate"`
	MinBaseFee               uint64 `json:"minBaseFee"`
	TargetGas                uint64 `json:"targetGas"`
	BaseFeeChangeDenominator uint64 `json:"baseFeeChangeDenominator"`
	MinBlockGasCost          uint64 `json:"minBlockGasCost"`
	MaxBlockGasCost          uint64 `json:"maxBlockGasCost"`
	BlockGasCostStep         uint64 `json:"blockGasCostStep"`
}

// WarpConfig contains warp messaging configuration
type WarpConfig struct {
	BlockTimestamp               uint64 `json:"blockTimestamp"`
	QuorumNumerator              uint64 `json:"quorumNumerator"`
	RequirePrimaryNetworkSigners bool   `json:"requirePrimaryNetworkSigners"`
}

// ChainEntry defines an additional chain to create at genesis.
// Used for embedding chains directly in the P-Chain genesis.
type ChainEntry struct {
	VMID        string `json:"vmID"`        // VM ID (base58 encoded)
	Name        string `json:"name"`        // Human-readable chain name
	GenesisData string `json:"genesisData"` // Genesis JSON (string-encoded)
}

// Balance represents an EVM account balance
type Balance struct {
	Balance string `json:"balance"`
}

// ConfigOutput is the JSON output format for genesis configuration
// This matches what luxd expects for primary network genesis
type ConfigOutput struct {
	NetworkID                  uint32           `json:"networkID"`
	Allocations                []AllocationJSON `json:"allocations"`
	StartTime                  uint64           `json:"startTime"`
	InitialStakeDuration       uint64           `json:"initialStakeDuration"`
	InitialStakeDurationOffset uint64           `json:"initialStakeDurationOffset"`
	InitialStakedFunds         []string         `json:"initialStakedFunds"`
	InitialStakers             []StakerJSON     `json:"initialStakers"`
	XChainGenesis              string           `json:"xChainGenesis,omitempty"` // X-Chain: UTXO Exchange VM (small JSON: {"symbol":"LUX","name":"Lux","denomination":9}); empty → skip X-Chain
	CChainGenesis              string           `json:"cChainGenesis"`
	DChainGenesis              string           `json:"dChainGenesis,omitempty"` // D-Chain: DEX VM genesis
	QChainGenesis              string           `json:"qChainGenesis,omitempty"` // Q-Chain: Quantum VM genesis
	AChainGenesis              string           `json:"aChainGenesis,omitempty"` // A-Chain: Attestation/AI VM genesis
	BChainGenesis              string           `json:"bChainGenesis,omitempty"` // B-Chain: Bridge VM genesis
	TChainGenesis              string           `json:"tChainGenesis,omitempty"` // T-Chain: Threshold VM genesis
	ZChainGenesis              string           `json:"zChainGenesis,omitempty"` // Z-Chain: ZK VM genesis
	GChainGenesis              string           `json:"gChainGenesis,omitempty"` // G-Chain: Graph VM genesis
	KChainGenesis              string           `json:"kChainGenesis,omitempty"` // K-Chain: KMS VM genesis
	Chains                     []ChainEntry     `json:"chains,omitempty"`        // Additional genesis chains
	SecurityProfile            *SecurityProfile `json:"securityProfile,omitempty"`
	Message                    string           `json:"message"`
}

// UnparsedConfig is an alias for ConfigOutput (for backward compatibility)
type UnparsedConfig = ConfigOutput

// ToJSON converts Config to ConfigOutput for JSON marshaling
func (c *Config) ToJSON(hrp string) *ConfigOutput {
	if hrp == "" {
		hrp = "lux"
	}

	allocations := make([]AllocationJSON, 0, len(c.Allocations))
	for _, a := range c.Allocations {
		utxoAddr, _ := PChainPrefix.Format(hrp, a.UTXOAddr[:])
		allocations = append(allocations, AllocationJSON{
			EVMAddr:        fmt.Sprintf("0x%s", a.EVMAddr.Hex()),
			UTXOAddr:       utxoAddr,
			InitialAmount:  a.InitialAmount,
			UnlockSchedule: a.UnlockSchedule,
		})
	}

	stakedFunds := make([]string, 0, len(c.InitialStakedFunds))
	for _, addr := range c.InitialStakedFunds {
		s, _ := PChainPrefix.Format(hrp, addr[:])
		stakedFunds = append(stakedFunds, s)
	}

	stakers := make([]StakerJSON, 0, len(c.InitialStakers))
	for _, s := range c.InitialStakers {
		rewardAddr, _ := PChainPrefix.Format(hrp, s.RewardAddress[:])
		stakers = append(stakers, StakerJSON{
			NodeID:        s.NodeID.String(),
			RewardAddress: rewardAddr,
			DelegationFee: s.DelegationFee,
			Signer:        s.Signer,
			PQIdentity:    s.PQIdentity,
		})
	}

	return &ConfigOutput{
		NetworkID:                  c.NetworkID,
		Allocations:                allocations,
		StartTime:                  c.StartTime,
		InitialStakeDuration:       c.InitialStakeDuration,
		InitialStakeDurationOffset: c.InitialStakeDurationOffset,
		InitialStakedFunds:         stakedFunds,
		InitialStakers:             stakers,
		XChainGenesis:              c.XChainGenesis,
		CChainGenesis:              c.CChainGenesis,
		DChainGenesis:              c.DChainGenesis,
		QChainGenesis:              c.QChainGenesis,
		AChainGenesis:              c.AChainGenesis,
		BChainGenesis:              c.BChainGenesis,
		TChainGenesis:              c.TChainGenesis,
		ZChainGenesis:              c.ZChainGenesis,
		GChainGenesis:              c.GChainGenesis,
		KChainGenesis:              c.KChainGenesis,
		SecurityProfile:            c.SecurityProfile,
		Message:                    c.Message,
	}
}

// MarshalJSON implements json.Marshaler for Config
func (c *Config) MarshalJSON() ([]byte, error) {
	hrp := constants.GetHRP(c.NetworkID)
	return json.Marshal(c.ToJSON(hrp))
}

