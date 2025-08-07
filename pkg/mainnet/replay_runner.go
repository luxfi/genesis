package mainnet

import (
	"github.com/luxfi/genesis/pkg/application"
)

type ReplayOptions struct {
	// Key generation
	KeysDir      string
	GenerateKeys bool

	// Genesis configuration
	GenesisDB     string
	GenesisDBType string
	NetworkID     string

	// Node configuration
	DataDir      string
	DBType       string
	CChainDBType string
	HTTPPort     int
	StakingPort  int

	// Consensus parameters
	SnowSampleSize int
	SnowQuorumSize int
	K              int
	AlphaPreference int
	AlphaConfidence int
	Beta           int

	// Execution
	SkipLaunch bool
	SingleNode bool
	LogLevel   string
	EnableStaking bool

	// Multi-node support
	NumNodes   int
	BasePort   int
}

type ReplayRunner struct {
	*SimpleReplayRunner
}

func NewReplayRunner(app *application.Genesis) *ReplayRunner {
	return &ReplayRunner{
		SimpleReplayRunner: NewSimpleReplayRunner(app),
	}
}

func (r *ReplayRunner) Run(opts ReplayOptions) error {
	return r.SimpleReplayRunner.Run(opts)
}