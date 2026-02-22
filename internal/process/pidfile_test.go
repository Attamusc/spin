package process

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestWriteAndReadPID(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.pid")

	err := WritePID(path, 12345)
	if err != nil {
		t.Fatalf("WritePID failed: %v", err)
	}

	pid, err := ReadPID(path)
	if err != nil {
		t.Fatalf("ReadPID failed: %v", err)
	}
	if pid != 12345 {
		t.Errorf("expected PID 12345, got %d", pid)
	}
}

func TestWritePID_CreatesDirectories(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "dir", "test.pid")

	err := WritePID(path, 99999)
	if err != nil {
		t.Fatalf("WritePID failed: %v", err)
	}

	pid, err := ReadPID(path)
	if err != nil {
		t.Fatalf("ReadPID failed: %v", err)
	}
	if pid != 99999 {
		t.Errorf("expected PID 99999, got %d", pid)
	}
}

func TestReadPID_MissingFile(t *testing.T) {
	_, err := ReadPID("/nonexistent/path/test.pid")
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

func TestReadPID_CorruptFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.pid")
	os.WriteFile(path, []byte("not-a-number"), 0644)

	_, err := ReadPID(path)
	if err == nil {
		t.Fatal("expected error for corrupt PID file, got nil")
	}
}

func TestIsProcessAlive_CurrentProcess(t *testing.T) {
	// Current process should be alive
	if !IsProcessAlive(os.Getpid()) {
		t.Error("expected current process to be alive")
	}
}

func TestIsProcessAlive_DeadProcess(t *testing.T) {
	// Start a process and wait for it to finish
	cmd := exec.Command("true")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	pid := cmd.Process.Pid
	cmd.Wait()

	// The process should be dead now
	if IsProcessAlive(pid) {
		t.Error("expected finished process to be dead")
	}
}

func TestCleanStalePIDFile_StaleProcess(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.pid")

	// Start and wait for a process to complete
	cmd := exec.Command("true")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	pid := cmd.Process.Pid
	cmd.Wait()

	// Write the dead PID
	if err := WritePID(path, pid); err != nil {
		t.Fatal(err)
	}

	cleaned, err := CleanStalePIDFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cleaned {
		t.Error("expected stale PID file to be cleaned")
	}

	// File should be gone
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("expected PID file to be removed")
	}
}

func TestCleanStalePIDFile_AliveProcess(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.pid")

	// Write current process PID (it's alive)
	if err := WritePID(path, os.Getpid()); err != nil {
		t.Fatal(err)
	}

	cleaned, err := CleanStalePIDFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cleaned {
		t.Error("expected alive PID file to not be cleaned")
	}

	// File should still exist
	if _, err := os.Stat(path); err != nil {
		t.Error("expected PID file to still exist")
	}
}

func TestCleanStalePIDFile_MissingFile(t *testing.T) {
	cleaned, err := CleanStalePIDFile("/nonexistent/test.pid")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cleaned {
		t.Error("expected no cleanup for missing file")
	}
}

func TestPIDFilePath(t *testing.T) {
	path := PIDFilePath("/data/.spin", "attamusc", "mindtrail")
	expected := filepath.Join("/data/.spin", "attamusc", "mindtrail", "live.pid")
	if path != expected {
		t.Errorf("expected %q, got %q", expected, path)
	}
}

func TestLogFilePath(t *testing.T) {
	path := LogFilePath("/data/.spin", "attamusc", "mindtrail")
	expected := filepath.Join("/data/.spin", "attamusc", "mindtrail", "logs", "live.log")
	if path != expected {
		t.Errorf("expected %q, got %q", expected, path)
	}
}
