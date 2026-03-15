package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/attamusc/spin/internal/config"
	"github.com/attamusc/spin/internal/discovery"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "spin",
	Short: "Local project runner with data profile switching",
	Long: `spin runs local dev scripts in two modes:
  - dev mode: foregrounded, for active development
  - live mode: backgrounded, for a stable running instance

Each mode uses a separate SQLite database for data isolation.`,
	RunE: runRoot,
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}

func runRoot(cmd *cobra.Command, args []string) error {
	cyan := color.New(color.FgCyan).SprintFunc()
	green := color.New(color.FgGreen).SprintFunc()
	bold := color.New(color.Bold).SprintFunc()

	// Show running instances
	globalCfg, err := config.LoadGlobal()
	if err == nil {
		entries := scanRunningInstances(globalCfg)
		if len(entries) > 0 {
			fmt.Fprintf(os.Stderr, "%s\n\n", bold("Running instances:"))
			w := tabwriter.NewWriter(os.Stderr, 0, 0, 3, ' ', 0)
			fmt.Fprintln(w, "PROJECT\tMODE\tPORT\tPID\tUPTIME")
			for _, e := range entries {
				portStr := "-"
				if e.port > 0 {
					portStr = fmt.Sprintf("%d", e.port)
				}
				fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%s\n",
					cyan(fmt.Sprintf("%s/%s", e.owner, e.repo)), green("live"), portStr, e.pid, formatDuration(e.uptime))
			}
			w.Flush()
			fmt.Fprintln(os.Stderr)
		} else {
			fmt.Fprintln(os.Stderr, "No live instances running.")
			fmt.Fprintln(os.Stderr)
		}
	}

	// Show brief help
	fmt.Fprintf(os.Stderr, "%s\n\n", bold("Common commands:"))
	fmt.Fprintf(os.Stderr, "  %s           Navigate to a project directory\n", cyan("spin go"))
	fmt.Fprintf(os.Stderr, "  %s          Run a project in dev mode (foregrounded)\n", cyan("spin dev"))
	fmt.Fprintf(os.Stderr, "  %s           Start a project in live mode (backgrounded)\n", cyan("spin up"))
	fmt.Fprintf(os.Stderr, "  %s         Stop a live instance\n", cyan("spin down"))
	fmt.Fprintf(os.Stderr, "  %s       Show running instances\n", cyan("spin status"))
	fmt.Fprintf(os.Stderr, "  %s         Tail live instance logs\n", cyan("spin logs"))
	fmt.Fprintf(os.Stderr, "  %s         List all projects\n", cyan("spin list"))
	fmt.Fprintf(os.Stderr, "  %s        Clone a git repo\n", cyan("spin clone"))
	fmt.Fprintf(os.Stderr, "  %s          Create a new project\n", cyan("spin new"))
	fmt.Fprintf(os.Stderr, "  %s       Fetch latest for all repos\n", cyan("spin update"))
	fmt.Fprintf(os.Stderr, "  %s         Initialize a new .spin.toml\n", cyan("spin init"))
	fmt.Fprintf(os.Stderr, "  %s           Database operations (copy, snapshot, restore, reset)\n", cyan("spin db"))
	fmt.Fprintln(os.Stderr)
	fmt.Fprintf(os.Stderr, "Run %s for full usage.\n", cyan("spin help"))

	return nil
}

// completeProjectNames returns a cobra completion function that suggests
// discovered project names (owner/repo format) for tab completion.
// This uses ScanAll to include all projects, not just those with .spin.toml.
func completeProjectNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	// Only complete the first argument (project name)
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	globalCfg, err := config.LoadGlobal()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	projects, err := discovery.ScanAll(globalCfg.ProjectsRoot)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	var names []string
	for _, p := range projects {
		names = append(names, p.Name)
	}

	return names, cobra.ShellCompDirectiveNoFileComp
}
