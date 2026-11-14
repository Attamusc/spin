package cmd

import (
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var devCmd = &cobra.Command{
	Use:   "dev [project]",
	Short: "Run a project in dev mode (foregrounded)",
	Long: `Run a project in dev mode with the foregrounded process.
The dev profile uses a separate database for data isolation.

If no project argument is given, the project is resolved from the current directory.`,
	Args:              cobra.MaximumNArgs(1),
	ValidArgsFunction: completeProjectNames,
	RunE:              runDev,
}

func init() {
	rootCmd.AddCommand(devCmd)
}

func runDev(cmd *cobra.Command, args []string) error {
	project, err := loadProject(args)
	if err != nil {
		return err
	}
	exitCode, err := project.runDev(func() {
		cyan := color.New(color.FgCyan).SprintFunc()
		color.New(color.FgGreen).Fprintf(cmd.ErrOrStderr(), "Starting dev mode for %s...\n", cyan(project.Name))
	})
	if err != nil {
		return err
	}
	if exitCode != 0 {
		cmd.SilenceErrors, cmd.SilenceUsage = true, true
		return exitError(exitCode)
	}

	return nil
}
