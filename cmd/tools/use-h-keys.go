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
    fmt.Println("Using H keys as canonical mapping")
    fmt.Println("==================================")
    
    db, err := pebble.Open(DB_PATH, &pebble.Options{ReadOnly: true})
    if err != nil {
        log.Fatal("Failed to open database:", err)
    }
    defer db.Close()
    
    // Build H key -> block number mapping
    fmt.Println("Step 1: Building H key mapping...")
    hMapping := make(map[uint64][]byte)
    
    iter, _ := db.NewIter(&pebble.IterOptions{})
    count := 0
    for iter.First(); iter.Valid(); iter.Next() {
        key := iter.Key()
        
        if len(key) == 65 && bytes.Equal(key[:32], namespace) && key[32] == 'H' {
            hash := key[33:65]
            value := iter.Value()
            
            if len(value) == 8 {
                blockNum := binary.BigEndian.Uint64(value)
                hMapping[blockNum] = hash
                
                if blockNum < 5 || (blockNum > 0 && blockNum%100000 == 0) {
                    fmt.Printf("  Block %d -> H-hash %x\n", blockNum, hash)
                }
                count++
            }
        }
    }
    iter.Close()
    
    fmt.Printf("Found %d H keys (canonical mapping)\n\n", count)
    
    // The H-hash is NOT the block hash, it's something else
    // Let's try to find what the actual block hashes are
    
    fmt.Println("Step 2: Finding actual block headers...")
    
    // Since the H-hashes don't match headers directly, let's enumerate all headers
    // and see if we can match them to blocks somehow
    
    headerCount := 0
    iter2, _ := db.NewIter(&pebble.IterOptions{})
    headers := make(map[string][]byte) // hash -> header data
    
    for iter2.First(); iter2.Valid(); iter2.Next() {
        key := iter2.Key()
        
        if len(key) == 65 && bytes.Equal(key[:32], namespace) && key[32] == 'h' {
            hash := key[33:65]
            value := iter2.Value()
            headers[hex.EncodeToString(hash)] = value
            headerCount++
            
            if headerCount <= 5 {
                fmt.Printf("  Header %d: hash=%x, size=%d bytes\n", headerCount, hash, len(value))
            }
        }
    }
    iter2.Close()
    
    fmt.Printf("Found %d headers total\n\n", headerCount)
    
    // Try to replay blocks using H-hash as a lookup
    fmt.Println("Step 3: Attempting replay with H-mapping...")
    
    currentHeight := getCurrentHeight()
    fmt.Printf("Current C-chain height: %d\n\n", currentHeight)
    
    startBlock := currentHeight
    if startBlock > 0 {
        startBlock++
    }
    endBlock := startBlock + 5
    
    fmt.Printf("Attempting to replay blocks %d to %d\n", startBlock, endBlock)
    
    successCount := 0
    errorCount := 0
    startTime := time.Now()
    
    for blockNum := startBlock; blockNum <= endBlock; blockNum++ {
        hHash, exists := hMapping[blockNum]
        if !exists {
            fmt.Printf("Block %d: No H key found\n", blockNum)
            errorCount++
            continue
        }
        
        fmt.Printf("\nBlock %d (H-hash: %x):\n", blockNum, hHash)
        
        // The H-hash might be related to the actual block hash
        // Let's try to find headers that might correspond
        
        // Try the H-hash directly as header hash (we know this doesn't work but let's confirm)
        headerKey := append(namespace, 'h')
        headerKey = append(headerKey, hHash...)
        
        headerRLP, closer, err := db.Get(headerKey)
        if err == nil {
            closer.Close()
            fmt.Printf("  Found header with H-hash! Size: %d bytes\n", len(headerRLP))
            
            // Try to submit this
            if submitBlock(blockNum, headerRLP, nil) {
                fmt.Printf("  ✅ Successfully submitted\n")
                successCount++
            } else {
                fmt.Printf("  ❌ Failed to submit\n")
                errorCount++
            }
        } else {
            // H-hash doesn't directly map to header
            // We need a different approach
            fmt.Printf("  No header with H-hash\n")
            
            // The H-hash might encode information about the block
            // Or there might be another mapping we're missing
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
    if bodyRLP == nil {
        bodyRLP = []byte{0xc0, 0xc0}
    }
    
    totalLen := len(headerRLP) + len(bodyRLP)
    blockRLP := make([]byte, 0, totalLen+10)
    
    if totalLen < 56 {
        blockRLP = append(blockRLP, 0xc0+byte(totalLen))
    } else {
        lenBytes := encodeLength(totalLen)
        blockRLP = append(blockRLP, 0xf7+byte(len(lenBytes)))
        blockRLP = append(blockRLP, lenBytes...)
    }
    
    blockRLP = append(blockRLP, headerRLP...)
    blockRLP = append(blockRLP, bodyRLP...)
    
    req := RPCRequest{
        JSONRPC: "2.0",
        Method:  "debug_insertBlock",
        Params:  []interface{}{"0x" + hex.EncodeToString(blockRLP)},
        ID:      1,
    }
    
    data, _ := json.Marshal(req)
    resp, err := http.Post(RPC_URL, "application/json", bytes.NewReader(data))
    if err != nil {
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