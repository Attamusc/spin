package cmd

import (
	"fmt"
	"os"
	"strings"

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
func Execute() int {
	err := rootCmd.Execute()
	if code, ok := err.(exitError); ok {
		return int(code)
	}
	if err != nil {
		return 1
	}
	return 0
}

func runRoot(cmd *cobra.Command, args []string) error {
	cyan := color.New(color.FgCyan).SprintFunc()
	bold := color.New(color.Bold).SprintFunc()

	// Dashboard and status share the same non-mutating observation.
	result := observeStatus()
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	emitStatusDiagnostics(os.Stderr, result)
	fmt.Fprintf(os.Stderr, "%s\n\n", bold("Running instances:"))
	renderErr := renderStatus(os.Stderr, result, true)
	if !result.Complete {
		fmt.Fprintln(os.Stderr, "Status observation incomplete.")
	}
	fmt.Fprintln(os.Stderr)

	fmt.Fprintf(os.Stderr, "%s\n\n", bold("Common commands:"))
	for _, name := range []string{"go", "dev", "up", "down", "status", "logs", "list", "clone", "new", "update", "init", "db"} {
		command, _, _ := cmd.Find([]string{name})
		fmt.Fprintf(os.Stderr, "  %-23s %s\n", cyan("spin "+name), command.Short)
	}
	fmt.Fprintln(os.Stderr)
	fmt.Fprintf(os.Stderr, "Run %s for full usage.\n", cyan("spin help"))

	if renderErr != nil {
		return renderErr
	}
	if !result.Complete {
		return errStatusIncomplete
	}
	return nil
}

// Project completion includes unconfigured directories, as navigation does.
func completeProjectNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	// Only complete the first argument (project name)
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	globalCfg, err := config.LoadGlobal()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	// Completion can suggest accessible projects without claiming a complete
	// inventory; actual resolution still fails closed if uniqueness is unknown.
	projects, _ := discovery.ScanAll(globalCfg.ProjectsRoot)
	names := fuzzyFilterProjects(projects, toComplete)
	return names, cobra.ShellCompDirectiveNoFileComp
}

// fuzzyFilterProjects filters a list of discovered projects using segment-wise
// substring matching against toComplete. If toComplete is empty, returns all names.
func fuzzyFilterProjects(projects []discovery.DiscoveredProject, toComplete string) []string {
	if toComplete == "" {
		var names []string
		for _, p := range projects {
			names = append(names, p.Name)
		}
		return names
	}

	lower := strings.ToLower(toComplete)

	if strings.Contains(toComplete, "/") {
		// owner/repo pattern — match each segment
		parts := strings.SplitN(lower, "/", 2)
		ownerPattern := parts[0]
		repoPattern := parts[1]

		var names []string
		for _, p := range projects {
			ownerMatch := ownerPattern == "" || strings.Contains(strings.ToLower(p.Owner), ownerPattern)
			repoMatch := repoPattern == "" || strings.Contains(strings.ToLower(p.Repo), repoPattern)
			if ownerMatch && repoMatch {
				names = append(names, p.Name)
			}
		}
		return names
	}

	// Short name — match against repo
	var names []string
	for _, p := range projects {
		if strings.Contains(strings.ToLower(p.Repo), lower) {
			names = append(names, p.Name)
		}
	}
	return names
}
