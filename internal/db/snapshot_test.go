package db

import (
	"bytes"
	"database/sql"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func testDB(t *testing.T, path string) *sql.DB {
	t.Helper()
	d, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = d.Exec("CREATE TABLE IF NOT EXISTS items(value); INSERT INTO items VALUES (42)"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	return d
}
func value(t *testing.T, path string) int {
	t.Helper()
	d, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	var v int
	if err = d.QueryRow("SELECT value FROM items").Scan(&v); err != nil {
		t.Fatal(err)
	}
	return v
}
func TestCopyDBLiveWAL(t *testing.T) {
	dir := t.TempDir()
	src, dst := filepath.Join(dir, "src.db"), filepath.Join(dir, "dst.db")
	d := testDB(t, src)
	if _, err := d.Exec("PRAGMA journal_mode=WAL; UPDATE items SET value=99"); err != nil {
		t.Fatal(err)
	}
	if err := CopyDB(src, dst); err != nil {
		t.Fatal(err)
	}
	if got := value(t, dst); got != 99 {
		t.Fatal(got)
	}
}
func TestCopyDBPreservesRowIDs(t *testing.T) {
	dir := t.TempDir()
	src, dst := filepath.Join(dir, "src.db"), filepath.Join(dir, "dst.db")
	d := testDB(t, src)
	if _, err := d.Exec("PRAGMA journal_mode=WAL; DELETE FROM items; INSERT INTO items(rowid,value) VALUES(7,70),(42,420)"); err != nil {
		t.Fatal(err)
	}
	if err := CopyDB(src, dst); err != nil {
		t.Fatal(err)
	}
	copy, err := sql.Open("sqlite", dst)
	if err != nil {
		t.Fatal(err)
	}
	defer copy.Close()
	var ids string
	if err := copy.QueryRow("SELECT group_concat(rowid, ',') FROM (SELECT rowid FROM items ORDER BY rowid)").Scan(&ids); err != nil {
		t.Fatal(err)
	}
	if ids != "7,42" {
		t.Fatalf("rowids = %s, want 7,42", ids)
	}
}

func TestCopyDBBusySourceUntouched(t *testing.T) {
	dir := t.TempDir()
	src, dst := filepath.Join(dir, "src.db"), filepath.Join(dir, "dst.db")
	source := testDB(t, src)
	testDB(t, dst).Close()
	before, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := source.Exec("BEGIN EXCLUSIVE; UPDATE items SET value=7"); err != nil {
		t.Fatal(err)
	}
	defer source.Exec("ROLLBACK")
	if err := CopyDB(src, dst); err == nil {
		t.Fatal("accepted busy source")
	}
	after, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("target changed")
	}
	temps, err := filepath.Glob(filepath.Join(dir, ".spin-snapshot-*"))
	if err != nil || len(temps) != 0 {
		t.Fatalf("temporary files: %v, %v", temps, err)
	}
}

func TestCopyDBRefusesActiveTarget(t *testing.T) {
	for _, wal := range []bool{false, true} {
		t.Run(map[bool]string{false: "writer", true: "idle WAL"}[wal], func(t *testing.T) {
			dir := t.TempDir()
			src, dst := filepath.Join(dir, "src.db"), filepath.Join(dir, "dst.db")
			testDB(t, src).Close()
			d := testDB(t, dst)
			statement := "BEGIN IMMEDIATE; UPDATE items SET value=7"
			if wal {
				statement = "PRAGMA journal_mode=WAL; UPDATE items SET value=7"
			}
			if _, err := d.Exec(statement); err != nil {
				t.Fatal(err)
			}
			if err := CopyDB(src, dst); err == nil {
				t.Fatal("accepted active target")
			}
			if err := RemoveDB(dst); err == nil {
				t.Fatal("reset accepted active target")
			}
			if !wal {
				d.Exec("ROLLBACK")
			}
		})
	}
}
func TestCopyDBFailedSourceUntouched(t *testing.T) {
	dir := t.TempDir()
	dst := filepath.Join(dir, "dst.db")
	testDB(t, dst).Close()
	before, _ := os.ReadFile(dst)
	for _, src := range []string{filepath.Join(dir, "missing"), filepath.Join(dir, "invalid")} {
		if filepath.Base(src) == "invalid" {
			os.WriteFile(src, []byte("not sqlite"), 0600)
		}
		if err := CopyDB(src, dst); err == nil {
			t.Fatal("expected failure")
		}
		after, _ := os.ReadFile(dst)
		if !bytes.Equal(before, after) {
			t.Fatal("target changed")
		}
		temps, _ := filepath.Glob(filepath.Join(dir, ".spin-snapshot-*"))
		if len(temps) != 0 {
			t.Fatal(temps)
		}
	}
}
func TestRestoreRecoversStaleWAL(t *testing.T) {
	// Abrupt process exit leaves real, committed WAL sidecars behind.
	if path := os.Getenv("SPIN_TEST_CRASH_DB"); path != "" {
		d, err := sql.Open("sqlite", path)
		if err != nil {
			os.Exit(2)
		}
		if _, err = d.Exec("PRAGMA journal_mode=WAL; CREATE TABLE items(value); INSERT INTO items VALUES (7)"); err != nil {
			os.Exit(3)
		}
		os.Exit(0)
	}
	dir := t.TempDir()
	dst, src := filepath.Join(dir, "dst.db"), filepath.Join(dir, "src.db")
	child := exec.Command(os.Args[0], "-test.run=^TestRestoreRecoversStaleWAL$")
	child.Env = append(os.Environ(), "SPIN_TEST_CRASH_DB="+dst)
	if out, err := child.CombinedOutput(); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	if _, err := os.Stat(dst + "-wal"); err != nil {
		t.Fatal(err)
	}
	testDB(t, src).Close()
	before, _ := os.Stat(dst)
	if err := CopyDB(src, dst); err != nil {
		t.Fatal(err)
	}
	after, _ := os.Stat(dst)
	if !os.SameFile(before, after) {
		t.Fatal("inode changed")
	}
	if got := value(t, dst); got != 42 {
		t.Fatal(got)
	}
	for _, suffix := range []string{"-wal", "-shm"} {
		if _, err := os.Stat(dst + suffix); !os.IsNotExist(err) {
			t.Fatalf("retained %s", suffix)
		}
	}
	if err := RemoveDB(dst); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dst); !os.IsNotExist(err) {
		t.Fatal("not removed")
	}
}
func TestDatabaseUnderConfiguredRootAlias(t *testing.T) {
	root := t.TempDir()
	alias := filepath.Join(t.TempDir(), "root-alias")
	if err := os.Symlink(root, alias); err != nil {
		t.Fatal(err)
	}
	src, dst := filepath.Join(alias, "src.db"), filepath.Join(alias, "dst.db")
	testDB(t, src).Close()
	if err := CopyDB(src, dst); err != nil {
		t.Fatalf("rejected root alias: %v", err)
	}
	if got := value(t, dst); got != 42 {
		t.Fatal(got)
	}
}

func TestSymlinkRefused(t *testing.T) {
	dir := t.TempDir()
	src, dst := filepath.Join(dir, "src.db"), filepath.Join(dir, "dst.db")
	testDB(t, src).Close()
	if err := os.Symlink(src, dst); err != nil {
		t.Fatal(err)
	}
	if err := CopyDB(src, dst); err == nil {
		t.Fatal("accepted symlink")
	}
	if err := RemoveDB(dst); err == nil {
		t.Fatal("removed symlink")
	}
}
