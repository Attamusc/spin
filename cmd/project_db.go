package cmd

import (
	"errors"
	"fmt"
	"os"
	"syscall"

	"github.com/attamusc/spin/internal/db"
	"github.com/attamusc/spin/internal/process"
)

func (p *managedProject) copyDB(sourceProfile, targetProfile string) error {
	source, err := p.dbPath(sourceProfile)
	if err != nil {
		return err
	}
	target, err := p.dbPath(targetProfile)
	if err != nil {
		return err
	}
	if sourceProfile == targetProfile {
		return fmt.Errorf("source and target profiles must be different")
	}
	return p.mutateDB("copy", targetProfile, func() error { return db.CopyDB(source, target) })
}

func (p *managedProject) snapshotDB(profile, name string) (string, error) {
	snapshot, err := p.snapshotPath(name)
	if err != nil {
		return "", err
	}
	source, err := p.dbPath(profile)
	if err != nil {
		return "", err
	}
	if err := db.CopyDB(source, snapshot); err != nil {
		return "", fmt.Errorf("failed to snapshot database: %w", err)
	}
	return snapshot, nil
}

func (p *managedProject) restoreDB(profile, name string) error {
	snapshot, err := p.snapshotPath(name)
	if err != nil {
		return err
	}
	target, err := p.dbPath(profile)
	if err != nil {
		return err
	}
	return p.mutateDB("restore", profile, func() error { return db.CopyDB(snapshot, target) })
}

func (p *managedProject) resetDB(profile string) error {
	target, err := p.dbPath(profile)
	if err != nil {
		return err
	}
	return p.mutateDB("reset", profile, func() error { return db.RemoveDB(target) })
}

func (p *managedProject) mutateDB(operation, profile string, mutate func() error) error {
	release, err := process.LockProfile(p.global.DataRoot, p.Owner, p.Repo, profile)
	if err != nil {
		return err
	}
	defer release()
	if profile == "live" {
		pidFile, err := p.statePath("live.pid")
		if err != nil {
			return err
		}
		pid, err := process.ReadPID(pidFile)
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		if err == nil && !errors.Is(p.probe(pid), syscall.ESRCH) {
			return fmt.Errorf("cannot %s '%s' profile while it is running or its state is unknown", operation, profile)
		}
	}
	if err := mutate(); err != nil {
		return fmt.Errorf("failed to %s database: %w", operation, err)
	}
	return nil
}
