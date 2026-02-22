package process

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestStartBackground_ProcessRuns(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "logs", "test.log")
	pidFile := filepath.Join(dir, "test.pid")

	pid, err := StartBackground(BackgroundOptions{
		Command: "echo hello-background && sleep 30",
		Dir:     dir,
		LogFile: logFile,
		PIDFile: pidFile,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	defer func() {
		_ = StopBackground(pid)
	}()

	if pid <= 0 {
		t.Errorf("expected positive PID, got %d", pid)
	}

	// Verify PID file exists and is correct
	readPid, err := ReadPID(pidFile)
	if err != nil {
		t.Fatalf("could not read PID file: %v", err)
	}
	if readPid != pid {
		t.Errorf("PID file has %d, expected %d", readPid, pid)
	}

	// Verify process is alive
	if !IsProcessAlive(pid) {
		t.Error("expected process to be alive")
	}

	// Wait a moment for log output
	time.Sleep(500 * time.Millisecond)

	// Verify log file was created and has content
	data, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("could not read log file: %v", err)
	}
	if !strings.Contains(string(data), "hello-background") {
		t.Errorf("expected log output, got %q", string(data))
	}
}

func TestStartBackground_LogFileTruncated(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "logs", "test.log")
	pidFile := filepath.Join(dir, "test.pid")

	// Write some existing content to the log file
	os.MkdirAll(filepath.Dir(logFile), 0755)
	os.WriteFile(logFile, []byte("old-content\n"), 0644)

	pid, err := StartBackground(BackgroundOptions{
		Command: "echo new-content",
		Dir:     dir,
		LogFile: logFile,
		PIDFile: pidFile,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Wait for process to finish writing
	time.Sleep(500 * time.Millisecond)

	// Clean up
	_ = StopBackground(pid)

	data, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("could not read log file: %v", err)
	}

	if strings.Contains(string(data), "old-content") {
		t.Error("expected old content to be truncated")
	}
	if !strings.Contains(string(data), "new-content") {
		t.Errorf("expected new content in log, got %q", string(data))
	}
}

func TestStartBackground_PreHook(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "logs", "test.log")
	pidFile := filepath.Join(dir, "test.pid")
	hookFile := filepath.Join(dir, "hook_ran.txt")

	pid, err := StartBackground(BackgroundOptions{
		Command: "sleep 30",
		Dir:     dir,
		LogFile: logFile,
		PIDFile: pidFile,
		PreHook: "echo hook-executed > " + hookFile,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() {
		_ = StopBackground(pid)
	}()

	data, err := os.ReadFile(hookFile)
	if err != nil {
		t.Fatalf("hook file not created: %v", err)
	}
	if !strings.Contains(string(data), "hook-executed") {
		t.Errorf("expected hook output, got %q", string(data))
	}
}

func TestStopBackground(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "logs", "test.log")
	pidFile := filepath.Join(dir, "test.pid")

	pid, err := StartBackground(BackgroundOptions{
		Command: "sleep 300",
		Dir:     dir,
		LogFile: logFile,
		PIDFile: pidFile,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !IsProcessAlive(pid) {
		t.Fatal("process should be alive")
	}

	err = StopBackground(pid)
	if err != nil {
		t.Fatalf("StopBackground failed: %v", err)
	}

	// Process should be dead
	time.Sleep(200 * time.Millisecond)
	if IsProcessAlive(pid) {
		t.Error("expected process to be dead after stop")
	}
}
