package process

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// WritePID writes a PID to the given file path. Creates parent directories if needed.
func WritePID(path string, pid int) error {
	if err := validatePID(pid); err != nil {
		return err
	}
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
		return 0, fmt.Errorf("corrupt PID file %s: %w", path, errMalformedPID)
	}
	if err := validatePID(pid); err != nil {
		return 0, err
	}
	return pid, nil
}

// IsProcessAlive checks if a process with the given PID is alive.
// Uses syscall.Kill(pid, 0) which checks existence without sending a signal.
var errMalformedPID = errors.New("PID must be between 1 and 2147483647")

func validatePID(pid int) error {
	if pid < 1 || int64(pid) > 2147483647 {
		return errMalformedPID
	}
	return nil
}

// Probe checks process existence without suppressing permission or other errors.
func Probe(pid int) error {
	if err := validatePID(pid); err != nil {
		return err
	}
	return syscall.Kill(pid, 0)
}

func IsProcessAlive(pid int) bool {
	err := Probe(pid)
	return err == nil || errors.Is(err, syscall.EPERM)
}

// CleanStalePIDFile removes a PID file if the process it references is no longer alive.
// Returns true if the PID file was stale and removed.
func CleanStalePIDFile(path string) (bool, error) {
	pid, err := ReadPID(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		if !errors.Is(err, errMalformedPID) {
			return false, err
		}
	} else {
		err = Probe(pid)
		if err == nil {
			return false, nil
		}
		if !errors.Is(err, syscall.ESRCH) {
			return false, err
		}
	}

	if err := os.Remove(path); err != nil {
		return false, fmt.Errorf("could not remove stale PID file: %w", err)
	}
	return true, nil
}

// PIDFilePath returns the path to the PID file for a given project.
func PIDFilePath(dataRoot, owner, repo string) string {
	return filepath.Join(dataRoot, owner, repo, "live.pid")
}

// LogFilePath returns the path to the log file for a given project.
func LogFilePath(dataRoot, owner, repo string) string {
	return filepath.Join(dataRoot, owner, repo, "logs", "live.log")
}
