package db

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/attamusc/spin/internal/process"
)

// WalCheckpoint runs a WAL checkpoint on the given SQLite database.
// Only runs if the WAL file exists. Shells out to sqlite3.
func WalCheckpoint(dbPath string) error {
	walPath := dbPath + "-wal"
	if _, err := os.Stat(walPath); os.IsNotExist(err) {
		return nil // No WAL file, nothing to do
	}

	cmd := exec.Command("sqlite3", dbPath, "PRAGMA wal_checkpoint(TRUNCATE);")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("WAL checkpoint failed: %w\nOutput: %s", err, string(output))
	}
	return nil
}

// CopyDB copies a SQLite database file, plus any -shm and -wal sidecar files.
// The destination parent directory is created if it does not exist.
func CopyDB(src, dst string) error {
	// Ensure destination directory exists
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return fmt.Errorf("could not create destination directory: %w", err)
	}

	// Copy the main DB file
	if err := copyFile(src, dst); err != nil {
		return fmt.Errorf("could not copy database: %w", err)
	}

	// Copy sidecar files if they exist
	for _, suffix := range []string{"-shm", "-wal"} {
		srcSidecar := src + suffix
		if _, err := os.Stat(srcSidecar); err == nil {
			dstSidecar := dst + suffix
			if err := copyFile(srcSidecar, dstSidecar); err != nil {
				return fmt.Errorf("could not copy %s: %w", suffix, err)
			}
		}
	}

	return nil
}

// RemoveDB removes a SQLite database file and its sidecar files (-shm, -wal).
func RemoveDB(dbPath string) error {
	for _, suffix := range []string{"", "-shm", "-wal"} {
		p := dbPath + suffix
		if _, err := os.Stat(p); err == nil {
			if err := os.Remove(p); err != nil {
				return fmt.Errorf("could not remove %s: %w", p, err)
			}
		}
	}
	return nil
}

// IsProfileRunning checks if a live instance is running for the given project.
// Only the "live" profile has PID files; "dev" is transient and always returns false.
func IsProfileRunning(dataRoot, owner, repo, profile string) bool {
	if profile != "live" {
		return false
	}

	pidFile := process.PIDFilePath(dataRoot, owner, repo)
	pid, err := process.ReadPID(pidFile)
	if err != nil {
		return false
	}
	return process.IsProcessAlive(pid)
}

// SnapshotDir returns the path to the snapshots directory for a project.
func SnapshotDir(dataRoot, owner, repo string) string {
	return filepath.Join(dataRoot, owner, repo, "snapshots")
}

// SnapshotPath returns the path to a named snapshot file.
func SnapshotPath(dataRoot, owner, repo, name string) string {
	return filepath.Join(SnapshotDir(dataRoot, owner, repo), name+".db")
}

// copyFile copies a single file from src to dst.
func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	srcInfo, err := srcFile.Stat()
	if err != nil {
		return err
	}

	dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, srcInfo.Mode())
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}
