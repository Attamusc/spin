package cmd

import (
	"fmt"

	"github.com/attamusc/spin/internal/config"
	"github.com/attamusc/spin/internal/process"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var devCmd = &cobra.Command{
	Use:   "dev [project]",
	Short: "Run a project in dev mode (foregrounded)",
	Long: `Run a project in dev mode with the foregrounded process.
The dev profile uses a separate database for data isolation.

If no project argument is given, the project is resolved from the current directory.`,
	Args:              cobra.MaximumNArgs(1),
	ValidArgsFunction: completeProjectNames,
	RunE:              runDev,
}

func init() {
	rootCmd.AddCommand(devCmd)
}

func runDev(cmd *cobra.Command, args []string) error {
	globalCfg, project, err := loadProject(args)
	if err != nil {
		return err
	}
	release, err := process.LockProfile(globalCfg.DataRoot, project.Owner, project.Repo, "dev")
	if err != nil {
		return err
	}
	defer release()

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

	cyan := color.New(color.FgCyan).SprintFunc()
	color.New(color.FgGreen).Fprintf(cmd.ErrOrStderr(), "Starting dev mode for %s...\n", cyan(project.Name))

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
		cmd.SilenceErrors, cmd.SilenceUsage = true, true
		return exitError(exitCode)
	}

	return nil
}
