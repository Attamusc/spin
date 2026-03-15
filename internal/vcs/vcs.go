package vcs

import (
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
	if hasJJ() {
		cmd := exec.Command("jj", "git", "clone", "--colocate", url, targetDir)
		cmd.Stdout = os.Stderr // info output to stderr (stdout reserved for paths)
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("jj git clone failed: %w", err)
		}
		return nil
	}

	if hasGit() {
		cmd := exec.Command("git", "clone", url, targetDir)
		cmd.Stdout = os.Stderr
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("git clone failed: %w", err)
		}
		return nil
	}

	return fmt.Errorf("neither jj nor git found in PATH")
}

// Init initializes a new VCS repository in the given directory.
// For jj: runs "jj git init --colocate" in dir.
// For git: runs "git init" in dir.
func Init(dir string) error {
	if hasJJ() {
		cmd := exec.Command("jj", "git", "init", "--colocate")
		cmd.Dir = dir
		cmd.Stdout = os.Stderr
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("jj git init failed: %w", err)
		}
		return nil
	}

	if hasGit() {
		cmd := exec.Command("git", "init")
		cmd.Dir = dir
		cmd.Stdout = os.Stderr
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("git init failed: %w", err)
		}
		return nil
	}

	return fmt.Errorf("neither jj nor git found in PATH")
}

// Fetch fetches latest changes from remote for the repo in dir.
// For jj: runs "jj git fetch" in dir.
// For git: runs "git fetch --all" in dir.
func Fetch(dir string) error {
	if IsJJRepo(dir) {
		cmd := exec.Command("jj", "git", "fetch")
		cmd.Dir = dir
		cmd.Stdout = os.Stderr
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("jj git fetch failed: %w", err)
		}
		return nil
	}

	if IsGitRepo(dir) {
		cmd := exec.Command("git", "fetch", "--all")
		cmd.Dir = dir
		cmd.Stdout = os.Stderr
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("git fetch failed: %w", err)
		}
		return nil
	}

	return fmt.Errorf("directory %s is not a jj or git repository", dir)
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
