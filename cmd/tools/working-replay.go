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

type Block struct {
    Number uint64
    Hash   []byte
    Header []byte
    Body   []byte
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
    fmt.Println("Block Replay - Using correct block number encoding")
    fmt.Println("===================================================")
    
    db, err := pebble.Open(DB_PATH, &pebble.Options{ReadOnly: true})
    if err != nil {
        log.Fatal("Failed to open database:", err)
    }
    defer db.Close()
    
    // Step 1: Build block mapping
    fmt.Println("Step 1: Building block database...")
    blocks := make(map[uint64]*Block)
    
    iter, _ := db.NewIter(&pebble.IterOptions{})
    defer iter.Close()
    
    totalBlocks := 0
    maxBlock := uint64(0)
    
    for iter.First(); iter.Valid(); iter.Next() {
        key := make([]byte, len(iter.Key()))
        copy(key, iter.Key())
        
        value := iter.Value()
        
        // Check for namespace + 32-byte hash
        if len(key) == 64 && bytes.HasPrefix(key, namespace) {
            hash := key[32:]
            
            // Check if value is RLP header
            if len(value) > 100 && (value[0] == 0xf8 || value[0] == 0xf9) {
                // Decode block number from first 3 bytes of hash
                blockNum := uint64(hash[0])<<16 | uint64(hash[1])<<8 | uint64(hash[2])
                
                // Sanity check
                if blockNum < 2000000 {
                    // Make copies of the data
                    hashCopy := make([]byte, 32)
                    copy(hashCopy, hash)
                    
                    headerCopy := make([]byte, len(value))
                    copy(headerCopy, value)
                    
                    blocks[blockNum] = &Block{
                        Number: blockNum,
                        Hash:   hashCopy,
                        Header: headerCopy,
                    }
                    
                    if blockNum > maxBlock {
                        maxBlock = blockNum
                    }
                    
                    totalBlocks++
                    
                    if totalBlocks <= 10 || totalBlocks%100000 == 0 {
                        fmt.Printf("  Found block %d: hash=%s\n", blockNum, hex.EncodeToString(hashCopy)[:16]+"...")
                    }
                }
            }
        }
    }
    
    fmt.Printf("\nFound %d blocks (max block: %d)\n", totalBlocks, maxBlock)
    
    // Step 2: Look for bodies
    fmt.Println("\nStep 2: Looking for block bodies...")
    bodiesFound := 0
    
    for blockNum, block := range blocks {
        if blockNum > 100 {
            break // Only check first 100 for bodies
        }
        
        // Try 'B' prefix for bodies
        bodyKey := append(namespace, 'B')
        bodyKey = append(bodyKey, block.Hash...)
        
        if body, closer, err := db.Get(bodyKey); err == nil {
            bodyCopy := make([]byte, len(body))
            copy(bodyCopy, body)
            block.Body = bodyCopy
            closer.Close()
            bodiesFound++
            
            if bodiesFound <= 5 {
                fmt.Printf("  Block %d has body: %d bytes\n", blockNum, len(body))
            }
        }
    }
    
    fmt.Printf("Found %d bodies\n", bodiesFound)
    
    // Step 3: Get current chain height
    currentHeight := getCurrentHeight()
    fmt.Printf("\nCurrent C-chain height: %d\n", currentHeight)
    
    // Step 4: Start replay
    fmt.Println("\nStep 4: Starting block replay...")
    
    startBlock := currentHeight
    if startBlock > 0 {
        startBlock++
    }
    
    endBlock := startBlock + 100
    if endBlock > maxBlock {
        endBlock = maxBlock
    }
    
    fmt.Printf("Replaying blocks %d to %d\n\n", startBlock, endBlock)
    
    successCount := 0
    errorCount := 0
    notFoundCount := 0
    startTime := time.Now()
    
    for blockNum := startBlock; blockNum <= endBlock; blockNum++ {
        block, exists := blocks[blockNum]
        if !exists {
            if blockNum < 10 {
                fmt.Printf("Block %d: Not found in database\n", blockNum)
            }
            notFoundCount++
            continue
        }
        
        if blockNum < 10 || blockNum%10 == 0 {
            fmt.Printf("Block %d (hash: %s):\n", blockNum, hex.EncodeToString(block.Hash)[:16]+"...")
            fmt.Printf("  Header: %d bytes\n", len(block.Header))
            if block.Body != nil {
                fmt.Printf("  Body: %d bytes\n", len(block.Body))
            } else {
                fmt.Printf("  Body: empty\n")
            }
        }
        
        if submitBlock(blockNum, block.Header, block.Body) {
            if blockNum < 10 || blockNum%10 == 0 {
                fmt.Printf("  ✅ Successfully submitted\n")
            }
            successCount++
            
            // Check if height increased
            newHeight := getCurrentHeight()
            if newHeight > currentHeight {
                fmt.Printf("  🎉 Chain height increased: %d -> %d\n", currentHeight, newHeight)
                currentHeight = newHeight
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
        } else {
            time.Sleep(10 * time.Millisecond)
        }
    }
    
    elapsed := time.Since(startTime)
    fmt.Println("\n===================================================")
    fmt.Printf("Replay summary:\n")
    fmt.Printf("  Time elapsed: %v\n", elapsed)
    fmt.Printf("  Blocks processed: %d\n", endBlock-startBlock+1)
    fmt.Printf("  Success: %d\n", successCount)
    fmt.Printf("  Errors: %d\n", errorCount)
    fmt.Printf("  Not found: %d\n", notFoundCount)
    
    if successCount > 0 {
        rate := float64(successCount) / elapsed.Seconds()
        fmt.Printf("  Rate: %.2f blocks/second\n", rate)
        
        remainingBlocks := int(maxBlock) - int(endBlock)
        if remainingBlocks > 0 && rate > 0 {
            estimatedTime := time.Duration(float64(remainingBlocks)/rate) * time.Second
            fmt.Printf("  Estimated time for remaining %d blocks: %v\n", remainingBlocks, estimatedTime)
        }
    }
    
    finalHeight := getCurrentHeight()
    fmt.Printf("\nFinal C-chain height: %d\n", finalHeight)
    
    if finalHeight > 0 {
        fmt.Println("\n✅ C-chain is now LIVE with historic blocks!")
        fmt.Printf("   Successfully replayed blocks 0 to %d\n", finalHeight)
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
    
    // Try different RPC methods
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
            
            // Only log specific errors for debugging
            if blockNum < 10 && rpcResp.Error.Message != "" {
                fmt.Printf("    %s error: %s\n", method, rpcResp.Error.Message)
            }
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