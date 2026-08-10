package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"

	"github.com/attamusc/spin/internal/config"
	"github.com/attamusc/spin/internal/process"
	"github.com/attamusc/spin/internal/resolve"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var openCmd = &cobra.Command{
	Use:   "open [project]",
	Short: "Open a running project in the browser",
	Long: `Open a project's URL in the default browser.
Determines the port from the live profile if running, otherwise from the dev profile config.

If no project argument is given, the project is resolved from the current directory.`,
	Args:              cobra.MaximumNArgs(1),
	ValidArgsFunction: completeProjectNames,
	RunE:              runOpen,
}

func init() {
	rootCmd.AddCommand(openCmd)
}

func openerForOS(goos string) (string, error) {
	switch goos {
	case "darwin":
		return "open", nil
	case "linux":
		return "xdg-open", nil
	default:
		return "", fmt.Errorf("unsupported operating system %q", goos)
	}
}

func runBrowserCommand(name string, args ...string) error {
	return exec.Command(name, args...).Run()
}

func openURL(goos, url string, run func(string, ...string) error) error {
	opener, err := openerForOS(goos)
	if err != nil {
		return err
	}
	if err := run(opener, url); err != nil {
		return fmt.Errorf("failed to open browser with %s: %w", opener, err)
	}
	return nil
}

func runOpen(cmd *cobra.Command, args []string) error {
	globalCfg, err := config.LoadGlobal()
	if err != nil {
		return fmt.Errorf("failed to load global config: %w", err)
	}

	var arg string
	if len(args) > 0 {
		arg = args[0]
	}
	project, err := resolve.Resolve(arg, globalCfg.ProjectsRoot)
	if err != nil {
		return fmt.Errorf("failed to resolve project: %w", err)
	}

	cyan := color.New(color.FgCyan).SprintFunc()

	// Determine port: check live profile first (if running), then dev config
	port := 0
	pidFile := process.PIDFilePath(globalCfg.DataRoot, project.Owner, project.Repo)
	pid, err := process.ReadPID(pidFile)
	if err == nil && process.IsProcessAlive(pid) && project.Config.Live.Port > 0 {
		port = project.Config.Live.Port
	} else if project.Config.Dev.Port > 0 {
		port = project.Config.Dev.Port
	} else if project.Config.Live.Port > 0 {
		port = project.Config.Live.Port
	}

	if port == 0 {
		return fmt.Errorf("no port configured for %s (set [dev].port or [live].port in .spin.toml)", project.Name)
	}

	url := fmt.Sprintf("http://localhost:%d", port)
	color.New(color.FgGreen).Fprintf(os.Stderr, "Opening %s (%s)...\n", cyan(project.Name), url)

	return openURL(runtime.GOOS, url, runBrowserCommand)
}
