package cmd

import (
	"fmt"
	"os"

	"github.com/attamusc/spin/internal/config"
	"github.com/attamusc/spin/internal/process"
	"github.com/attamusc/spin/internal/resolve"
	"github.com/spf13/cobra"
)

var devCmd = &cobra.Command{
	Use:   "dev [project]",
	Short: "Run a project in dev mode (foregrounded)",
	Long: `Run a project in dev mode with the foregrounded process.
The dev profile uses a separate database for data isolation.

If no project argument is given, the project is resolved from the current directory.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runDev,
}

func init() {
	rootCmd.AddCommand(devCmd)
}

func runDev(cmd *cobra.Command, args []string) error {
	// Load global config
	globalCfg, err := config.LoadGlobal()
	if err != nil {
		return fmt.Errorf("failed to load global config: %w", err)
	}

	// Resolve project
	var project *resolve.ResolvedProject
	if len(args) > 0 {
		// Full name resolution will be added in Phase 3
		// For now, only CWD-based resolution is supported
		return fmt.Errorf("project argument not yet supported, run from within a project directory")
	}

	project, err = resolve.FromCWD(globalCfg.ProjectsRoot)
	if err != nil {
		return fmt.Errorf("failed to resolve project: %w", err)
	}

	// Resolve DB path for dev profile
	dbPath, err := config.ResolveProfileDBPath(
		globalCfg.DataRoot,
		project.Owner,
		project.Repo,
		"dev",
		project.Config.DBFilename,
	)
	if err != nil {
		return fmt.Errorf("failed to resolve DB path: %w", err)
	}

	// Resolve environment variables
	env := config.ResolveEnv(project.Config.Dev.Env, dbPath)

	fmt.Fprintf(os.Stderr, "Starting dev mode for %s...\n", project.Name)

	// Run the foreground process
	exitCode, err := process.RunForeground(process.ForegroundOptions{
		Command: project.Config.Dev.Command,
		Dir:     project.Dir,
		Env:     env,
		PreHook: project.Config.Dev.PreHook,
	})
	if err != nil {
		return err
	}
	if exitCode != 0 {
		os.Exit(exitCode)
	}

	return nil
}
