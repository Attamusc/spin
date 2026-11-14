package process

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

// BackgroundOptions configures a background process execution.
type BackgroundOptions struct {
	Command string
	Dir     string
	Env     map[string]string
	LogFile string
	PIDFile string
	PreHook string
}

// StartBackground starts a command as a detached background process.
// stdout/stderr are redirected to the log file. Returns the child PID.
func StartBackground(opts BackgroundOptions) (int, error) {
	// Run pre-hook if defined (in foreground, before backgrounding)
	if opts.PreHook != "" {
		if err := runHook(opts.PreHook, opts.Dir, opts.Env); err != nil {
			return 0, fmt.Errorf("pre-hook failed: %w", err)
		}
	}

	// Create log file directory
	logDir := filepath.Dir(opts.LogFile)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return 0, fmt.Errorf("could not create log directory: %w", err)
	}

	// Open log file in write mode (truncate)
	logFile, err := os.OpenFile(opts.LogFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return 0, fmt.Errorf("could not open log file: %w", err)
	}

	cmd := exec.Command("sh", "-c", opts.Command)
	cmd.Dir = opts.Dir
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.Env = mergeEnv(opts.Env)

	// Create a new session so the process is detached
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true,
	}

	// Start the process (non-blocking)
	if err := cmd.Start(); err != nil {
		logFile.Close()
		return 0, fmt.Errorf("could not start background process: %w", err)
	}

	pid := cmd.Process.Pid

	// Write PID file
	if err := WritePID(opts.PIDFile, pid); err != nil {
		// Try to kill the process if we can't write the PID file
		_ = syscall.Kill(-pid, syscall.SIGKILL)
		_ = cmd.Wait()
		logFile.Close()
		return 0, fmt.Errorf("could not write PID file: %w", err)
	}

	// Wait in a goroutine to reap the child and avoid zombies.
	// In production, the spin CLI exits immediately after Start so the
	// child is reparented to init/launchd. In tests, we need to reap.
	go func() {
		_ = cmd.Wait()
		logFile.Close()
	}()

	return pid, nil
}

// StopBackground stops a background process by PID.
// Sends SIGTERM first, waits up to 5 seconds, then sends SIGKILL.
// Signals are sent to the process group (-pid) since background processes
// are started in their own session with Setsid.
func StopBackground(pid int) error {
	if err := validatePID(pid); err != nil {
		return err
	}
	pg, err := syscall.Getpgid(pid)
	if err != nil {
		return fmt.Errorf("could not verify process group: %w", err)
	}
	if pg != pid {
		return fmt.Errorf("PID %d is not a process group leader", pid)
	}
	if err := Probe(pid); err != nil {
		return fmt.Errorf("could not probe PID %d: %w", pid, err)
	}
	return terminateGroup(pid, syscall.Kill, sleepMs)
}

func terminateGroup(pid int, signal func(int, syscall.Signal) error, pause func(int)) error {
	pgid := -pid
	if err := signal(pgid, syscall.SIGTERM); err != nil {
		if errors.Is(err, syscall.ESRCH) {
			return nil
		}
		return fmt.Errorf("could not send SIGTERM to group %d: %w", pid, err)
	}
	// Descendants may outlive the leader. Darwin can return EPERM while only
	// zombies remain; retry within the budget, but only ESRCH proves absence.
	var probeErr error
	for i := 0; i < 50; i++ {
		probeErr = signal(pgid, 0)
		if errors.Is(probeErr, syscall.ESRCH) {
			return nil
		}
		if probeErr != nil && !errors.Is(probeErr, syscall.EPERM) {
			return fmt.Errorf("could not probe group %d: %w", pid, probeErr)
		}
		pause(100)
	}
	if probeErr != nil {
		return fmt.Errorf("could not probe group %d: %w", pid, probeErr)
	}
	if err := signal(pgid, syscall.SIGKILL); err != nil && !errors.Is(err, syscall.ESRCH) {
		return fmt.Errorf("could not send SIGKILL to group %d: %w", pid, err)
	}

	return nil
}
