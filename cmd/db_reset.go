package cmd

import (
	"fmt"
	"os"

	"github.com/attamusc/spin/internal/config"
	"github.com/attamusc/spin/internal/db"
	"github.com/attamusc/spin/internal/resolve"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var dbResetCmd = &cobra.Command{
	Use:   "reset [profile]",
	Short: "Delete a profile's database",
	Long: `Delete a profile's SQLite database file (and WAL/SHM files if present).

The application will recreate the database on the next start.
Will refuse if the profile is "live" and the process is running.`,
	Args: cobra.ExactArgs(1),
	RunE: runDBReset,
}

func init() {
	dbCmd.AddCommand(dbResetCmd)
}

func runDBReset(cmd *cobra.Command, args []string) error {
	profile := args[0]

	if profile != "dev" && profile != "live" {
		return fmt.Errorf("profile must be 'dev' or 'live', got %q", profile)
	}

	globalCfg, err := config.LoadGlobal()
	if err != nil {
		return fmt.Errorf("failed to load global config: %w", err)
	}

	project, err := resolve.FromCWD(globalCfg.ProjectsRoot)
	if err != nil {
		return fmt.Errorf("failed to resolve project: %w", err)
	}

	// Check if profile is running
	if db.IsProfileRunning(globalCfg.DataRoot, project.Owner, project.Repo, profile) {
		return fmt.Errorf("cannot reset '%s' profile while it is running. Run 'spin down' first", profile)
	}

	// Get DB path
	dbPath, err := config.ResolveProfileDBPath(
		globalCfg.DataRoot,
		project.Owner,
		project.Repo,
		profile,
		project.Config.DBFilename,
	)
	if err != nil {
		return fmt.Errorf("failed to resolve DB path: %w", err)
	}

	// Check DB exists
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "No database found for %s (%s profile)\n", project.Name, profile)
		return nil
	}

	// Delete DB and sidecar files
	if err := db.RemoveDB(dbPath); err != nil {
		return fmt.Errorf("failed to reset database: %w", err)
	}

	cyan := color.New(color.FgCyan).SprintFunc()
	color.New(color.FgGreen).Fprintf(os.Stderr, "Reset %s database for %s\n", profile, cyan(project.Name))
	fmt.Fprintf(os.Stderr, "The database will be recreated on next start.\n")

	return nil
}
