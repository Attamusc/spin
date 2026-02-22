package cmd

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/attamusc/spin/internal/config"
	"github.com/attamusc/spin/internal/discovery"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "Show all discoverable projects",
	Long:  `Recursively scan the projects root for directories containing .spin.toml.`,
	RunE:  runList,
}

func init() {
	rootCmd.AddCommand(listCmd)
}

func runList(cmd *cobra.Command, args []string) error {
	globalCfg, err := config.LoadGlobal()
	if err != nil {
		return fmt.Errorf("failed to load global config: %w", err)
	}

	projects, err := discovery.Scan(globalCfg.ProjectsRoot)
	if err != nil {
		return fmt.Errorf("failed to scan projects: %w", err)
	}

	if len(projects) == 0 {
		fmt.Println("No projects found.")
		fmt.Printf("Add a .spin.toml file to a project under %s to get started.\n", globalCfg.ProjectsRoot)
		fmt.Println("Run 'spin init' in a project directory to generate one.")
		return nil
	}

	cyan := color.New(color.FgCyan).SprintFunc()

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 4, ' ', 0)
	for _, p := range projects {
		// Replace home dir with ~ for display
		displayDir := p.Dir
		home, err := os.UserHomeDir()
		if err == nil {
			displayDir = strings.Replace(displayDir, home, "~", 1)
		}
		fmt.Fprintf(w, "%s\t%s\n", cyan(p.Name), displayDir)
	}
	w.Flush()

	return nil
}
