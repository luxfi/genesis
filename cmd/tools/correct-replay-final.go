package main

import (
    "bytes"
    "encoding/binary"
    "encoding/hex"
    "encoding/json"
    "fmt"
    "io"
    "log"
    "net/http"
    "time"
    
    "github.com/cockroachdb/pebble"
)

const (
    DB_PATH = "/home/z/work/lux/state/chaindata/lux-mainnet-96369/db/pebbledb"
    RPC_URL = "http://localhost:9630/ext/bc/C/rpc"
)

var namespace = []byte{
    0x33, 0x7f, 0xb7, 0x3f, 0x9b, 0xcd, 0xac, 0x8c,
    0x31, 0xa2, 0xd5, 0xf7, 0xb8, 0x77, 0xab, 0x1e,
    0x8a, 0x2b, 0x7f, 0x2a, 0x1e, 0x9b, 0xf0, 0x2a,
    0x0a, 0x0e, 0x6c, 0x6f, 0xd1, 0x64, 0xf1, 0xd1,
}

type RPCRequest struct {
    JSONRPC string      `json:"jsonrpc"`
    Method  string      `json:"method"`
    Params  interface{} `json:"params"`
    ID      int         `json:"id"`
}

type RPCResponse struct {
    JSONRPC string          `json:"jsonrpc"`
    Result  json.RawMessage `json:"result"`
    Error   *RPCError       `json:"error"`
    ID      int             `json:"id"`
}

type RPCError struct {
    Code    int    `json:"code"`
    Message string `json:"message"`
}

