package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// GlobalConfig holds the global spin configuration.
type GlobalConfig struct {
	ProjectsRoot string            `toml:"projects_root"`
	DefaultHost  string            `toml:"default_host"`  // default git host for cloning/navigation
	DefaultOwner string            `toml:"default_owner"` // default owner for short-name resolution
	Templates    map[string]string `toml:"templates"`     // named template directories for spin new
	DataRoot     string            // computed: {ProjectsRoot}/.spin/
}

// globalConfigPath returns the path to the global config file.
func globalConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("could not determine home directory: %w", err)
	}
	return filepath.Join(home, ".config", "spin", "config.toml"), nil
}

// expandHome expands a leading ~ in a path to the user's home directory.
func expandHome(path string) (string, error) {
	if len(path) == 0 {
		return path, nil
	}
	if path[0] != '~' {
		return path, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("could not determine home directory: %w", err)
	}
	if len(path) == 1 {
		return home, nil
	}
	if path[1] == '/' {
		return filepath.Join(home, path[2:]), nil
	}
	return path, nil
}

// LoadGlobal loads the global config from ~/.config/spin/config.toml.
// If the file doesn't exist, defaults are used (ProjectsRoot = ~/projects).
func LoadGlobal() (*GlobalConfig, error) {
	return LoadGlobalFrom("")
}

// LoadGlobalFrom loads the global config from the specified path.
// If configPath is empty, the default path is used.
// If the file doesn't exist, defaults are used.
func LoadGlobalFrom(configPath string) (*GlobalConfig, error) {
	cfg := &GlobalConfig{}

	if configPath == "" {
		var err error
		configPath, err = globalConfigPath()
		if err != nil {
			return nil, err
		}
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Use defaults
			home, err := os.UserHomeDir()
			if err != nil {
				return nil, fmt.Errorf("could not determine home directory: %w", err)
			}
			cfg.ProjectsRoot = filepath.Join(home, "projects")
		} else {
			return nil, fmt.Errorf("could not read config file %s: %w", configPath, err)
		}
	} else {
		if err := toml.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("could not parse config file %s: %w", configPath, err)
		}
	}

	// Expand ~ in ProjectsRoot
	expanded, err := expandHome(cfg.ProjectsRoot)
	if err != nil {
		return nil, err
	}
	cfg.ProjectsRoot = expanded

	// Apply defaults for new fields
	if cfg.DefaultHost == "" {
		cfg.DefaultHost = "github.com"
	}
	// DefaultOwner defaults to "" (no short-name-to-clone fallback)

	// Expand ~ in template paths
	for name, tmplPath := range cfg.Templates {
		expanded, err := expandHome(tmplPath)
		if err != nil {
			return nil, fmt.Errorf("could not expand template path %q: %w", tmplPath, err)
		}
		cfg.Templates[name] = expanded
	}

	// Verify ProjectsRoot exists
	info, err := os.Stat(cfg.ProjectsRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("projects_root directory does not exist: %s", cfg.ProjectsRoot)
		}
		return nil, fmt.Errorf("could not stat projects_root: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("projects_root is not a directory: %s", cfg.ProjectsRoot)
	}

	// Compute DataRoot
	cfg.DataRoot = filepath.Join(cfg.ProjectsRoot, ".spin")

	return cfg, nil
}
