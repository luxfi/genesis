package main

import (
	"testing"
	"github.com/stretchr/testify/assert"
)

func TestReplayerCommand(t *testing.T) {
	// Test that the command is properly registered
	cmd := getReplayerCmd()
	
	assert.NotNil(t, cmd)
	assert.Equal(t, "replay [source-db]", cmd.Use)
	assert.Contains(t, cmd.Short, "Replay blockchain blocks")
	
	// Test flags
	flags := cmd.Flags()
	assert.NotNil(t, flags.Lookup("rpc"))
	assert.NotNil(t, flags.Lookup("start"))
	assert.NotNil(t, flags.Lookup("end"))
	assert.NotNil(t, flags.Lookup("direct-db"))
	assert.NotNil(t, flags.Lookup("output"))
}

func TestEncodeSubnetBlockNumber(t *testing.T) {
	// Test block number encoding
	tests := []struct {
		number   uint64
		expected []byte
	}{
		{0, []byte{0, 0, 0, 0, 0, 0, 0, 0}},
		{1, []byte{0, 0, 0, 0, 0, 0, 0, 1}},
		{256, []byte{0, 0, 0, 0, 0, 0, 1, 0}},
		{65536, []byte{0, 0, 0, 0, 0, 1, 0, 0}},
	}
	
	for _, tt := range tests {
		result := encodeSubnetBlockNumber(tt.number)
		assert.Equal(t, tt.expected, result, "Failed for number %d", tt.number)
	}
}

func TestMakeKeys(t *testing.T) {
	// Test key construction
	blockNum := uint64(12345)
	hash := [32]byte{1, 2, 3, 4, 5, 6, 7, 8}
	
	// Test header key
	headerKey := makeHeaderKey(blockNum, hash)
	assert.Equal(t, 41, len(headerKey)) // 8 bytes num + 32 bytes hash + 1 byte suffix
	assert.Equal(t, byte('h'), headerKey[40])
	
	// Test body key
	bodyKey := makeBodyKey(blockNum, hash)
	assert.Equal(t, 41, len(bodyKey))
	assert.Equal(t, byte('b'), bodyKey[0])
	
	// Test receipts key
	receiptsKey := makeReceiptsKey(blockNum, hash)
	assert.Equal(t, 41, len(receiptsKey))
	assert.Equal(t, byte('r'), receiptsKey[0])
	
	// Test canonical key
	canonicalKey := makeCanonicalKey(blockNum)
	assert.Equal(t, 10, len(canonicalKey))
	assert.Equal(t, byte('h'), canonicalKey[0])
	assert.Equal(t, byte('n'), canonicalKey[9])
}