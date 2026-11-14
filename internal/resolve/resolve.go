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
	Host   string
	Owner  string
	Repo   string
	Name   string
	Dir    string
	Config *config.ProjectConfig
}

// ResolvedDir holds project directory info without requiring .spin.toml.
type ResolvedDir struct {
	Host  string
	Owner string
	Repo  string
	Name  string
	Dir   string
}

// Resolve resolves a configured project by name, or from CWD for an empty argument.
func Resolve(arg, projectsRoot string) (*ResolvedProject, error) {
	if arg == "" {
		return FromCWD(projectsRoot)
	}
	return FromName(arg, projectsRoot)
}

// FromCWD resolves the configured project containing the working directory.
func FromCWD(projectsRoot string) (*ResolvedProject, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("could not get working directory: %w", err)
	}
	return FromDir(cwd, projectsRoot)
}

// projectDirectory identifies the depth-three root and checks physical containment.
func projectDirectory(dir, root string) (*ResolvedDir, error) {
	absRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return nil, err
	}
	absRoot, err = filepath.Abs(absRoot)
	if err != nil {
		return nil, err
	}
	absDir, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return nil, err
	}
	absDir, err = filepath.Abs(absDir)
	if err != nil {
		return nil, err
	}
	rel, err := filepath.Rel(absRoot, absDir)
	if err != nil {
		return nil, err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return nil, fmt.Errorf("current directory %s is not within projects_root %s", absDir, absRoot)
	}
	parts := strings.Split(rel, string(filepath.Separator))
	if len(parts) < 3 {
		return nil, fmt.Errorf("current directory is not deep enough within projects_root to identify a project (need host/owner/repo)")
	}
	for _, part := range parts[:3] {
		if err := config.ValidateName(part); err != nil {
			return nil, err
		}
	}
	return &ResolvedDir{Host: parts[0], Owner: parts[1], Repo: parts[2], Name: parts[1] + "/" + parts[2], Dir: filepath.Join(absRoot, parts[0], parts[1], parts[2])}, nil
}

// FromDir resolves a configured project containing dir. Configuration belongs at
// host/owner/repo, not at an arbitrary parent or nested directory.
func FromDir(dir, projectsRoot string) (*ResolvedProject, error) {
	p, err := projectDirectory(dir, projectsRoot)
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(filepath.Join(p.Dir, ".spin.toml")); err != nil {
		return nil, fmt.Errorf("no .spin.toml found at project root %s: %w", p.Dir, err)
	}
	projects, err := inventory(projectsRoot, true)
	if err != nil {
		return nil, err
	}
	if _, err := match(p.Name, projects, true); err != nil {
		return nil, err
	}
	return loadProject(p)
}

func loadProject(p *ResolvedDir) (*ResolvedProject, error) {
	cfg, err := config.LoadProject(p.Dir)
	if err != nil {
		return nil, err
	}
	return &ResolvedProject{Host: p.Host, Owner: p.Owner, Repo: p.Repo, Name: p.Name, Dir: p.Dir, Config: cfg}, nil
}

// FromName resolves exact names before case-insensitive substring matches.
func FromName(name, projectsRoot string) (*ResolvedProject, error) {
	p, err := byName(name, projectsRoot, true)
	if err != nil {
		return nil, err
	}
	return loadProject(p)
}

// ResolveAny resolves directories without requiring configuration.
func ResolveAny(arg, projectsRoot string) (*ResolvedDir, error) {
	if arg != "" {
		return byName(arg, projectsRoot, false)
	}
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("could not get working directory: %w", err)
	}
	return projectDirectory(cwd, projectsRoot)
}

func inventory(root string, configured bool) ([]discovery.DiscoveredProject, error) {
	var projects []discovery.DiscoveredProject
	var err error
	if configured {
		projects, err = discovery.Scan(root)
	} else {
		projects, err = discovery.ScanAll(root)
	}
	if err != nil {
		return nil, fmt.Errorf("could not scan projects: %w", err)
	}
	return projects, nil
}

func byName(name, root string, configured bool) (*ResolvedDir, error) {
	parts := strings.Split(name, "/")
	if len(parts) > 2 {
		return nil, fmt.Errorf("invalid project name %q: expected repo or owner/repo", name)
	}
	for _, part := range parts {
		if err := config.ValidateName(part); err != nil {
			return nil, err
		}
	}
	projects, err := inventory(root, configured)
	if err != nil {
		return nil, err
	}
	return match(name, projects, configured)
}

func match(name string, projects []discovery.DiscoveredProject, configured bool) (*ResolvedDir, error) {
	parts := strings.Split(name, "/")
	var exact, fuzzy []discovery.DiscoveredProject
	for _, p := range projects {
		exactMatch := p.Repo == name
		fuzzyMatch := fuzzyMatchSegment(p.Repo, name)
		if !configured {
			exactMatch = strings.EqualFold(p.Repo, name)
		}
		if len(parts) == 2 {
			exactMatch = p.Name == name
			fuzzyMatch = fuzzyMatchSegment(p.Owner, parts[0]) && fuzzyMatchSegment(p.Repo, parts[1])
		}
		if exactMatch {
			exact = append(exact, p)
		}
		if fuzzyMatch {
			fuzzy = append(fuzzy, p)
		}
	}
	matches := exact
	if len(matches) == 0 {
		matches = fuzzy
	}
	switch len(matches) {
	case 0:
		return nil, fmt.Errorf("project %q not found. Run 'spin list' to see available projects", name)
	case 1:
		p := matches[0]
		return &ResolvedDir{Host: p.Host, Owner: p.Owner, Repo: p.Repo, Name: p.Name, Dir: p.Dir}, nil
	default:
		names := make([]string, 0, len(matches))
		for _, p := range matches {
			names = append(names, p.Host+"/"+p.Name)
		}
		return nil, fmt.Errorf("ambiguous project name or pattern %q matches multiple projects:\n  %s\nPlease specify as owner/repo when unique, or remove the cross-host collision", name, strings.Join(names, "\n  "))
	}
}

func fuzzyMatchSegment(value, pattern string) bool {
	return pattern != "" && strings.Contains(strings.ToLower(value), strings.ToLower(pattern))
}
