package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"
	"time"

	"github.com/attamusc/spin/internal/config"
	"github.com/attamusc/spin/internal/process"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show all running live instances",
	RunE:  runStatus,
}

func init() {
	rootCmd.AddCommand(statusCmd)
}

type runningInstance struct {
	owner  string
	repo   string
	pid    int
	port   int
	uptime time.Duration
}

func runStatus(cmd *cobra.Command, args []string) error {
	globalCfg, err := config.LoadGlobal()
	if err != nil {
		return fmt.Errorf("failed to load global config: %w", err)
	}

	entries := scanRunningInstances(globalCfg)

	if len(entries) == 0 {
		fmt.Println("No live instances running.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "PROJECT\tMODE\tPORT\tPID\tUPTIME")

	for _, e := range entries {
		portStr := "-"
		if e.port > 0 {
			portStr = fmt.Sprintf("%d", e.port)
		}
		fmt.Fprintf(w, "%s/%s\t%s\t%s\t%d\t%s\n",
			e.owner, e.repo, "live", portStr, e.pid, formatDuration(e.uptime))
	}

	w.Flush()
	return nil
}

// scanRunningInstances scans the data root for live.pid files and returns
// information about running instances.
func scanRunningInstances(globalCfg *config.GlobalConfig) []runningInstance {
	var entries []runningInstance

	// Walk {data_root}/*/*/live.pid
	dataRoot := globalCfg.DataRoot
	owners, err := os.ReadDir(dataRoot)
	if err != nil {
		return entries
	}

	for _, ownerEntry := range owners {
		if !ownerEntry.IsDir() {
			continue
		}
		ownerPath := filepath.Join(dataRoot, ownerEntry.Name())
		repos, err := os.ReadDir(ownerPath)
		if err != nil {
			continue
		}

		for _, repoEntry := range repos {
			if !repoEntry.IsDir() {
				continue
			}
			pidPath := filepath.Join(ownerPath, repoEntry.Name(), "live.pid")

			pid, err := process.ReadPID(pidPath)
			if err != nil {
				continue
			}

			if !process.IsProcessAlive(pid) {
				// Clean stale PID file
				_ = os.Remove(pidPath)
				continue
			}

			// Calculate uptime from PID file mtime
			info, err := os.Stat(pidPath)
			var uptime time.Duration
			if err == nil {
				uptime = time.Since(info.ModTime())
			}

			// Try to load project config for port
			port := 0
			projectDir := findProjectDir(globalCfg.ProjectsRoot, ownerEntry.Name(), repoEntry.Name())
			if projectDir != "" {
				cfg, err := config.LoadProject(projectDir)
				if err == nil {
					port = cfg.Live.Port
				}
			}

			entries = append(entries, runningInstance{
				owner:  ownerEntry.Name(),
				repo:   repoEntry.Name(),
				pid:    pid,
				port:   port,
				uptime: uptime,
			})
		}
	}

	return entries
}

// findProjectDir tries to find the project directory for a given owner/repo
// by scanning {projects_root}/*/owner/repo for .spin.toml
func findProjectDir(projectsRoot, owner, repo string) string {
	hosts, err := os.ReadDir(projectsRoot)
	if err != nil {
		return ""
	}

	for _, host := range hosts {
		if !host.IsDir() || host.Name()[0] == '.' {
			continue
		}
		dir := filepath.Join(projectsRoot, host.Name(), owner, repo)
		if _, err := os.Stat(filepath.Join(dir, ".spin.toml")); err == nil {
			return dir
		}
	}
	return ""
}

// formatDuration formats a duration as a human-readable string (e.g., "2h13m")
func formatDuration(d time.Duration) string {
	d = d.Round(time.Minute)

	if d < time.Minute {
		return "<1m"
	}

	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60

	if hours > 0 {
		return fmt.Sprintf("%dh%dm", hours, minutes)
	}
	return fmt.Sprintf("%dm", minutes)
}