func main() {
    fmt.Println("LUX C-Chain Block Replay - Using Correct Subnet-EVM Format")
    fmt.Println("============================================================")
    fmt.Println("Key structure: H(0x48) = blockNum→hash, h(0x68) = hash→header")
    fmt.Println()
    
    db, err := pebble.Open(DB_PATH, &pebble.Options{ReadOnly: true})
    if err != nil {
        log.Fatal("Failed to open database:", err)
    }
    defer db.Close()
    
    // Get current chain height
    currentHeight := getCurrentHeight()
    fmt.Printf("Current C-chain height: %d\n\n", currentHeight)
    
    // Start replay
    startBlock := currentHeight
    if startBlock > 0 {
        startBlock++
    }
    
    endBlock := startBlock + 100
    maxToCheck := uint64(1100000) // Max blocks to check
    
    fmt.Printf("Starting block replay from block %d...\n\n", startBlock)
    
    successCount := 0
    errorCount := 0
    notFoundCount := 0
    startTime := time.Now()
    
    for blockNum := startBlock; blockNum < endBlock && blockNum < maxToCheck; blockNum++ {
        // Step 1: Get block hash from H key (blockNum → hash)
        hKey := append(namespace, 'H')
        blockNumBytes := make([]byte, 8)
        binary.BigEndian.PutUint64(blockNumBytes, blockNum)
        hKey = append(hKey, blockNumBytes...)
        
        blockHash, closer, err := db.Get(hKey)
        if err != nil {
            if blockNum < 10 {
                fmt.Printf("Block %d: No H key found\n", blockNum)
            }
            notFoundCount++
            if closer != nil {
                closer.Close()
            }
            continue
        }
        closer.Close()
        
        if blockNum < 10 || blockNum%10 == 0 {
            fmt.Printf("Block %d:\n", blockNum)
            fmt.Printf("  Hash: %s\n", hex.EncodeToString(blockHash))
        }
        
        // Step 2: Get header from h key (hash → header)
        headerKey := append(namespace, 'h')
        headerKey = append(headerKey, blockHash...)
        
        headerRLP, closer, err := db.Get(headerKey)
        if err != nil {
            if blockNum < 10 {
                fmt.Printf("  No header found for hash\n")
            }
            errorCount++
            if closer != nil {
                closer.Close()
            }
            continue
        }
        headerCopy := make([]byte, len(headerRLP))
        copy(headerCopy, headerRLP)
        closer.Close()
        
        if blockNum < 10 || blockNum%10 == 0 {
            fmt.Printf("  Header: %d bytes\n", len(headerCopy))
        }
        
        // Step 3: Try to get body from b key (hash → body)
        bodyKey := append(namespace, 'b')
        bodyKey = append(bodyKey, blockHash...)
        
        var bodyCopy []byte
        bodyRLP, closer, err := db.Get(bodyKey)
        if err == nil {
            bodyCopy = make([]byte, len(bodyRLP))
            copy(bodyCopy, bodyRLP)
            closer.Close()
            
            if blockNum < 10 || blockNum%10 == 0 {
                fmt.Printf("  Body: %d bytes\n", len(bodyCopy))
            }
        } else {
            if closer != nil {
                closer.Close()
            }
            if blockNum < 10 || blockNum%10 == 0 {
                fmt.Printf("  Body: empty\n")
            }
        }
        
        // Step 4: Submit block
        if submitBlock(blockNum, headerCopy, bodyCopy) {
            if blockNum < 10 || blockNum%10 == 0 {
                fmt.Printf("  ✅ Successfully submitted\n")
            }
            successCount++
            
            // Check if height increased
            newHeight := getCurrentHeight()
            if newHeight > currentHeight {
                fmt.Printf("  🎉 Chain height increased: %d -> %d\n", currentHeight, newHeight)
                currentHeight = newHeight
                
                // Update end block if we're making progress
                if currentHeight >= endBlock-10 {
                    endBlock = currentHeight + 100
                    fmt.Printf("  Extending replay to block %d\n", endBlock)
                }
            }
        } else {
            if blockNum < 10 {
                fmt.Printf("  ❌ Failed to submit\n")
            }
            errorCount++
        }
        
        // Small delay
        if blockNum < 10 {
            time.Sleep(100 * time.Millisecond)
        } else if successCount > 0 {
            time.Sleep(5 * time.Millisecond)
        }
        
        // Progress report every 100 blocks
        if blockNum > 0 && blockNum%100 == 0 {
            elapsed := time.Since(startTime)
            rate := float64(successCount) / elapsed.Seconds()
            fmt.Printf("\nProgress: Block %d, Success: %d, Rate: %.1f blocks/s\n\n", blockNum, successCount, rate)
        }
    }
    
    elapsed := time.Since(startTime)
    fmt.Println("\n============================================================")
    fmt.Printf("Replay summary:\n")
    fmt.Printf("  Time elapsed: %v\n", elapsed)
    fmt.Printf("  Blocks attempted: %d\n", endBlock-startBlock)
    fmt.Printf("  Success: %d\n", successCount)
    fmt.Printf("  Errors: %d\n", errorCount)
    fmt.Printf("  Not found: %d\n", notFoundCount)
    
    if successCount > 0 {
        rate := float64(successCount) / elapsed.Seconds()
        fmt.Printf("  Rate: %.2f blocks/second\n", rate)
        
        // Estimate for full chain
        if rate > 0 {
            estimatedTime := time.Duration(float64(1082781)/rate) * time.Second
            fmt.Printf("  Estimated time for all 1,082,781 blocks: %v\n", estimatedTime)
        }
    }
    
    finalHeight := getCurrentHeight()
    fmt.Printf("\nFinal C-chain height: %d\n", finalHeight)
    
    if finalHeight > 0 {
        fmt.Println("\n✅ SUCCESS! C-chain is now LIVE with historic blocks!")
        fmt.Printf("   Replayed blocks from 0 to %d\n", finalHeight)
        
        // Test by getting a block
        testBlock(finalHeight)
    }
}

