package cmd

import (
	"fmt"
	"os"

	"github.com/attamusc/spin/internal/config"
	"github.com/attamusc/spin/internal/db"
	"github.com/attamusc/spin/internal/resolve"
	"github.com/spf13/cobra"
)

var snapshotCmd = &cobra.Command{
	Use:   "snapshot [profile] [name]",
	Short: "Create a named snapshot of a profile's database",
	Long: `Create a named snapshot of a profile's SQLite database.

The snapshot is saved to {data_root}/{owner}/{repo}/snapshots/{name}.db.
A WAL checkpoint is performed before copying.

Profile must be "dev" or "live".`,
	Args: cobra.ExactArgs(2),
	RunE: runSnapshot,
}

func init() {
	rootCmd.AddCommand(snapshotCmd)
}

func runSnapshot(cmd *cobra.Command, args []string) error {
	profile := args[0]
	name := args[1]

	if profile != "dev" && profile != "live" {
		return fmt.Errorf("profile must be 'dev' or 'live', got %q", profile)
	}

	globalCfg, err := config.LoadGlobal()
	if err != nil {
		return fmt.Errorf("failed to load global config: %w", err)
	}

	// Resolve project from CWD (snapshot doesn't take a project arg in positional args)
	project, err := resolve.FromCWD(globalCfg.ProjectsRoot)
	if err != nil {
		return fmt.Errorf("failed to resolve project: %w", err)
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

	// Check DB file exists
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return fmt.Errorf("no database found at %s. Run the project first to create it", dbPath)
	}

	// Run WAL checkpoint
	if err := db.WalCheckpoint(dbPath); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: WAL checkpoint failed: %v\n", err)
	}

	// Copy to snapshot location
	snapshotPath := db.SnapshotPath(globalCfg.DataRoot, project.Owner, project.Repo, name)
	if err := db.CopyDB(dbPath, snapshotPath); err != nil {
		return fmt.Errorf("failed to create snapshot: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Snapshot '%s' created for %s (%s profile)\n", name, project.Name, profile)
	fmt.Fprintf(os.Stderr, "  %s\n", snapshotPath)

	return nil
}
