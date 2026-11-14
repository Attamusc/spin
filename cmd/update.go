package cmd

import (
	"errors"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"

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
	ValidArgsFunction: completeProjectNames,
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
	ctx, stop := signal.NotifyContext(cmd.Context(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	globalCfg, err := config.LoadGlobal()
	if err != nil {
		return fmt.Errorf("failed to load global config: %w", err)
	}

	cyan := color.New(color.FgCyan).SprintFunc()

	// Single project update
	if len(args) > 0 {
		rd, err := resolve.ResolveAny(args[0], globalCfg.ProjectsRoot)
		if err != nil {
			return fmt.Errorf("failed to resolve project: %w", err)
		}

		if !vcs.IsRepo(rd.Dir) {
			return fmt.Errorf("%s is not a git/jj repository", rd.Name)
		}

		fmt.Fprintf(os.Stderr, "Fetching %s...\n", cyan(rd.Name))
		if err := vcs.FetchContext(ctx, rd.Dir); err != nil {
			return fmt.Errorf("fetch failed for %s: %w", rd.Name, err)
		}
		color.New(color.FgGreen).Fprintf(os.Stderr, "Updated %s\n", cyan(rd.Name))
		return nil
	}

	// All projects update
	projects, scanErr := discovery.ScanAll(globalCfg.ProjectsRoot)
	if len(projects) == 0 {
		if scanErr == nil {
			fmt.Fprintln(os.Stderr, "No projects found.")
		}
		return scanErr
	}

	fmt.Fprintf(os.Stderr, "Fetching %d projects...\n", len(projects))

	// Fetch in parallel with bounded concurrency
	const maxConcurrency = 8
	jobs := make(chan int)
	results := make([]updateResult, len(projects))
	var wg sync.WaitGroup

	for range min(maxConcurrency, len(projects)) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range jobs {
				proj := projects[idx]
				r := updateResult{name: proj.Name, status: "updated"}
				if err := ctx.Err(); err != nil {
					r.status, r.err = "failed", err
				} else if !vcs.IsRepo(proj.Dir) {
					r.status = "skipped"
				} else if err := vcs.FetchContext(ctx, proj.Dir); err != nil {
					r.status, r.err = "failed", err
				}
				results[idx] = r
			}
		}()
	}
schedule:
	for i := range projects {
		select {
		case jobs <- i:
		case <-ctx.Done():
			for j := i; j < len(projects); j++ {
				results[j] = updateResult{name: projects[j].Name, status: "failed", err: ctx.Err()}
			}
			break schedule
		}
	}
	close(jobs)
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
	if failed > 0 {
		return errors.Join(scanErr, fmt.Errorf("fetch failed for %d projects", failed))
	}
	return errors.Join(scanErr, ctx.Err())
}