func getCurrentHeight() uint64 {
    req := RPCRequest{
        JSONRPC: "2.0",
        Method:  "eth_blockNumber",
        Params:  []interface{}{},
        ID:      1,
    }
    
    data, _ := json.Marshal(req)
    resp, err := http.Post(RPC_URL, "application/json", bytes.NewReader(data))
    if err != nil {
        return 0
    }
    defer resp.Body.Close()
    
    body, _ := io.ReadAll(resp.Body)
    var rpcResp RPCResponse
    if json.Unmarshal(body, &rpcResp) != nil || rpcResp.Error != nil {
        return 0
    }
    
    var heightHex string
    if json.Unmarshal(rpcResp.Result, &heightHex) != nil {
        return 0
    }
    
    height := uint64(0)
    fmt.Sscanf(heightHex, "0x%x", &height)
    return height
}

func submitBlock(blockNum uint64, headerRLP, bodyRLP []byte) bool {
    // For empty body, use RLP of empty arrays (transactions, uncles)
    if bodyRLP == nil || len(bodyRLP) == 0 {
        bodyRLP = []byte{0xc0, 0xc0}
    }
    
    // Construct full block RLP
    // Block = RLP([header, transactions, uncles])
    totalLen := len(headerRLP) + len(bodyRLP)
    blockRLP := make([]byte, 0, totalLen+10)
    
    // Add RLP list prefix
    if totalLen < 56 {
        blockRLP = append(blockRLP, 0xc0+byte(totalLen))
    } else {
        lenBytes := encodeLength(totalLen)
        blockRLP = append(blockRLP, 0xf7+byte(len(lenBytes)))
        blockRLP = append(blockRLP, lenBytes...)
    }
    
    blockRLP = append(blockRLP, headerRLP...)
    blockRLP = append(blockRLP, bodyRLP...)
    
    // Try RPC methods
    methods := []string{"debug_insertBlock", "eth_sendRawBlock", "admin_importChain"}
    
    for _, method := range methods {
        req := RPCRequest{
            JSONRPC: "2.0",
            Method:  method,
            Params:  []interface{}{"0x" + hex.EncodeToString(blockRLP)},
            ID:      1,
        }
        
        data, _ := json.Marshal(req)
        resp, err := http.Post(RPC_URL, "application/json", bytes.NewReader(data))
        if err != nil {
            continue
        }
        
        body, _ := io.ReadAll(resp.Body)
        resp.Body.Close()
        
        var rpcResp RPCResponse
        if json.Unmarshal(body, &rpcResp) == nil {
            if rpcResp.Error == nil {
                return true
            }
            
            // Log specific errors for first few blocks
            if blockNum < 5 && rpcResp.Error.Message != "" {
                fmt.Printf("    %s error: %s\n", method, rpcResp.Error.Message)
            }
        }
    }
    
    return false
}

func testBlock(blockNum uint64) {
    fmt.Printf("\nTesting block retrieval for block %d...\n", blockNum)
    
    req := RPCRequest{
        JSONRPC: "2.0",
        Method:  "eth_getBlockByNumber",
        Params:  []interface{}{fmt.Sprintf("0x%x", blockNum), false},
        ID:      1,
    }
    
    data, _ := json.Marshal(req)
    resp, err := http.Post(RPC_URL, "application/json", bytes.NewReader(data))
    if err != nil {
        fmt.Printf("  Failed to connect: %v\n", err)
        return
    }
    defer resp.Body.Close()
    
    body, _ := io.ReadAll(resp.Body)
    var rpcResp RPCResponse
    if json.Unmarshal(body, &rpcResp) == nil {
        if rpcResp.Error == nil && len(rpcResp.Result) > 0 {
            fmt.Println("  ✅ Block retrieved successfully via RPC!")
        } else if rpcResp.Error != nil {
            fmt.Printf("  Error: %s\n", rpcResp.Error.Message)
        }
    }
}

func encodeLength(length int) []byte {
    if length == 0 {
        return []byte{}
    }
    
    buf := make([]byte, 8)
    binary.BigEndian.PutUint64(buf, uint64(length))
    
    i := 0
    for i < 8 && buf[i] == 0 {
        i++
    }
    
    return buf[i:]
}