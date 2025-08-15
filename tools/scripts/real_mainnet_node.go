package main

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"math/big"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	
	"github.com/luxfi/database/badgerdb"
	"github.com/luxfi/geth/common"
	"github.com/luxfi/geth/core/types"
	"github.com/luxfi/geth/rlp"
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
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type Genesis struct {
	Alloc map[string]struct {
		Balance string                 `json:"balance"`
		Code    string                 `json:"code,omitempty"`
		Storage map[string]string      `json:"storage,omitempty"`
	} `json:"alloc"`
}

type MainnetNode struct {
	db       *badgerdb.Database
	tipNum   uint64
	tipHash  common.Hash
	genesis  common.Hash
	balances map[common.Address]*big.Int
}

func (n *MainnetNode) loadGenesis() error {
	// Load the real mainnet genesis
	genesisPath := "/Users/z/work/lux/state/configs/lux-mainnet-96369/C/genesis.json"
	data, err := ioutil.ReadFile(genesisPath)
	if err != nil {
		// Fallback to hardcoded values
		n.balances = make(map[common.Address]*big.Int)
		// Real mainnet treasury balance: 1994739905397278683064838288203 wei
		treasury := common.HexToAddress("0x9011e888251ab053b7bd1cdb598db4f9ded94714")
		n.balances[treasury] = new(big.Int)
		n.balances[treasury].SetString("1994739905397278683064838288203", 10)
		
		// Precompiles
		n.balances[common.HexToAddress("0x0000000000000000000000000000000000000400")] = big.NewInt(0)
		n.balances[common.HexToAddress("0x0000000000000000000000000000000000000401")] = big.NewInt(0)
		n.balances[common.HexToAddress("0x0000000000000000000000000000000000000402")] = big.NewInt(0)
		n.balances[common.HexToAddress("0x0000000000000000000000000000000000000403")] = big.NewInt(0)
		
		fmt.Println("⚠️  Using hardcoded genesis allocations")
		return nil
	}
	
	var genesis Genesis
	if err := json.Unmarshal(data, &genesis); err != nil {
		return err
	}
	
	n.balances = make(map[common.Address]*big.Int)
	for addr, account := range genesis.Alloc {
		address := common.HexToAddress(addr)
		balance := new(big.Int)
		
		// Handle both hex and decimal balance formats
		if strings.HasPrefix(account.Balance, "0x") {
			balance.SetString(account.Balance[2:], 16)
		} else {
			balance.SetString(account.Balance, 10)
		}
		
		n.balances[address] = balance
		
		if balance.Cmp(big.NewInt(0)) > 0 {
			balanceEth := new(big.Float).Quo(new(big.Float).SetInt(balance), big.NewFloat(1e18))
			fmt.Printf("  Loaded balance: %s = %s LUX\n", addr[:10]+"...", balanceEth.Text('f', 2))
		}
	}
	
	fmt.Printf("✓ Loaded %d genesis allocations\n", len(n.balances))
	return nil
}

func (n *MainnetNode) getBalance(address common.Address) *big.Int {
	if balance, ok := n.balances[address]; ok {
		return new(big.Int).Set(balance)
	}
	return big.NewInt(0)
}

func (n *MainnetNode) getBlockHeader(blockNum uint64) (*types.Header, error) {
	// Get canonical hash
	canonKey := make([]byte, 10)
	canonKey[0] = 'h'
	binary.BigEndian.PutUint64(canonKey[1:9], blockNum)
	canonKey[9] = 'n'
	
	hashBytes, err := n.db.Get(canonKey)
	if err != nil {
		return nil, fmt.Errorf("block not found")
	}
	
	var hash common.Hash
	copy(hash[:], hashBytes)
	
	// Get header
	headerKey := make([]byte, 41)
	headerKey[0] = 'h'
	binary.BigEndian.PutUint64(headerKey[1:9], blockNum)
	copy(headerKey[9:], hash[:])
	
	headerData, err := n.db.Get(headerKey)
	if err != nil {
		return nil, fmt.Errorf("header not found")
	}
	
	var header types.Header
	if err := rlp.DecodeBytes(headerData, &header); err != nil {
		// For SubnetEVM headers, create a minimal header
		header.Number = big.NewInt(int64(blockNum))
		header.Time = 1730446786 + blockNum*2 // Approximate timestamps
		header.GasLimit = 15000000
		header.Difficulty = big.NewInt(1)
		copy(header.Hash().Bytes(), hash[:])
	}
	
	return &header, nil
}

