package main

import (
    "encoding/json"
    "fmt"
    "io/ioutil"
    "log"
    "os"
    "path/filepath"
)

func main() {
    // Create P-Chain genesis for network 96369
    pchainGenesis := map[string]interface{}{
        "networkID": 96369,
        "allocations": []map[string]interface{}{
            {
                "ethAddr":       "0x8db97C7cEcE249c2b98bDC0226Cc4C2A57BF52FC",
                "luxAddr":       "P-lux18jma8ppw3nhx5r4ap8clazz0dps7rv5u00z96u",
                "initialAmount": 1000000000000000000,
                "unlockSchedule": []map[string]interface{}{
                    {
                        "amount":   1000000000000,
                        "locktime": 1640995200,
                    },
                },
            },
        },
        "startTime":               1754362800,
        "initialStakeDuration":    31536000,
        "initialStakeDurationOffset": 5400,
        "initialStakedFunds": []string{
            "P-lux18jma8ppw3nhx5r4ap8clazz0dps7rv5u00z96u",
        },
        "initialStakers": []map[string]interface{}{
            {
                "nodeID":         "NodeID-111111111111111111116DBWJs",
                "rewardAddress":  "P-lux18jma8ppw3nhx5r4ap8clazz0dps7rv5u00z96u",
                "delegationFee":  20000,
            },
        },
        "message": "Lux Mainnet with Migrated C-Chain",
    }
    
    // Create configs directory
    configDir := "/home/z/.luxd/configs"
    if err := os.MkdirAll(configDir, 0755); err != nil {
        log.Fatal("Failed to create config dir:", err)
    }
    
    // Write network genesis
    genesisPath := filepath.Join(configDir, "genesis.json")
    data, err := json.MarshalIndent(pchainGenesis, "", "  ")
    if err != nil {
        log.Fatal("Failed to marshal genesis:", err)
    }
    
    if err := ioutil.WriteFile(genesisPath, data, 0644); err != nil {
        log.Fatal("Failed to write genesis:", err)
    }
    
    fmt.Printf("Created P-Chain genesis at %s\n", genesisPath)
    
    // Also create node config
    nodeConfig := map[string]interface{}{
        "network-id": 96369,
        "http-host": "0.0.0.0",
        "http-port": 9630,
        "sybil-protection-enabled": false,
        "consensus-sample-size": 1,
        "consensus-quorum-size": 1,
        "chain-data-dir": "/home/z/.luxd/chainData",
    }
    
    configPath := filepath.Join(configDir, "node.json")
    data, err = json.MarshalIndent(nodeConfig, "", "  ")
    if err != nil {
        log.Fatal("Failed to marshal config:", err)
    }
    
    if err := ioutil.WriteFile(configPath, data, 0644); err != nil {
        log.Fatal("Failed to write config:", err)
    }
    
    fmt.Printf("Created node config at %s\n", configPath)
}