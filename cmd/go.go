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
  (empty)       List project names (Ctrl+Space opens the shell's fzf picker)

If the project doesn't exist locally, a clone command is suggested.`,
	Args:              cobra.MaximumNArgs(1),
	ValidArgsFunction: completeProjectNames,
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
		if len(projects) == 0 {
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.ErrOrStderr(), "No projects found.")
			return nil
		}
		// Print all project names for fzf selection
		for _, p := range projects {
			fmt.Fprintln(cmd.OutOrStdout(), p.Name)
		}
		return err
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
