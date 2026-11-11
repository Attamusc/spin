package cmd

import (
	"database/sql"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/attamusc/spin/internal/process"
)

func TestDBCLI(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "spin")
	if out, err := exec.Command("go", "build", "-o", binary, "..").CombinedOutput(); err != nil {
		t.Fatalf("build: %v: %s", err, out)
	}
	home := t.TempDir()
	root := filepath.Join(home, "projects")
	project := filepath.Join(root, "github.com", "owner", "repo")
	if err := os.MkdirAll(project, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, ".spin.toml"), []byte("[dev]\ncommand='true'\n[live]\ncommand='true'\n"), 0600); err != nil {
		t.Fatal(err)
	}
	run := func(ok bool, args ...string) {
		t.Helper()
		c := exec.Command(binary, append([]string{"db"}, args...)...)
		c.Dir = project
		c.Env = append(os.Environ(), "HOME="+home, "NO_COLOR=1")
		out, err := c.CombinedOutput()
		if (err == nil) != ok {
			t.Fatalf("%v: %v: %s", args, err, out)
		}
	}
	state := filepath.Join(root, ".spin", "owner", "repo")
	live := filepath.Join(state, "live", "data.db")
	dev := filepath.Join(state, "dev", "data.db")
	// Invalid names must fail before creating the state tree.
	for _, name := range []string{"../escape", "/absolute", "..", "bad\\name", ""} {
		run(false, "snapshot", "live", name)
	}
	if _, err := os.Stat(state); !os.IsNotExist(err) {
		t.Fatal("invalid name created state")
	}
	if err := os.MkdirAll(filepath.Dir(live), 0755); err != nil {
		t.Fatal(err)
	}
	d, err := sql.Open("sqlite", live)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = d.Exec("PRAGMA journal_mode=WAL; CREATE TABLE items(value); INSERT INTO items VALUES(42)"); err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	run(true, "copy", "live", "dev")
	run(true, "snapshot", "live", "saved")
	check := func(path string) {
		t.Helper()
		conn, err := sql.Open("sqlite", path)
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Close()
		var v int
		if err = conn.QueryRow("SELECT value FROM items").Scan(&v); err != nil || v != 42 {
			t.Fatalf("value=%d err=%v", v, err)
		}
	}
	check(dev)
	unlock, err := process.LockProfile(filepath.Join(root, ".spin"), "owner", "repo", "dev")
	if err != nil {
		t.Fatal(err)
	}
	run(false, "restore", "dev", "saved")
	run(false, "reset", "dev")
	unlock()
	check(dev)
	run(true, "reset", "dev")
	if _, err = os.Stat(dev); !os.IsNotExist(err) {
		t.Fatal("reset retained DB")
	}
	run(true, "restore", "dev", "saved")
	check(dev)
	// A real SQLite writer, independent of spin's advisory profile lock.
	writer, err := sql.Open("sqlite", dev)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = writer.Exec("BEGIN IMMEDIATE; UPDATE items SET value=7"); err != nil {
		t.Fatal(err)
	}
	run(false, "copy", "live", "dev")
	run(false, "reset", "dev")
	writer.Exec("ROLLBACK")
	writer.Close()
	check(dev)
	if err = process.WritePID(process.PIDFilePath(filepath.Join(root, ".spin"), "owner", "repo"), os.Getpid()); err != nil {
		t.Fatal(err)
	}
	run(false, "reset", "live")
	run(false, "restore", "live", "saved")
}
