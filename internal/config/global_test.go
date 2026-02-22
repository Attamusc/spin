package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadGlobal_DefaultWhenNoConfigFile(t *testing.T) {
	// Use a non-existent config file path
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "nonexistent", "config.toml")

	// Create a projects directory at the default location to satisfy validation
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	defaultProjectsRoot := filepath.Join(home, "projects")

	// Check if default projects root exists; skip if not
	if _, err := os.Stat(defaultProjectsRoot); os.IsNotExist(err) {
		t.Skip("default ~/projects directory does not exist")
	}

	cfg, err := LoadGlobalFrom(configPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.ProjectsRoot != defaultProjectsRoot {
		t.Errorf("expected ProjectsRoot=%q, got %q", defaultProjectsRoot, cfg.ProjectsRoot)
	}
	if cfg.DataRoot != filepath.Join(defaultProjectsRoot, ".spin") {
		t.Errorf("expected DataRoot=%q, got %q", filepath.Join(defaultProjectsRoot, ".spin"), cfg.DataRoot)
	}
}

func TestLoadGlobal_ValidConfigFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a projects root directory
	projectsRoot := filepath.Join(tmpDir, "my-projects")
	if err := os.MkdirAll(projectsRoot, 0755); err != nil {
		t.Fatal(err)
	}

	// Write config file
	configPath := filepath.Join(tmpDir, "config.toml")
	content := `projects_root = "` + projectsRoot + `"` + "\n"
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadGlobalFrom(configPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.ProjectsRoot != projectsRoot {
		t.Errorf("expected ProjectsRoot=%q, got %q", projectsRoot, cfg.ProjectsRoot)
	}
	if cfg.DataRoot != filepath.Join(projectsRoot, ".spin") {
		t.Errorf("expected DataRoot=%q, got %q", filepath.Join(projectsRoot, ".spin"), cfg.DataRoot)
	}
}

func TestLoadGlobal_TildeExpansion(t *testing.T) {
	tmpDir := t.TempDir()

	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}

	// Create projects dir in home
	projectsRoot := filepath.Join(home, "projects")
	if _, err := os.Stat(projectsRoot); os.IsNotExist(err) {
		t.Skip("~/projects does not exist")
	}

	// Write config file with ~ path
	configPath := filepath.Join(tmpDir, "config.toml")
	content := "projects_root = \"~/projects\"\n"
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadGlobalFrom(configPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := filepath.Join(home, "projects")
	if cfg.ProjectsRoot != expected {
		t.Errorf("expected ProjectsRoot=%q, got %q", expected, cfg.ProjectsRoot)
	}
}

func TestLoadGlobal_MissingProjectsRoot(t *testing.T) {
	tmpDir := t.TempDir()

	configPath := filepath.Join(tmpDir, "config.toml")
	content := `projects_root = "` + filepath.Join(tmpDir, "nonexistent") + `"` + "\n"
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadGlobalFrom(configPath)
	if err == nil {
		t.Fatal("expected error for missing projects_root, got nil")
	}
}

func TestExpandHome(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		input    string
		expected string
	}{
		{"~/foo", filepath.Join(home, "foo")},
		{"~", home},
		{"/absolute/path", "/absolute/path"},
		{"relative/path", "relative/path"},
		{"", ""},
	}

	for _, tt := range tests {
		result, err := expandHome(tt.input)
		if err != nil {
			t.Errorf("expandHome(%q): unexpected error: %v", tt.input, err)
			continue
		}
		if result != tt.expected {
			t.Errorf("expandHome(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}
