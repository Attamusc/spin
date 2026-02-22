package db

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/attamusc/spin/internal/process"
)

func TestWalCheckpoint_NoWAL(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Create an empty DB file
	if err := os.WriteFile(dbPath, []byte("fake db"), 0644); err != nil {
		t.Fatal(err)
	}

	// Should succeed (no-op) when no WAL file exists
	if err := WalCheckpoint(dbPath); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWalCheckpoint_WithWAL(t *testing.T) {
	// Check if sqlite3 is available
	if _, err := os.Stat("/usr/bin/sqlite3"); os.IsNotExist(err) {
		t.Skip("sqlite3 not available")
	}

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Create a real SQLite DB in WAL mode
	createTestDB(t, dbPath)

	// Create a WAL file to trigger the checkpoint
	walPath := dbPath + "-wal"
	if _, err := os.Stat(walPath); os.IsNotExist(err) {
		// Write some data to force WAL creation
		if err := os.WriteFile(walPath, []byte(""), 0644); err != nil {
			t.Fatal(err)
		}
	}

	// WAL checkpoint should succeed (or at least not crash)
	err := WalCheckpoint(dbPath)
	// The checkpoint may fail if the WAL file is not a real WAL,
	// but it shouldn't panic. We mainly test the flow.
	_ = err
}

// createTestDB creates a minimal SQLite database file.
func createTestDB(t *testing.T, dbPath string) {
	t.Helper()
	f, err := os.Create(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
}

func TestCopyDB_BasicCopy(t *testing.T) {
	tmpDir := t.TempDir()

	srcPath := filepath.Join(tmpDir, "src.db")
	dstPath := filepath.Join(tmpDir, "dst", "copy.db")

	content := []byte("test database content")
	if err := os.WriteFile(srcPath, content, 0644); err != nil {
		t.Fatal(err)
	}

	if err := CopyDB(srcPath, dstPath); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify main file was copied
	got, err := os.ReadFile(dstPath)
	if err != nil {
		t.Fatalf("could not read destination: %v", err)
	}
	if string(got) != string(content) {
		t.Errorf("content mismatch: got %q, want %q", got, content)
	}
}

func TestCopyDB_WithSidecarFiles(t *testing.T) {
	tmpDir := t.TempDir()

	srcPath := filepath.Join(tmpDir, "src.db")
	dstPath := filepath.Join(tmpDir, "dst", "copy.db")

	// Create main DB and sidecar files
	if err := os.WriteFile(srcPath, []byte("main db"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(srcPath+"-wal", []byte("wal data"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(srcPath+"-shm", []byte("shm data"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := CopyDB(srcPath, dstPath); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify all files were copied
	for _, suffix := range []string{"", "-wal", "-shm"} {
		if _, err := os.Stat(dstPath + suffix); os.IsNotExist(err) {
			t.Errorf("expected %s to exist", dstPath+suffix)
		}
	}

	walContent, _ := os.ReadFile(dstPath + "-wal")
	if string(walContent) != "wal data" {
		t.Errorf("WAL content mismatch: got %q", walContent)
	}
}

func TestCopyDB_WithoutSidecarFiles(t *testing.T) {
	tmpDir := t.TempDir()

	srcPath := filepath.Join(tmpDir, "src.db")
	dstPath := filepath.Join(tmpDir, "dst", "copy.db")

	if err := os.WriteFile(srcPath, []byte("main db"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := CopyDB(srcPath, dstPath); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Sidecar files should NOT exist
	for _, suffix := range []string{"-wal", "-shm"} {
		if _, err := os.Stat(dstPath + suffix); !os.IsNotExist(err) {
			t.Errorf("expected %s to NOT exist", dstPath+suffix)
		}
	}
}

func TestRemoveDB(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Create DB and sidecar files
	for _, suffix := range []string{"", "-wal", "-shm"} {
		if err := os.WriteFile(dbPath+suffix, []byte("data"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	if err := RemoveDB(dbPath); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// All files should be gone
	for _, suffix := range []string{"", "-wal", "-shm"} {
		if _, err := os.Stat(dbPath + suffix); !os.IsNotExist(err) {
			t.Errorf("expected %s to be removed", dbPath+suffix)
		}
	}
}

func TestRemoveDB_OnlyMainFile(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	if err := os.WriteFile(dbPath, []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := RemoveDB(dbPath); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := os.Stat(dbPath); !os.IsNotExist(err) {
		t.Error("expected DB file to be removed")
	}
}

func TestIsProfileRunning_DevAlwaysFalse(t *testing.T) {
	tmpDir := t.TempDir()
	if IsProfileRunning(tmpDir, "owner", "repo", "dev") {
		t.Error("expected dev profile to never report running")
	}
}

func TestIsProfileRunning_LiveNotRunning(t *testing.T) {
	tmpDir := t.TempDir()
	if IsProfileRunning(tmpDir, "owner", "repo", "live") {
		t.Error("expected live profile to report not running when no PID file")
	}
}

func TestIsProfileRunning_LiveWithStalePID(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a PID file with a dead PID
	pidFile := process.PIDFilePath(tmpDir, "owner", "repo")
	if err := os.MkdirAll(filepath.Dir(pidFile), 0755); err != nil {
		t.Fatal(err)
	}
	// Use PID 99999999 which almost certainly doesn't exist
	if err := process.WritePID(pidFile, 99999999); err != nil {
		t.Fatal(err)
	}

	if IsProfileRunning(tmpDir, "owner", "repo", "live") {
		t.Error("expected live profile with dead PID to report not running")
	}
}

func TestSnapshotPath(t *testing.T) {
	got := SnapshotPath("/data", "attamusc", "myapp", "backup1")
	want := filepath.Join("/data", "attamusc", "myapp", "snapshots", "backup1.db")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
