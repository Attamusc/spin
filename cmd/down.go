package cmd

import (
	"fmt"
	"os"

	"github.com/attamusc/spin/internal/config"
	"github.com/attamusc/spin/internal/process"
	"github.com/attamusc/spin/internal/resolve"
	"github.com/spf13/cobra"
)

var downCmd = &cobra.Command{
	Use:   "down [project]",
	Short: "Stop a backgrounded live instance",
	Long: `Stop a project's live mode background process.
Sends SIGTERM first, then SIGKILL after 5 seconds if needed.

If no project argument is given, the project is resolved from the current directory.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runDown,
}

func init() {
	rootCmd.AddCommand(downCmd)
}

func runDown(cmd *cobra.Command, args []string) error {
	globalCfg, err := config.LoadGlobal()
	if err != nil {
		return fmt.Errorf("failed to load global config: %w", err)
	}

	// Resolve project (from argument or CWD)
	var arg string
	if len(args) > 0 {
		arg = args[0]
	}
	project, err := resolve.Resolve(arg, globalCfg.ProjectsRoot)
	if err != nil {
		return fmt.Errorf("failed to resolve project: %w", err)
	}

	pidFile := process.PIDFilePath(globalCfg.DataRoot, project.Owner, project.Repo)

	pid, err := process.ReadPID(pidFile)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "%s is not running\n", project.Name)
			return nil
		}
		return fmt.Errorf("could not read PID file: %w", err)
	}

	if !process.IsProcessAlive(pid) {
		// Clean up stale PID file
		_ = os.Remove(pidFile)
		fmt.Fprintf(os.Stderr, "%s is already stopped (cleaned up stale PID file)\n", project.Name)
		return nil
	}

	// Stop the process
	fmt.Fprintf(os.Stderr, "Stopping %s (PID %d)...\n", project.Name, pid)
	if err := process.StopBackground(pid); err != nil {
		return fmt.Errorf("failed to stop process: %w", err)
	}

	// Remove PID file
	_ = os.Remove(pidFile)

	fmt.Fprintf(os.Stderr, "Stopped %s\n", project.Name)
	return nil
}
