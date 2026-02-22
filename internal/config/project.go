package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

// ProjectConfig represents the .spin.toml configuration file.
type ProjectConfig struct {
	DBFilename string        `toml:"db_filename"`
	Dev        ProfileConfig `toml:"dev"`
	Live       ProfileConfig `toml:"live"`
}

// ProfileConfig represents a single profile (dev or live) configuration.
type ProfileConfig struct {
	Command string            `toml:"command"`
	Port    int               `toml:"port,omitempty"`
	Env     map[string]string `toml:"env"`
	PreHook string            `toml:"pre_hook,omitempty"`
}

// LoadProject parses .spin.toml from the given directory.
func LoadProject(dir string) (*ProjectConfig, error) {
	path := filepath.Join(dir, ".spin.toml")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("could not read project config: %w", err)
	}

	cfg := &ProjectConfig{}
	if err := toml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("could not parse %s: %w", path, err)
	}

	// Validate required fields
	if cfg.Dev.Command == "" {
		return nil, fmt.Errorf("missing required field: [dev].command in %s", path)
	}
	if cfg.Live.Command == "" {
		return nil, fmt.Errorf("missing required field: [live].command in %s", path)
	}

	return cfg, nil
}

// ResolveProfileDBPath returns the absolute path to the profile's DB file
// and creates parent directories if they don't exist.
func ResolveProfileDBPath(dataRoot, owner, repo, profile, dbFilename string) (string, error) {
	dir := filepath.Join(dataRoot, owner, repo, profile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("could not create data directory %s: %w", dir, err)
	}
	return filepath.Join(dir, dbFilename), nil
}

// ResolveEnv replaces {profile_db_path} in all env values with the given path.
func ResolveEnv(env map[string]string, profileDBPath string) map[string]string {
	resolved := make(map[string]string, len(env))
	for k, v := range env {
		resolved[k] = strings.ReplaceAll(v, "{profile_db_path}", profileDBPath)
	}
	return resolved
}
