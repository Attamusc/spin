package discovery

import (
	"os"
	"path/filepath"
	"testing"
)

func createDiscoverableProject(t *testing.T, base, host, owner, repo string) {
	t.Helper()
	dir := filepath.Join(base, host, owner, repo)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	content := `
db_filename = "data.db"

[dev]
command = "npm run dev"

[live]
command = "npm run dev"
`
	if err := os.WriteFile(filepath.Join(dir, ".spin.toml"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestScan_FindsProjects(t *testing.T) {
	tmpDir := t.TempDir()

	createDiscoverableProject(t, tmpDir, "github.com", "attamusc", "mindtrail")
	createDiscoverableProject(t, tmpDir, "github.com", "attamusc", "otherthing")
	createDiscoverableProject(t, tmpDir, "gitlab.com", "someone", "project")

	projects, err := Scan(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(projects) != 3 {
		t.Fatalf("expected 3 projects, got %d", len(projects))
	}

	// Check that we found the expected projects
	names := make(map[string]bool)
	for _, p := range projects {
		names[p.Name] = true
	}

	expected := []string{"attamusc/mindtrail", "attamusc/otherthing", "someone/project"}
	for _, name := range expected {
		if !names[name] {
			t.Errorf("expected project %q not found", name)
		}
	}
}

func TestScan_SkipsHiddenDirs(t *testing.T) {
	tmpDir := t.TempDir()

	createDiscoverableProject(t, tmpDir, "github.com", "attamusc", "mindtrail")

	// Create a .spin.toml in a hidden directory (should be skipped)
	hiddenDir := filepath.Join(tmpDir, ".git", "attamusc", "hidden")
	os.MkdirAll(hiddenDir, 0755)
	os.WriteFile(filepath.Join(hiddenDir, ".spin.toml"), []byte(`
[dev]
command = "test"
[live]
command = "test"
`), 0644)

	// Create a .spin.toml in node_modules (should be skipped)
	nmDir := filepath.Join(tmpDir, "github.com", "attamusc", "mindtrail", "node_modules", "dep")
	os.MkdirAll(nmDir, 0755)
	os.WriteFile(filepath.Join(nmDir, ".spin.toml"), []byte(`
[dev]
command = "test"
[live]
command = "test"
`), 0644)

	projects, err := Scan(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(projects) != 1 {
		t.Fatalf("expected 1 project, got %d", len(projects))
	}

	if projects[0].Name != "attamusc/mindtrail" {
		t.Errorf("expected attamusc/mindtrail, got %q", projects[0].Name)
	}
}

func TestScan_EmptyRoot(t *testing.T) {
	tmpDir := t.TempDir()

	projects, err := Scan(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(projects) != 0 {
		t.Errorf("expected 0 projects, got %d", len(projects))
	}
}

func TestScan_ProjectDetails(t *testing.T) {
	tmpDir := t.TempDir()

	createDiscoverableProject(t, tmpDir, "github.com", "attamusc", "mindtrail")

	projects, err := Scan(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(projects) != 1 {
		t.Fatalf("expected 1 project, got %d", len(projects))
	}

	p := projects[0]
	if p.Host != "github.com" {
		t.Errorf("expected Host=github.com, got %q", p.Host)
	}
	if p.Owner != "attamusc" {
		t.Errorf("expected Owner=attamusc, got %q", p.Owner)
	}
	if p.Repo != "mindtrail" {
		t.Errorf("expected Repo=mindtrail, got %q", p.Repo)
	}
	if p.Name != "attamusc/mindtrail" {
		t.Errorf("expected Name=attamusc/mindtrail, got %q", p.Name)
	}

	expectedDir := filepath.Join(tmpDir, "github.com", "attamusc", "mindtrail")
	if p.Dir != expectedDir {
		t.Errorf("expected Dir=%q, got %q", expectedDir, p.Dir)
	}
}
