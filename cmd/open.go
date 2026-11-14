package cmd

import (
	"fmt"
	"os/exec"
	"runtime"

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
	project, err := loadProject(args)
	if err != nil {
		return err
	}

	cyan := color.New(color.FgCyan).SprintFunc()

	port, err := project.browserPort()
	if err != nil {
		return err
	}

	url := fmt.Sprintf("http://localhost:%d", port)
	color.New(color.FgGreen).Fprintf(cmd.ErrOrStderr(), "Opening %s (%s)...\n", cyan(project.Name), url)

	return openURL(runtime.GOOS, url, runBrowserCommand)
}
