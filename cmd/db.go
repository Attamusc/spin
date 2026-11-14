package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var dbCmd = &cobra.Command{Use: "db", Short: "Database operations", Long: "Database operations for spin project profiles."}

func init() {
	rootCmd.AddCommand(dbCmd)
	for _, spec := range []struct {
		use, short string
		argc       int
	}{
		{"copy [source-profile] [target-profile]", "Copy one profile's database to another", 2},
		{"snapshot [profile] [name]", "Create a named snapshot of a profile's database", 2},
		{"restore [profile] [name]", "Restore a profile's database from a named snapshot", 2},
		{"reset [profile]", "Delete a profile's database", 1},
	} {
		dbCmd.AddCommand(&cobra.Command{Use: spec.use, Short: spec.short, Args: cobra.ExactArgs(spec.argc), RunE: runDB})
	}
}

func runDB(cmd *cobra.Command, args []string) error {
	project, err := loadProject(nil)
	if err != nil {
		return err
	}
	profile := args[0]
	switch cmd.Name() {
	case "copy":
		if err := project.copyDB(profile, args[1]); err != nil {
			return err
		}
		fmt.Fprintf(cmd.ErrOrStderr(), "Copied %s -> %s for %s\n", profile, args[1], project.Name)
	case "snapshot":
		path, err := project.snapshotDB(profile, args[1])
		if err != nil {
			return err
		}
		fmt.Fprintf(cmd.ErrOrStderr(), "Snapshot '%s' created for %s (%s profile)\n  %s\n", args[1], project.Name, profile, path)
	case "restore":
		if err := project.restoreDB(profile, args[1]); err != nil {
			return err
		}
		fmt.Fprintf(cmd.ErrOrStderr(), "Restored snapshot '%s' to %s (%s profile)\n", args[1], project.Name, profile)
	case "reset":
		if err := project.resetDB(profile); err != nil {
			return err
		}
		fmt.Fprintf(cmd.ErrOrStderr(), "Reset %s database for %s\nThe database will be recreated on next start.\n", profile, project.Name)
	}
	return nil
}
