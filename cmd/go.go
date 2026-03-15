package cmd

import (
	"fmt"
	"os"

	"github.com/attamusc/spin/internal/config"
	"github.com/attamusc/spin/internal/discovery"
	"github.com/attamusc/spin/internal/resolve"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var goCmd = &cobra.Command{
	Use:   "go [project]",
	Short: "Navigate to a project directory",
	Long: `Navigate to a project directory. The shell wrapper (from spin shell-init)
intercepts the output and cd's into the printed path.

Project formats:
  owner/repo    Navigate to a specific project
  repo          Short name lookup (case-insensitive)
  (empty)       Interactive picker via fzf (requires shell integration)

If the project doesn't exist locally and can be identified as owner/repo,
you'll be prompted to clone it.`,
	Args:              cobra.MaximumNArgs(1),
	ValidArgsFunction: completeAllProjectNames,
	RunE:              runGo,
}

func init() {
	rootCmd.AddCommand(goCmd)
}

func runGo(cmd *cobra.Command, args []string) error {
	globalCfg, err := config.LoadGlobal()
	if err != nil {
		return fmt.Errorf("failed to load global config: %w", err)
	}

	var arg string
	if len(args) > 0 {
		arg = args[0]
	}

	if arg == "" {
		// No argument — list all projects for piping to fzf (or the shell widget handles it)
		projects, err := discovery.ScanAll(globalCfg.ProjectsRoot)
		if err != nil {
			return fmt.Errorf("failed to scan projects: %w", err)
		}
		if len(projects) == 0 {
			fmt.Fprintln(os.Stderr, "No projects found.")
			return nil
		}
		// Print all project names for fzf selection
		for _, p := range projects {
			fmt.Println(p.Name)
		}
		return nil
	}

	rd, err := resolve.ResolveAny(arg, globalCfg.ProjectsRoot, globalCfg.DefaultOwner)
	if err != nil {
		// Not found — suggest cloning if it looks like an owner/repo
		cyan := color.New(color.FgCyan).SprintFunc()
		fmt.Fprintf(os.Stderr, "%v\n", err)
		fmt.Fprintf(os.Stderr, "\nTo clone it: %s\n", cyan(fmt.Sprintf("spin clone %s", arg)))
		return fmt.Errorf("project not found")
	}

	cyan := color.New(color.FgCyan).SprintFunc()
	fmt.Fprintf(os.Stderr, "→ %s\n", cyan(rd.Name))

	// Print path as the last line of stdout — shell wrapper cd's into this
	fmt.Println(rd.Dir)

	return nil
}

// completeAllProjectNames returns a cobra completion function that suggests
// ALL discovered project names (including those without .spin.toml).
func completeAllProjectNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
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
