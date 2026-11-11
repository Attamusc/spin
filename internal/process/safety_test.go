package process

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
)

func TestInvalidPIDBoundaries(t *testing.T) {
	for _, pid := range []int{-1, 0, 2147483648} {
		path := filepath.Join(t.TempDir(), "pid")
		if err := WritePID(path, pid); err == nil {
			t.Errorf("WritePID accepted %d", pid)
		}
		if IsProcessAlive(pid) {
			t.Errorf("IsProcessAlive accepted %d", pid)
		}
		if err := StopBackground(pid); err == nil {
			t.Errorf("StopBackground accepted %d", pid)
		}
	}
	for _, value := range []string{"0", "-1", "2147483648"} {
		path := filepath.Join(t.TempDir(), "pid")
		if err := os.WriteFile(path, []byte(value), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := ReadPID(path); err == nil {
			t.Errorf("ReadPID accepted %s", value)
		}
	}
}

func TestProbeAndStopNonLeader(t *testing.T) {
	if err := Probe(0); err == nil {
		t.Fatal("Probe accepted zero")
	}
	child := exec.Command("sleep", "30")
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = child.Process.Kill(); _ = child.Wait() }()
	if err := Probe(child.Process.Pid); err != nil {
		t.Fatal(err)
	}
	if err := StopBackground(child.Process.Pid); err == nil {
		t.Fatal("accepted non-group leader")
	}
	if err := Probe(child.Process.Pid); err != nil {
		t.Fatalf("stop affected child: %v", err)
	}
}

func TestCleanupMalformed(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pid")
	if err := os.WriteFile(path, []byte("-1"), 0600); err != nil {
		t.Fatal(err)
	}
	if removed, err := CleanStalePIDFile(path); !removed || err != nil {
		t.Fatalf("removed=%v err=%v", removed, err)
	}
	if _, err := os.Stat(path); !errors.Is(err, syscall.ENOENT) {
		t.Fatalf("file remains: %v", err)
	}
}

func TestCleanupPreservesReadErrors(t *testing.T) {
	path := t.TempDir()
	if removed, err := CleanStalePIDFile(path); removed || err == nil {
		t.Fatalf("removed=%v err=%v", removed, err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
}
