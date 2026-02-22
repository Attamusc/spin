package discovery

import (
	"os"
	"path/filepath"
	"strings"
)

// DiscoveredProject represents a project found during scanning.
type DiscoveredProject struct {
	Host  string // e.g., "github.com"
	Owner string // e.g., "attamusc"
	Repo  string // e.g., "mindtrail"
	Name  string // "owner/repo"
	Dir   string // absolute path
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

// Scan recursively scans projectsRoot for directories containing .spin.toml.
// It expects the directory structure: {projectsRoot}/{host}/{owner}/{repo}/.spin.toml
func Scan(projectsRoot string) ([]DiscoveredProject, error) {
	var projects []DiscoveredProject

	absRoot, err := filepath.Abs(projectsRoot)
	if err != nil {
		return nil, err
	}

	err = filepath.WalkDir(absRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}

		// Skip hidden and excluded directories
		if d.IsDir() {
			name := d.Name()
			if strings.HasPrefix(name, ".") || hiddenDirs[name] {
				return filepath.SkipDir
			}
			return nil
		}

		// Look for .spin.toml files
		if d.Name() != ".spin.toml" {
			return nil
		}

		projectDir := filepath.Dir(path)
		rel, err := filepath.Rel(absRoot, projectDir)
		if err != nil {
			return nil
		}

		parts := strings.Split(rel, string(filepath.Separator))
		if len(parts) < 3 {
			return nil // Not deep enough for host/owner/repo
		}

		host := parts[0]
		owner := parts[1]
		repo := parts[2]

		projects = append(projects, DiscoveredProject{
			Host:  host,
			Owner: owner,
			Repo:  repo,
			Name:  owner + "/" + repo,
			Dir:   projectDir,
		})

		return nil
	})

	return projects, err
}
