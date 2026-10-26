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
	Short: "Show all projects",
	Long: `List all projects under the projects root directory.

By default, shows every project directory (host/owner/repo) regardless
of whether it has a .spin.toml configuration file. Projects with a
.spin.toml are marked with a * indicator.

Flags:
  --configured / -c    Show only projects with .spin.toml
  --format plain       Output one name per line (for scripting/fzf)`,
	Aliases: []string{"ls"},
	RunE:    runList,
}

func init() {
	listCmd.Flags().BoolP("configured", "c", false, "show only .spin.toml projects")
	listCmd.Flags().String("format", "", "output format: plain (one name per line)")
	rootCmd.AddCommand(listCmd)
}

func runList(cmd *cobra.Command, args []string) error {
	globalCfg, err := config.LoadGlobal()
	if err != nil {
		return fmt.Errorf("failed to load global config: %w", err)
	}

	configuredOnly, _ := cmd.Flags().GetBool("configured")
	format, _ := cmd.Flags().GetString("format")

	var projects []discovery.DiscoveredProject

	if configuredOnly {
		projects, err = discovery.Scan(globalCfg.ProjectsRoot)
	} else {
		projects, err = discovery.ScanAll(globalCfg.ProjectsRoot)
	}
	if err != nil {
		return fmt.Errorf("failed to scan projects: %w", err)
	}

	// Plain format — one name per line, for scripting and fzf
	if format == "plain" {
		for _, p := range projects {
			fmt.Println(p.Name)
		}
		return nil
	}

	if len(projects) == 0 {
		if configuredOnly {
			fmt.Println("No configured projects found.")
			fmt.Printf("Add a .spin.toml file to a project under %s to get started.\n", globalCfg.ProjectsRoot)
			fmt.Println("Run 'spin init' in a project directory to generate one.")
		} else {
			fmt.Println("No projects found.")
			fmt.Printf("Clone a repo with 'spin clone <repo>' or create one with 'spin new <name>'.\n")
		}
		return nil
	}

	// Table format (default)
	cyan := color.New(color.FgCyan).SprintFunc()
	green := color.New(color.FgGreen).SprintFunc()

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 4, ' ', 0)
	for _, p := range projects {
		displayDir := p.Dir
		home, err := os.UserHomeDir()
		if err == nil {
			displayDir = strings.Replace(displayDir, home, "~", 1)
		}
		cfgMarker := ""
		if p.HasConfig {
			cfgMarker = green("*")
		}
		fmt.Fprintf(w, "%s\t%s\t%s\n", cyan(p.Name), cfgMarker, displayDir)
	}
	w.Flush()

	return nil
}
