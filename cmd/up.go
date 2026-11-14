package cmd

import (
	"fmt"
	"io"

	"github.com/attamusc/spin/internal/config"
	"github.com/attamusc/spin/internal/resolve"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var upCmd = &cobra.Command{
	Use:   "up [project]",
	Short: "Run a project in live mode (backgrounded)",
	Long: `Start a project in live mode as a background process.
The live profile uses a separate database for data isolation.
stdout/stderr are redirected to a log file.

If no project argument is given, the project is resolved from the current directory.`,
	Args:              cobra.MaximumNArgs(1),
	ValidArgsFunction: completeProjectNames,
	RunE:              runUp,
}

func init() {
	rootCmd.AddCommand(upCmd)
}

func runUp(cmd *cobra.Command, args []string) error {
	project, err := loadProject(args)
	if err != nil {
		return err
	}
	started, err := project.startLive(func() {
		checkPortConflicts(cmd.ErrOrStderr(), project.global, project.ResolvedProject)
	})
	if err != nil {
		return err
	}
	if started.alreadyRunning {
		fmt.Fprintf(cmd.ErrOrStderr(), "%s is already running (PID %d)\n", project.Name, started.pid)
		return nil
	}

	cyan := color.New(color.FgCyan).SprintFunc()
	green := color.New(color.FgGreen)
	green.Fprintf(cmd.ErrOrStderr(), "Started %s in live mode (PID %d)", cyan(project.Name), started.pid)
	if project.Config.Live.Port > 0 {
		green.Fprintf(cmd.ErrOrStderr(), " on port %d", project.Config.Live.Port)
	}
	fmt.Fprintln(cmd.ErrOrStderr())

	return nil
}

// checkPortConflicts warns if any running instance has the same port configured.
func checkPortConflicts(out io.Writer, globalCfg *config.GlobalConfig, project *resolve.ResolvedProject) {
	if project.Config.Live.Port == 0 {
		return
	}

	// Scan for running instances
	result := newStatusObserver().collect(globalCfg)
	emitStatusDiagnostics(out, result)
	if !result.Complete {
		fmt.Fprintln(out, "Warning: port conflict observation incomplete (advisory only).")
	}
	for _, entry := range result.Instances {
		if entry.State != "live" || entry.Port == nil || entry.PID == nil {
			continue
		}
		if entry.Owner == project.Owner && entry.Repo == project.Repo {
			continue // Skip self
		}
		if *entry.Port == project.Config.Live.Port {
			color.New(color.FgYellow).Fprintf(out, "Warning: port %d is also configured for %s/%s (PID %d)\n",
				project.Config.Live.Port, entry.Owner, entry.Repo, *entry.PID)
		}
	}
}
