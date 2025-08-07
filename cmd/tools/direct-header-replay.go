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
    "github.com/ethereum/go-ethereum/core/types"
    "github.com/ethereum/go-ethereum/rlp"
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
    fmt.Println("Direct Header Replay - Extract block numbers from headers")
    fmt.Println("=========================================================")
    
    db, err := pebble.Open(DB_PATH, &pebble.Options{ReadOnly: true})
    if err != nil {
        log.Fatal("Failed to open database:", err)
    }
    defer db.Close()
    
    // Collect all headers and try to decode their block numbers
    fmt.Println("\nStep 1: Collecting headers and extracting block numbers...")
    
    blockData := make(map[uint64]*BlockInfo)
    
    iter, _ := db.NewIter(&pebble.IterOptions{})
    headerCount := 0
    
    for iter.First(); iter.Valid(); iter.Next() {
        key := iter.Key()
        
        // Look for headers: namespace + 'h' + hash
        if len(key) == 65 && bytes.Equal(key[:32], namespace) && key[32] == 'h' {
            hash := key[33:65]
            headerRLP := iter.Value()
            
            // Try to decode the header to get block number
            var header types.Header
            if err := rlp.DecodeBytes(headerRLP, &header); err == nil {
                blockNum := header.Number.Uint64()
                
                if blockNum < 10 {
                    fmt.Printf("  Block %d: hash=%x\n", blockNum, hash)
                }
                
                blockData[blockNum] = &BlockInfo{
                    Hash:      hash,
                    HeaderRLP: headerRLP,
                }
                
                headerCount++
            }
        }
    }
    iter.Close()
    
    fmt.Printf("Found %d headers\n", headerCount)
    
    // Now get bodies for these blocks
    fmt.Println("\nStep 2: Getting bodies for blocks...")
    
    for blockNum, info := range blockData {
        if blockNum >= 10 {
            continue // Just do first 10 for testing
        }
        
        // Get body with namespace + 'b' + hash
        bodyKey := append(namespace, 'b')
        bodyKey = append(bodyKey, info.Hash...)
        
        bodyRLP, closer, err := db.Get(bodyKey)
        if err == nil {
            closer.Close()
            info.BodyRLP = bodyRLP
            fmt.Printf("  Block %d: body found (%d bytes)\n", blockNum, len(bodyRLP))
        } else {
            fmt.Printf("  Block %d: no body (empty block)\n", blockNum)
        }
    }
    
    // Step 3: Start replay
    fmt.Println("\nStep 3: Starting block replay...")
    
    currentHeight := getCurrentHeight()
    fmt.Printf("Current C-chain height: %d\n", currentHeight)
    
    startBlock := currentHeight
    if startBlock > 0 {
        startBlock++
    }
    
    endBlock := startBlock + 10 // Just do 10 blocks for testing
    
    fmt.Printf("Replaying blocks %d to %d\n", startBlock, endBlock)
    
    successCount := 0
    errorCount := 0
    startTime := time.Now()
    
    for blockNum := startBlock; blockNum <= endBlock; blockNum++ {
        info, exists := blockData[blockNum]
        if !exists {
            fmt.Printf("Block %d: Not found\n", blockNum)
            errorCount++
            continue
        }
        
        fmt.Printf("\nBlock %d (hash: %s):\n", blockNum, hex.EncodeToString(info.Hash))
        
        if submitBlock(blockNum, info.HeaderRLP, info.BodyRLP) {
            fmt.Printf("  ✅ Successfully submitted\n")
            successCount++
        } else {
            fmt.Printf("  ❌ Failed to submit\n")
            errorCount++
        }
        
        time.Sleep(50 * time.Millisecond)
    }
    
    elapsed := time.Since(startTime)
    fmt.Println("\n==================================================")
    fmt.Printf("Replay summary:\n")
    fmt.Printf("  Time elapsed: %v\n", elapsed)
    fmt.Printf("  Success: %d\n", successCount)
    fmt.Printf("  Errors: %d\n", errorCount)
}

type BlockInfo struct {
    Hash      []byte
    HeaderRLP []byte
    BodyRLP   []byte
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
    // The headerRLP and bodyRLP are already RLP encoded
    // We need to combine them into a complete block
    
    // For empty body, use RLP of empty arrays
    if bodyRLP == nil {
        bodyRLP = []byte{0xc0, 0xc0} // RLP([],[])
    }
    
    // Combine header and body
    totalLen := len(headerRLP) + len(bodyRLP)
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
    
    blockRLP = append(blockRLP, headerRLP...)
    blockRLP = append(blockRLP, bodyRLP...)
    
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