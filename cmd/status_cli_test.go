package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestStatusCLIObservation(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "spin")
	build := exec.Command("go", "build", "-o", binary, "..")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v: %s", err, out)
	}
	home := t.TempDir()
	projects := filepath.Join(home, "projects")
	if err := os.MkdirAll(projects, 0755); err != nil {
		t.Fatal(err)
	}
	run := func(args ...string) ([]byte, []byte, error) {
		c := exec.Command(binary, args...)
		c.Env = append(os.Environ(), "HOME="+home, "NO_COLOR=1")
		var out, stderr bytes.Buffer
		c.Stdout = &out
		c.Stderr = &stderr
		err := c.Run()
		return out.Bytes(), stderr.Bytes(), err
	}
	out, stderr, err := run("status", "--format", "json")
	if err != nil {
		t.Fatalf("empty JSON: %v stdout=%s stderr=%s", err, out, stderr)
	}
	var result map[string]any
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatal(err)
	}
	if result["schema_version"] != float64(1) || result["complete"] != true || len(result["instances"].([]any)) != 0 || len(stderr) != 0 {
		t.Fatalf("unexpected: %s %s", out, stderr)
	}
	pidPath := filepath.Join(projects, ".spin", "owner", "repo", "live.pid")
	if err := os.MkdirAll(filepath.Dir(pidPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pidPath, []byte("0\n"), 0644); err != nil {
		t.Fatal(err)
	}
	before, _ := os.Stat(pidPath)
	for _, args := range [][]string{{"status", "--format", "json"}, {"status", "--format", "plain"}, {}} {
		out, stderr, err = run(args...)
		if err == nil {
			t.Fatalf("incomplete should fail: %q %s %s", args, out, stderr)
		}
		if len(args) == 0 && !bytes.Contains(stderr, []byte("Common commands:")) {
			t.Fatalf("missing dashboard help: %s", stderr)
		}
	}
	after, err := os.Stat(pidPath)
	if err != nil || !before.ModTime().Equal(after.ModTime()) {
		t.Fatalf("observation changed PID file: %v", err)
	}
	for _, args := range [][]string{{"status", "--format", "yaml"}, {"status", "extra"}, {"status", "--json"}} {
		out, stderr, err = run(args...)
		exit, ok := err.(*exec.ExitError)
		if !ok || exit.ExitCode() != 1 || len(out) != 0 || len(stderr) == 0 {
			t.Fatalf("invalid argv streams/exit %q: %v stdout=%s stderr=%s", args, err, out, stderr)
		}
	}
	if full, openErr := os.OpenFile("/dev/full", os.O_WRONLY, 0); openErr == nil {
		defer full.Close()
		c := exec.Command(binary, "status", "--format", "json")
		c.Env = append(os.Environ(), "HOME="+home)
		c.Stdout = full
		var failure bytes.Buffer
		c.Stderr = &failure
		if err := c.Run(); err == nil || !bytes.Contains(failure.Bytes(), []byte("Could not write status output")) {
			t.Fatalf("stdout failure: %v %s", err, failure.Bytes())
		}
	}
}
