package genesis

import (
	_ "embed"
	"encoding/json"
)

// CChainGenesisMainnet96369 contains the C-chain genesis for network 96369
// This matches the migrated blockchain data with:
// - Block 0 Hash: 0x3f4fa2a0b0ce089f52bf0ae9199c75ffdd76ecafc987794050cb0d286f1ec61e
// - Timestamp: 0x672485c2 (1730446786)
// - GasLimit: 0xb71b00 (12000000)
// - Treasury: 0x9011E888251AB053B7bD1cdB598Db4f9DEd94714
//
//go:embed cchain_genesis_96369.json
var CChainGenesisMainnet96369 []byte

// CChainGenesisConfig represents the C-chain genesis configuration
type CChainGenesisConfig struct {
	Config struct {
		ChainID                 int64  `json:"chainId"`
		HomesteadBlock          int64  `json:"homesteadBlock"`
		EIP150Block             int64  `json:"eip150Block"`
		EIP155Block             int64  `json:"eip155Block"`
		EIP158Block             int64  `json:"eip158Block"`
		ByzantiumBlock          int64  `json:"byzantiumBlock"`
		ConstantinopleBlock     int64  `json:"constantinopleBlock"`
		PetersburgBlock         int64  `json:"petersburgBlock"`
		IstanbulBlock           int64  `json:"istanbulBlock"`
		MuirGlacierBlock        int64  `json:"muirGlacierBlock"`
		BerlinBlock             int64  `json:"berlinBlock"`
		LondonBlock             int64  `json:"londonBlock"`
		ArrowGlacierBlock       int64  `json:"arrowGlacierBlock"`
		GrayGlacierBlock        int64  `json:"grayGlacierBlock"`
		MergeNetsplitBlock      int64  `json:"mergeNetsplitBlock"`
		ShanghaiTime            int64  `json:"shanghaiTime"`
		CancunTime              int64  `json:"cancunTime"`
		TerminalTotalDifficulty int64  `json:"terminalTotalDifficulty"`
		BlobSchedule            struct {
			Cancun struct {
				Target         int `json:"target"`
				Max            int `json:"max"`
				UpdateFraction int `json:"updateFraction"`
			} `json:"cancun"`
		} `json:"blobSchedule"`
	} `json:"config"`
	Nonce      string            `json:"nonce"`
	Timestamp  string            `json:"timestamp"`
	ExtraData  string            `json:"extraData"`
	GasLimit   string            `json:"gasLimit"`
	Difficulty string            `json:"difficulty"`
	MixHash    string            `json:"mixHash"`
	Coinbase   string            `json:"coinbase"`
	Alloc      map[string]struct {
		Balance string `json:"balance"`
	} `json:"alloc"`
	Number     string `json:"number"`
	GasUsed    string `json:"gasUsed"`
	ParentHash string `json:"parentHash"`
}

// GetCChainGenesis96369 returns the parsed C-chain genesis for network 96369
func GetCChainGenesis96369() (*CChainGenesisConfig, error) {
	var genesis CChainGenesisConfig
	err := json.Unmarshal(CChainGenesisMainnet96369, &genesis)
	if err != nil {
		return nil, err
	}
	return &genesis, nil
}

// GetCChainGenesis96369JSON returns the C-chain genesis as a JSON string
func GetCChainGenesis96369JSON() string {
	return string(CChainGenesisMainnet96369)
}