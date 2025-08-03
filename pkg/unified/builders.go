package unified

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
	
	"github.com/luxfi/genesis/pkg/core"
)

// pChainBuilder implements GenesisBuilder for P-Chain
type pChainBuilder struct {
	config *builderConfig[*PChainGenesis]
}

func (b *pChainBuilder) Generate() (*PChainGenesis, error) {
	genesis := &PChainGenesis{
		NetworkID:                  uint32(b.config.network.NetworkID),
		StartTime:                  uint64(time.Now().Unix()),
		InitialStakeDuration:       31536000, // 1 year
		InitialStakeDurationOffset: 5400,     // 90 minutes
		Message:                    b.config.network.Genesis.Message,
		Allocations:                []core.Allocation{},
		InitialStakedFunds:         []string{},
		InitialStakers:             []core.InitialStaker{},
	}
	
	// Apply allocations
	if b.config.allocations != nil {
		for addr, amount := range b.config.allocations {
			// Implementation would convert addresses and create allocations
			_ = addr
			_ = amount
		}
	}
	
	// Apply validators
	if b.config.validators != nil {
		for _, v := range b.config.validators {
			genesis.InitialStakers = append(genesis.InitialStakers, core.InitialStaker{
				NodeID:        v.NodeID,
				RewardAddress: v.RewardAddress,
				DelegationFee: v.DelegationFee,
			})
		}
	}
	
	// Apply transforms
	result := genesis
	for _, transform := range b.config.transforms {
		var err error
		result, err = transform(result)
		if err != nil {
			return nil, fmt.Errorf("transform failed: %w", err)
		}
	}
	
	return result, nil
}

func (b *pChainBuilder) Validate(genesis *PChainGenesis) error {
	if genesis.NetworkID == 0 {
		return fmt.Errorf("network ID must be non-zero")
	}
	if len(genesis.InitialStakers) == 0 {
		return fmt.Errorf("must have at least one initial staker")
	}
	return nil
}

func (b *pChainBuilder) Export(genesis *PChainGenesis, path string) error {
	data, err := json.MarshalIndent(genesis, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal genesis: %w", err)
	}
	
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}
	
	return os.WriteFile(path, data, 0644)
}

func (b *pChainBuilder) With(opts ...Option[*PChainGenesis]) GenesisBuilder[*PChainGenesis] {
	for _, opt := range opts {
		opt(b.config)
	}
	return b
}

func (b *pChainBuilder) Transform(fn TransformFunc[*PChainGenesis]) GenesisBuilder[*PChainGenesis] {
	b.config.transforms = append(b.config.transforms, fn)
	return b
}

// cChainBuilder implements GenesisBuilder for C-Chain
type cChainBuilder struct {
	config *builderConfig[*CChainGenesis]
}

func (b *cChainBuilder) Generate() (*CChainGenesis, error) {
	genesis := &CChainGenesis{
		Config: core.ChainConfig{
			ChainID:        b.config.network.ChainID,
			HomesteadBlock: 0,
			EIP150Block:    0,
			EIP155Block:    0,
			EIP158Block:    0,
			// ... other EVM config
		},
		Nonce:      "0x0",
		Timestamp:  fmt.Sprintf("0x%x", time.Now().Unix()),
		ExtraData:  "0x00",
		GasLimit:   "0x1C9C380", // 30M
		Difficulty: "0x1",
		MixHash:    "0x0000000000000000000000000000000000000000000000000000000000000000",
		Coinbase:   "0x0000000000000000000000000000000000000000",
		Alloc:      make(map[string]core.Account),
		Number:     "0x0",
		GasUsed:    "0x0",
		ParentHash: "0x0000000000000000000000000000000000000000000000000000000000000000",
	}
	
	// Apply allocations
	if b.config.allocations != nil {
		for addr, amount := range b.config.allocations {
			genesis.Alloc[addr] = core.Account{
				Balance: fmt.Sprintf("%d", amount),
			}
		}
	}
	
	// Apply transforms
	result := genesis
	for _, transform := range b.config.transforms {
		var err error
		result, err = transform(result)
		if err != nil {
			return nil, fmt.Errorf("transform failed: %w", err)
		}
	}
	
	return result, nil
}

func (b *cChainBuilder) Validate(genesis *CChainGenesis) error {
	if genesis.Config.ChainID == 0 {
		return fmt.Errorf("chain ID must be non-zero")
	}
	return nil
}

func (b *cChainBuilder) Export(genesis *CChainGenesis, path string) error {
	data, err := json.MarshalIndent(genesis, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal genesis: %w", err)
	}
	
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}
	
	return os.WriteFile(path, data, 0644)
}

func (b *cChainBuilder) With(opts ...Option[*CChainGenesis]) GenesisBuilder[*CChainGenesis] {
	for _, opt := range opts {
		opt(b.config)
	}
	return b
}

func (b *cChainBuilder) Transform(fn TransformFunc[*CChainGenesis]) GenesisBuilder[*CChainGenesis] {
	b.config.transforms = append(b.config.transforms, fn)
	return b
}

// xChainBuilder implements GenesisBuilder for X-Chain
type xChainBuilder struct {
	config *builderConfig[*XChainGenesis]
}

