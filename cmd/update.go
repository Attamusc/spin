package cmd

import (
	"fmt"
	"os"
	"sync"

	"github.com/attamusc/spin/internal/config"
	"github.com/attamusc/spin/internal/discovery"
	"github.com/attamusc/spin/internal/resolve"
	"github.com/attamusc/spin/internal/vcs"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var updateCmd = &cobra.Command{
	Use:   "update [project]",
	Short: "Fetch latest changes for projects",
	Long: `Fetch the latest changes from remote for one or all projects.

If a project argument is given, fetches only that project.
If no argument is given, fetches all projects in parallel.

Uses jj git fetch (preferred) or git fetch depending on the repo type.
Non-repo directories are skipped.`,
	Args:              cobra.MaximumNArgs(1),
	ValidArgsFunction: completeAllProjectNames,
	RunE:              runUpdate,
}

func init() {
	rootCmd.AddCommand(updateCmd)
}

type updateResult struct {
	name   string
	status string // "updated", "skipped", "failed"
	err    error
}

func runUpdate(cmd *cobra.Command, args []string) error {
	globalCfg, err := config.LoadGlobal()
	if err != nil {
		return fmt.Errorf("failed to load global config: %w", err)
	}

	cyan := color.New(color.FgCyan).SprintFunc()

	// Single project update
	if len(args) > 0 {
		rd, err := resolve.ResolveAny(args[0], globalCfg.ProjectsRoot, globalCfg.DefaultOwner)
		if err != nil {
			return fmt.Errorf("failed to resolve project: %w", err)
		}

		if !vcs.IsRepo(rd.Dir) {
			return fmt.Errorf("%s is not a git/jj repository", rd.Name)
		}

		fmt.Fprintf(os.Stderr, "Fetching %s...\n", cyan(rd.Name))
		if err := vcs.Fetch(rd.Dir); err != nil {
			return fmt.Errorf("fetch failed for %s: %w", rd.Name, err)
		}
		color.New(color.FgGreen).Fprintf(os.Stderr, "Updated %s\n", cyan(rd.Name))
		return nil
	}

	// All projects update
	projects, err := discovery.ScanAll(globalCfg.ProjectsRoot)
	if err != nil {
		return fmt.Errorf("failed to scan projects: %w", err)
	}

	if len(projects) == 0 {
		fmt.Fprintln(os.Stderr, "No projects found.")
		return nil
	}

	fmt.Fprintf(os.Stderr, "Fetching %d projects...\n", len(projects))

	// Fetch in parallel with bounded concurrency
	const maxConcurrency = 8
	sem := make(chan struct{}, maxConcurrency)
	results := make([]updateResult, len(projects))
	var wg sync.WaitGroup

	for i, p := range projects {
		wg.Add(1)
		go func(idx int, proj discovery.DiscoveredProject) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			if !vcs.IsRepo(proj.Dir) {
				results[idx] = updateResult{name: proj.Name, status: "skipped"}
				return
			}

			if err := vcs.Fetch(proj.Dir); err != nil {
				results[idx] = updateResult{name: proj.Name, status: "failed", err: err}
				return
			}

			results[idx] = updateResult{name: proj.Name, status: "updated"}
		}(i, p)
	}

	wg.Wait()

	// Report results
	green := color.New(color.FgGreen).SprintFunc()
	yellow := color.New(color.FgYellow).SprintFunc()
	red := color.New(color.FgRed).SprintFunc()

	var updated, skipped, failed int
	for _, r := range results {
		switch r.status {
		case "updated":
			updated++
		case "skipped":
			skipped++
		case "failed":
			failed++
			fmt.Fprintf(os.Stderr, "  %s %s: %v\n", red("FAIL"), cyan(r.name), r.err)
		}
	}

	fmt.Fprintf(os.Stderr, "\n%s %d updated", green("Done:"), updated)
	if skipped > 0 {
		fmt.Fprintf(os.Stderr, ", %s %d skipped (not repos)", yellow("⚠"), skipped)
	}
	if failed > 0 {
		fmt.Fprintf(os.Stderr, ", %s %d failed", red("✗"), failed)
	}
	fmt.Fprintln(os.Stderr)

	return nil
}
