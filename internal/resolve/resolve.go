package resolve

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/attamusc/spin/internal/config"
	"github.com/attamusc/spin/internal/discovery"
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

// Resolve resolves a project from an optional argument.
// If arg is empty, resolves from CWD. Otherwise uses name resolution.
func Resolve(arg, projectsRoot string) (*ResolvedProject, error) {
	if arg == "" {
		return FromCWD(projectsRoot)
	}
	return FromName(arg, projectsRoot)
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

// FromName resolves a project from a name argument using the following strategy:
// 1. Full "owner/repo" match — scan {projects_root}/*/{argument}/.spin.toml
// 2. Short name match — scan all projects, match the repo portion
func FromName(name, projectsRoot string) (*ResolvedProject, error) {
	absRoot, err := filepath.Abs(projectsRoot)
	if err != nil {
		return nil, fmt.Errorf("could not resolve absolute path: %w", err)
	}

	// If name contains /, try owner/repo match
	if strings.Contains(name, "/") {
		return resolveOwnerRepo(name, absRoot)
	}

	// Short name match
	return resolveShortName(name, absRoot)
}

// resolveOwnerRepo resolves "owner/repo" by scanning {projects_root}/*/owner/repo
func resolveOwnerRepo(ownerRepo, projectsRoot string) (*ResolvedProject, error) {
	hosts, err := os.ReadDir(projectsRoot)
	if err != nil {
		return nil, fmt.Errorf("could not read projects root: %w", err)
	}

	for _, host := range hosts {
		if !host.IsDir() || strings.HasPrefix(host.Name(), ".") {
			continue
		}
		dir := filepath.Join(projectsRoot, host.Name(), ownerRepo)
		spinToml := filepath.Join(dir, ".spin.toml")
		if _, err := os.Stat(spinToml); err == nil {
			return resolveFromProjectDir(dir, projectsRoot)
		}
	}

	return nil, fmt.Errorf("project %q not found. Run 'spin list' to see available projects", ownerRepo)
}

// resolveShortName resolves a short repo name by scanning all projects.
func resolveShortName(name, projectsRoot string) (*ResolvedProject, error) {
	projects, err := discovery.Scan(projectsRoot)
	if err != nil {
		return nil, fmt.Errorf("could not scan projects: %w", err)
	}

	var matches []discovery.DiscoveredProject
	for _, p := range projects {
		if p.Repo == name {
			matches = append(matches, p)
		}
	}

	switch len(matches) {
	case 0:
		// No match — provide suggestions
		var suggestions []string
		for _, p := range projects {
			if strings.Contains(p.Repo, name) || strings.Contains(p.Name, name) {
				suggestions = append(suggestions, p.Name)
			}
		}
		if len(suggestions) > 0 {
			return nil, fmt.Errorf("project %q not found. Did you mean one of these?\n  %s",
				name, strings.Join(suggestions, "\n  "))
		}
		return nil, fmt.Errorf("project %q not found. Run 'spin list' to see available projects", name)

	case 1:
		return resolveFromProjectDir(matches[0].Dir, projectsRoot)

	default:
		// Multiple matches — list them
		var names []string
		for _, m := range matches {
			names = append(names, m.Name)
		}
		return nil, fmt.Errorf("ambiguous project name %q matches multiple projects:\n  %s\nPlease specify as owner/repo",
			name, strings.Join(names, "\n  "))
	}
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
