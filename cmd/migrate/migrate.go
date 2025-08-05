package migrate

import (
	"github.com/spf13/cobra"
)

// Cmd is the migrate command
var Cmd = &cobra.Command{
	Use:   "migrate",
	Short: "Database migration tools",
	Long:  `Tools for migrating data between different database backends.`,
}