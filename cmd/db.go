package cmd

import (
	"github.com/spf13/cobra"
)

var dbCmd = &cobra.Command{
	Use:   "db",
	Short: "Database operations",
	Long: `Database operations for spin project profiles.

Subcommands:
  copy       Copy one profile's database to another
  snapshot   Create a named snapshot of a profile's database
  restore    Restore a profile's database from a snapshot
  reset      Delete a profile's database`,
}

func init() {
	rootCmd.AddCommand(dbCmd)
}
