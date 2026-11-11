package cmd

import (
	"fmt"
	"os"

	"github.com/attamusc/spin/internal/config"
	"github.com/attamusc/spin/internal/process"
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
	globalCfg, project, err := loadProject(args)
	if err != nil {
		return err
	}
	release, err := process.LockProfile(globalCfg.DataRoot, project.Owner, project.Repo, "live")
	if err != nil {
		return err
	}
	defer release()

	// Check if already running
	pidFile := process.PIDFilePath(globalCfg.DataRoot, project.Owner, project.Repo)
	if _, err := process.CleanStalePIDFile(pidFile); err != nil {
		return err
	}
	if pid, err := process.ReadPID(pidFile); err == nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "%s is already running (PID %d)\n", project.Name, pid)
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}

	// Port conflict detection
	checkPortConflicts(globalCfg, project)

	// Resolve DB path for live profile
	dbPath, err := config.ResolveProfileDBPath(
		globalCfg.DataRoot,
		project.Owner,
		project.Repo,
		"live",
		project.Config.DBFilename,
	)
	if err != nil {
		return fmt.Errorf("failed to resolve DB path: %w", err)
	}

	// Resolve environment variables
	env := config.ResolveEnv(project.Config.Live.Env, dbPath)

	// Construct log and PID file paths
	logFile, err := config.ProjectPath(globalCfg.DataRoot, project.Owner, project.Repo, "logs", "live.log")
	if err != nil {
		return err
	}

	// Start background process
	childPID, err := process.StartBackground(process.BackgroundOptions{
		Command: project.Config.Live.Command,
		Dir:     project.Dir,
		Env:     env,
		LogFile: logFile,
		PIDFile: pidFile,
		PreHook: project.Config.Live.PreHook,
	})
	if err != nil {
		return fmt.Errorf("failed to start live mode: %w", err)
	}

	cyan := color.New(color.FgCyan).SprintFunc()
	green := color.New(color.FgGreen)
	green.Fprintf(os.Stderr, "Started %s in live mode (PID %d)", cyan(project.Name), childPID)
	if project.Config.Live.Port > 0 {
		green.Fprintf(os.Stderr, " on port %d", project.Config.Live.Port)
	}
	fmt.Fprintln(os.Stderr)

	return nil
}

// checkPortConflicts warns if any running instance has the same port configured.
func checkPortConflicts(globalCfg *config.GlobalConfig, project *resolve.ResolvedProject) {
	if project.Config.Live.Port == 0 {
		return
	}

	// Scan for running instances
	result := newStatusObserver().collect(globalCfg)
	emitStatusDiagnostics(os.Stderr, result)
	if !result.Complete {
		fmt.Fprintln(os.Stderr, "Warning: port conflict observation incomplete (advisory only).")
	}
	for _, entry := range result.Instances {
		if entry.State != "live" || entry.Port == nil || entry.PID == nil {
			continue
		}
		if entry.Owner == project.Owner && entry.Repo == project.Repo {
			continue // Skip self
		}
		if *entry.Port == project.Config.Live.Port {
			color.New(color.FgYellow).Fprintf(os.Stderr, "Warning: port %d is also configured for %s/%s (PID %d)\n",
				project.Config.Live.Port, entry.Owner, entry.Repo, *entry.PID)
		}
	}
}
