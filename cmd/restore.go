package cmd

import (
	"fmt"
	"os"

	"github.com/attamusc/spin/internal/config"
	"github.com/attamusc/spin/internal/db"
	"github.com/attamusc/spin/internal/resolve"
	"github.com/spf13/cobra"
)

var restoreCmd = &cobra.Command{
	Use:   "restore [profile] [name]",
	Short: "Restore a profile's database from a named snapshot",
	Long: `Restore a profile's SQLite database from a previously saved snapshot.

The snapshot at {data_root}/{owner}/{repo}/snapshots/{name}.db is copied
to the profile's database path, overwriting any existing data.

Will refuse to restore to the "live" profile if the process is running.`,
	Args: cobra.ExactArgs(2),
	RunE: runRestore,
}

func init() {
	rootCmd.AddCommand(restoreCmd)
}

func runRestore(cmd *cobra.Command, args []string) error {
	profile := args[0]
	name := args[1]

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

	// Check if live profile is running
	if db.IsProfileRunning(globalCfg.DataRoot, project.Owner, project.Repo, profile) {
		return fmt.Errorf("cannot restore to '%s' profile while it is running. Run 'spin down' first", profile)
	}

	// Check snapshot exists
	snapshotPath := db.SnapshotPath(globalCfg.DataRoot, project.Owner, project.Repo, name)
	if _, err := os.Stat(snapshotPath); os.IsNotExist(err) {
		return fmt.Errorf("snapshot '%s' not found at %s", name, snapshotPath)
	}

	// Get the DB path for the profile
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

	// Copy snapshot to profile's DB path
	if err := db.CopyDB(snapshotPath, dbPath); err != nil {
		return fmt.Errorf("failed to restore snapshot: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Restored snapshot '%s' to %s (%s profile)\n", name, project.Name, profile)

	return nil
}
