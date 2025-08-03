package cmd

import (
	"strconv"

	"github.com/luxfi/genesis/pkg/application"
	"github.com/luxfi/genesis/pkg/database"
	"github.com/spf13/cobra"
)

// NewDatabaseCmd creates the database command with subcommands
func NewDatabaseCmd(app *application.Genesis) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "database",
		Short: "Database operations",
		Long:  "Commands for managing and inspecting blockchain databases",
	}

	cmd.AddCommand(newDatabaseWriteHeightCmd(app))
	cmd.AddCommand(newDatabaseGetCanonicalCmd(app))
	cmd.AddCommand(newDatabaseStatusCmd(app))
	cmd.AddCommand(newDatabasePrepareMigrationCmd(app))
	cmd.AddCommand(newDatabaseCompactCmd(app))

	return cmd
}

func newDatabaseWriteHeightCmd(app *application.Genesis) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "write-height [db-path] [height]",
		Short: "Write a Height key to the database",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			height, err := strconv.ParseUint(args[1], 10, 64)
			if err != nil {
				return err
			}
			
			dbMgr := database.New(app)
			return dbMgr.WriteHeight(args[0], height)
		},
	}
	return cmd
}

func newDatabaseGetCanonicalCmd(app *application.Genesis) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get-canonical [db-path] [height]",
		Short: "Get the canonical block hash at a specific height",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			height, err := strconv.ParseUint(args[1], 10, 64)
			if err != nil {
				return err
			}
			
			dbMgr := database.New(app)
			hash, err := dbMgr.GetCanonicalHash(args[0], height)
			if err != nil {
				return err
			}
			
			app.Log.Info("Canonical hash", "height", height, "hash", hash)
			return nil
		},
	}
	return cmd
}

func newDatabaseStatusCmd(app *application.Genesis) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status [db-path]",
		Short: "Check database status and statistics",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dbMgr := database.New(app)
			return dbMgr.CheckStatus(args[0])
		},
	}
	return cmd
}

func newDatabasePrepareMigrationCmd(app *application.Genesis) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "prepare-migration [db-path] [height]",
		Short: "Prepare database for LUX_GENESIS migration",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			height, err := strconv.ParseUint(args[1], 10, 64)
			if err != nil {
				return err
			}
			
			dbMgr := database.New(app)
			return dbMgr.PrepareMigration(args[0], height)
		},
	}
	return cmd
}

func newDatabaseCompactCmd(app *application.Genesis) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "compact-ancient [db-path] [block-num]",
		Short: "Compact ancient data in the database",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			blockNum, err := strconv.ParseUint(args[1], 10, 64)
			if err != nil {
				return err
			}
			
			dbMgr := database.New(app)
			return dbMgr.CompactAncient(args[0], blockNum)
		},
	}
	return cmd
}