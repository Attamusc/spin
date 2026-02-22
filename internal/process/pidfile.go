package process

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// WritePID writes a PID to the given file path. Creates parent directories if needed.
func WritePID(path string, pid int) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("could not create PID file directory: %w", err)
	}
	return os.WriteFile(path, []byte(strconv.Itoa(pid)), 0644)
}

// ReadPID reads a PID from the given file path.
func ReadPID(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, fmt.Errorf("corrupt PID file %s: %w", path, err)
	}
	return pid, nil
}

// IsProcessAlive checks if a process with the given PID is alive.
// Uses syscall.Kill(pid, 0) which checks existence without sending a signal.
func IsProcessAlive(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil
}

// CleanStalePIDFile removes a PID file if the process it references is no longer alive.
// Returns true if the PID file was stale and removed.
func CleanStalePIDFile(path string) (bool, error) {
	pid, err := ReadPID(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		// Corrupt PID file - remove it
		_ = os.Remove(path)
		return true, nil
	}

	if !IsProcessAlive(pid) {
		if err := os.Remove(path); err != nil {
			return false, fmt.Errorf("could not remove stale PID file: %w", err)
		}
		return true, nil
	}

	return false, nil
}

// PIDFilePath returns the path to the PID file for a given project.
func PIDFilePath(dataRoot, owner, repo string) string {
	return filepath.Join(dataRoot, owner, repo, "live.pid")
}

// LogFilePath returns the path to the log file for a given project.
func LogFilePath(dataRoot, owner, repo string) string {
	return filepath.Join(dataRoot, owner, repo, "logs", "live.log")
}
