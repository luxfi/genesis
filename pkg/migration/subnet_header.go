package migration

import (
	"math/big"

	"github.com/luxfi/geth/common"
	"github.com/luxfi/geth/core/types"
)

// SubnetEVMHeader represents a header from SubnetEVM which may have additional fields
type SubnetEVMHeader struct {
	ParentHash  common.Hash      `json:"parentHash"       gencodec:"required"`
	UncleHash   common.Hash      `json:"sha3Uncles"       gencodec:"required"`
	Coinbase    common.Address   `json:"miner"`
	Root        common.Hash      `json:"stateRoot"        gencodec:"required"`
	TxHash      common.Hash      `json:"transactionsRoot" gencodec:"required"`
	ReceiptHash common.Hash      `json:"receiptsRoot"     gencodec:"required"`
	Bloom       types.Bloom      `json:"logsBloom"        gencodec:"required"`
	Difficulty  *big.Int         `json:"difficulty"       gencodec:"required"`
	Number      *big.Int         `json:"number"           gencodec:"required"`
	GasLimit    uint64           `json:"gasLimit"         gencodec:"required"`
	GasUsed     uint64           `json:"gasUsed"          gencodec:"required"`
	Time        uint64           `json:"timestamp"        gencodec:"required"`
	Extra       []byte           `json:"extraData"        gencodec:"required"`
	MixDigest   common.Hash      `json:"mixHash"`
	Nonce       types.BlockNonce `json:"nonce"`
	
	// EIP-1559 fields
	BaseFee *big.Int `json:"baseFeePerGas" rlp:"optional"`
	
	// Additional SubnetEVM fields
	BlockGasCost         *big.Int    `json:"blockGasCost"         rlp:"optional"`
	ExtDataHash          common.Hash `json:"extDataHash"          rlp:"optional"`
	BlockExtraData       []byte      `json:"blockExtraData"       rlp:"optional"`
	
	// EIP-4844 fields (may be present in newer versions)
	BlobGasUsed          *uint64     `json:"blobGasUsed"          rlp:"optional"`
	ExcessBlobGas        *uint64     `json:"excessBlobGas"        rlp:"optional"`
	
	// EIP-4895 fields
	WithdrawalsHash      *common.Hash `json:"withdrawalsHash"      rlp:"optional"`
	
	// EIP-4788 fields
	ParentBeaconBlockRoot *common.Hash `json:"parentBeaconBlockRoot" rlp:"optional"`
}

// ToStandardHeader converts SubnetEVM header to standard geth header
func (h *SubnetEVMHeader) ToStandardHeader() *types.Header {
	header := &types.Header{
		ParentHash:  h.ParentHash,
		UncleHash:   h.UncleHash,
		Coinbase:    h.Coinbase,
		Root:        h.Root,
		TxHash:      h.TxHash,
		ReceiptHash: h.ReceiptHash,
		Bloom:       h.Bloom,
		Difficulty:  h.Difficulty,
		Number:      h.Number,
		GasLimit:    h.GasLimit,
		GasUsed:     h.GasUsed,
		Time:        h.Time,
		Extra:       h.Extra,
		MixDigest:   h.MixDigest,
		Nonce:       h.Nonce,
	}
	
	// Copy optional fields if present
	if h.BaseFee != nil {
		header.BaseFee = new(big.Int).Set(h.BaseFee)
	}
	
	return header
}