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
    fmt.Println("Simple Replay - Working around canonical mapping")
    fmt.Println("=================================================")
    
    db, err := pebble.Open(DB_PATH, &pebble.Options{ReadOnly: true})
    if err != nil {
        log.Fatal("Failed to open database:", err)
    }
    defer db.Close()
    
    // Since we can't easily decode headers without the full go-ethereum,
    // and the H keys don't directly map to block hashes,
    // let's try a different approach:
    // The headers we saw have a pattern in their hashes.
    
    // Sample header hashes from the test output:
    // 00000000000000003f4fa2a0b0ce089f52bf0ae9199c75ffdd76ecafc9877940
    // 0000000000000001465e28596f984637c0afaa8c6eaa74e53793925f5febe600
    // 00000000000000027f4bf144681894ecd0a391609941ffc02b4a82f346932784
    
    // The first 8 bytes seem to encode the block number!
    // Let's test this hypothesis
    
    fmt.Println("\nStep 1: Testing if header hashes encode block numbers...")
    
    blockToHash := make(map[uint64][]byte)
    
    // Try to find headers for blocks 0-10 using the pattern
    for blockNum := uint64(0); blockNum <= 10; blockNum++ {
        // Scan for headers that have the block number in first 8 bytes
        // Let's scan for headers that might match
        iter, _ := db.NewIter(&pebble.IterOptions{})
        for iter.First(); iter.Valid(); iter.Next() {
            key := iter.Key()
            
            if len(key) == 65 && bytes.Equal(key[:32], namespace) && key[32] == 'h' {
                hash := key[33:65]
                
                // Check if first 8 bytes could be the block number
                if len(hash) >= 8 {
                    possibleBlockNum := binary.BigEndian.Uint64(hash[0:8])
                    if possibleBlockNum == blockNum {
                        fmt.Printf("  Found header for block %d: hash=%x\n", blockNum, hash)
                        blockToHash[blockNum] = hash
                        break
                    }
                }
            }
        }
        iter.Close()
    }
    
    fmt.Printf("Found headers for %d blocks\n", len(blockToHash))
    
    // Step 2: Get current chain height
    currentHeight := getCurrentHeight()
    fmt.Printf("\nCurrent C-chain height: %d\n", currentHeight)
    
    // Step 3: Start replay with what we found
    fmt.Println("\nStep 3: Starting block replay...")
    
    startBlock := currentHeight
    endBlock := uint64(10)
    if len(blockToHash) < 10 {
        endBlock = uint64(len(blockToHash) - 1)
    }
    
    successCount := 0
    errorCount := 0
    startTime := time.Now()
    
    for blockNum := startBlock; blockNum <= endBlock; blockNum++ {
        hash, exists := blockToHash[blockNum]
        if !exists {
            fmt.Printf("Block %d: Not found\n", blockNum)
            errorCount++
            continue
        }
        
        fmt.Printf("\nBlock %d (hash: %s):\n", blockNum, hex.EncodeToString(hash))
        
        // Get header
        headerKey := append(namespace, 'h')
        headerKey = append(headerKey, hash...)
        
        headerRLP, closer, err := db.Get(headerKey)
        if err != nil {
            fmt.Printf("  Header not found\n")
            errorCount++
            continue
        }
        closer.Close()
        
        // Get body
        bodyKey := append(namespace, 'b')
        bodyKey = append(bodyKey, hash...)
        
        bodyRLP, closer, err := db.Get(bodyKey)
        if err != nil {
            bodyRLP = nil // Empty body
        } else {
            closer.Close()
        }
        
        if submitBlock(blockNum, headerRLP, bodyRLP) {
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
    // For empty body, use RLP of empty arrays
    if bodyRLP == nil {
        bodyRLP = []byte{0xc0, 0xc0}
    }
    
    // Combine header and body
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
    
    // Try submission
    req := RPCRequest{
        JSONRPC: "2.0",
        Method:  "debug_insertBlock",
        Params:  []interface{}{"0x" + hex.EncodeToString(blockRLP)},
        ID:      1,
    }
    
    data, _ := json.Marshal(req)
    resp, err := http.Post(RPC_URL, "application/json", bytes.NewReader(data))
    if err != nil {
        fmt.Printf("    Connection error: %v\n", err)
        return false
    }
    
    body, _ := io.ReadAll(resp.Body)
    resp.Body.Close()
    
    var rpcResp RPCResponse
    if json.Unmarshal(body, &rpcResp) == nil {
        if rpcResp.Error == nil {
            return true
        }
        fmt.Printf("    RPC error: %s\n", rpcResp.Error.Message)
    }
    
    return false
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