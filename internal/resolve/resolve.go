package resolve

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/attamusc/spin/internal/config"
	"github.com/attamusc/spin/internal/discovery"
)

// fuzzyMatchSegment returns true if value contains pattern (case-insensitive).
// Returns false for empty patterns to avoid matching everything.
func fuzzyMatchSegment(value, pattern string) bool {
	if pattern == "" {
		return false
	}
	return strings.Contains(strings.ToLower(value), strings.ToLower(pattern))
}

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

	// Fuzzy fallback: scan all .spin.toml projects and match segments
	parts := strings.SplitN(ownerRepo, "/", 2)
	ownerPattern, repoPattern := parts[0], parts[1]

	projects, err := discovery.Scan(projectsRoot)
	if err != nil {
		return nil, fmt.Errorf("could not scan projects: %w", err)
	}

	var matches []discovery.DiscoveredProject
	for _, p := range projects {
		if fuzzyMatchSegment(p.Owner, ownerPattern) && fuzzyMatchSegment(p.Repo, repoPattern) {
			matches = append(matches, p)
		}
	}

	switch len(matches) {
	case 1:
		return resolveFromProjectDir(matches[0].Dir, projectsRoot)
	case 0:
		return nil, fmt.Errorf("project %q not found. Run 'spin list' to see available projects", ownerRepo)
	default:
		var names []string
		for _, m := range matches {
			names = append(names, m.Name)
		}
		return nil, fmt.Errorf("ambiguous project pattern %q matches multiple projects:\n  %s\nPlease be more specific",
			ownerRepo, strings.Join(names, "\n  "))
	}
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
		// No exact match — try fuzzy (substring) match on repo name
		var fuzzyMatches []discovery.DiscoveredProject
		for _, p := range projects {
			if fuzzyMatchSegment(p.Repo, name) {
				fuzzyMatches = append(fuzzyMatches, p)
			}
		}
		switch len(fuzzyMatches) {
		case 1:
			return resolveFromProjectDir(fuzzyMatches[0].Dir, projectsRoot)
		case 0:
			return nil, fmt.Errorf("project %q not found. Run 'spin list' to see available projects", name)
		default:
			var names []string
			for _, m := range fuzzyMatches {
				names = append(names, m.Name)
			}
			return nil, fmt.Errorf("ambiguous project name %q matches multiple projects:\n  %s\nPlease specify as owner/repo",
				name, strings.Join(names, "\n  "))
		}

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

// ResolvedDir holds project directory info without requiring .spin.toml.
// Used by navigation commands (spin go) that work with any project directory.
type ResolvedDir struct {
	Host  string // e.g., "github.com"
	Owner string // e.g., "attamusc"
	Repo  string // e.g., "mindtrail"
	Name  string // "owner/repo" for display
	Dir   string // absolute path to project directory
}

// ResolveAny resolves a project directory by name without requiring .spin.toml.
// Falls back to defaultOwner for bare repo names when looking up owner/repo.
// If arg is empty, resolves from CWD.
func ResolveAny(arg, projectsRoot, defaultOwner string) (*ResolvedDir, error) {
	if arg == "" {
		return resolveAnyCWD(projectsRoot)
	}
	return resolveAnyByName(arg, projectsRoot, defaultOwner)
}

// resolveAnyCWD resolves a project directory from the current working directory
// without requiring .spin.toml. The CWD must be within a host/owner/repo
// directory under projectsRoot.
func resolveAnyCWD(projectsRoot string) (*ResolvedDir, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("could not get working directory: %w", err)
	}

	absDir, err := filepath.Abs(cwd)
	if err != nil {
		return nil, fmt.Errorf("could not resolve absolute path: %w", err)
	}
	absRoot, err := filepath.Abs(projectsRoot)
	if err != nil {
		return nil, fmt.Errorf("could not resolve absolute path: %w", err)
	}

	if !strings.HasPrefix(absDir, absRoot+string(filepath.Separator)) && absDir != absRoot {
		return nil, fmt.Errorf("current directory %s is not within projects_root %s", absDir, absRoot)
	}

	rel, err := filepath.Rel(absRoot, absDir)
	if err != nil {
		return nil, fmt.Errorf("could not compute relative path: %w", err)
	}

	parts := strings.Split(rel, string(filepath.Separator))
	if len(parts) < 3 {
		return nil, fmt.Errorf("current directory is not deep enough within projects_root to identify a project (need host/owner/repo)")
	}

	host := parts[0]
	owner := parts[1]
	repo := parts[2]
	dir := filepath.Join(absRoot, host, owner, repo)

	return &ResolvedDir{
		Host:  host,
		Owner: owner,
		Repo:  repo,
		Name:  owner + "/" + repo,
		Dir:   dir,
	}, nil
}

