package resolve

import (
	"os"
	"path/filepath"
	"strings"
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

// --- CWD-based resolution (FromDir) ---

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

// --- Full name resolution (FromName / Resolve) ---

func TestResolve_EmptyArg_FallsBackToCWD(t *testing.T) {
	// With empty arg, Resolve calls FromCWD which depends on os.Getwd().
	// We can't easily control CWD in tests, so just verify it returns an
	// error (since our CWD is not within a temp projects root).
	tmpDir := t.TempDir()

	_, err := Resolve("", tmpDir)
	// Should error because real CWD is not within tmpDir
	if err == nil {
		t.Fatal("expected error when CWD is outside projects_root, got nil")
	}
}

func TestFromName_OwnerRepo_SingleHost(t *testing.T) {
	tmpDir := t.TempDir()
	createProject(t, tmpDir, "github.com", "attamusc", "mindtrail")

	rp, err := FromName("attamusc/mindtrail", tmpDir)
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
}

func TestFromName_OwnerRepo_MultipleHosts(t *testing.T) {
	tmpDir := t.TempDir()
	// Same owner/repo under two hosts — only one has .spin.toml
	createProject(t, tmpDir, "github.com", "attamusc", "coolapp")
	// Create dir under gitlab but WITHOUT .spin.toml
	gitlabDir := filepath.Join(tmpDir, "gitlab.com", "attamusc", "coolapp")
	if err := os.MkdirAll(gitlabDir, 0755); err != nil {
		t.Fatal(err)
	}

	rp, err := FromName("attamusc/coolapp", tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rp.Host != "github.com" {
		t.Errorf("expected Host=github.com, got %q", rp.Host)
	}
}

func TestFromName_OwnerRepo_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	createProject(t, tmpDir, "github.com", "attamusc", "mindtrail")

	_, err := FromName("someone/nonexistent", tmpDir)
	if err == nil {
		t.Fatal("expected error for non-existent owner/repo, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' in error, got: %v", err)
	}
}

func TestFromName_ShortName_UniqueMatch(t *testing.T) {
	tmpDir := t.TempDir()
	createProject(t, tmpDir, "github.com", "attamusc", "mindtrail")

	rp, err := FromName("mindtrail", tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rp.Owner != "attamusc" {
		t.Errorf("expected Owner=attamusc, got %q", rp.Owner)
	}
	if rp.Repo != "mindtrail" {
		t.Errorf("expected Repo=mindtrail, got %q", rp.Repo)
	}
}

func TestFromName_ShortName_AmbiguousMatch(t *testing.T) {
	tmpDir := t.TempDir()
	// Two different owners, same repo name
	createProject(t, tmpDir, "github.com", "attamusc", "webapp")
	createProject(t, tmpDir, "github.com", "otheruser", "webapp")

	_, err := FromName("webapp", tmpDir)
	if err == nil {
		t.Fatal("expected error for ambiguous short name, got nil")
	}
	if !strings.Contains(err.Error(), "ambiguous") {
		t.Errorf("expected 'ambiguous' in error, got: %v", err)
	}
	// Should list both matches
	if !strings.Contains(err.Error(), "attamusc/webapp") {
		t.Errorf("expected attamusc/webapp in suggestions, got: %v", err)
	}
	if !strings.Contains(err.Error(), "otheruser/webapp") {
		t.Errorf("expected otheruser/webapp in suggestions, got: %v", err)
	}
}

func TestFromName_ShortName_NoMatch_WithSuggestions(t *testing.T) {
	tmpDir := t.TempDir()
	createProject(t, tmpDir, "github.com", "attamusc", "mindtrail")

	_, err := FromName("mind", tmpDir)
	if err == nil {
		t.Fatal("expected error for no exact match, got nil")
	}
	// "mind" is a substring of "mindtrail", so we should get a suggestion
	if !strings.Contains(err.Error(), "mindtrail") || !strings.Contains(err.Error(), "Did you mean") {
		t.Errorf("expected suggestion containing 'mindtrail', got: %v", err)
	}
}

func TestFromName_ShortName_NoMatch_NoSuggestions(t *testing.T) {
	tmpDir := t.TempDir()
	createProject(t, tmpDir, "github.com", "attamusc", "mindtrail")

	_, err := FromName("zzzzz", tmpDir)
	if err == nil {
		t.Fatal("expected error for no match, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' in error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "spin list") {
		t.Errorf("expected 'spin list' hint in error, got: %v", err)
	}
}

func TestResolve_WithArg_OwnerRepo(t *testing.T) {
	tmpDir := t.TempDir()
	createProject(t, tmpDir, "github.com", "attamusc", "myapp")

	rp, err := Resolve("attamusc/myapp", tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rp.Repo != "myapp" {
		t.Errorf("expected Repo=myapp, got %q", rp.Repo)
	}
}

func TestResolve_WithArg_ShortName(t *testing.T) {
	tmpDir := t.TempDir()
	createProject(t, tmpDir, "github.com", "attamusc", "myapp")

	rp, err := Resolve("myapp", tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rp.Repo != "myapp" {
		t.Errorf("expected Repo=myapp, got %q", rp.Repo)
	}
}
