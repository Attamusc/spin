package cmd

import (
	"fmt"
	"os"

	"github.com/attamusc/spin/internal/config"
	"github.com/attamusc/spin/internal/db"
	"github.com/attamusc/spin/internal/resolve"
	"github.com/spf13/cobra"
)

var cloneCmd = &cobra.Command{
	Use:   "clone [source-profile] [target-profile]",
	Short: "Copy one profile's database to another",
	Long: `Copy a profile's SQLite database to another profile.

For example, clone the live database to dev for debugging:
  spin clone live dev

A WAL checkpoint is performed on the source before copying.
Will refuse if the target is "live" and the process is running.`,
	Args: cobra.ExactArgs(2),
	RunE: runClone,
}

func init() {
	rootCmd.AddCommand(cloneCmd)
}

func runClone(cmd *cobra.Command, args []string) error {
	srcProfile := args[0]
	dstProfile := args[1]

	for _, p := range []string{srcProfile, dstProfile} {
		if p != "dev" && p != "live" {
			return fmt.Errorf("profile must be 'dev' or 'live', got %q", p)
		}
	}

	if srcProfile == dstProfile {
		return fmt.Errorf("source and target profiles must be different")
	}

	globalCfg, err := config.LoadGlobal()
	if err != nil {
		return fmt.Errorf("failed to load global config: %w", err)
	}

	project, err := resolve.FromCWD(globalCfg.ProjectsRoot)
	if err != nil {
		return fmt.Errorf("failed to resolve project: %w", err)
	}

	// Check if target profile is running
	if db.IsProfileRunning(globalCfg.DataRoot, project.Owner, project.Repo, dstProfile) {
		return fmt.Errorf("cannot clone to '%s' profile while it is running. Run 'spin down' first", dstProfile)
	}

	// Get source DB path
	srcDBPath, err := config.ResolveProfileDBPath(
		globalCfg.DataRoot,
		project.Owner,
		project.Repo,
		srcProfile,
		project.Config.DBFilename,
	)
	if err != nil {
		return fmt.Errorf("failed to resolve source DB path: %w", err)
	}

	// Check source DB exists
	if _, err := os.Stat(srcDBPath); os.IsNotExist(err) {
		return fmt.Errorf("no database found for '%s' profile at %s", srcProfile, srcDBPath)
	}

	// WAL checkpoint the source
	if err := db.WalCheckpoint(srcDBPath); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: WAL checkpoint failed: %v\n", err)
	}

	// Get destination DB path
	dstDBPath, err := config.ResolveProfileDBPath(
		globalCfg.DataRoot,
		project.Owner,
		project.Repo,
		dstProfile,
		project.Config.DBFilename,
	)
	if err != nil {
		return fmt.Errorf("failed to resolve destination DB path: %w", err)
	}

	// Copy source to destination
	if err := db.CopyDB(srcDBPath, dstDBPath); err != nil {
		return fmt.Errorf("failed to clone database: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Cloned %s → %s for %s\n", srcProfile, dstProfile, project.Name)

	return nil
}
