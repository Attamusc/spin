package cmd

import (
	"fmt"
	"os"

	"github.com/attamusc/spin/internal/process"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var downCmd = &cobra.Command{
	Use:   "down [project]",
	Short: "Stop a backgrounded live instance",
	Long: `Stop a project's live mode background process.
Sends SIGTERM first, then SIGKILL after 5 seconds if needed.

If no project argument is given, the project is resolved from the current directory.`,
	Args:              cobra.MaximumNArgs(1),
	ValidArgsFunction: completeProjectNames,
	RunE:              runDown,
}

func init() {
	rootCmd.AddCommand(downCmd)
}

func runDown(cmd *cobra.Command, args []string) error {
	globalCfg, project, err := loadProject(args)
	if err != nil {
		return err
	}
	release, err := process.LockProfile(globalCfg.DataRoot, project.Owner, project.Repo, "live")
	if err != nil {
		return err
	}
	defer release()

	pidFile := process.PIDFilePath(globalCfg.DataRoot, project.Owner, project.Repo)

	cyan := color.New(color.FgCyan).SprintFunc()

	pid, err := process.ReadPID(pidFile)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "%s is not running\n", cyan(project.Name))
			return nil
		}
		return fmt.Errorf("could not read PID file: %w", err)
	}

	if stale, err := process.CleanStalePIDFile(pidFile); err != nil {
		return err
	} else if stale {
		color.New(color.FgYellow).Fprintf(os.Stderr, "%s is already stopped (cleaned up stale PID file)\n", cyan(project.Name))
		return nil
	}

	// Stop the process
	fmt.Fprintf(os.Stderr, "Stopping %s (PID %d)...\n", cyan(project.Name), pid)
	if err := process.StopBackground(pid); err != nil {
		return fmt.Errorf("failed to stop process: %w", err)
	}

	// Remove PID file
	if err := os.Remove(pidFile); err != nil && !os.IsNotExist(err) {
		return err
	}

	color.New(color.FgGreen).Fprintf(os.Stderr, "Stopped %s\n", cyan(project.Name))
	return nil
}
