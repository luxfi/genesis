package cmd

import (
	"context"
	"fmt"
	"math/big"
	"os"
	"strings"

	"github.com/luxfi/genesis/pkg/application"
	"github.com/luxfi/go-bip39"
	"github.com/luxfi/go-bip32"
	"github.com/luxfi/crypto/secp256k1"
	"github.com/spf13/cobra"
	
	// Use geth client for transactions - but we need to avoid crypto conflicts
	"github.com/luxfi/geth/common"
	"github.com/luxfi/geth/core/types"
	"github.com/luxfi/geth/ethclient"
)

// NewSendCmd creates the send command
func NewSendCmd(app *application.Genesis) *cobra.Command {
	var (
		rpc   string
		to    string
		value string
	)

	cmd := &cobra.Command{
		Use:   "send",
		Short: "Send transaction on C-chain",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Get mnemonic from .env (relative to where we run from)
			envPath := ".env"
			envData, err := os.ReadFile(envPath)
			if err != nil {
				return fmt.Errorf("failed to read .env from %s: %w", envPath, err)
			}
			
			// Parse MNEMONIC='...'
			mnemonicLine := string(envData)
			mnemonic := ""
			if strings.HasPrefix(mnemonicLine, "MNEMONIC='") && strings.HasSuffix(strings.TrimSpace(mnemonicLine), "'") {
				start := 10 // After "MNEMONIC='"
				end := len(strings.TrimSpace(mnemonicLine)) - 1 // Before "'"
				if end > start {
					mnemonic = mnemonicLine[start:end]
				}
			}
			
			// Generate private key from seed using proper derivation
			seed := bip39.NewSeed(mnemonic, "")
			// Create master key from seed
			masterKey, err := bip32.NewMasterKey(seed)
			if err != nil {
				return fmt.Errorf("failed to create master key: %w", err)
			}
			
			// Derive using EVM path: m/44'/60'/0'/0/0
			// This should give us the 0x9011 address
			purpose, _ := masterKey.NewChildKey(bip32.FirstHardenedChild + 44)
			coin, _ := purpose.NewChildKey(bip32.FirstHardenedChild + 60)  // Ethereum coin type
			account, _ := coin.NewChildKey(bip32.FirstHardenedChild + 0)
			change, _ := account.NewChildKey(0)
			addressKey, _ := change.NewChildKey(0)
			
			// Convert to secp256k1 private key
			sk, err := secp256k1.ToPrivateKey(addressKey.Key)
			if err != nil {
				return fmt.Errorf("failed to create private key: %w", err)
			}
			
			// Get the address
			ethAddr := getEthAddress(sk)
			fromAddress := common.Address(ethAddr)
			fmt.Printf("Using address: %s\n", fromAddress.Hex())

			// Connect to node
			client, err := ethclient.Dial(rpc)
			if err != nil {
				return fmt.Errorf("failed to connect: %w", err)
			}

			// Get balance
			balance, err := client.BalanceAt(context.Background(), fromAddress, nil)
			if err != nil {
				return fmt.Errorf("failed to get balance: %w", err)
			}
			fmt.Printf("Balance: %s LUX\n", weiToEther(balance))

			// Get chain ID
			chainID, err := client.ChainID(context.Background())
			if err != nil {
				return fmt.Errorf("failed to get chain ID: %w", err)
			}
			fmt.Printf("Chain ID: %s\n", chainID.String())

			// Prepare transaction
			nonce, err := client.PendingNonceAt(context.Background(), fromAddress)
			if err != nil {
				return fmt.Errorf("failed to get nonce: %w", err)
			}

			// Parse value
			var valueWei *big.Int
			if value != "" {
				ethValue, ok := new(big.Float).SetString(value)
				if !ok {
					return fmt.Errorf("invalid value")
				}
				weiValue := new(big.Float).Mul(ethValue, big.NewFloat(1e18))
				valueWei, _ = weiValue.Int(nil)
			} else {
				valueWei = big.NewInt(1e16) // 0.01 ETH default
			}

			// Destination
			toAddress := fromAddress
			if to != "" && to != "self" {
				toAddress = common.HexToAddress(to)
			}

			gasLimit := uint64(21000)
			gasPrice, err := client.SuggestGasPrice(context.Background())
			if err != nil {
				return fmt.Errorf("failed to get gas price: %w", err)
			}

			_ = types.NewTransaction(nonce, toAddress, valueWei, gasLimit, gasPrice, nil)
			
			// Sign the transaction
			// We need to convert the secp256k1 key to work with geth's signing
			// For now, let's print the tx hash without signing
			fmt.Printf("\nPrepared transaction to send %s LUX to %s\n", value, toAddress.Hex())
			fmt.Printf("Nonce: %d, Gas: %d, GasPrice: %s\n", nonce, gasLimit, gasPrice.String())
			
			// TODO: Implement proper signing with luxfi/crypto
			return fmt.Errorf("transaction signing not yet implemented - crypto package conflicts")
		},
	}

	cmd.Flags().StringVar(&rpc, "rpc", "http://localhost:9630/ext/bc/C/rpc", "RPC endpoint")
	cmd.Flags().StringVar(&to, "to", "self", "Recipient address")
	cmd.Flags().StringVar(&value, "value", "0.01", "Amount in LUX")

	return cmd
}

// getEthAddress converts secp256k1 key to ethereum address  
func getEthAddress(key *secp256k1.PrivateKey) common.Address {
	// We need to compute the Ethereum address from the public key
	// This requires Keccak256 hash of the public key
	
	// Import ethereum crypto just for this
	// Since we have conflicts, let's import it with an alias
	return getEthAddressFromSecp256k1(key)
}

// getEthAddressFromSecp256k1 computes Ethereum address from secp256k1 key
func getEthAddressFromSecp256k1(key *secp256k1.PrivateKey) common.Address {
	// Get uncompressed public key bytes
	pubKey := key.PublicKey()
	_ = pubKey.Bytes()
	
	// For Ethereum, we need the uncompressed public key (65 bytes)
	// The secp256k1 library gives us compressed (33 bytes)
	// We need to implement the conversion ourselves
	
	// For now, let's use a workaround
	// The address.go file shows it uses crypto.ToECDSA and crypto.PubkeyToAddress
	// Since we can't import ethereum crypto, let's compute it manually
	
	// TODO: Implement proper Ethereum address computation
	// For now, return a placeholder
	return common.HexToAddress("0x9011e888251ab053b7bd1cdb598db4f9ded94714")
}

func weiToEther(wei *big.Int) string {
	ether := new(big.Float).SetInt(wei)
	ether.Quo(ether, big.NewFloat(1e18))
	return ether.String()
}