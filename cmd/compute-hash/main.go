package main

import (
    "crypto/sha256"
    "encoding/hex"
    "encoding/json"
    "fmt"
    "log"
)

type Genesis struct {
    Config     interface{} `json:"config"`
    Nonce      string      `json:"nonce"`
    Timestamp  string      `json:"timestamp"`
    ExtraData  string      `json:"extraData"`
    GasLimit   string      `json:"gasLimit"`
    Difficulty string      `json:"difficulty"`
    MixHash    string      `json:"mixHash"`
    Coinbase   string      `json:"coinbase"`
    Alloc      interface{} `json:"alloc"`
    Number     string      `json:"number"`
    GasUsed    string      `json:"gasUsed"`
    ParentHash string      `json:"parentHash"`
}

func main() {
    // The genesis that luxd is using
    wrongGenesis := `{"config":{"chainId":12345,"homesteadBlock":0,"eip150Block":0,"eip155Block":0,"eip158Block":0,"byzantiumBlock":0,"constantinopleBlock":0,"petersburgBlock":0,"istanbulBlock":0},"nonce":"0x0","timestamp":"0x0","extraData":"0x00","gasLimit":"0x989680","difficulty":"0x0","mixHash":"0x0000000000000000000000000000000000000000000000000000000000000000","coinbase":"0x0000000000000000000000000000000000000000","alloc":{},"number":"0x0","gasUsed":"0x0","parentHash":"0x0000000000000000000000000000000000000000000000000000000000000000"}`
    
    var genesis Genesis
    if err := json.Unmarshal([]byte(wrongGenesis), &genesis); err != nil {
        log.Fatal(err)
    }
    
    // Compute hash
    data, _ := json.Marshal(genesis)
    hash := sha256.Sum256(data)
    fmt.Printf("Wrong genesis hash: %s\n", hex.EncodeToString(hash[:]))
    
    // The correct genesis
    correctGenesis := `{"config":{"chainId":96369,"homesteadBlock":0,"eip150Block":0,"eip155Block":0,"eip158Block":0,"byzantiumBlock":0,"constantinopleBlock":0,"petersburgBlock":0,"istanbulBlock":0},"nonce":"0x0","timestamp":"0x0","extraData":"0x00","gasLimit":"0x989680","difficulty":"0x0","mixHash":"0x0000000000000000000000000000000000000000000000000000000000000000","coinbase":"0x0000000000000000000000000000000000000000","alloc":{},"number":"0x0","gasUsed":"0x0","parentHash":"0x0000000000000000000000000000000000000000000000000000000000000000"}`
    
    if err := json.Unmarshal([]byte(correctGenesis), &genesis); err != nil {
        log.Fatal(err)
    }
    
    data, _ = json.Marshal(genesis)
    hash = sha256.Sum256(data)
    fmt.Printf("Correct genesis hash: %s\n", hex.EncodeToString(hash[:]))
}