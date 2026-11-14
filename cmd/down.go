package cmd

import (
	"fmt"
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
	project, err := loadProject(args)
	if err != nil {
		return err
	}
	cyan := color.New(color.FgCyan).SprintFunc()
	stopped, err := project.stopLive(func(pid int) {
		fmt.Fprintf(cmd.ErrOrStderr(), "Stopping %s (PID %d)...\n", cyan(project.Name), pid)
	})
	if err != nil {
		return err
	}
	switch {
	case stopped.pid == 0:
		fmt.Fprintf(cmd.ErrOrStderr(), "%s is not running\n", cyan(project.Name))
	case stopped.stale:
		color.New(color.FgYellow).Fprintf(cmd.ErrOrStderr(), "%s is already stopped (cleaned up stale PID file)\n", cyan(project.Name))
	default:
		color.New(color.FgGreen).Fprintf(cmd.ErrOrStderr(), "Stopped %s\n", cyan(project.Name))
	}
	return nil
}