func (b *xChainBuilder) Generate() (*XChainGenesis, error) {
	genesis := &XChainGenesis{
		NetworkID:     uint32(b.config.network.NetworkID),
		InitialSupply: 0,
		Allocations:   []core.XChainAllocation{},
	}
	
	// Apply allocations
	if b.config.allocations != nil {
		for addr, amount := range b.config.allocations {
			genesis.Allocations = append(genesis.Allocations, core.XChainAllocation{
				Address: addr,
				Balance: amount,
			})
			genesis.InitialSupply += amount
		}
	}
	
	// Apply transforms
	result := genesis
	for _, transform := range b.config.transforms {
		var err error
		result, err = transform(result)
		if err != nil {
			return nil, fmt.Errorf("transform failed: %w", err)
		}
	}
	
	return result, nil
}

func (b *xChainBuilder) Validate(genesis *XChainGenesis) error {
	if genesis.NetworkID == 0 {
		return fmt.Errorf("network ID must be non-zero")
	}
	if genesis.InitialSupply == 0 {
		return fmt.Errorf("initial supply must be non-zero")
	}
	return nil
}

func (b *xChainBuilder) Export(genesis *XChainGenesis, path string) error {
	data, err := json.MarshalIndent(genesis, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal genesis: %w", err)
	}
	
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}
	
	return os.WriteFile(path, data, 0644)
}

func (b *xChainBuilder) With(opts ...Option[*XChainGenesis]) GenesisBuilder[*XChainGenesis] {
	for _, opt := range opts {
		opt(b.config)
	}
	return b
}

func (b *xChainBuilder) Transform(fn TransformFunc[*XChainGenesis]) GenesisBuilder[*XChainGenesis] {
	b.config.transforms = append(b.config.transforms, fn)
	return b
}

// unifiedBuilder builds all three chains together
type unifiedBuilder struct {
	config   *builderConfig[*UnifiedGenesis]
	pBuilder GenesisBuilder[*PChainGenesis]
	cBuilder GenesisBuilder[*CChainGenesis]
	xBuilder GenesisBuilder[*XChainGenesis]
}

func (b *unifiedBuilder) Generate() (*UnifiedGenesis, error) {
	// Generate C-Chain first (needed by P-Chain)
	cGenesis, err := b.cBuilder.With(
		WithAllocations[*CChainGenesis](b.config.allocations),
	).Generate()
	if err != nil {
		return nil, fmt.Errorf("failed to generate C-Chain: %w", err)
	}
	
	// Marshal C-Chain for inclusion in P-Chain
	cData, err := json.Marshal(cGenesis)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal C-Chain: %w", err)
	}
	
	// Generate P-Chain with C-Chain genesis
	pGenesis, err := b.pBuilder.With(
		WithValidators[*PChainGenesis](b.config.validators),
		WithAllocations[*PChainGenesis](b.config.allocations),
		WithMetadata[*PChainGenesis]("cChainGenesis", string(cData)),
	).Generate()
	if err != nil {
		return nil, fmt.Errorf("failed to generate P-Chain: %w", err)
	}
	
	// Generate X-Chain
	xGenesis, err := b.xBuilder.With(
		WithAllocations[*XChainGenesis](b.config.allocations),
	).Generate()
	if err != nil {
		return nil, fmt.Errorf("failed to generate X-Chain: %w", err)
	}
	
	unified := &UnifiedGenesis{
		P: pGenesis,
		C: cGenesis,
		X: xGenesis,
	}
	
	// Apply transforms
	result := unified
	for _, transform := range b.config.transforms {
		var err error
		result, err = transform(result)
		if err != nil {
			return nil, fmt.Errorf("transform failed: %w", err)
		}
	}
	
	return result, nil
}

func (b *unifiedBuilder) Validate(genesis *UnifiedGenesis) error {
	if err := b.pBuilder.Validate(genesis.P); err != nil {
		return fmt.Errorf("P-Chain validation failed: %w", err)
	}
	if err := b.cBuilder.Validate(genesis.C); err != nil {
		return fmt.Errorf("C-Chain validation failed: %w", err)
	}
	if err := b.xBuilder.Validate(genesis.X); err != nil {
		return fmt.Errorf("X-Chain validation failed: %w", err)
	}
	return nil
}

func (b *unifiedBuilder) Export(genesis *UnifiedGenesis, basePath string) error {
	// Export each chain to its directory
	if err := b.pBuilder.Export(genesis.P, filepath.Join(basePath, "P", "genesis.json")); err != nil {
		return fmt.Errorf("failed to export P-Chain: %w", err)
	}
	if err := b.cBuilder.Export(genesis.C, filepath.Join(basePath, "C", "genesis.json")); err != nil {
		return fmt.Errorf("failed to export C-Chain: %w", err)
	}
	if err := b.xBuilder.Export(genesis.X, filepath.Join(basePath, "X", "genesis.json")); err != nil {
		return fmt.Errorf("failed to export X-Chain: %w", err)
	}
	return nil
}

func (b *unifiedBuilder) With(opts ...Option[*UnifiedGenesis]) GenesisBuilder[*UnifiedGenesis] {
	for _, opt := range opts {
		opt(b.config)
	}
	return b
}

func (b *unifiedBuilder) Transform(fn TransformFunc[*UnifiedGenesis]) GenesisBuilder[*UnifiedGenesis] {
	b.config.transforms = append(b.config.transforms, fn)
	return b
}