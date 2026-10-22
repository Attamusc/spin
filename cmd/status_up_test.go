package cmd

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"testing"

	"github.com/attamusc/spin/internal/config"
	"github.com/attamusc/spin/internal/resolve"
)

func TestStatusPortCheckAdvisory(t *testing.T) {
	for _, state := range []string{"live", "stale", "malformed", "unknown"} {
		t.Run(state, func(t *testing.T) {
			pid := fmt.Sprint(os.Getpid())
			if state == "unknown" {
				// A foreign privileged PID provides an actual EPERM probe without sending a signal.
				if err := syscall.Kill(1, 0); err != syscall.EPERM {
					t.Skipf("PID 1 does not provide EPERM on this host: %v", err)
				}
				pid = "1"
			}
			if state == "stale" {
				child := exec.Command("sh", "-c", "exit 0")
				if err := child.Run(); err != nil {
					t.Fatal(err)
				}
				pid = fmt.Sprint(child.ProcessState.Pid())
			}
			if state == "malformed" {
				pid = "0"
			}
			cfg, path, _ := statusFixture(t, "other", "app", pid)
			before, _ := os.Stat(path)
			project := &resolve.ResolvedProject{Owner: "self", Repo: "app", Config: &config.ProjectConfig{Live: config.ProfileConfig{Port: 3000}}}
			read, write, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			previous := os.Stderr
			os.Stderr = write
			func() { defer func() { os.Stderr = previous; write.Close() }(); checkPortConflicts(cfg, project) }()
			raw, err := io.ReadAll(read)
			read.Close()
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(raw), "also configured") != (state == "live") {
				t.Fatalf("%s warnings: %s", state, raw)
			}
			if state == "unknown" && !strings.Contains(string(raw), "pid_probe_failed:") {
				t.Fatalf("missing EPERM diagnostic: %s", raw)
			}
			if (state == "malformed" || state == "unknown") && !strings.Contains(string(raw), "advisory only") {
				t.Fatalf("missing incomplete warning: %s", raw)
			}
			after, err := os.Stat(path)
			if err != nil || !before.ModTime().Equal(after.ModTime()) {
				t.Fatal("advisory removed/changed PID")
			}
		})
	}
}
