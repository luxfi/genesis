
package cmd

import (
	"fmt"
	"math/big"

	"github.com/luxfi/genesis/pkg/application"
	"github.com/luxfi/genesis/pkg/balance"
	"github.com/luxfi/geth/common"
	"github.com/spf13/cobra"
)

// NewCheckCmd creates the `check` command for verification tools.
func NewCheckCmd(app *application.Genesis) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "check",
		Short: "Run checks and verification on blockchain data",
		Long:  `The check command provides tools to verify the integrity, state, and history of the blockchain data.`,
	}

	// Future checks like `check migration`, `check db`, etc. can be added here.

	return cmd
}

// NewBalanceCmd creates the `balance` command as a top-level command.
func NewBalanceCmd(app *application.Genesis) *cobra.Command {
	var ( // Using a closure to hold the flag variables
		dbPath  string
		address string
	)

	cmd := &cobra.Command{
		Use:   "balance",
		Short: "Checks the LUX balance of an address directly from the database",
		Long: `This command opens the specified blockchain database (BadgerDB) and directly reads the state trie to find and print the balance of a given address.

It serves as a replacement for the numerous older, standalone balance checking scripts.`, 
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dbPath == "" {
				return fmt.Errorf("the --db-path flag is required")
			}
			if address == "" {
				return fmt.Errorf("the --address flag is required")
			}

			addr := common.HexToAddress(address)

			cfg := balance.Config{DBPath: dbPath}
			checker, err := balance.NewChecker(cfg)
			if err != nil {
				return fmt.Errorf("failed to initialize balance checker: %w", err)
			}
			defer checker.Close()

			cmd.Printf("🔍 Checking balance for %s\n", addr.Hex())
			cmd.Printf("   Database: %s\n\n", dbPath)

			// Get chain status
			status, err := checker.GetChainStatus()
			if err != nil {
				cmd.Printf("⚠️ Could not get chain status: %v\n", err)
			} else {
				cmd.Printf("✅ Chain Status: %s\n", status)
			}

			// Get account balance
			info, err := checker.GetBalance(addr)
			if err != nil {
				return fmt.Errorf("error checking balance: %w", err)
			}

			// Convert balance to LUX (divide by 1e18)
			balanceLUX := new(big.Float).Quo(new(big.Float).SetInt(info.Balance), big.NewFloat(1e18))

			cmd.Printf("✅ Account Found:\n")
			cmd.Printf("   Balance: %s wei\n", info.Balance.String())
			cmd.Printf("   Balance: %.18f LUX\n", balanceLUX)
			cmd.Printf("   Nonce:   %d\n", info.Nonce)

			return nil
		},
	}

	// Add flags
	cmd.Flags().StringVar(&dbPath, "db-path", "", "Absolute path to the chaindata database (e.g., ~/.luxd/network-96369/chains/X.../ethdb)")
	cmd.Flags().StringVar(&address, "address", "", "The hex-encoded address to check")

	return cmd
}
