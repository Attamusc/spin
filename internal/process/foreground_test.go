package process

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunForeground_SimpleCommand(t *testing.T) {
	dir := t.TempDir()

	exitCode, err := RunForeground(ForegroundOptions{
		Command: "echo hello",
		Dir:     dir,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exitCode != 0 {
		t.Errorf("expected exit code 0, got %d", exitCode)
	}
}

func TestRunForeground_EnvInjection(t *testing.T) {
	dir := t.TempDir()
	outFile := filepath.Join(dir, "env_out.txt")

	exitCode, err := RunForeground(ForegroundOptions{
		Command: "env > " + outFile,
		Dir:     dir,
		Env: map[string]string{
			"SPIN_TEST_VAR": "hello_spin",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exitCode != 0 {
		t.Errorf("expected exit code 0, got %d", exitCode)
	}

	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("could not read env output: %v", err)
	}

	if !strings.Contains(string(data), "SPIN_TEST_VAR=hello_spin") {
		t.Errorf("expected SPIN_TEST_VAR in env output, got: %s", string(data))
	}
}

func TestRunForeground_ExitCodePropagation(t *testing.T) {
	dir := t.TempDir()

	exitCode, err := RunForeground(ForegroundOptions{
		Command: "exit 42",
		Dir:     dir,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exitCode != 42 {
		t.Errorf("expected exit code 42, got %d", exitCode)
	}
}

func TestRunForeground_PreHook(t *testing.T) {
	dir := t.TempDir()
	hookFile := filepath.Join(dir, "hook_ran.txt")

	exitCode, err := RunForeground(ForegroundOptions{
		Command: "echo main",
		Dir:     dir,
		PreHook: "echo hook > " + hookFile,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exitCode != 0 {
		t.Errorf("expected exit code 0, got %d", exitCode)
	}

	data, err := os.ReadFile(hookFile)
	if err != nil {
		t.Fatalf("hook file not created: %v", err)
	}
	if strings.TrimSpace(string(data)) != "hook" {
		t.Errorf("expected hook output, got %q", string(data))
	}
}

func TestRunForeground_PreHookFailure(t *testing.T) {
	dir := t.TempDir()

	_, err := RunForeground(ForegroundOptions{
		Command: "echo should-not-run",
		Dir:     dir,
		PreHook: "exit 1",
	})
	if err == nil {
		t.Fatal("expected error from failed pre-hook, got nil")
	}
}

func TestRunForeground_PreHookEnv(t *testing.T) {
	dir := t.TempDir()
	outFile := filepath.Join(dir, "hook_env.txt")

	exitCode, err := RunForeground(ForegroundOptions{
		Command: "true",
		Dir:     dir,
		PreHook: "env > " + outFile,
		Env: map[string]string{
			"SPIN_HOOK_VAR": "hook_value",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exitCode != 0 {
		t.Errorf("expected exit code 0, got %d", exitCode)
	}

	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("could not read hook env output: %v", err)
	}

	if !strings.Contains(string(data), "SPIN_HOOK_VAR=hook_value") {
		t.Errorf("expected SPIN_HOOK_VAR in hook env output")
	}
}
