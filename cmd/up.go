package cmd

import (
	"fmt"
	"os"

	"github.com/attamusc/spin/internal/config"
	"github.com/attamusc/spin/internal/process"
	"github.com/attamusc/spin/internal/resolve"
	"github.com/spf13/cobra"
)

var upCmd = &cobra.Command{
	Use:   "up [project]",
	Short: "Run a project in live mode (backgrounded)",
	Long: `Start a project in live mode as a background process.
The live profile uses a separate database for data isolation.
stdout/stderr are redirected to a log file.

If no project argument is given, the project is resolved from the current directory.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runUp,
}

func init() {
	rootCmd.AddCommand(upCmd)
}

func runUp(cmd *cobra.Command, args []string) error {
	// Load global config
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

	// Check if already running
	pidFile := process.PIDFilePath(globalCfg.DataRoot, project.Owner, project.Repo)
	pid, err := process.ReadPID(pidFile)
	if err == nil && process.IsProcessAlive(pid) {
		fmt.Fprintf(os.Stderr, "%s is already running (PID %d)\n", project.Name, pid)
		return nil
	}
	// Clean up stale PID file if present
	if err == nil {
		process.CleanStalePIDFile(pidFile)
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
	logFile := process.LogFilePath(globalCfg.DataRoot, project.Owner, project.Repo)

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

	fmt.Fprintf(os.Stderr, "Started %s in live mode (PID %d)", project.Name, childPID)
	if project.Config.Live.Port > 0 {
		fmt.Fprintf(os.Stderr, " on port %d", project.Config.Live.Port)
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
	entries := scanRunningInstances(globalCfg)
	for _, entry := range entries {
		if entry.owner == project.Owner && entry.repo == project.Repo {
			continue // Skip self
		}
		if entry.port == project.Config.Live.Port {
			fmt.Fprintf(os.Stderr, "Warning: port %d is also configured for %s/%s (PID %d)\n",
				project.Config.Live.Port, entry.owner, entry.repo, entry.pid)
		}
	}
}
