package process

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
)

// ForegroundOptions configures a foreground process execution.
type ForegroundOptions struct {
	Command string
	Dir     string
	Env     map[string]string
	PreHook string
}

// RunForeground executes a command in the foreground with signal forwarding.
// It connects stdin/stdout/stderr directly and forwards SIGINT/SIGTERM to the child.
// Returns the exit code of the child process.
func RunForeground(opts ForegroundOptions) (int, error) {
	// Run pre-hook if defined
	if opts.PreHook != "" {
		if err := runHook(opts.PreHook, opts.Dir, opts.Env); err != nil {
			return 1, fmt.Errorf("pre-hook failed: %w", err)
		}
	}

	cmd := exec.Command("sh", "-c", opts.Command)
	cmd.Dir = opts.Dir
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = mergeEnv(opts.Env)

	// Start the process
	if err := cmd.Start(); err != nil {
		return 1, fmt.Errorf("could not start command: %w", err)
	}

	// Set up signal forwarding
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Forward signals to child in a goroutine
	go func() {
		for sig := range sigChan {
			if cmd.Process != nil {
				_ = cmd.Process.Signal(sig)
			}
		}
	}()

	// Wait for child to exit
	err := cmd.Wait()
	signal.Stop(sigChan)
	close(sigChan)

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr.ExitCode(), nil
		}
		return 1, err
	}
	return 0, nil
}

// runHook executes a hook command via sh -c. Returns error if non-zero exit.
func runHook(hook, dir string, env map[string]string) error {
	cmd := exec.Command("sh", "-c", hook)
	cmd.Dir = dir
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = mergeEnv(env)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("hook command %q failed: %w", hook, err)
	}
	return nil
}

// mergeEnv merges the given env vars on top of the current environment.
func mergeEnv(env map[string]string) []string {
	if len(env) == 0 {
		return os.Environ()
	}

	// Start with current environment
	result := os.Environ()

	// Add/override with provided env vars
	for k, v := range env {
		result = append(result, k+"="+v)
	}

	return result
}
