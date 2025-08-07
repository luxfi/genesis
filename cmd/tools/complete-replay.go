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
    "os"
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
    fmt.Println("=============================================================")
    fmt.Println("       LUX C-Chain Complete Block Extraction & Replay       ")
    fmt.Println("=============================================================")
    fmt.Println()
    fmt.Println("Database: " + DB_PATH)
    fmt.Println("RPC URL:  " + RPC_URL)
    fmt.Println()
    
    if len(os.Args) > 1 {
        switch os.Args[1] {
        case "extract":
            extractBlocks()
        case "replay":
            replayBlocks()
        case "export":
            exportBlocks()
        default:
            showUsage()
        }
    } else {
        // Default: try replay
        replayBlocks()
    }
}

func showUsage() {
    fmt.Println("Usage:")
    fmt.Println("  go run complete-replay.go          # Attempt block replay via RPC")
    fmt.Println("  go run complete-replay.go extract  # Extract and analyze blocks")
    fmt.Println("  go run complete-replay.go export   # Export blocks to JSON files")
    fmt.Println("  go run complete-replay.go replay   # Replay blocks via RPC")
}

func extractBlocks() {
    fmt.Println("📊 Extracting and analyzing blocks from database...")
    fmt.Println()
    
    db, err := pebble.Open(DB_PATH, &pebble.Options{ReadOnly: true})
    if err != nil {
        log.Fatal("Failed to open database:", err)
    }
    defer db.Close()
    
    blocks := make(map[uint64]*Block)
    
    // Phase 1: Find all headers (namespace + hash format)
    fmt.Println("Phase 1: Scanning for headers...")
    iter, _ := db.NewIter(&pebble.IterOptions{})
    defer iter.Close()
    
    headerCount := 0
    for iter.First(); iter.Valid(); iter.Next() {
        key := make([]byte, len(iter.Key()))
        copy(key, iter.Key())
        value := iter.Value()
        
        if len(key) == 64 && bytes.HasPrefix(key, namespace) {
            hash := key[32:]
            
            // Check for RLP header
            if len(value) > 100 && (value[0] == 0xf8 || value[0] == 0xf9) {
                // Decode block number from first 3 bytes
                blockNum := uint64(hash[0])<<16 | uint64(hash[1])<<8 | uint64(hash[2])
                
                if blockNum < 1500000 {
                    hashCopy := make([]byte, 32)
                    copy(hashCopy, hash)
                    headerCopy := make([]byte, len(value))
                    copy(headerCopy, value)
                    
                    blocks[blockNum] = &Block{
                        Number: blockNum,
                        Hash:   hashCopy,
                        Header: headerCopy,
                    }
                    headerCount++
                    
                    if headerCount%100000 == 0 {
                        fmt.Printf("  Found %d headers...\n", headerCount)
                    }
                }
            }
        }
    }
    
    fmt.Printf("✅ Found %d headers\n\n", headerCount)
    
    // Phase 2: Find bodies (b prefix)
    fmt.Println("Phase 2: Scanning for bodies...")
    bodyCount := 0
    
    for blockNum, block := range blocks {
        if blockNum > 1000 {
            break // Only check first 1000 for bodies
        }
        
        bodyKey := append(namespace, 'b')
        bodyKey = append(bodyKey, block.Hash...)
        
        if body, closer, err := db.Get(bodyKey); err == nil {
            block.Body = make([]byte, len(body))
            copy(block.Body, body)
            closer.Close()
            bodyCount++
        } else if closer != nil {
            closer.Close()
        }
    }
    
    fmt.Printf("✅ Found %d bodies\n\n", bodyCount)
    
    // Phase 3: Analysis
    fmt.Println("📈 Block Analysis:")
    fmt.Println("==================")
    
    // Find gaps
    missing := []uint64{}
    for i := uint64(0); i < 100; i++ {
        if _, exists := blocks[i]; !exists {
            missing = append(missing, i)
        }
    }
    
    if len(missing) > 0 {
        fmt.Printf("Missing blocks in first 100: %v\n", missing)
    } else {
        fmt.Println("✅ First 100 blocks are complete")
    }
    
    // Find max block
    maxBlock := uint64(0)
    for blockNum := range blocks {
        if blockNum > maxBlock {
            maxBlock = blockNum
        }
    }
    
    fmt.Printf("Highest block number: %d\n", maxBlock)
    fmt.Printf("Total blocks found: %d\n", len(blocks))
    fmt.Printf("Coverage: %.2f%%\n", float64(len(blocks))/float64(maxBlock+1)*100)
}