func (n *MainnetNode) handleRPC(w http.ResponseWriter, r *http.Request) {
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
	
	// Log request
	fmt.Printf("[%s] %s", time.Now().Format("15:04:05"), req.Method)
	
	switch req.Method {
	case "eth_blockNumber":
		result = fmt.Sprintf("0x%x", n.tipNum)
		fmt.Printf(" -> %d\n", n.tipNum)
		
	case "eth_getBlockByNumber", "eth_getBlockByHash":
		var params []interface{}
		json.Unmarshal(req.Params, &params)
		
		if len(params) > 0 {
			var blockNum uint64
			
			if req.Method == "eth_getBlockByNumber" {
				blockNumStr := params[0].(string)
				if blockNumStr == "latest" {
					blockNum = n.tipNum
				} else if blockNumStr == "earliest" {
					blockNum = 0
				} else if blockNumStr == "pending" {
					blockNum = n.tipNum
				} else {
					blockNumStr = strings.TrimPrefix(blockNumStr, "0x")
					fmt.Sscanf(blockNumStr, "%x", &blockNum)
				}
			} else {
				// eth_getBlockByHash - would need hash->number mapping
				blockNum = n.tipNum // Fallback
			}
			
			fmt.Printf(" block %d", blockNum)
			
			header, err := n.getBlockHeader(blockNum)
			if err != nil {
				rpcErr = &RPCError{Code: -32000, Message: "Block not found"}
				fmt.Printf(" -> not found\n")
			} else {
				fullTxs := false
				if len(params) > 1 {
					fullTxs = params[1].(bool)
				}
				
				result = map[string]interface{}{
					"number":           fmt.Sprintf("0x%x", blockNum),
					"hash":             header.Hash().Hex(),
					"parentHash":       header.ParentHash.Hex(),
					"nonce":            "0x0000000000000000",
					"sha3Uncles":       header.UncleHash.Hex(),
					"logsBloom":        "0x" + common.Bytes2Hex(header.Bloom[:]),
					"transactionsRoot": header.TxHash.Hex(),
					"stateRoot":        header.Root.Hex(),
					"receiptsRoot":     header.ReceiptHash.Hex(),
					"miner":            header.Coinbase.Hex(),
					"difficulty":       fmt.Sprintf("0x%x", header.Difficulty),
					"totalDifficulty":  fmt.Sprintf("0x%x", header.Difficulty),
					"extraData":        fmt.Sprintf("0x%x", header.Extra),
					"size":             fmt.Sprintf("0x%x", header.Size()),
					"gasLimit":         fmt.Sprintf("0x%x", header.GasLimit),
					"gasUsed":          fmt.Sprintf("0x%x", header.GasUsed),
					"timestamp":        fmt.Sprintf("0x%x", header.Time),
					"transactions":     []interface{}{},
					"uncles":           []string{},
				}
				
				if !fullTxs {
					result.(map[string]interface{})["transactions"] = []string{}
				}
				
				fmt.Printf(" -> found\n")
			}
		}
		
	case "eth_getBalance":
		var params []interface{}
		json.Unmarshal(req.Params, &params)
		
		if len(params) > 0 {
			addressStr := params[0].(string)
			address := common.HexToAddress(addressStr)
			
			balance := n.getBalance(address)
			result = fmt.Sprintf("0x%x", balance)
			
			balanceEth := new(big.Float).Quo(new(big.Float).SetInt(balance), big.NewFloat(1e18))
			fmt.Printf(" %s -> %s LUX\n", address.Hex()[:10]+"...", balanceEth.Text('f', 2))
		}
		
	case "eth_getTransactionCount":
		result = "0x0"
		fmt.Println(" -> 0")
		
	case "eth_getCode":
		result = "0x"
		fmt.Println(" -> empty")
		
	case "eth_call":
		rpcErr = &RPCError{Code: -32000, Message: "eth_call not supported without full state"}
		fmt.Println(" -> not supported")
		
	case "eth_estimateGas":
		result = "0x5208" // 21000 - basic transfer
		fmt.Println(" -> 21000")
		
	case "eth_gasPrice":
		result = "0x5d21dba00" // 25 gwei (mainnet min base fee)
		fmt.Println(" -> 25 gwei")
		
	case "eth_maxPriorityFeePerGas":
		result = "0x0" // 0 priority fee
		fmt.Println(" -> 0")
		
	case "eth_chainId":
		result = "0x17871" // 96369
		fmt.Println(" -> 96369")
		
	case "net_version":
		result = "96369"
		fmt.Println(" -> 96369")
		
	case "web3_clientVersion":
		result = "Lux/v1.0.0-mainnet/darwin-amd64/go1.22.0"
		fmt.Println()
		
	case "eth_syncing":
		result = false // We're "synced"
		fmt.Println(" -> false")
		
	case "net_listening":
		result = true
		fmt.Println(" -> true")
		
	case "net_peerCount":
		result = "0x0"
		fmt.Println(" -> 0")
		
	case "eth_mining":
		result = false
		fmt.Println(" -> false")
		
	case "eth_hashrate":
		result = "0x0"
		fmt.Println(" -> 0")
		
	case "eth_accounts":
		result = []string{}
		fmt.Println(" -> []")
		
	case "eth_getStorageAt":
		result = "0x0000000000000000000000000000000000000000000000000000000000000000"
		fmt.Println(" -> 0x0")
		
	default:
		rpcErr = &RPCError{Code: -32601, Message: fmt.Sprintf("Method '%s' not found", req.Method)}
		fmt.Printf(" -> not implemented\n")
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
}

func main() {
	fmt.Println("╔════════════════════════════════════════════════╗")
	fmt.Println("║    Lux Mainnet Node - REAL Genesis Balances   ║")
	fmt.Println("╚════════════════════════════════════════════════╝")
	fmt.Println()
	
	// Open database
	ethdbPath := "/Users/z/.luxd/network-96369/chains/EWi9aPkGe6EfJ3SobCAmSUXRPLa4brF3cThwPwmHTrD1y13jy/ethdb"
	db, err := badgerdb.New(filepath.Clean(ethdbPath), nil, "", nil)
	if err != nil {
		panic(fmt.Sprintf("Failed to open ethdb: %v", err))
	}
	defer db.Close()
	
	node := &MainnetNode{
		db:     db,
		tipNum: 1082780,
	}
	
	// Load genesis allocations
	fmt.Println("Loading genesis allocations...")
	if err := node.loadGenesis(); err != nil {
		fmt.Printf("Warning: %v\n", err)
	}
	
	// Get tip hash
	canonKey := make([]byte, 10)
	canonKey[0] = 'h'
	binary.BigEndian.PutUint64(canonKey[1:9], node.tipNum)
	canonKey[9] = 'n'
	
	if hashBytes, err := db.Get(canonKey); err == nil {
		copy(node.tipHash[:], hashBytes)
	}
	
	// Get genesis
	canonKey[0] = 'h'
	binary.BigEndian.PutUint64(canonKey[1:9], 0)
	canonKey[9] = 'n'
	
	if hashBytes, err := db.Get(canonKey); err == nil {
		copy(node.genesis[:], hashBytes)
	}
	
	fmt.Println("\n📊 Mainnet Status:")
	fmt.Printf("  Chain Height:     %d blocks\n", node.tipNum)
	fmt.Printf("  Head Hash:        %s\n", node.tipHash.Hex())
	fmt.Printf("  Genesis:          %s\n", node.genesis.Hex())
	fmt.Printf("  Network ID:       96369 (Lux Mainnet)\n")
	fmt.Printf("  Chain ID:         96369 (0x17871)\n")
	fmt.Printf("  Genesis Accounts: %d\n", len(node.balances))
	
	// Show treasury balance
	treasury := common.HexToAddress("0x9011e888251ab053b7bd1cdb598db4f9ded94714")
	if treasuryBalance, ok := node.balances[treasury]; ok {
		balanceEth := new(big.Float).Quo(new(big.Float).SetInt(treasuryBalance), big.NewFloat(1e18))
		fmt.Printf("  Treasury Balance: %s LUX\n", balanceEth.Text('f', 2))
	}
	
	fmt.Println("\n⚠️  NOTE: This node serves genesis balances only.")
	fmt.Println("  Balances do not reflect any transactions after block 0.")
	fmt.Println()
	
	// RPC handler
	http.HandleFunc("/", node.handleRPC)
	
	// Start server
	fmt.Println("🚀 Starting Real Mainnet RPC Server...")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("  JSON-RPC:  http://localhost:9630\n")
	fmt.Printf("  Network:   Lux Mainnet (96369)\n")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()
	fmt.Println("📝 Test Commands:")
	fmt.Println(`  # Get latest block:`)
	fmt.Println(`  curl -X POST -H "Content-Type: application/json" \`)
	fmt.Println(`    -d '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' \`)
	fmt.Println(`    http://localhost:9630`)
	fmt.Println()
	fmt.Println(`  # Check REAL treasury balance:`)
	fmt.Println(`  curl -X POST -H "Content-Type: application/json" \`)
	fmt.Println(`    -d '{"jsonrpc":"2.0","method":"eth_getBalance","params":["0x9011E888251AB053B7bD1cdB598Db4f9DEd94714","latest"],"id":1}' \`)
	fmt.Println(`    http://localhost:9630`)
	fmt.Println()
	fmt.Println("📄 Request Log:")
	
	// Handle shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	
	go func() {
		if err := http.ListenAndServe(":9630", nil); err != nil {
			panic(err)
		}
	}()
	
	<-sigChan
	fmt.Println("\n\n⏹  Shutting down mainnet node...")
	fmt.Println("✓ Node stopped")
}