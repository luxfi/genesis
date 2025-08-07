package main

import (
    "bytes"
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
    fmt.Println("Direct Block Replay - Using discovered hash pattern")
    fmt.Println("=====================================================")
    fmt.Println("Pattern: Block hashes encode block number in first 3 bytes")
    fmt.Println()
    
    db, err := pebble.Open(DB_PATH, &pebble.Options{ReadOnly: true})
    if err != nil {
        log.Fatal("Failed to open database:", err)
    }
    defer db.Close()
    
    // Build block map by scanning for RLP headers
    fmt.Println("Building block map...")
    blocks := make(map[uint64]struct{
        hash []byte
        header []byte
        body []byte
    })
    
    iter, _ := db.NewIter(&pebble.IterOptions{})
    defer iter.Close()
    
    for iter.First(); iter.Valid(); iter.Next() {
        key := make([]byte, len(iter.Key()))
        copy(key, iter.Key())
        value := iter.Value()
        
        // Look for namespace + 32-byte hash
        if len(key) == 64 && bytes.HasPrefix(key, namespace) {
            hash := key[32:]
            
            // Check if value is RLP header
            if len(value) > 100 && (value[0] == 0xf8 || value[0] == 0xf9) {
                // Decode block number from first 3 bytes
                blockNum := uint64(hash[0])<<16 | uint64(hash[1])<<8 | uint64(hash[2])
                
                if blockNum < 1500000 {
                    hashCopy := make([]byte, 32)
                    copy(hashCopy, hash)
                    headerCopy := make([]byte, len(value))
                    copy(headerCopy, value)
                    
                    blocks[blockNum] = struct{
                        hash []byte
                        header []byte
                        body []byte
                    }{hashCopy, headerCopy, nil}
                }
            }
        }
    }
    
    fmt.Printf("Found %d blocks\n\n", len(blocks))
    
    // Display first 10 blocks to verify
    fmt.Println("First 10 blocks found:")
    for i := uint64(0); i < 10; i++ {
        if block, exists := blocks[i]; exists {
            fmt.Printf("  Block %d: hash=%s, header=%d bytes\n", 
                i, hex.EncodeToString(block.hash)[:16]+"...", len(block.header))
        } else {
            fmt.Printf("  Block %d: NOT FOUND\n", i)
        }
    }
    
    // Get current height
    currentHeight := getCurrentHeight()
    fmt.Printf("\nCurrent C-chain height: %d\n", currentHeight)
    
    // Start replay
    startBlock := currentHeight
    if startBlock > 0 {
        startBlock++
    }
    
    endBlock := startBlock + 1000
    maxBlock := uint64(len(blocks))
    if endBlock > maxBlock {
        endBlock = maxBlock
    }
    
    fmt.Printf("\nStarting replay from block %d to %d...\n\n", startBlock, endBlock)
    
    successCount := 0
    errorCount := 0
    notFoundCount := 0
    startTime := time.Now()
    
    for blockNum := startBlock; blockNum < endBlock; blockNum++ {
        block, exists := blocks[blockNum]
        if !exists {
            notFoundCount++
            continue
        }
        
        if blockNum < 10 || blockNum%100 == 0 {
            fmt.Printf("Block %d:\n", blockNum)
            fmt.Printf("  Hash: %s\n", hex.EncodeToString(block.hash)[:16]+"...")
            fmt.Printf("  Header: %d bytes\n", len(block.header))
        }
        
        // Look for body with 'b' prefix
        bodyKey := append(namespace, 'b')
        bodyKey = append(bodyKey, block.hash...)
        
        if bodyData, closer, err := db.Get(bodyKey); err == nil {
            block.body = make([]byte, len(bodyData))
            copy(block.body, bodyData)
            closer.Close()
            
            if blockNum < 10 || blockNum%100 == 0 {
                fmt.Printf("  Body: %d bytes\n", len(block.body))
            }
        } else {
            if closer != nil {
                closer.Close()
            }
            if blockNum < 10 || blockNum%100 == 0 {
                fmt.Printf("  Body: empty\n")
            }
        }
        
        // Submit block
        if submitBlock(blockNum, block.header, block.body) {
            if blockNum < 10 || blockNum%100 == 0 {
                fmt.Printf("  ✅ Successfully submitted\n")
            }
            successCount++
            
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
        
        // Delay
        if successCount > 0 {
            time.Sleep(5 * time.Millisecond)
        } else {
            time.Sleep(50 * time.Millisecond)
        }
        
        // Progress report
        if blockNum > 0 && blockNum%100 == 0 {
            elapsed := time.Since(startTime)
            rate := float64(successCount) / elapsed.Seconds()
            fmt.Printf("\nProgress: Block %d, Success: %d, Rate: %.1f blocks/s\n\n", blockNum, successCount, rate)
        }
    }
    
    elapsed := time.Since(startTime)
    fmt.Println("\n=====================================================")
    fmt.Printf("Replay complete!\n")
    fmt.Printf("  Time: %v\n", elapsed)
    fmt.Printf("  Success: %d\n", successCount)
    fmt.Printf("  Errors: %d\n", errorCount)
    fmt.Printf("  Not found: %d\n", notFoundCount)
    
    if successCount > 0 {
        rate := float64(successCount) / elapsed.Seconds()
        fmt.Printf("  Rate: %.2f blocks/second\n", rate)
    }
    
    finalHeight := getCurrentHeight()
    fmt.Printf("\nFinal C-chain height: %d\n", finalHeight)
    
    if finalHeight > 0 {
        fmt.Println("\n✅ SUCCESS! C-chain is LIVE with replayed blocks!")
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
    if bodyRLP == nil || len(bodyRLP) == 0 {
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
            if blockNum < 5 {
                fmt.Printf("    %s: %s\n", method, rpcResp.Error.Message)
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
    for i := 7; i >= 0; i-- {
        buf[i] = byte(length & 0xff)
        length >>= 8
        if length == 0 {
            return buf[i:]
        }
    }
    
    return buf
}