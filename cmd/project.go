package cmd

import (
	"fmt"

	"github.com/attamusc/spin/internal/config"
	"github.com/attamusc/spin/internal/resolve"
)

func loadProject(args []string) (*config.GlobalConfig, *resolve.ResolvedProject, error) {
	cfg, err := config.LoadGlobal()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load global config: %w", err)
	}
	arg := ""
	if len(args) > 0 {
		arg = args[0]
	}
	project, err := resolve.Resolve(arg, cfg.ProjectsRoot)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to resolve project: %w", err)
	}
	if _, err := config.ProjectPath(cfg.ProjectsRoot, ".spin", project.Owner, project.Repo, "live.pid"); err != nil {
		return nil, nil, err
	}
	return cfg, project, nil
}

type exitError int

func (e exitError) Error() string { return fmt.Sprintf("command exited with status %d", e) }
