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
	cmd.AddCommand(newDBInspectCmd(app, &dbPath))
	cmd.AddCommand(newDBFindTipCmd(app, &dbPath))
	cmd.AddCommand(newDBStatsCmd(app, &dbPath))
	// Future: db repair, db compact, db verify

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

// newDBInspectCmd creates the `db inspect` subcommand.
func newDBInspectCmd(app *application.Genesis, dbPath *string) *cobra.Command {
	var key string

	cmd := &cobra.Command{
		Use:   "inspect",
		Short: "Inspect specific database entries",
		Long:  `Decode and display the contents of specific database keys with proper formatting.`,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			database, err := openDatabase(*dbPath)
			if err != nil {
				return err
			}
			defer database.Close()

			inspector := db.NewInspector(app, database)

			cmd.Printf("🔍 Inspecting key in database at %s...\n", *dbPath)

			keyBytes, err := hex.DecodeString(key)
			if err != nil {
				return fmt.Errorf("invalid hex key: %w", err)
			}

			result, err := inspector.InspectKey(keyBytes)
			if err != nil {
				return fmt.Errorf("inspection failed: %w", err)
			}

			cmd.Printf("✅ Key inspection result:\n")
			cmd.Printf("  Type: %s\n", result.Type)
			cmd.Printf("  Decoded: %s\n", result.Decoded)
			if result.Details != "" {
				cmd.Printf("  Details: %s\n", result.Details)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&key, "key", "", "Hex-encoded key to inspect")
	cmd.MarkFlagRequired("key")

	return cmd
}

// newDBFindTipCmd creates the `db find-tip` subcommand.
func newDBFindTipCmd(app *application.Genesis, dbPath *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "find-tip",
		Short: "Find the tip (latest block) in the database",
		Long:  `Scans the database to find the highest block number and its associated data.`,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			database, err := openDatabase(*dbPath)
			if err != nil {
				return err
			}
			defer database.Close()

			inspector := db.NewInspector(app, database)

			cmd.Printf("🔍 Finding tip block in database at %s...\n", *dbPath)

			tip, err := inspector.FindTip()
			if err != nil {
				return fmt.Errorf("find tip failed: %w", err)
			}

			cmd.Printf("✅ Found tip block:\n")
			cmd.Printf("  Height: %d\n", tip.Height)
			cmd.Printf("  Hash: 0x%x\n", tip.Hash)
			cmd.Printf("  Parent: 0x%x\n", tip.ParentHash)
			cmd.Printf("  Timestamp: %d\n", tip.Timestamp)

			return nil
		},
	}

	return cmd
}

// newDBStatsCmd creates the `db stats` subcommand.
func newDBStatsCmd(app *application.Genesis, dbPath *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stats",
		Short: "Display database statistics",
		Long:  `Shows comprehensive statistics about the database including size, key count, and type distribution.`,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			database, err := openDatabase(*dbPath)
			if err != nil {
				return err
			}
			defer database.Close()

			inspector := db.NewInspector(app, database)

			cmd.Printf("📊 Gathering statistics for database at %s...\n", *dbPath)

			stats, err := inspector.GetStats()
			if err != nil {
				return fmt.Errorf("stats gathering failed: %w", err)
			}

			cmd.Printf("✅ Database Statistics:\n")
			cmd.Printf("  Total Keys: %d\n", stats.TotalKeys)
			cmd.Printf("  Total Size: %s\n", formatBytes(stats.TotalSize))
			cmd.Printf("\n  Key Type Distribution:\n")
			for keyType, count := range stats.KeyTypes {
				cmd.Printf("    %s: %d\n", keyType, count)
			}
			if stats.LatestBlock > 0 {
				cmd.Printf("\n  Latest Block: %d\n", stats.LatestBlock)
			}

			return nil
		},
	}

	return cmd
}

// formatBytes formats bytes into human-readable format
func formatBytes(bytes uint64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := uint64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
