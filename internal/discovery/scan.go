package discovery

import (
	"os"
	"path/filepath"
	"strings"
)

// DiscoveredProject represents a project found during scanning.
type DiscoveredProject struct {
	Host      string // e.g., "github.com"
	Owner     string // e.g., "attamusc"
	Repo      string // e.g., "mindtrail"
	Name      string // "owner/repo"
	Dir       string // absolute path
	HasConfig bool   // true if .spin.toml exists in the project directory
}

// hiddenDirs are directories to skip during scanning.
var hiddenDirs = map[string]bool{
	".git":         true,
	".jj":          true,
	".spin":        true,
	".hg":          true,
	".svn":         true,
	"node_modules": true,
	".next":        true,
	".nuxt":        true,
	"dist":         true,
	"build":        true,
	".turbo":       true,
}

// ScanAll finds all directories at depth 3 (host/owner/repo) under projectsRoot,
// regardless of whether they have a .spin.toml. It uses direct directory reads
// at each level rather than a recursive walk for efficiency.
func ScanAll(projectsRoot string) ([]DiscoveredProject, error) {
	absRoot, err := filepath.Abs(projectsRoot)
	if err != nil {
		return nil, err
	}

	var projects []DiscoveredProject

	// Level 1: hosts (e.g., github.com)
	hosts, err := os.ReadDir(absRoot)
	if err != nil {
		return nil, err
	}

	for _, host := range hosts {
		if !host.IsDir() || strings.HasPrefix(host.Name(), ".") || hiddenDirs[host.Name()] {
			continue
		}

		hostPath := filepath.Join(absRoot, host.Name())

		// Level 2: owners (e.g., attamusc)
		owners, err := os.ReadDir(hostPath)
		if err != nil {
			continue // skip unreadable host dirs
		}

		for _, owner := range owners {
			if !owner.IsDir() || strings.HasPrefix(owner.Name(), ".") || hiddenDirs[owner.Name()] {
				continue
			}

			ownerPath := filepath.Join(hostPath, owner.Name())

			// Level 3: repos (e.g., mindtrail)
			repos, err := os.ReadDir(ownerPath)
			if err != nil {
				continue // skip unreadable owner dirs
			}

			for _, repo := range repos {
				if !repo.IsDir() || strings.HasPrefix(repo.Name(), ".") || hiddenDirs[repo.Name()] {
					continue
				}

				repoPath := filepath.Join(ownerPath, repo.Name())
				spinToml := filepath.Join(repoPath, ".spin.toml")
				_, statErr := os.Stat(spinToml)
				hasConfig := statErr == nil

				projects = append(projects, DiscoveredProject{
					Host:      host.Name(),
					Owner:     owner.Name(),
					Repo:      repo.Name(),
					Name:      owner.Name() + "/" + repo.Name(),
					Dir:       repoPath,
					HasConfig: hasConfig,
				})
			}
		}
	}

	return projects, nil
}

// Scan scans projectsRoot for directories containing .spin.toml.
// It expects the directory structure: {projectsRoot}/{host}/{owner}/{repo}/.spin.toml
// This is equivalent to ScanAll filtered to only projects with HasConfig == true.
func Scan(projectsRoot string) ([]DiscoveredProject, error) {
	all, err := ScanAll(projectsRoot)
	if err != nil {
		return nil, err
	}

	var configured []DiscoveredProject
	for _, p := range all {
		if p.HasConfig {
			configured = append(configured, p)
		}
	}

	return configured, nil
}
