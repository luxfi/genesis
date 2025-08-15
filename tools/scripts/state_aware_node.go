package main

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"os"
	"path/filepath"

	"github.com/luxfi/database/badgerdb"
	"github.com/luxfi/geth/common"
	"github.com/luxfi/geth/core/state"
	"github.com/luxfi/geth/core/types"
	"github.com/luxfi/geth/rlp"
	"github.com/luxfi/geth/trie"
)

type StateAwareNode struct {
	db          *badgerdb.Database
	stateDB     state.Database
	latestRoot  common.Hash
	blockHeight uint64
}

func NewStateAwareNode() (*StateAwareNode, error) {
	fmt.Println("╔════════════════════════════════════════════════════════╗")
	fmt.Println("║    Lux Mainnet Node - FULL STATE ACCESS               ║")
	fmt.Println("╚════════════════════════════════════════════════════════╝")
	fmt.Println()

	// Open the migrated database
	dbPath := "/Users/z/.luxd/network-96369/chains/EWi9aPkGe6EfJ3SobCAmSUXRPLa4brF3cThwPwmHTrD1y13jy/ethdb"
	fmt.Printf("Opening database: %s\n", dbPath)
	
	db, err := badgerdb.New(filepath.Clean(dbPath), nil, "", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %v", err)
	}

	node := &StateAwareNode{
		db:          db,
		blockHeight: 1082780,
	}

	// Get the latest block state root
	if err := node.loadLatestState(); err != nil {
		return nil, err
	}

	return node, nil
}

func (n *StateAwareNode) loadLatestState() error {
	// Get the canonical hash for the latest block
	canonKey := make([]byte, 10)
	canonKey[0] = 'h'
	binary.BigEndian.PutUint64(canonKey[1:9], n.blockHeight)
	canonKey[9] = 'n'
	
	hashBytes, err := n.db.Get(canonKey)
	if err != nil {
		return fmt.Errorf("failed to get canonical hash: %v", err)
	}
	
	var blockHash common.Hash
	copy(blockHash[:], hashBytes)
	fmt.Printf("Latest block hash: %s\n", blockHash.Hex())
	
	// Get the header for this block
	headerKey := append([]byte{'h'}, append(make([]byte, 8), blockHash[:]...)...)
	binary.BigEndian.PutUint64(headerKey[1:9], n.blockHeight)
	
	headerRLP, err := n.db.Get(headerKey)
	if err != nil {
		return fmt.Errorf("failed to get header: %v", err)
	}
	
	// Decode the header to get the state root
	var header types.Header
	if err := rlp.DecodeBytes(headerRLP, &header); err != nil {
		return fmt.Errorf("failed to decode header: %v", err)
	}
	
	n.latestRoot = header.Root
	fmt.Printf("State root: %s\n", n.latestRoot.Hex())
	
	// Create state database wrapper
	n.stateDB = state.NewDatabaseWithConfig(n, &trie.Config{})
	
	return nil
}

// Database interface implementation for state access
func (n *StateAwareNode) OpenTrie(root common.Hash) (state.Trie, error) {
	return trie.NewSecure(root, trie.NewDatabase(n))
}

func (n *StateAwareNode) OpenStorageTrie(addrHash, root common.Hash) (state.Trie, error) {
	return trie.NewSecure(root, trie.NewDatabase(n))
}

func (n *StateAwareNode) CopyTrie(state.Trie) state.Trie {
	panic("not implemented")
}

func (n *StateAwareNode) ContractCode(addrHash, codeHash common.Hash) ([]byte, error) {
	// Code is stored with 'c' prefix + code hash
	key := append([]byte{'c'}, codeHash.Bytes()...)
	return n.db.Get(key)
}

func (n *StateAwareNode) ContractCodeSize(addrHash, codeHash common.Hash) (int, error) {
	code, err := n.ContractCode(addrHash, codeHash)
	return len(code), err
}

func (n *StateAwareNode) TrieDB() *trie.Database {
	return trie.NewDatabase(n)
}

// Implement ethdb.KeyValueReader interface
func (n *StateAwareNode) Get(key []byte) ([]byte, error) {
	return n.db.Get(key)
}

func (n *StateAwareNode) Has(key []byte) (bool, error) {
	return n.db.Has(key)
}

