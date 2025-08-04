package cmd

import (
	"context"
	"fmt"
	"math/big"
	"os/exec"
	"strings"

	"github.com/luxfi/genesis/pkg/application"
	"github.com/luxfi/geth/common"
	"github.com/luxfi/geth/core/types"
	"github.com/luxfi/geth/ethclient"
	"github.com/spf13/cobra"
)

// NewTxCmd creates the transaction command
func NewTxCmd(app *application.Genesis) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tx",
		Short: "Transaction management commands",
	}

	cmd.AddCommand(newSendCmd(app))
	cmd.AddCommand(newStatusCmd(app))

	return cmd
}

func newSendCmd(app *application.Genesis) *cobra.Command {
	var (
		rpc   string
		to    string
		value string
	)

	cmd := &cobra.Command{
		Use:   "send",
		Short: "Send transaction from address derived from .env",
		RunE: func(cmd *cobra.Command, args []string) error {
			// The address we know is funded from genesis
			fromAddress := common.HexToAddress("0x9011e888251ab053b7bd1cdb598db4f9ded94714")
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
			fmt.Printf("Balance: %s ETH\n", weiToEther(balance))

			// Get chain ID
			chainID, err := client.NetworkID(context.Background())
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
			valueWei := new(big.Int)
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

			// For now, let's check if personal API is enabled
			// We'll sign the transaction using personal_sendTransaction
			fmt.Printf("\nTransaction prepared. To send it, we need to unlock the account.\n")
			fmt.Printf("\nTrying to send via personal API...\n")

			// Create personal_sendTransaction request
			curlCmd := exec.Command("curl", "-s", "-X", "POST", "-H", "Content-Type: application/json",
				"-d", fmt.Sprintf(`{"jsonrpc":"2.0","id":1,"method":"personal_sendTransaction","params":[{"from":"%s","to":"%s","value":"%s","gas":"0x5208","gasPrice":"%s"},"password"]}`,
					fromAddress.Hex(), toAddress.Hex(), fmt.Sprintf("0x%x", valueWei), fmt.Sprintf("0x%x", gasPrice)),
				rpc)

			output, err := curlCmd.CombinedOutput()
			if err != nil {
				return fmt.Errorf("failed to send transaction: %w\nOutput: %s", err, output)
			}

			fmt.Printf("Response: %s\n", output)

			// Check if it contains an error
			if strings.Contains(string(output), "error") {
				fmt.Printf("\nNote: Personal API might not be enabled or account not unlocked.\n")
				fmt.Printf("To enable it, start luxd with --api-personal-enabled flag\n")
				return fmt.Errorf("transaction failed")
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&rpc, "rpc", "http://localhost:9630/ext/bc/C/rpc", "RPC endpoint")
	cmd.Flags().StringVar(&to, "to", "self", "Recipient address")
	cmd.Flags().StringVar(&value, "value", "0.01", "Amount in ETH")

	return cmd
}

func newStatusCmd(app *application.Genesis) *cobra.Command {
	var rpc string

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Check chain status",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := ethclient.Dial(rpc)
			if err != nil {
				return fmt.Errorf("failed to connect: %w", err)
			}

			// Get latest block
			blockNum, err := client.BlockNumber(context.Background())
			if err != nil {
				return fmt.Errorf("failed to get block number: %w", err)
			}
			fmt.Printf("Current block: %d\n", blockNum)

			// Get chain ID
			chainID, err := client.NetworkID(context.Background())
			if err != nil {
				return fmt.Errorf("failed to get chain ID: %w", err)
			}
			fmt.Printf("Chain ID: %s\n", chainID.String())

			// Get latest block details
			block, err := client.BlockByNumber(context.Background(), nil)
			if err != nil {
				return fmt.Errorf("failed to get latest block: %w", err)
			}

			fmt.Printf("Block hash: %s\n", block.Hash().Hex())
			fmt.Printf("Block time: %d\n", block.Time())
			fmt.Printf("Transactions: %d\n", len(block.Transactions()))

			return nil
		},
	}

	cmd.Flags().StringVar(&rpc, "rpc", "http://localhost:9630/ext/bc/C/rpc", "RPC endpoint")

	return cmd
}


func weiToEther(wei *big.Int) string {
	ether := new(big.Float).SetInt(wei)
	ether.Quo(ether, big.NewFloat(1e18))
	return ether.String()
}
