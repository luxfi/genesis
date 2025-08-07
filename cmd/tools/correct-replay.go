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
    fmt.Println("LUX Block Replay - Using correct Subnet-EVM format")
    fmt.Println("===================================================")
    fmt.Println("Canonical: c<height> → <block hash>")
    fmt.Println("Headers:   h<block hash> → <RLP header>")
    fmt.Println("Bodies:    b<block hash> → <RLP block>")
    fmt.Println()
    
    db, err := pebble.Open(DB_PATH, &pebble.Options{ReadOnly: true})
    if err != nil {
        log.Fatal("Failed to open database:", err)
    }
    defer db.Close()
    
    // Step 1: Find canonical blocks using 'c' prefix
    fmt.Println("Step 1: Finding canonical blocks (c prefix)...")
    
    blockHashes := make(map[uint64][]byte)
    
    for blockNum := uint64(0); blockNum <= 100; blockNum++ {
        // Build canonical key: namespace + 'c' + height(8 bytes)
        canonicalKey := append(namespace, 'c')
        heightBytes := make([]byte, 8)
        binary.BigEndian.PutUint64(heightBytes, blockNum)
        canonicalKey = append(canonicalKey, heightBytes...)
        
        // Get the block hash
        blockHash, closer, err := db.Get(canonicalKey)
        if err == nil {
            closer.Close()
            blockHashes[blockNum] = blockHash
            if blockNum < 5 {
                fmt.Printf("  Block %d -> hash %s\n", blockNum, hex.EncodeToString(blockHash))
            }
        }
    }
    
    fmt.Printf("Found %d canonical blocks\n", len(blockHashes))
    
    if len(blockHashes) == 0 {
        fmt.Println("\nNo canonical blocks found with 'c' prefix.")
        fmt.Println("Checking if database uses different format...")
        
        // Try without namespace
        fmt.Println("\nTrying without namespace prefix:")
        for blockNum := uint64(0); blockNum <= 10; blockNum++ {
            // Try just 'c' + height
            canonicalKey := []byte{'c'}
            heightBytes := make([]byte, 8)
            binary.BigEndian.PutUint64(heightBytes, blockNum)
            canonicalKey = append(canonicalKey, heightBytes...)
            
            blockHash, closer, err := db.Get(canonicalKey)
            if err == nil {
                closer.Close()
                blockHashes[blockNum] = blockHash
                fmt.Printf("  Block %d -> hash %s\n", blockNum, hex.EncodeToString(blockHash))
            }
        }
    }
    
    if len(blockHashes) == 0 {
        fmt.Println("\nStill no canonical blocks. Database may use different format.")
        return
    }
    
    // Step 2: Get current chain height
    currentHeight := getCurrentHeight()
    fmt.Printf("\nCurrent C-chain height: %d\n", currentHeight)
    
    // Step 3: Start replay
    fmt.Println("\nStep 3: Starting block replay...")
    
    startBlock := currentHeight
    if startBlock > 0 {
        startBlock++
    }
    
    endBlock := startBlock + 10
    if endBlock > 100 {
        endBlock = 100
    }
    
    fmt.Printf("Replaying blocks %d to %d\n", startBlock, endBlock)
    
    successCount := 0
    errorCount := 0
    startTime := time.Now()
    
    for blockNum := startBlock; blockNum <= endBlock; blockNum++ {
        blockHash, exists := blockHashes[blockNum]
        if !exists {
            fmt.Printf("Block %d: Not found in canonical mapping\n", blockNum)
            errorCount++
            continue
        }
        
        fmt.Printf("\nBlock %d (hash: %s):\n", blockNum, hex.EncodeToString(blockHash)[:16]+"...")
        
        // Get header: namespace + 'h' + block hash
        headerKey := append(namespace, 'h')
        headerKey = append(headerKey, blockHash...)
        
        headerRLP, closer, err := db.Get(headerKey)
        if err != nil {
            // Try without namespace
            headerKey = append([]byte{'h'}, blockHash...)
            headerRLP, closer, err = db.Get(headerKey)
            if err != nil {
                fmt.Printf("  Header not found\n")
                errorCount++
                continue
            }
        }
        closer.Close()
        fmt.Printf("  Header: %d bytes\n", len(headerRLP))
        
        // Get body: namespace + 'b' + block hash
        bodyKey := append(namespace, 'b')
        bodyKey = append(bodyKey, blockHash...)
        
        bodyRLP, closer, err := db.Get(bodyKey)
        if err != nil {
            // Try without namespace
            bodyKey = append([]byte{'b'}, blockHash...)
            bodyRLP, closer, err = db.Get(bodyKey)
            if err != nil {
                // Empty body is OK
                bodyRLP = nil
                fmt.Printf("  Body: empty\n")
            } else {
                closer.Close()
                fmt.Printf("  Body: %d bytes\n", len(bodyRLP))
            }
        } else {
            closer.Close()
            fmt.Printf("  Body: %d bytes\n", len(bodyRLP))
        }
        
        if submitBlock(blockNum, headerRLP, bodyRLP) {
            fmt.Printf("  ✅ Successfully submitted\n")
            successCount++
            
            newHeight := getCurrentHeight()
            if newHeight > currentHeight {
                fmt.Printf("  Chain height increased: %d -> %d\n", currentHeight, newHeight)
                currentHeight = newHeight
            }
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
    fmt.Printf("  Blocks processed: %d\n", endBlock-startBlock+1)
    fmt.Printf("  Success: %d\n", successCount)
    fmt.Printf("  Errors: %d\n", errorCount)
    
    finalHeight := getCurrentHeight()
    fmt.Printf("\nFinal C-chain height: %d\n", finalHeight)
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
    if bodyRLP == nil {
        bodyRLP = []byte{0xc0, 0xc0} // Empty arrays
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
    methods := []string{"debug_insertBlock", "eth_sendRawBlock", "admin_importChain"}
    
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
                return true
            }
            fmt.Printf("    Error: %s\n", rpcResp.Error.Message)
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
    
    i := 0
    for i < 8 && buf[i] == 0 {
        i++
    }
    
    return buf[i:]
}