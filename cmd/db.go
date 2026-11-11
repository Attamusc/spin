package cmd

import (
	"errors"
	"fmt"
	"os"
	"syscall"

	"github.com/attamusc/spin/internal/config"
	"github.com/attamusc/spin/internal/db"
	"github.com/attamusc/spin/internal/process"
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
	operation, profile := cmd.Name(), args[0]
	profiles := []string{profile}
	if operation == "copy" {
		profiles = append(profiles, args[1])
	}
	for _, p := range profiles {
		if p != "dev" && p != "live" {
			return fmt.Errorf("profile must be 'dev' or 'live', got %q", p)
		}
	}
	if operation == "copy" && args[0] == args[1] {
		return fmt.Errorf("source and target profiles must be different")
	}
	if operation == "snapshot" || operation == "restore" {
		if err := config.ValidateName(args[1]); err != nil {
			return fmt.Errorf("invalid snapshot name: %w", err)
		}
	}
	cfg, project, err := loadProject(nil)
	if err != nil {
		return err
	}
	path := func(p string) (string, error) {
		return config.ResolveProfileDBPath(cfg.DataRoot, project.Owner, project.Repo, p, project.Config.DBFilename)
	}
	src, err := path(profile)
	if err != nil {
		return err
	}
	dst := src
	if operation == "copy" {
		profile = args[1]
		dst, err = path(profile)
		if err != nil {
			return err
		}
	}
	if operation == "snapshot" || operation == "restore" {
		snapshot, err := config.ProjectPath(cfg.DataRoot, project.Owner, project.Repo, "snapshots", args[1]+".db")
		if err != nil {
			return err
		}
		if operation == "snapshot" {
			dst = snapshot
		} else {
			src = snapshot
		}
	}
	if operation != "snapshot" {
		unlock, err := process.LockProfile(cfg.DataRoot, project.Owner, project.Repo, profile)
		if err != nil {
			return err
		}
		defer unlock()
		if profile == "live" {
			pid, err := process.ReadPID(process.PIDFilePath(cfg.DataRoot, project.Owner, project.Repo))
			if err != nil && !os.IsNotExist(err) {
				return err
			}
			if err == nil && !errors.Is(process.Probe(pid), syscall.ESRCH) {
				return fmt.Errorf("cannot %s '%s' profile while it is running or its state is unknown", operation, profile)
			}
		}
	}
	if operation == "reset" {
		err = db.RemoveDB(dst)
	} else {
		err = db.CopyDB(src, dst)
	}
	if err != nil {
		return fmt.Errorf("failed to %s database: %w", operation, err)
	}
	switch operation {
	case "copy":
		fmt.Fprintf(os.Stderr, "Copied %s -> %s for %s\n", args[0], profile, project.Name)
	case "snapshot":
		fmt.Fprintf(os.Stderr, "Snapshot '%s' created for %s (%s profile)\n  %s\n", args[1], project.Name, profile, dst)
	case "restore":
		fmt.Fprintf(os.Stderr, "Restored snapshot '%s' to %s (%s profile)\n", args[1], project.Name, profile)
	case "reset":
		fmt.Fprintf(os.Stderr, "Reset %s database for %s\nThe database will be recreated on next start.\n", profile, project.Name)
	}
	return nil
}
