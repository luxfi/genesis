package cmd

import (
	"encoding/hex"
	"fmt"

	"github.com/luxfi/genesis/pkg/application"
	"github.com/luxfi/genesis/pkg/db"
	"github.com/spf13/cobra"
)

// NewDBCmd creates the `db` command and its subcommands.
func NewDBCmd(app *application.Genesis) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:   "db",
		Short: "Provides tools for low-level database inspection",
		Long:  `The db command is the new home for all low-level database inspection, scanning, and analysis tools.`, // Corrected: Removed unnecessary backticks around the string literal.
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if dbPath == "" {
				return fmt.Errorf("the --db-path flag is required")
			}
			return nil
		},
	}

	cmd.PersistentFlags().StringVar(&dbPath, "db-path", "", "Path to the blockchain database")

	// Add subcommands
	cmd.AddCommand(newDBScanCmd(app, &dbPath))

	return cmd
}

// newDBScanCmd creates the `db scan` subcommand.
func newDBScanCmd(app *application.Genesis, dbPath *string) *cobra.Command {
	var (
		prefix string
		limit  int
	)

	cmd := &cobra.Command{
		Use:   "scan",
		Short: "Scans the database for keys and values",
		Long:  `Iterates over the database, optionally filtering by a key prefix, and prints the keys and values.`, // Corrected: Removed unnecessary backticks around the string literal.
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			database, err := openDatabase(*dbPath) // Using the helper from extract.go for now
			if err != nil {
				return err
			}
			defer database.Close()

			inspector := db.NewInspector(app, database)
			opts := db.ScanOptions{
				Prefix: []byte(prefix),
				Limit:  limit,
			}

			cmd.Printf("🔍 Scanning database at %s...\n", *dbPath)
			if prefix != "" {
				cmd.Printf("   Prefix: %s\n", prefix)
			}

			results, err := inspector.Scan(opts)
			if err != nil {
				return fmt.Errorf("scan failed: %w", err)
			}

			cmd.Printf("✅ Found %d results:\n", len(results))
			for _, res := range results {
				cmd.Printf("  - Key: 0x%s\n    Value: 0x%s\n", hex.EncodeToString(res.Key), hex.EncodeToString(res.Value))
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&prefix, "prefix", "", "Only scan for keys with this prefix")
	cmd.Flags().IntVar(&limit, "limit", 100, "Limit the number of results returned")

	return cmd
}