func replayBlocks() {
    fmt.Println("🔄 Starting block replay via RPC...")
    fmt.Println()
    
    // Check available RPC methods
    if !checkRPCMethods() {
        fmt.Println("⚠️  Warning: Required RPC methods not available")
        fmt.Println("   Restart luxd with: --api-admin-enabled=true")
        fmt.Println("   Attempting replay anyway...")
        fmt.Println()
    }
    
    db, err := pebble.Open(DB_PATH, &pebble.Options{ReadOnly: true})
    if err != nil {
        log.Fatal("Failed to open database:", err)
    }
    defer db.Close()
    
    // Build block map
    fmt.Println("Loading blocks from database...")
    blocks := loadBlocks(db)
    fmt.Printf("Loaded %d blocks\n\n", len(blocks))
    
    // Get current height
    currentHeight := getCurrentHeight()
    fmt.Printf("Current C-chain height: %d\n", currentHeight)
    
    // Start replay
    startBlock := currentHeight
    if startBlock > 0 {
        startBlock++
    }
    
    endBlock := startBlock + 100
    
    fmt.Printf("\nReplaying blocks %d to %d...\n\n", startBlock, endBlock)
    
    successCount := 0
    errorCount := 0
    startTime := time.Now()
    
    for blockNum := startBlock; blockNum < endBlock; blockNum++ {
        block, exists := blocks[blockNum]
        if !exists {
            continue
        }
        
        if blockNum < 10 || blockNum%10 == 0 {
            fmt.Printf("Block %d: ", blockNum)
        }
        
        // Try to get body
        bodyKey := append(namespace, 'b')
        bodyKey = append(bodyKey, block.Hash...)
        
        if body, closer, err := db.Get(bodyKey); err == nil {
            block.Body = make([]byte, len(body))
            copy(block.Body, body)
            closer.Close()
        } else if closer != nil {
            closer.Close()
        }
        
        // Submit block
        success := false
        
        // Try different methods
        if tryEthSendRawTransaction(block) {
            success = true
        } else if tryImportRawBlock(block) {
            success = true
        }
        
        if success {
            if blockNum < 10 || blockNum%10 == 0 {
                fmt.Println("✅")
            }
            successCount++
            
            newHeight := getCurrentHeight()
            if newHeight > currentHeight {
                fmt.Printf("  🎉 Height increased: %d -> %d\n", currentHeight, newHeight)
                currentHeight = newHeight
            }
        } else {
            if blockNum < 10 || blockNum%10 == 0 {
                fmt.Println("❌")
            }
            errorCount++
        }
        
        time.Sleep(10 * time.Millisecond)
    }
    
    elapsed := time.Since(startTime)
    fmt.Println("\n=============================================================")
    fmt.Printf("Replay Summary:\n")
    fmt.Printf("  Time: %v\n", elapsed)
    fmt.Printf("  Success: %d\n", successCount)
    fmt.Printf("  Errors: %d\n", errorCount)
    
    if successCount > 0 {
        rate := float64(successCount) / elapsed.Seconds()
        fmt.Printf("  Rate: %.2f blocks/second\n", rate)
    }
    
    finalHeight := getCurrentHeight()
    fmt.Printf("\nFinal height: %d\n", finalHeight)
}

