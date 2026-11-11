package vcs

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// hasJJ reports whether jj is available on the system PATH.
func hasJJ() bool {
	_, err := exec.LookPath("jj")
	return err == nil
}

// hasGit reports whether git is available on the system PATH.
func hasGit() bool {
	_, err := exec.LookPath("git")
	return err == nil
}

// Clone clones a git repo into targetDir using jj (preferred) or git.
// The URL should be a full git-compatible URL (e.g., https://github.com/owner/repo.git).
// For jj: runs "jj git clone --colocate <url> <targetDir>".
// For git: runs "git clone <url> <targetDir>".
func Clone(url, targetDir string) error {
	return preferred("", []string{"git", "clone", "--colocate", url, targetDir}, []string{"clone", url, targetDir})
}

// Init initializes a repository, preferring jj when available.
func Init(dir string) error {
	return preferred(dir, []string{"git", "init", "--colocate"}, []string{"init"})
}

func preferred(dir string, jjArgs, gitArgs []string) error {
	if hasJJ() {
		return run(context.Background(), dir, "jj", jjArgs...)
	}
	if hasGit() {
		return run(context.Background(), dir, "git", gitArgs...)
	}
	return fmt.Errorf("neither jj nor git found in PATH")
}

// Fetch fetches latest changes from remote for the repo in dir.
func Fetch(dir string) error { return FetchContext(context.Background(), dir) }

// FetchContext fetches latest changes until ctx is canceled.
func FetchContext(ctx context.Context, dir string) error {
	if IsJJRepo(dir) {
		return run(ctx, dir, "jj", "git", "fetch")
	}
	if IsGitRepo(dir) {
		return run(ctx, dir, "git", "fetch", "--all")
	}
	return fmt.Errorf("directory %s is not a jj or git repository", dir)
}

func run(ctx context.Context, dir, tool string, args ...string) error {
	cmd := exec.CommandContext(ctx, tool, args...)
	cmd.Dir = dir
	cmd.Stdout, cmd.Stderr = os.Stderr, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s %s failed: %w", tool, args[0], err)
	}
	return nil
}

// IsRepo returns true if dir contains a .jj or .git directory.
func IsRepo(dir string) bool {
	return IsJJRepo(dir) || IsGitRepo(dir)
}

// IsJJRepo returns true if dir contains a .jj directory.
func IsJJRepo(dir string) bool {
	info, err := os.Stat(filepath.Join(dir, ".jj"))
	return err == nil && info.IsDir()
}

// IsGitRepo returns true if dir contains a .git directory.
func IsGitRepo(dir string) bool {
	info, err := os.Stat(filepath.Join(dir, ".git"))
	return err == nil && info.IsDir()
}
