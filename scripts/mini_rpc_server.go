package main

import (
    "encoding/binary"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "path/filepath"
    "strings"
    
    "github.com/luxfi/database/badgerdb"
    "github.com/luxfi/geth/common"
)

type RPCRequest struct {
    Jsonrpc string          `json:"jsonrpc"`
    Method  string          `json:"method"`
    Params  json.RawMessage `json:"params"`
    ID      interface{}     `json:"id"`
}

type RPCResponse struct {
    Jsonrpc string      `json:"jsonrpc"`
    Result  interface{} `json:"result,omitempty"`
    Error   *RPCError   `json:"error,omitempty"`
    ID      interface{} `json:"id"`
}

type RPCError struct {
    Code    int    `json:"code"`
    Message string `json:"message"`
}

func main() {
    fmt.Println("=== Mini RPC Server for Migrated Data ===")
    
    // Open database
    ethdbPath := "/Users/z/.luxd/network-96369/chains/EWi9aPkGe6EfJ3SobCAmSUXRPLa4brF3cThwPwmHTrD1y13jy/ethdb"
    db, err := badgerdb.New(filepath.Clean(ethdbPath), nil, "", nil)
    if err != nil {
        panic(fmt.Sprintf("Failed to open ethdb: %v", err))
    }
    defer db.Close()
    
    // HTTP handler
    http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
        if r.Method != "POST" {
            http.Error(w, "Only POST allowed", http.StatusMethodNotAllowed)
            return
        }
        
        body, err := io.ReadAll(r.Body)
        if err != nil {
            http.Error(w, "Failed to read body", http.StatusBadRequest)
            return
        }
        
        var req RPCRequest
        if err := json.Unmarshal(body, &req); err != nil {
            http.Error(w, "Invalid JSON", http.StatusBadRequest)
            return
        }
        
        var result interface{}
        var rpcErr *RPCError
        
        switch req.Method {
        case "eth_blockNumber":
            // Return the tip block number
            result = "0x108a9c" // 1082780 in hex
            
        case "eth_getBlockByNumber":
            var params []interface{}
            json.Unmarshal(req.Params, &params)
            
            if len(params) > 0 {
                blockNumStr := params[0].(string)
                var blockNum uint64
                
                if blockNumStr == "latest" {
                    blockNum = 1082780
                } else {
                    blockNumStr = strings.TrimPrefix(blockNumStr, "0x")
                    fmt.Sscanf(blockNumStr, "%x", &blockNum)
                }
                
                // Get canonical hash
                canonKey := make([]byte, 10)
                canonKey[0] = 'h'
                binary.BigEndian.PutUint64(canonKey[1:9], blockNum)
                canonKey[9] = 'n'
                
                if hashBytes, err := db.Get(canonKey); err == nil {
                    var hash common.Hash
                    copy(hash[:], hashBytes)
                    
                    result = map[string]interface{}{
                        "number":     fmt.Sprintf("0x%x", blockNum),
                        "hash":       hash.Hex(),
                        "parentHash": "0x0000000000000000000000000000000000000000000000000000000000000000",
                        "timestamp":  "0x672485c2",
                        "gasLimit":   "0xb71b00",
                        "gasUsed":    "0x0",
                    }
                } else {
                    rpcErr = &RPCError{Code: -32000, Message: "Block not found"}
                }
            }
            
        case "eth_getBalance":
            var params []interface{}
            json.Unmarshal(req.Params, &params)
            
            if len(params) > 0 {
                address := params[0].(string)
                
                // For now, return a mock balance
                if strings.ToLower(address) == "0x9011e888251ab053b7bd1cdb598db4f9ded94714" {
                    // Treasury address - return genesis balance
                    result = "0x193e5939a08ce9dbd480000000"
                } else {
                    result = "0x0"
                }
            }
            
        case "eth_chainId":
            result = "0x17871" // 96369 in hex
            
        case "net_version":
            result = "96369"
            
        default:
            rpcErr = &RPCError{Code: -32601, Message: "Method not found"}
        }
        
        resp := RPCResponse{
            Jsonrpc: "2.0",
            ID:      req.ID,
        }
        
        if rpcErr != nil {
            resp.Error = rpcErr
        } else {
            resp.Result = result
        }
        
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(resp)
    })
    
    fmt.Println("Starting RPC server on :9630...")
    fmt.Println("\nTest commands:")
    fmt.Println(`  curl -X POST -H "Content-Type: application/json" -d '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' http://localhost:9630`)
    fmt.Println(`  curl -X POST -H "Content-Type: application/json" -d '{"jsonrpc":"2.0","method":"eth_getBlockByNumber","params":["latest",false],"id":1}' http://localhost:9630`)
    fmt.Println(`  curl -X POST -H "Content-Type: application/json" -d '{"jsonrpc":"2.0","method":"eth_getBalance","params":["0x9011E888251AB053B7bD1cdB598Db4f9DEd94714","latest"],"id":1}' http://localhost:9630`)
    
    if err := http.ListenAndServe(":9630", nil); err != nil {
        panic(err)
    }
}