func exportBlocks() {
    fmt.Println("📁 Exporting blocks to JSON files...")
    fmt.Println()
    
    db, err := pebble.Open(DB_PATH, &pebble.Options{ReadOnly: true})
    if err != nil {
        log.Fatal("Failed to open database:", err)
    }
    defer db.Close()
    
    blocks := loadBlocks(db)
    
    // Create export directory
    os.Mkdir("block-export", 0755)
    
    exported := 0
    for blockNum := uint64(0); blockNum < 100 && blockNum < uint64(len(blocks)); blockNum++ {
        block, exists := blocks[blockNum]
        if !exists {
            continue
        }
        
        data := map[string]interface{}{
            "number": blockNum,
            "hash":   hex.EncodeToString(block.Hash),
            "header": hex.EncodeToString(block.Header),
        }
        
        if block.Body != nil {
            data["body"] = hex.EncodeToString(block.Body)
        }
        
        jsonData, _ := json.MarshalIndent(data, "", "  ")
        filename := fmt.Sprintf("block-export/block_%06d.json", blockNum)
        os.WriteFile(filename, jsonData, 0644)
        
        exported++
        if exported%10 == 0 {
            fmt.Printf("  Exported %d blocks...\n", exported)
        }
    }
    
    fmt.Printf("\n✅ Exported %d blocks to block-export/\n", exported)
}

func loadBlocks(db *pebble.DB) map[uint64]*Block {
    blocks := make(map[uint64]*Block)
    
    iter, _ := db.NewIter(&pebble.IterOptions{})
    defer iter.Close()
    
    for iter.First(); iter.Valid(); iter.Next() {
        key := make([]byte, len(iter.Key()))
        copy(key, iter.Key())
        value := iter.Value()
        
        if len(key) == 64 && bytes.HasPrefix(key, namespace) {
            hash := key[32:]
            
            if len(value) > 100 && (value[0] == 0xf8 || value[0] == 0xf9) {
                blockNum := uint64(hash[0])<<16 | uint64(hash[1])<<8 | uint64(hash[2])
                
                if blockNum < 1500000 {
                    hashCopy := make([]byte, 32)
                    copy(hashCopy, hash)
                    headerCopy := make([]byte, len(value))
                    copy(headerCopy, value)
                    
                    blocks[blockNum] = &Block{
                        Number: blockNum,
                        Hash:   hashCopy,
                        Header: headerCopy,
                    }
                }
            }
        }
    }
    
    return blocks
}

func checkRPCMethods() bool {
    req := RPCRequest{
        JSONRPC: "2.0",
        Method:  "rpc_modules",
        Params:  []interface{}{},
        ID:      1,
    }
    
    data, _ := json.Marshal(req)
    resp, err := http.Post(RPC_URL, "application/json", bytes.NewReader(data))
    if err != nil {
        return false
    }
    defer resp.Body.Close()
    
    body, _ := io.ReadAll(resp.Body)
    
    // Check if admin or debug modules are available
    return bytes.Contains(body, []byte("admin")) || bytes.Contains(body, []byte("debug"))
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

func tryEthSendRawTransaction(block *Block) bool {
    // Try as a raw transaction (won't work for blocks but worth trying)
    return false
}

func tryImportRawBlock(block *Block) bool {
    if block.Body == nil || len(block.Body) == 0 {
        block.Body = []byte{0xc0, 0xc0}
    }
    
    totalLen := len(block.Header) + len(block.Body)
    blockRLP := make([]byte, 0, totalLen+10)
    
    if totalLen < 56 {
        blockRLP = append(blockRLP, 0xc0+byte(totalLen))
    } else {
        lenBytes := encodeLength(totalLen)
        blockRLP = append(blockRLP, 0xf7+byte(len(lenBytes)))
        blockRLP = append(blockRLP, lenBytes...)
    }
    
    blockRLP = append(blockRLP, block.Header...)
    blockRLP = append(blockRLP, block.Body...)
    
    // Try various import methods
    methods := []string{"debug_insertBlock", "admin_importChain", "eth_sendRawBlock", "eth_submitBlock"}
    
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
        if json.Unmarshal(body, &rpcResp) == nil && rpcResp.Error == nil {
            return true
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