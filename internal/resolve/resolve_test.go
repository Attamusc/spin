package resolve

import (
	"os"
	"path/filepath"
	"testing"
)

// helper to create a project directory with a .spin.toml
func createProject(t *testing.T, base, host, owner, repo string) string {
	t.Helper()
	dir := filepath.Join(base, host, owner, repo)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}

	content := `
db_filename = "data.db"

[dev]
command = "npm run dev"
port = 3001

[live]
command = "npm run dev"
port = 3000
`
	if err := os.WriteFile(filepath.Join(dir, ".spin.toml"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestFromDir_ProjectRoot(t *testing.T) {
	tmpDir := t.TempDir()
	projectDir := createProject(t, tmpDir, "github.com", "attamusc", "mindtrail")

	rp, err := FromDir(projectDir, tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rp.Host != "github.com" {
		t.Errorf("expected Host=github.com, got %q", rp.Host)
	}
	if rp.Owner != "attamusc" {
		t.Errorf("expected Owner=attamusc, got %q", rp.Owner)
	}
	if rp.Repo != "mindtrail" {
		t.Errorf("expected Repo=mindtrail, got %q", rp.Repo)
	}
	if rp.Name != "attamusc/mindtrail" {
		t.Errorf("expected Name=attamusc/mindtrail, got %q", rp.Name)
	}
	if rp.Dir != projectDir {
		t.Errorf("expected Dir=%q, got %q", projectDir, rp.Dir)
	}
}

func TestFromDir_Subdirectory(t *testing.T) {
	tmpDir := t.TempDir()
	projectDir := createProject(t, tmpDir, "github.com", "attamusc", "mindtrail")

	// Create a subdirectory
	subDir := filepath.Join(projectDir, "src", "components")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatal(err)
	}

	rp, err := FromDir(subDir, tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rp.Repo != "mindtrail" {
		t.Errorf("expected Repo=mindtrail, got %q", rp.Repo)
	}
	if rp.Dir != projectDir {
		t.Errorf("expected Dir=%q, got %q", projectDir, rp.Dir)
	}
}

func TestFromDir_OutsideProjectsRoot(t *testing.T) {
	tmpDir := t.TempDir()
	outsideDir := t.TempDir()

	_, err := FromDir(outsideDir, tmpDir)
	if err == nil {
		t.Fatal("expected error for directory outside projects_root, got nil")
	}
}

func TestFromDir_NoSpinToml(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a directory structure without .spin.toml
	dir := filepath.Join(tmpDir, "github.com", "attamusc", "noproj")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}

	_, err := FromDir(dir, tmpDir)
	if err == nil {
		t.Fatal("expected error for missing .spin.toml, got nil")
	}
}
