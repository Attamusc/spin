package resolve

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/attamusc/spin/internal/config"
)

// ResolvedProject holds the fully resolved project information.
type ResolvedProject struct {
	Host   string                // e.g., "github.com"
	Owner  string                // e.g., "attamusc"
	Repo   string                // e.g., "mindtrail"
	Name   string                // "owner/repo" for display
	Dir    string                // absolute path to project directory
	Config *config.ProjectConfig // parsed .spin.toml
}

// FromCWD resolves a project by walking up from the current working directory
// looking for .spin.toml. It derives the project identity from the path
// relative to projectsRoot.
func FromCWD(projectsRoot string) (*ResolvedProject, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("could not get working directory: %w", err)
	}
	return FromDir(cwd, projectsRoot)
}

// FromDir resolves a project by walking up from the given directory
// looking for .spin.toml.
func FromDir(dir, projectsRoot string) (*ResolvedProject, error) {
	// Normalize paths
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("could not resolve absolute path: %w", err)
	}
	absProjectsRoot, err := filepath.Abs(projectsRoot)
	if err != nil {
		return nil, fmt.Errorf("could not resolve absolute path: %w", err)
	}

	// Check that the directory is within projects_root
	if !strings.HasPrefix(absDir, absProjectsRoot+string(filepath.Separator)) && absDir != absProjectsRoot {
		return nil, fmt.Errorf("current directory %s is not within projects_root %s", absDir, absProjectsRoot)
	}

	// Walk up looking for .spin.toml
	current := absDir
	for {
		spinToml := filepath.Join(current, ".spin.toml")
		if _, err := os.Stat(spinToml); err == nil {
			return resolveFromProjectDir(current, absProjectsRoot)
		}

		// Don't walk above projects_root
		if current == absProjectsRoot {
			break
		}

		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}

	return nil, fmt.Errorf("no .spin.toml found in %s or any parent directory up to %s", absDir, absProjectsRoot)
}

// resolveFromProjectDir extracts host/owner/repo from a project directory path
// relative to projectsRoot, and loads the project config.
func resolveFromProjectDir(projectDir, projectsRoot string) (*ResolvedProject, error) {
	rel, err := filepath.Rel(projectsRoot, projectDir)
	if err != nil {
		return nil, fmt.Errorf("could not compute relative path: %w", err)
	}

	parts := strings.Split(rel, string(filepath.Separator))
	if len(parts) < 3 {
		return nil, fmt.Errorf("project path %s does not match expected host/owner/repo structure", projectDir)
	}

	host := parts[0]
	owner := parts[1]
	repo := parts[2]

	cfg, err := config.LoadProject(projectDir)
	if err != nil {
		return nil, err
	}

	return &ResolvedProject{
		Host:   host,
		Owner:  owner,
		Repo:   repo,
		Name:   owner + "/" + repo,
		Dir:    projectDir,
		Config: cfg,
	}, nil
}
