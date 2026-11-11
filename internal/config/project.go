package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"

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

	if cfg.DBFilename == "" {
		cfg.DBFilename = "data.db"
	}
	if err := ValidateName(cfg.DBFilename); err != nil {
		return nil, fmt.Errorf("invalid db_filename: %w", err)
	}
	return cfg, nil
}

// ValidateName validates a single filesystem name, not a relative path.
func ValidateName(name string) error {
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, `/\\`) || strings.ContainsFunc(name, unicode.IsControl) {
		return fmt.Errorf("invalid name %q: expected one path component", name)
	}
	return nil
}

// ProjectPath rejects traversal and existing symlink components before mutation.
// A configured root may itself be a symlink; descendants must not be aliases.
func ProjectPath(root string, parts ...string) (string, error) {
	path, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	for _, part := range parts {
		if err := ValidateName(part); err != nil {
			return "", err
		}
		path = filepath.Join(path, part)
		info, err := os.Lstat(path)
		if err == nil && info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("symlink path component is unsupported: %s", path)
		}
		if err != nil && !os.IsNotExist(err) {
			return "", err
		}
	}
	return path, nil
}

// ResolveProfileDBPath returns the absolute path to the profile's DB file
// and creates parent directories if they don't exist.
func ResolveProfileDBPath(dataRoot, owner, repo, profile, dbFilename string) (string, error) {
	if profile != "dev" && profile != "live" {
		return "", fmt.Errorf("profile must be 'dev' or 'live', got %q", profile)
	}
	if dbFilename == "" {
		dbFilename = "data.db"
	}
	path, err := ProjectPath(filepath.Dir(dataRoot), filepath.Base(dataRoot), owner, repo, profile, dbFilename)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return "", fmt.Errorf("could not create data directory: %w", err)
	}
	return path, nil
}

// ResolveEnv replaces {profile_db_path} in all env values with the given path.
func ResolveEnv(env map[string]string, profileDBPath string) map[string]string {
	resolved := make(map[string]string, len(env))
	for k, v := range env {
		resolved[k] = strings.ReplaceAll(v, "{profile_db_path}", profileDBPath)
	}
	return resolved
}
