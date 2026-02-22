package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadProject_Valid(t *testing.T) {
	dir := t.TempDir()

	content := `
db_filename = "data.db"

[dev]
command = "npm run dev"
port = 3001
env = { DATABASE_URL = "file:{profile_db_path}", PORT = "3001" }

[live]
command = "npm run dev"
port = 3000
env = { DATABASE_URL = "file:{profile_db_path}", PORT = "3000" }
`
	if err := os.WriteFile(filepath.Join(dir, ".spin.toml"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadProject(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.DBFilename != "data.db" {
		t.Errorf("expected DBFilename=data.db, got %q", cfg.DBFilename)
	}
	if cfg.Dev.Command != "npm run dev" {
		t.Errorf("expected Dev.Command='npm run dev', got %q", cfg.Dev.Command)
	}
	if cfg.Dev.Port != 3001 {
		t.Errorf("expected Dev.Port=3001, got %d", cfg.Dev.Port)
	}
	if cfg.Live.Command != "npm run dev" {
		t.Errorf("expected Live.Command='npm run dev', got %q", cfg.Live.Command)
	}
	if cfg.Live.Port != 3000 {
		t.Errorf("expected Live.Port=3000, got %d", cfg.Live.Port)
	}
	if cfg.Dev.Env["DATABASE_URL"] != "file:{profile_db_path}" {
		t.Errorf("expected DATABASE_URL template, got %q", cfg.Dev.Env["DATABASE_URL"])
	}
}

func TestLoadProject_MissingDevCommand(t *testing.T) {
	dir := t.TempDir()

	content := `
db_filename = "data.db"

[dev]
port = 3001

[live]
command = "npm run dev"
`
	if err := os.WriteFile(filepath.Join(dir, ".spin.toml"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadProject(dir)
	if err == nil {
		t.Fatal("expected error for missing dev command, got nil")
	}
}

func TestLoadProject_MissingLiveCommand(t *testing.T) {
	dir := t.TempDir()

	content := `
db_filename = "data.db"

[dev]
command = "npm run dev"

[live]
port = 3000
`
	if err := os.WriteFile(filepath.Join(dir, ".spin.toml"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadProject(dir)
	if err == nil {
		t.Fatal("expected error for missing live command, got nil")
	}
}

func TestLoadProject_PreHook(t *testing.T) {
	dir := t.TempDir()

	content := `
db_filename = "data.db"

[dev]
command = "npm run dev"
pre_hook = "echo setup"

[live]
command = "npm run dev"
pre_hook = "echo live-setup"
`
	if err := os.WriteFile(filepath.Join(dir, ".spin.toml"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadProject(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Dev.PreHook != "echo setup" {
		t.Errorf("expected Dev.PreHook='echo setup', got %q", cfg.Dev.PreHook)
	}
	if cfg.Live.PreHook != "echo live-setup" {
		t.Errorf("expected Live.PreHook='echo live-setup', got %q", cfg.Live.PreHook)
	}
}

func TestResolveProfileDBPath(t *testing.T) {
	tmpDir := t.TempDir()
	dataRoot := filepath.Join(tmpDir, "data")

	path, err := ResolveProfileDBPath(dataRoot, "attamusc", "mindtrail", "dev", "data.db")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := filepath.Join(dataRoot, "attamusc", "mindtrail", "dev", "data.db")
	if path != expected {
		t.Errorf("expected path=%q, got %q", expected, path)
	}

	// Verify directory was created
	dir := filepath.Dir(path)
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("directory not created: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("expected a directory")
	}
}

func TestResolveEnv(t *testing.T) {
	env := map[string]string{
		"DATABASE_URL": "file:{profile_db_path}",
		"PORT":         "3001",
		"OTHER":        "no-template",
	}

	resolved := ResolveEnv(env, "/data/attamusc/mindtrail/dev/data.db")

	if resolved["DATABASE_URL"] != "file:/data/attamusc/mindtrail/dev/data.db" {
		t.Errorf("expected resolved DATABASE_URL, got %q", resolved["DATABASE_URL"])
	}
	if resolved["PORT"] != "3001" {
		t.Errorf("expected PORT=3001, got %q", resolved["PORT"])
	}
	if resolved["OTHER"] != "no-template" {
		t.Errorf("expected OTHER unchanged, got %q", resolved["OTHER"])
	}
}

func TestLoadProject_MissingFile(t *testing.T) {
	dir := t.TempDir()

	_, err := LoadProject(dir)
	if err == nil {
		t.Fatal("expected error for missing .spin.toml, got nil")
	}
}
