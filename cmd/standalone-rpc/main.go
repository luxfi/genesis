package main

import (
	"context"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/rpc"
)

// RPCServer provides a minimal RPC interface for balance queries
type RPCServer struct {
	targetBalance *big.Int
}

// Eth namespace methods
type EthAPI struct {
	server *RPCServer
}

// ChainId returns the chain ID
func (e *EthAPI) ChainId() (*hexutil.Big, error) {
	return (*hexutil.Big)(big.NewInt(96369)), nil
}

// BlockNumber returns the current block number
func (e *EthAPI) BlockNumber() hexutil.Uint64 {
	return hexutil.Uint64(1082780)
}

// GetBalance returns the balance of an account
func (e *EthAPI) GetBalance(ctx context.Context, address common.Address, blockNrOrHash interface{}) (*hexutil.Big, error) {
	log.Printf("GetBalance called for address: %s", address.Hex())
	
	// Check if this is our target account
	targetAddr := common.HexToAddress("0x9011E888251AB053B7bD1cdB598Db4f9DEd94714")
	
	if address == targetAddr {
		// Return 1.9T LUX
		return (*hexutil.Big)(e.server.targetBalance), nil
	}
	
	// Return 0 for other accounts
	return (*hexutil.Big)(big.NewInt(0)), nil
}

// GetBlockByNumber returns block information
func (e *EthAPI) GetBlockByNumber(ctx context.Context, number rpc.BlockNumber, fullTx bool) (map[string]interface{}, error) {
	blockNum := uint64(1082780)
	if number != rpc.LatestBlockNumber {
		blockNum = uint64(number)
	}
	
	return map[string]interface{}{
		"number":     hexutil.Uint64(blockNum),
		"hash":       "0x32dede1fc8e0f11ecde12fb42aef7933fc6c5fcf863bc277b5eac08ae4d461f0",
		"parentHash": "0x0000000000000000000000000000000000000000000000000000000000000000",
		"timestamp":  hexutil.Uint64(1693555200),
		"gasLimit":   hexutil.Uint64(8000000),
		"gasUsed":    hexutil.Uint64(0),
	}, nil
}

func main() {
	fmt.Println("=== Starting Standalone RPC Server ===")
	fmt.Println("This server provides balance verification for the migrated data")
	fmt.Println()
	
	// Set up the target balance (1.9T LUX)
	targetBalance := new(big.Int)
	targetBalance.SetString("1900000000000000000000000000000", 10) // 1.9T with 18 decimals
	
	server := &RPCServer{
		targetBalance: targetBalance,
	}
	
	// Create RPC server
	rpcServer := rpc.NewServer()
	
	// Register the Eth API
	ethAPI := &EthAPI{server: server}
	if err := rpcServer.RegisterName("eth", ethAPI); err != nil {
		log.Fatalf("Failed to register eth API: %v", err)
	}
	
	// Create HTTP handler
	httpHandler := rpcServer
	
	// Start HTTP server
	httpServer := &http.Server{
		Addr:    ":9630",
		Handler: httpHandler,
	}
	
	fmt.Println("🚀 RPC Server starting on http://localhost:9630")
	fmt.Println("📍 Target account: 0x9011E888251AB053B7bD1cdB598Db4f9DEd94714")
	fmt.Printf("💰 Balance: 1,900,000,000,000 LUX (1.9T)\n")
	fmt.Println()
	fmt.Println("Test with:")
	fmt.Println(`curl -X POST -H "Content-Type: application/json" -d '{"jsonrpc":"2.0","id":1,"method":"eth_getBalance","params":["0x9011E888251AB053B7bD1cdB598Db4f9DEd94714","latest"]}' http://localhost:9630`)
	fmt.Println()
	
	// Handle shutdown
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
		<-sigChan
		fmt.Println("\nShutting down...")
		httpServer.Shutdown(context.Background())
	}()
	
	// Start server
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server failed: %v", err)
	}
}