// GetBalance retrieves the balance from the state trie
func (n *StateAwareNode) GetBalance(address common.Address) (*big.Int, error) {
	// Create state at latest root
	stateObj, err := state.New(n.latestRoot, n.stateDB, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create state: %v", err)
	}
	
	// Get balance from state
	balance := stateObj.GetBalance(address)
	return balance, nil
}

// RPC handler
func (n *StateAwareNode) handleRPC(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Jsonrpc string          `json:"jsonrpc"`
		Method  string          `json:"method"`
		Params  json.RawMessage `json:"params"`
		ID      interface{}     `json:"id"`
	}

	body, _ := io.ReadAll(r.Body)
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	var result interface{}
	var rpcErr error

	switch req.Method {
	case "eth_getBalance":
		var params []interface{}
		json.Unmarshal(req.Params, &params)
		if len(params) > 0 {
			addrStr := params[0].(string)
			address := common.HexToAddress(addrStr)
			
			balance, err := n.GetBalance(address)
			if err != nil {
				rpcErr = err
			} else {
				result = fmt.Sprintf("0x%x", balance)
				
				// Log the request
				balanceEth := new(big.Float).Quo(new(big.Float).SetInt(balance), big.NewFloat(1e18))
				fmt.Printf("[RPC] eth_getBalance %s -> %.2f LUX\n", addrStr[:10]+"...", balanceEth)
			}
		}
		
	case "eth_blockNumber":
		result = fmt.Sprintf("0x%x", n.blockHeight)
		fmt.Printf("[RPC] eth_blockNumber -> %d\n", n.blockHeight)
		
	case "eth_chainId":
		result = "0x17871" // 96369 in hex
		
	default:
		rpcErr = fmt.Errorf("method not supported: %s", req.Method)
	}

	resp := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      req.ID,
	}
	
	if rpcErr != nil {
		resp["error"] = map[string]interface{}{
			"code":    -32000,
			"message": rpcErr.Error(),
		}
	} else {
		resp["result"] = result
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func main() {
	node, err := NewStateAwareNode()
	if err != nil {
		panic(err)
	}
	defer node.db.Close()

	fmt.Println("\n📊 Mainnet Status:")
	fmt.Printf("  Chain Height:     %d blocks\n", node.blockHeight)
	fmt.Printf("  State Root:       %s\n", node.latestRoot.Hex())
	fmt.Printf("  Network ID:       96369 (Lux Mainnet)\n")
	fmt.Printf("  Chain ID:         96369 (0x17871)\n")
	
	fmt.Println("\n⚠️  NOTE: This node serves FULL STATE balances from block 1082780.")
	fmt.Println("  All account balances reflect the actual on-chain state.")
	
	// Test a few known addresses
	fmt.Println("\n🔍 Testing state access...")
	
	// Treasury
	treasury := common.HexToAddress("0x9011E888251AB053B7bD1cdB598Db4f9DEd94714")
	if balance, err := node.GetBalance(treasury); err == nil {
		balanceEth := new(big.Float).Quo(new(big.Float).SetInt(balance), big.NewFloat(1e18))
		fmt.Printf("  Treasury balance: %.2f LUX\n", balanceEth)
	}
	
	fmt.Println("\n🚀 Starting State-Aware RPC Server...")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("  JSON-RPC:  http://localhost:9631")
	fmt.Println("  Network:   Lux Mainnet (96369)")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	
	fmt.Println("\n📝 Test Commands:")
	fmt.Println("  # Check any address balance:")
	fmt.Println("  curl -X POST -H \"Content-Type: application/json\" \\")
	fmt.Println("    -d '{\"jsonrpc\":\"2.0\",\"method\":\"eth_getBalance\",\"params\":[\"0xEAbCC110fAcBfebabC66Ad6f9E7B67288e720B59\",\"latest\"],\"id\":1}' \\")
	fmt.Println("    http://localhost:9631")
	
	fmt.Println("\n📄 Request Log:")
	
	// Start HTTP server
	http.HandleFunc("/", node.handleRPC)
	if err := http.ListenAndServe(":9631", nil); err != nil {
		fmt.Printf("Failed to start server: %v\n", err)
		os.Exit(1)
	}
}