// resolveAnyByName resolves a project directory by name without requiring .spin.toml.
func resolveAnyByName(name, projectsRoot, defaultOwner string) (*ResolvedDir, error) {
	absRoot, err := filepath.Abs(projectsRoot)
	if err != nil {
		return nil, fmt.Errorf("could not resolve absolute path: %w", err)
	}

	// If name contains /, treat as owner/repo
	if strings.Contains(name, "/") {
		return resolveOwnerRepoAny(name, absRoot)
	}

	// Short name — scan all projects for a match
	return resolveShortNameAny(name, absRoot, defaultOwner)
}

// resolveOwnerRepoAny resolves "owner/repo" by checking directory existence
// (no .spin.toml required) under any host in projectsRoot.
func resolveOwnerRepoAny(ownerRepo, projectsRoot string) (*ResolvedDir, error) {
	hosts, err := os.ReadDir(projectsRoot)
	if err != nil {
		return nil, fmt.Errorf("could not read projects root: %w", err)
	}

	for _, host := range hosts {
		if !host.IsDir() || strings.HasPrefix(host.Name(), ".") {
			continue
		}
		dir := filepath.Join(projectsRoot, host.Name(), ownerRepo)
		info, err := os.Stat(dir)
		if err == nil && info.IsDir() {
			parts := strings.SplitN(ownerRepo, "/", 2)
			return &ResolvedDir{
				Host:  host.Name(),
				Owner: parts[0],
				Repo:  parts[1],
				Name:  ownerRepo,
				Dir:   dir,
			}, nil
		}
	}

	// Fuzzy fallback: scan all project directories and match segments
	parts := strings.SplitN(ownerRepo, "/", 2)
	ownerPattern, repoPattern := parts[0], parts[1]

	allProjects, err := discovery.ScanAll(projectsRoot)
	if err != nil {
		return nil, fmt.Errorf("could not scan projects: %w", err)
	}

	var matches []discovery.DiscoveredProject
	for _, p := range allProjects {
		if fuzzyMatchSegment(p.Owner, ownerPattern) && fuzzyMatchSegment(p.Repo, repoPattern) {
			matches = append(matches, p)
		}
	}

	switch len(matches) {
	case 1:
		return &ResolvedDir{
			Host:  matches[0].Host,
			Owner: matches[0].Owner,
			Repo:  matches[0].Repo,
			Name:  matches[0].Name,
			Dir:   matches[0].Dir,
		}, nil
	case 0:
		return nil, fmt.Errorf("project %q not found. Run 'spin list' to see available projects", ownerRepo)
	default:
		var names []string
		for _, m := range matches {
			names = append(names, m.Name)
		}
		return nil, fmt.Errorf("ambiguous project pattern %q matches multiple projects:\n  %s\nPlease be more specific",
			ownerRepo, strings.Join(names, "\n  "))
	}
}

// resolveShortNameAny resolves a short repo name by scanning all projects
// (not just .spin.toml projects). If no match is found and defaultOwner is set,
// returns an error indicating the project could be cloned.
func resolveShortNameAny(name, projectsRoot, defaultOwner string) (*ResolvedDir, error) {
	projects, err := discovery.ScanAll(projectsRoot)
	if err != nil {
		return nil, fmt.Errorf("could not scan projects: %w", err)
	}

	var matches []discovery.DiscoveredProject
	for _, p := range projects {
		if strings.EqualFold(p.Repo, name) {
			matches = append(matches, p)
		}
	}

	switch len(matches) {
	case 0:
		// No exact match — try fuzzy (substring) match on repo name
		var fuzzyMatches []discovery.DiscoveredProject
		for _, p := range projects {
			if fuzzyMatchSegment(p.Repo, name) {
				fuzzyMatches = append(fuzzyMatches, p)
			}
		}
		switch len(fuzzyMatches) {
		case 1:
			return &ResolvedDir{
				Host:  fuzzyMatches[0].Host,
				Owner: fuzzyMatches[0].Owner,
				Repo:  fuzzyMatches[0].Repo,
				Name:  fuzzyMatches[0].Name,
				Dir:   fuzzyMatches[0].Dir,
			}, nil
		case 0:
			return nil, fmt.Errorf("project %q not found. Run 'spin list' to see available projects", name)
		default:
			var names []string
			for _, m := range fuzzyMatches {
				names = append(names, m.Name)
			}
			return nil, fmt.Errorf("ambiguous project name %q matches multiple projects:\n  %s\nPlease specify as owner/repo",
				name, strings.Join(names, "\n  "))
		}

	case 1:
		return &ResolvedDir{
			Host:  matches[0].Host,
			Owner: matches[0].Owner,
			Repo:  matches[0].Repo,
			Name:  matches[0].Name,
			Dir:   matches[0].Dir,
		}, nil

	default:
		var names []string
		for _, m := range matches {
			names = append(names, m.Name)
		}
		return nil, fmt.Errorf("ambiguous project name %q matches multiple projects:\n  %s\nPlease specify as owner/repo",
			name, strings.Join(names, "\n  "))
	}
}
