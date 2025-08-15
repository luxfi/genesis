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

	"golang.org/x/crypto/sha3"
	"github.com/luxfi/database/badgerdb"
	"github.com/luxfi/geth/common"
	"github.com/luxfi/geth/rlp"
)

// Account represents an Ethereum account
type Account struct {
	Nonce    uint64
	Balance  *big.Int
	Root     common.Hash
	CodeHash []byte
}

type DirectStateReader struct {
	db          *badgerdb.Database
	stateRoot   common.Hash
	blockHeight uint64
}

func NewDirectStateReader() (*DirectStateReader, error) {
	fmt.Println("╔════════════════════════════════════════════════════════╗")
	fmt.Println("║    Lux Mainnet - DIRECT STATE READER                  ║")
	fmt.Println("╚════════════════════════════════════════════════════════╝")
	fmt.Println()

	// Open the migrated database
	dbPath := "/Users/z/.luxd/network-96369/chains/EWi9aPkGe6EfJ3SobCAmSUXRPLa4brF3cThwPwmHTrD1y13jy/ethdb"
	fmt.Printf("Opening database: %s\n", dbPath)
	
	db, err := badgerdb.New(filepath.Clean(dbPath), nil, "", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %v", err)
	}

	reader := &DirectStateReader{
		db:          db,
		blockHeight: 1082780,
	}

	// Get the state root from the latest block
	if err := reader.loadStateRoot(); err != nil {
		return nil, err
	}

	return reader, nil
}

func (r *DirectStateReader) loadStateRoot() error {
	// Get the canonical hash for the latest block
	canonKey := make([]byte, 10)
	canonKey[0] = 'h'
	binary.BigEndian.PutUint64(canonKey[1:9], r.blockHeight)
	canonKey[9] = 'n'
	
	hashBytes, err := r.db.Get(canonKey)
	if err != nil {
		return fmt.Errorf("failed to get canonical hash: %v", err)
	}
	
	var blockHash common.Hash
	copy(blockHash[:], hashBytes)
	fmt.Printf("Latest block hash: %s\n", blockHash.Hex())
	
	// Get the header for this block
	headerKey := append([]byte{'h'}, append(make([]byte, 8), blockHash[:]...)...)
	binary.BigEndian.PutUint64(headerKey[1:9], r.blockHeight)
	
	headerRLP, err := r.db.Get(headerKey)
	if err != nil {
		return fmt.Errorf("failed to get header: %v", err)
	}
	
	// Decode header manually to get state root
	// Header RLP structure: [parentHash, uncleHash, coinbase, stateRoot, txRoot, receiptRoot, ...]
	var headerList []interface{}
	if err := rlp.DecodeBytes(headerRLP, &headerList); err != nil {
		return fmt.Errorf("failed to decode header: %v", err)
	}
	
	// State root is the 4th element (index 3)
	if len(headerList) > 3 {
		if stateRootBytes, ok := headerList[3].([]byte); ok {
			copy(r.stateRoot[:], stateRootBytes)
			fmt.Printf("State root: %s\n", r.stateRoot.Hex())
		}
	}
	
	return nil
}

// keccak256 computes the Keccak256 hash
func keccak256(data []byte) []byte {
	hasher := sha3.NewLegacyKeccak256()
	hasher.Write(data)
	return hasher.Sum(nil)
}

// GetAccountRLP gets the raw RLP data for an account from the state trie
func (r *DirectStateReader) GetAccountRLP(address common.Address) ([]byte, error) {
	// In a secure trie, the key is the hash of the address
	addrHash := keccak256(address.Bytes())
	
	// Try to find the account in the state trie
	// State trie nodes are stored with their hash as the key
	// We need to traverse the trie from the root
	
	// For now, let's try a direct lookup with common prefixes
	// State data can be stored with various prefixes
	
	// Try with 's' prefix (state nodes)
	stateKey := append([]byte{'s'}, addrHash...)
	if data, err := r.db.Get(stateKey); err == nil {
		return data, nil
	}
	
	// Try with 'S' prefix (storage)
	storageKey := append([]byte{'S'}, addrHash...)
	if data, err := r.db.Get(storageKey); err == nil {
		return data, nil
	}
	
	// Try with 'a' prefix (accounts)
	accountKey := append([]byte{'a'}, addrHash...)
	if data, err := r.db.Get(accountKey); err == nil {
		return data, nil
	}
	
	// Try direct hash lookup (state trie nodes are often stored by hash)
	if data, err := r.db.Get(addrHash); err == nil {
		return data, nil
	}
	
	return nil, fmt.Errorf("account not found in state")
}

// ScanForAccounts scans the database for account data patterns
func (r *DirectStateReader) ScanForAccounts() {
	fmt.Println("\n🔍 Scanning for account data patterns...")
	
	iter := r.db.NewIterator()
	defer iter.Release()
	
	samples := 0
	for iter.Next() && samples < 100 {
		key := iter.Key()
		value := iter.Value()
		
		// Look for RLP-encoded account structures
		// Account RLP should decode to [nonce, balance, stateRoot, codeHash]
		if len(value) > 20 && len(value) < 200 {
			var acc Account
			if err := rlp.DecodeBytes(value, &acc); err == nil {
				// Successfully decoded as account
				fmt.Printf("  Found account at key %x: balance=%s\n", key[:8], acc.Balance.String())
				samples++
			}
		}
	}
}

