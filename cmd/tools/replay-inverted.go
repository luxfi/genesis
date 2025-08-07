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
    "sort"
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
    fmt.Println("LUX Mainnet Block Replay Tool - Inverted Index Version")
    fmt.Println("======================================================")
    
    db, err := pebble.Open(DB_PATH, &pebble.Options{ReadOnly: true})
    if err != nil {
        log.Fatal("Failed to open database:", err)
    }
    defer db.Close()
    
    // Step 1: Build the canonical mapping from inverted H keys
    fmt.Println("\nStep 1: Building canonical block mapping from inverted H keys...")
    
    blockToHash := make(map[uint64][]byte)
    
    iter, _ := db.NewIter(&pebble.IterOptions{})
    count := 0
    
    for iter.First(); iter.Valid(); iter.Next() {
        key := iter.Key()
        
        // Look for namespace + H + hash (65 bytes)
        if len(key) == 65 && bytes.Equal(key[:32], namespace) && key[32] == 'H' {
            hash := make([]byte, 32)
            copy(hash, key[33:65])  // Make a copy of the hash
            value := iter.Value()
            
            // Value should be the block number (8 bytes)
            if len(value) == 8 {
                blockNum := binary.BigEndian.Uint64(value)
                blockToHash[blockNum] = hash
                
                if count < 5 {
                    fmt.Printf("  Block %d -> hash %s\n", blockNum, hex.EncodeToString(hash))
                }
                count++
            }
        }
    }
    iter.Close()
    
    fmt.Printf("Found %d canonical blocks\n", len(blockToHash))
    
    // Find block range
    blockNums := make([]uint64, 0, len(blockToHash))
    for blockNum := range blockToHash {
        blockNums = append(blockNums, blockNum)
    }
    sort.Slice(blockNums, func(i, j int) bool {
        return blockNums[i] < blockNums[j]
    })
    
    if len(blockNums) > 0 {
        fmt.Printf("Block range: %d to %d\n", blockNums[0], blockNums[len(blockNums)-1])
    }
    
    // Step 2: Get current chain height
    fmt.Println("\nStep 2: Checking current C-chain status...")
    currentHeight := getCurrentHeight()
    fmt.Printf("Current C-chain height: %d\n", currentHeight)
    
    // Step 3: Start replay
    fmt.Println("\nStep 3: Starting block replay...")
    
    startBlock := currentHeight
    if startBlock > 0 {
        startBlock++ // Start from next block
    }
    
    endBlock := startBlock + 100 // Do 100 blocks at a time
    if uint64(len(blockNums)) > 0 && endBlock > blockNums[len(blockNums)-1] {
        endBlock = blockNums[len(blockNums)-1]
    }
    
    fmt.Printf("Replaying blocks %d to %d\n", startBlock, endBlock)
    
    successCount := 0
    errorCount := 0
    startTime := time.Now()
    
    for blockNum := startBlock; blockNum <= endBlock; blockNum++ {
        hash, exists := blockToHash[blockNum]
        if !exists {
            fmt.Printf("Block %d: Not found in canonical mapping\n", blockNum)
            errorCount++
            continue
        }
        
        fmt.Printf("\nBlock %d (hash: %s):\n", blockNum, hex.EncodeToString(hash))
        
        // Get header (namespace + h + hash)
        headerKey := append(namespace, 'h')
        headerKey = append(headerKey, hash...)
        
        headerData, closer, err := db.Get(headerKey)
        if err != nil {
            fmt.Printf("  Header not found\n")
            errorCount++
            continue
        }
        closer.Close()
        fmt.Printf("  Header: %d bytes\n", len(headerData))
        
        // Get body (namespace + b + hash)
        bodyKey := append(namespace, 'b')
        bodyKey = append(bodyKey, hash...)
        
        bodyData, closer, err := db.Get(bodyKey)
        if err != nil {
            // Empty body is valid
            bodyData = nil
            fmt.Printf("  Body: empty\n")
        } else {
            closer.Close()
            fmt.Printf("  Body: %d bytes\n", len(bodyData))
        }
        
        // Try to submit the block
        if submitBlock(blockNum, headerData, bodyData) {
            fmt.Printf("  ✅ Successfully submitted\n")
            successCount++
        } else {
            fmt.Printf("  ❌ Failed to submit\n")
            errorCount++
        }
        
        // Check if height increased
        newHeight := getCurrentHeight()
        if newHeight > currentHeight {
            fmt.Printf("  Chain height increased: %d -> %d\n", currentHeight, newHeight)
            currentHeight = newHeight
        }
        
        time.Sleep(50 * time.Millisecond)
    }
    
    elapsed := time.Since(startTime)
    fmt.Println("\n" + "==================================================")
    fmt.Printf("Replay summary:\n")
    fmt.Printf("  Time elapsed: %v\n", elapsed)
    fmt.Printf("  Blocks processed: %d\n", endBlock-startBlock+1)
    fmt.Printf("  Success: %d\n", successCount)
    fmt.Printf("  Errors: %d\n", errorCount)
    if successCount > 0 {
        fmt.Printf("  Rate: %.2f blocks/second\n", float64(successCount)/elapsed.Seconds())
    }
    
    finalHeight := getCurrentHeight()
    fmt.Printf("\nFinal C-chain height: %d (added %d blocks)\n", finalHeight, finalHeight-currentHeight)
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

func submitBlock(blockNum uint64, headerData, bodyData []byte) bool {
    // The headerData and bodyData are already RLP encoded
    // We need to combine them into a complete block
    
    // For empty body, use RLP of empty arrays
    if bodyData == nil {
        bodyData = []byte{0xc0, 0xc0} // RLP([],[])
    }
    
    // Combine header and body
    totalLen := len(headerData) + len(bodyData)
    blockRLP := make([]byte, 0, totalLen+10)
    
    // Add RLP list prefix
    if totalLen < 56 {
        blockRLP = append(blockRLP, 0xc0+byte(totalLen))
    } else {
        // Long list encoding
        lenBytes := encodeLength(totalLen)
        blockRLP = append(blockRLP, 0xf7+byte(len(lenBytes)))
        blockRLP = append(blockRLP, lenBytes...)
    }
    
    blockRLP = append(blockRLP, headerData...)
    blockRLP = append(blockRLP, bodyData...)
    
    // Try different submission methods
    methods := []string{"eth_sendRawBlock", "debug_insertBlock", "admin_importChain"}
    
    for _, method := range methods {
        fmt.Printf("  Trying %s...\n", method)
        
        req := RPCRequest{
            JSONRPC: "2.0",
            Method:  method,
            Params:  []interface{}{"0x" + hex.EncodeToString(blockRLP)},
            ID:      1,
        }
        
        data, _ := json.Marshal(req)
        resp, err := http.Post(RPC_URL, "application/json", bytes.NewReader(data))
        if err != nil {
            fmt.Printf("    Connection error: %v\n", err)
            continue
        }
        
        body, _ := io.ReadAll(resp.Body)
        resp.Body.Close()
        
        var rpcResp RPCResponse
        if json.Unmarshal(body, &rpcResp) == nil {
            if rpcResp.Error == nil {
                return true // Success!
            }
            fmt.Printf("    RPC error: %s\n", rpcResp.Error.Message)
        }
    }
    
    return false
}

func encodeLength(length int) []byte {
    if length == 0 {
        return []byte{}
    }
    
    buf := make([]byte, 8)
    binary.BigEndian.PutUint64(buf, uint64(length))
    
    // Find first non-zero byte
    i := 0
    for i < 8 && buf[i] == 0 {
        i++
    }
    
    return buf[i:]
}