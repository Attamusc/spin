package cmd

import (
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "spin",
	Short: "Local project runner with data profile switching",
	Long: `spin runs local dev scripts in two modes:
  - dev mode: foregrounded, for active development
  - live mode: backgrounded, for a stable running instance

Each mode uses a separate SQLite database for data isolation.`,
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}