// GetBalance attempts to get balance for an address
func (r *DirectStateReader) GetBalance(address common.Address) (*big.Int, error) {
	// Try to get account RLP
	accountRLP, err := r.GetAccountRLP(address)
	if err != nil {
		// Account not found, balance is 0
		return big.NewInt(0), nil
	}
	
	// Decode the account
	var acc Account
	if err := rlp.DecodeBytes(accountRLP, &acc); err != nil {
		return nil, fmt.Errorf("failed to decode account: %v", err)
	}
	
	return acc.Balance, nil
}

// RPC handler
func (r *DirectStateReader) handleRPC(w http.ResponseWriter, r2 *http.Request) {
	if r2.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Jsonrpc string          `json:"jsonrpc"`
		Method  string          `json:"method"`
		Params  json.RawMessage `json:"params"`
		ID      interface{}     `json:"id"`
	}

	body, _ := io.ReadAll(r2.Body)
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
			
			balance, err := r.GetBalance(address)
			if err != nil {
				rpcErr = err
			} else {
				result = fmt.Sprintf("0x%x", balance)
				
				// Log the request
				balanceEth := new(big.Float).Quo(new(big.Float).SetInt(balance), big.NewFloat(1e18))
				fmt.Printf("[RPC] eth_getBalance %s -> %.6f LUX\n", addrStr[:10]+"...", balanceEth)
			}
		}
		
	case "eth_blockNumber":
		result = fmt.Sprintf("0x%x", r.blockHeight)
		fmt.Printf("[RPC] eth_blockNumber -> %d\n", r.blockHeight)
		
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

// AnalyzeStateData analyzes the structure of state data in the database
func (r *DirectStateReader) AnalyzeStateData() {
	fmt.Println("\n📊 Analyzing state data structure...")
	
	prefixCounts := make(map[byte]int)
	sizeBuckets := map[string]int{
		"tiny (<32)":       0,
		"small (32-64)":    0,
		"medium (64-256)":  0,
		"large (256-1024)": 0,
		"huge (>1024)":     0,
	}
	
	iter := r.db.NewIterator()
	defer iter.Release()
	
	total := 0
	for iter.Next() && total < 1000000 {
		key := iter.Key()
		value := iter.Value()
		
		if len(key) > 0 {
			prefixCounts[key[0]]++
		}
		
		// Size analysis
		size := len(value)
		if size < 32 {
			sizeBuckets["tiny (<32)"]++
		} else if size < 64 {
			sizeBuckets["small (32-64)"]++
		} else if size < 256 {
			sizeBuckets["medium (64-256)"]++
		} else if size < 1024 {
			sizeBuckets["large (256-1024)"]++
		} else {
			sizeBuckets["huge (>1024)"]++
		}
		
		total++
		if total%100000 == 0 {
			fmt.Printf("\r  Analyzed %d keys...", total)
		}
	}
	
	fmt.Printf("\r  Analyzed %d keys total\n", total)
	
	fmt.Println("\n  Key prefixes:")
	for prefix, count := range prefixCounts {
		if count > 1000 {
			fmt.Printf("    '%c' (0x%02x): %d keys\n", prefix, prefix, count)
		}
	}
	
	fmt.Println("\n  Value sizes:")
	for bucket, count := range sizeBuckets {
		fmt.Printf("    %s: %d\n", bucket, count)
	}
	
	// Look for account-like structures
	fmt.Println("\n  Looking for account structures...")
	iter2 := r.db.NewIterator()
	defer iter2.Release()
	
	accountsFound := 0
	for iter2.Next() && accountsFound < 10 {
		value := iter2.Value()
		
		// Account RLP is typically 70-120 bytes
		if len(value) >= 70 && len(value) <= 120 {
			var acc Account
			if err := rlp.DecodeBytes(value, &acc); err == nil && acc.Balance != nil {
				key := iter2.Key()
				balanceEth := new(big.Float).Quo(new(big.Float).SetInt(acc.Balance), big.NewFloat(1e18))
				fmt.Printf("    Found account at key %x...: %.6f LUX\n", key[:8], balanceEth)
				accountsFound++
			}
		}
	}
}

func main() {
	reader, err := NewDirectStateReader()
	if err != nil {
		panic(err)
	}
	defer reader.db.Close()

	fmt.Println("\n📊 Mainnet Status:")
	fmt.Printf("  Chain Height:     %d blocks\n", reader.blockHeight)
	fmt.Printf("  State Root:       %s\n", reader.stateRoot.Hex())
	fmt.Printf("  Network ID:       96369 (Lux Mainnet)\n")
	fmt.Printf("  Chain ID:         96369 (0x17871)\n")
	
	// Analyze the state data structure
	reader.AnalyzeStateData()
	
	// Try to scan for accounts
	reader.ScanForAccounts()
	
	fmt.Println("\n🚀 Starting Direct State Reader RPC Server...")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("  JSON-RPC:  http://localhost:9632")
	fmt.Println("  Network:   Lux Mainnet (96369)")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	
	fmt.Println("\n📝 Test Commands:")
	fmt.Println("  # Check address balance:")
	fmt.Println("  curl -X POST -H \"Content-Type: application/json\" \\")
	fmt.Println("    -d '{\"jsonrpc\":\"2.0\",\"method\":\"eth_getBalance\",\"params\":[\"0xEAbCC110fAcBfebabC66Ad6f9E7B67288e720B59\",\"latest\"],\"id\":1}' \\")
	fmt.Println("    http://localhost:9632")
	
	fmt.Println("\n📄 Request Log:")
	
	// Start HTTP server
	http.HandleFunc("/", reader.handleRPC)
	if err := http.ListenAndServe(":9632", nil); err != nil {
		fmt.Printf("Failed to start server: %v\n", err)
		os.Exit(1)
	}
}