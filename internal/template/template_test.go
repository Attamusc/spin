package template

import (
	"os"
	"path/filepath"
	"testing"
)

func TestApply_CopiesFiles(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()

	// Create source structure
	os.WriteFile(filepath.Join(src, "README.md"), []byte("# Hello"), 0644)
	os.MkdirAll(filepath.Join(src, "src"), 0755)
	os.WriteFile(filepath.Join(src, "src", "main.go"), []byte("package main"), 0644)

	if err := Apply(src, dst); err != nil {
		t.Fatalf("Apply() error: %v", err)
	}

	// Verify files were copied
	content, err := os.ReadFile(filepath.Join(dst, "README.md"))
	if err != nil {
		t.Fatalf("failed to read README.md: %v", err)
	}
	if string(content) != "# Hello" {
		t.Errorf("README.md content = %q, want %q", string(content), "# Hello")
	}

	content, err = os.ReadFile(filepath.Join(dst, "src", "main.go"))
	if err != nil {
		t.Fatalf("failed to read src/main.go: %v", err)
	}
	if string(content) != "package main" {
		t.Errorf("src/main.go content = %q, want %q", string(content), "package main")
	}
}

func TestApply_SkipsGitDir(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()

	// Create source with .git directory
	os.MkdirAll(filepath.Join(src, ".git", "objects"), 0755)
	os.WriteFile(filepath.Join(src, ".git", "HEAD"), []byte("ref: refs/heads/main"), 0644)
	os.WriteFile(filepath.Join(src, "file.txt"), []byte("hello"), 0644)

	if err := Apply(src, dst); err != nil {
		t.Fatalf("Apply() error: %v", err)
	}

	// .git should NOT be copied
	if _, err := os.Stat(filepath.Join(dst, ".git")); !os.IsNotExist(err) {
		t.Error("expected .git to be skipped, but it exists in destination")
	}

	// file.txt should be copied
	if _, err := os.Stat(filepath.Join(dst, "file.txt")); err != nil {
		t.Error("expected file.txt to be copied")
	}
}

func TestApply_SkipsJJDir(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()

	// Create source with .jj directory
	os.MkdirAll(filepath.Join(src, ".jj", "store"), 0755)
	os.WriteFile(filepath.Join(src, ".jj", "repo"), []byte("data"), 0644)
	os.WriteFile(filepath.Join(src, "file.txt"), []byte("hello"), 0644)

	if err := Apply(src, dst); err != nil {
		t.Fatalf("Apply() error: %v", err)
	}

	// .jj should NOT be copied
	if _, err := os.Stat(filepath.Join(dst, ".jj")); !os.IsNotExist(err) {
		t.Error("expected .jj to be skipped, but it exists in destination")
	}

	// file.txt should be copied
	if _, err := os.Stat(filepath.Join(dst, "file.txt")); err != nil {
		t.Error("expected file.txt to be copied")
	}
}

func TestApply_EmptySourceDir(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()

	if err := Apply(src, dst); err != nil {
		t.Fatalf("Apply() error: %v", err)
	}

	// dst should still exist and be empty (except for . and ..)
	entries, err := os.ReadDir(dst)
	if err != nil {
		t.Fatalf("failed to read dst: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected empty dst, got %d entries", len(entries))
	}
}

func TestApply_SourceNotDir(t *testing.T) {
	tmp := t.TempDir()
	file := filepath.Join(tmp, "file.txt")
	os.WriteFile(file, []byte("hello"), 0644)

	err := Apply(file, tmp)
	if err == nil {
		t.Fatal("expected error when source is not a directory")
	}
}

func TestApply_SourceNotExist(t *testing.T) {
	dst := t.TempDir()

	err := Apply("/nonexistent/path", dst)
	if err == nil {
		t.Fatal("expected error when source does not exist")
	}
}

func TestApply_PreservesNestedStructure(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()

	// Create deeply nested structure
	deep := filepath.Join(src, "a", "b", "c")
	os.MkdirAll(deep, 0755)
	os.WriteFile(filepath.Join(deep, "deep.txt"), []byte("deep"), 0644)
	os.WriteFile(filepath.Join(src, "a", "top.txt"), []byte("top"), 0644)

	if err := Apply(src, dst); err != nil {
		t.Fatalf("Apply() error: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(dst, "a", "b", "c", "deep.txt"))
	if err != nil {
		t.Fatalf("failed to read deep file: %v", err)
	}
	if string(content) != "deep" {
		t.Errorf("deep.txt content = %q, want %q", string(content), "deep")
	}

	content, err = os.ReadFile(filepath.Join(dst, "a", "top.txt"))
	if err != nil {
		t.Fatalf("failed to read top file: %v", err)
	}
	if string(content) != "top" {
		t.Errorf("top.txt content = %q, want %q", string(content), "top")
	}
}

func TestApply_PreservesFilePermissions(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()

	// Create an executable file
	scriptPath := filepath.Join(src, "script.sh")
	os.WriteFile(scriptPath, []byte("#!/bin/sh\necho hello"), 0755)

	if err := Apply(src, dst); err != nil {
		t.Fatalf("Apply() error: %v", err)
	}

	info, err := os.Stat(filepath.Join(dst, "script.sh"))
	if err != nil {
		t.Fatalf("failed to stat script.sh: %v", err)
	}

	// Check that the execute bit is preserved
	if info.Mode()&0111 == 0 {
		t.Errorf("expected executable permission, got %v", info.Mode())
	}
}
