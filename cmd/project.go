package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/attamusc/spin/internal/config"
	"github.com/attamusc/spin/internal/process"
	"github.com/attamusc/spin/internal/resolve"
)

type managedProject struct {
	*resolve.ResolvedProject
	global     *config.GlobalConfig
	probe      func(int) error
	foreground func(process.ForegroundOptions) (int, error)
}

func newManagedProject(cfg *config.GlobalConfig, project *resolve.ResolvedProject) *managedProject {
	return &managedProject{ResolvedProject: project, global: cfg, probe: process.Probe, foreground: process.RunForeground}
}

func loadProject(args []string) (*managedProject, error) {
	cfg, err := config.LoadGlobal()
	if err != nil {
		return nil, fmt.Errorf("failed to load global config: %w", err)
	}
	arg := ""
	if len(args) > 0 {
		arg = args[0]
	}
	project, err := resolve.Resolve(arg, cfg.ProjectsRoot)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve project: %w", err)
	}
	return newManagedProject(cfg, project), nil
}

func (p *managedProject) statePath(parts ...string) (string, error) {
	components := append([]string{filepath.Base(p.global.DataRoot), p.Owner, p.Repo}, parts...)
	return config.ProjectPath(filepath.Dir(p.global.DataRoot), components...)
}

func (p *managedProject) dbPath(profile string) (string, error) {
	return config.ResolveProfileDBPath(p.global.DataRoot, p.Owner, p.Repo, profile, p.Config.DBFilename)
}

func (p *managedProject) snapshotPath(name string) (string, error) {
	if err := config.ValidateName(name); err != nil {
		return "", fmt.Errorf("invalid snapshot name: %w", err)
	}
	return p.statePath("snapshots", name+".db")
}

func (p *managedProject) browserPort() (int, error) {
	pidFile, err := p.statePath("live.pid")
	if err != nil {
		return 0, err
	}
	pid, err := process.ReadPID(pidFile)
	selected := p.Config.Live
	if !(err == nil && process.IsProcessAlive(pid) && selected.Port != 0) && p.Config.Dev.Port != 0 {
		selected = p.Config.Dev
	}
	if err := config.ValidatePort(selected.Port); err != nil {
		return 0, fmt.Errorf("invalid selected port for %s: %w", p.Name, err)
	}
	if selected.Port == 0 {
		return 0, fmt.Errorf("no port configured for %s (set [dev].port or [live].port in .spin.toml)", p.Name)
	}
	return selected.Port, nil
}

type exitError int

func (e exitError) Error() string { return fmt.Sprintf("command exited with status %d", e) }
