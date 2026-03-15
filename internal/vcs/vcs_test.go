package vcs

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestIsRepo_JJ(t *testing.T) {
	tmpDir := t.TempDir()
	jjDir := filepath.Join(tmpDir, ".jj")
	if err := os.MkdirAll(jjDir, 0755); err != nil {
		t.Fatal(err)
	}

	if !IsRepo(tmpDir) {
		t.Error("expected IsRepo=true for directory with .jj")
	}
	if !IsJJRepo(tmpDir) {
		t.Error("expected IsJJRepo=true for directory with .jj")
	}
	if IsGitRepo(tmpDir) {
		t.Error("expected IsGitRepo=false for directory with only .jj")
	}
}

func TestIsRepo_Git(t *testing.T) {
	tmpDir := t.TempDir()
	gitDir := filepath.Join(tmpDir, ".git")
	if err := os.MkdirAll(gitDir, 0755); err != nil {
		t.Fatal(err)
	}

	if !IsRepo(tmpDir) {
		t.Error("expected IsRepo=true for directory with .git")
	}
	if !IsGitRepo(tmpDir) {
		t.Error("expected IsGitRepo=true for directory with .git")
	}
	if IsJJRepo(tmpDir) {
		t.Error("expected IsJJRepo=false for directory with only .git")
	}
}

func TestIsRepo_Colocated(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmpDir, ".jj"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(tmpDir, ".git"), 0755); err != nil {
		t.Fatal(err)
	}

	if !IsRepo(tmpDir) {
		t.Error("expected IsRepo=true for colocated repo")
	}
	if !IsJJRepo(tmpDir) {
		t.Error("expected IsJJRepo=true for colocated repo")
	}
	if !IsGitRepo(tmpDir) {
		t.Error("expected IsGitRepo=true for colocated repo")
	}
}

func TestIsRepo_NoRepo(t *testing.T) {
	tmpDir := t.TempDir()

	if IsRepo(tmpDir) {
		t.Error("expected IsRepo=false for empty directory")
	}
}

func TestIsRepo_FileNotDir(t *testing.T) {
	tmpDir := t.TempDir()

	// Create .git as a file (not a directory) — should not count
	if err := os.WriteFile(filepath.Join(tmpDir, ".git"), []byte("gitdir: ../somewhere"), 0644); err != nil {
		t.Fatal(err)
	}

	if IsGitRepo(tmpDir) {
		t.Error("expected IsGitRepo=false when .git is a file, not a directory")
	}
}

func TestInit_WithJJ(t *testing.T) {
	if _, err := exec.LookPath("jj"); err != nil {
		t.Skip("jj not found in PATH, skipping integration test")
	}

	tmpDir := t.TempDir()
	if err := Init(tmpDir); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	if !IsJJRepo(tmpDir) {
		t.Error("expected .jj directory after Init")
	}
	// jj git init --colocate creates both .jj and .git
	if !IsGitRepo(tmpDir) {
		t.Error("expected .git directory after jj git init --colocate")
	}
}

func TestFetch_NotARepo(t *testing.T) {
	tmpDir := t.TempDir()

	err := Fetch(tmpDir)
	if err == nil {
		t.Fatal("expected error for Fetch on non-repo directory, got nil")
	}
}

func TestClone_NoBinary(t *testing.T) {
	// Test with empty PATH so neither jj nor git is found
	origPath := os.Getenv("PATH")
	t.Setenv("PATH", "")
	defer os.Setenv("PATH", origPath)

	tmpDir := t.TempDir()
	target := filepath.Join(tmpDir, "target")

	err := Clone("https://example.com/repo.git", target)
	if err == nil {
		t.Fatal("expected error when no VCS binary available, got nil")
	}
}

func TestInit_NoBinary(t *testing.T) {
	origPath := os.Getenv("PATH")
	t.Setenv("PATH", "")
	defer os.Setenv("PATH", origPath)

	tmpDir := t.TempDir()

	err := Init(tmpDir)
	if err == nil {
		t.Fatal("expected error when no VCS binary available, got nil")
	}
}
