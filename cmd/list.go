package cmd

import (
	"errors"
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
	// Preserve available rows on incomplete scans; the error still makes the
	// command fail so scripts never confuse a partial inventory with a full one.
	// Plain format — one name per line, for scripting and fzf
	if format == "plain" {
		for _, p := range projects {
			if _, writeErr := fmt.Fprintln(cmd.OutOrStdout(), p.Name); writeErr != nil {
				return errors.Join(err, writeErr)
			}
		}
		return err
	}

	if len(projects) == 0 {
		if err != nil {
			return err
		}
		if configuredOnly {
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "No configured projects found.\nAdd a .spin.toml file to a project under %s to get started.\nRun 'spin init' in a project directory to generate one.\n", globalCfg.ProjectsRoot)
		} else {
			_, err = fmt.Fprintln(cmd.OutOrStdout(), "No projects found.\nClone a repo with 'spin clone <repo>' or create one with 'spin new <name>'.")
		}
		return err
	}

	// Table format (default)
	cyan := color.New(color.FgCyan).SprintFunc()
	green := color.New(color.FgGreen).SprintFunc()

	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 4, ' ', 0)
	for _, p := range projects {
		displayDir := p.Dir
		home, homeErr := os.UserHomeDir()
		if homeErr == nil {
			displayDir = strings.Replace(displayDir, home, "~", 1)
		}
		cfgMarker := ""
		if p.HasConfig {
			cfgMarker = green("*")
		}
		if _, writeErr := fmt.Fprintf(w, "%s\t%s\t%s\n", cyan(p.Name), cfgMarker, displayDir); writeErr != nil {
			return errors.Join(err, writeErr)
		}
	}
	return errors.Join(err, w.Flush())
}
