// Package unified provides the single, composable genesis generation system
package unified

import (
	"fmt"

	"github.com/luxfi/genesis/pkg/core"
)

// GenesisBuilder is the composable interface for all genesis operations
type GenesisBuilder[T any] interface {
	// Core operations
	Generate() (T, error)
	Validate(T) error
	Export(T, string) error
	
	// Composable operations
	With(opts ...Option[T]) GenesisBuilder[T]
	Transform(fn TransformFunc[T]) GenesisBuilder[T]
}

// Option is a functional option for configuring genesis
type Option[T any] func(*builderConfig[T]) error

// TransformFunc allows transforming genesis data
type TransformFunc[T any] func(T) (T, error)

// builderConfig holds configuration for genesis building
type builderConfig[T any] struct {
	network    core.Network
	validators []core.Validator
	allocations map[string]uint64
	transforms []TransformFunc[T]
	metadata   map[string]interface{}
}

// Chain-specific genesis types
type (
	PChainGenesis struct {
		NetworkID                  uint32                    `json:"networkID"`
		Allocations                []core.Allocation         `json:"allocations"`
		StartTime                  uint64                    `json:"startTime"`
		InitialStakeDuration       uint64                    `json:"initialStakeDuration"`
		InitialStakeDurationOffset uint64                    `json:"initialStakeDurationOffset"`
		InitialStakedFunds         []string                  `json:"initialStakedFunds"`
		InitialStakers             []core.InitialStaker      `json:"initialStakers"`
		CChainGenesis              string                    `json:"cChainGenesis"`
		Message                    string                    `json:"message"`
	}
	
	CChainGenesis struct {
		Config     core.ChainConfig          `json:"config"`
		Nonce      string                    `json:"nonce"`
		Timestamp  string                    `json:"timestamp"`
		ExtraData  string                    `json:"extraData"`
		GasLimit   string                    `json:"gasLimit"`
		Difficulty string                    `json:"difficulty"`
		MixHash    string                    `json:"mixHash"`
		Coinbase   string                    `json:"coinbase"`
		Alloc      map[string]core.Account   `json:"alloc"`
		Number     string                    `json:"number"`
		GasUsed    string                    `json:"gasUsed"`
		ParentHash string                    `json:"parentHash"`
	}
	
	XChainGenesis struct {
		NetworkID     uint32                    `json:"networkID"`
		InitialSupply uint64                    `json:"initialSupply"`
		Allocations   []core.XChainAllocation   `json:"allocations"`
	}
	
	UnifiedGenesis struct {
		P *PChainGenesis `json:"pChain"`
		C *CChainGenesis `json:"cChain"`
		X *XChainGenesis `json:"xChain"`
	}
)

// NewPChainBuilder creates a new P-Chain genesis builder
func NewPChainBuilder(network core.Network) GenesisBuilder[*PChainGenesis] {
	return &pChainBuilder{
		config: &builderConfig[*PChainGenesis]{
			network: network,
		},
	}
}

// NewCChainBuilder creates a new C-Chain genesis builder
func NewCChainBuilder(network core.Network) GenesisBuilder[*CChainGenesis] {
	return &cChainBuilder{
		config: &builderConfig[*CChainGenesis]{
			network: network,
		},
	}
}

// NewXChainBuilder creates a new X-Chain genesis builder
func NewXChainBuilder(network core.Network) GenesisBuilder[*XChainGenesis] {
	return &xChainBuilder{
		config: &builderConfig[*XChainGenesis]{
			network: network,
		},
	}
}

// NewUnifiedBuilder creates a builder for all three chains
func NewUnifiedBuilder(network core.Network) GenesisBuilder[*UnifiedGenesis] {
	return &unifiedBuilder{
		config: &builderConfig[*UnifiedGenesis]{
			network: network,
		},
		pBuilder: NewPChainBuilder(network),
		cBuilder: NewCChainBuilder(network),
		xBuilder: NewXChainBuilder(network),
	}
}

// Common options
func WithValidators[T any](validators []core.Validator) Option[T] {
	return func(cfg *builderConfig[T]) error {
		cfg.validators = validators
		return nil
	}
}

func WithAllocations[T any](allocations map[string]uint64) Option[T] {
	return func(cfg *builderConfig[T]) error {
		cfg.allocations = allocations
		return nil
	}
}

func WithMetadata[T any](key string, value interface{}) Option[T] {
	return func(cfg *builderConfig[T]) error {
		if cfg.metadata == nil {
			cfg.metadata = make(map[string]interface{})
		}
		cfg.metadata[key] = value
		return nil
	}
}