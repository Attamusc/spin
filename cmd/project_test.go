package cmd

import (
	"bytes"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/attamusc/spin/internal/config"
	"github.com/attamusc/spin/internal/process"
	"github.com/attamusc/spin/internal/resolve"
)

func managedProjectFixture(t *testing.T) *managedProject {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "example.com", "owner", "repo")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	return newManagedProject(&config.GlobalConfig{ProjectsRoot: root, DataRoot: filepath.Join(root, ".spin")}, &resolve.ResolvedProject{
		Host: "example.com", Owner: "owner", Repo: "repo", Name: "owner/repo", Dir: dir,
		Config: &config.ProjectConfig{
			DBFilename: "data.db",
			Dev:        config.ProfileConfig{Command: "true", PreHook: "true", Env: map[string]string{"DB": "{profile_db_path}"}},
			Live:       config.ProfileConfig{Command: "exec sleep 30"},
		},
	})
}

func managedDB(t *testing.T, p *managedProject, profile string) *sql.DB {
	t.Helper()
	path, err := p.dbPath(profile)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	d, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	if _, err := d.Exec("PRAGMA journal_mode=WAL; CREATE TABLE items(value); INSERT INTO items(rowid,value) VALUES(42,99)"); err != nil {
		t.Fatal(err)
	}
	return d
}

func TestManagedProjectRejectsLiveMutationBeforePreparation(t *testing.T) {
	for _, probeErr := range []error{nil, syscall.EPERM, syscall.EIO} {
		p := managedProjectFixture(t)
		p.probe = func(int) error { return probeErr }
		pidFile, err := p.statePath("live.pid")
		if err != nil {
			t.Fatal(err)
		}
		if err := process.WritePID(pidFile, os.Getpid()); err != nil {
			t.Fatal(err)
		}
		for _, action := range []func() error{
			func() error { return p.copyDB("dev", "live") },
			func() error { return p.restoreDB("live", "saved") },
			func() error { return p.resetDB("live") },
		} {
			if err := action(); err == nil || !strings.Contains(err.Error(), "running or its state is unknown") {
				t.Fatalf("probe %v: live mutation was not refused: %v", probeErr, err)
			}
		}
		for _, profile := range []string{"dev", "live"} {
			path, _ := p.dbPath(profile)
			if _, err := os.Stat(filepath.Dir(path)); !os.IsNotExist(err) {
				t.Fatalf("rejected mutation prepared %s: %v", profile, err)
			}
		}
		if pid, err := process.ReadPID(pidFile); err != nil || pid != os.Getpid() {
			t.Fatalf("rejection changed the PID record: %d, %v", pid, err)
		}
	}
}

func TestManagedProjectMalformedPIDRefusesMutations(t *testing.T) {
	for _, raw := range []string{"", "not-a-pid", "0", "-1", "2147483648"} {
		t.Run(raw, func(t *testing.T) {
			p := managedProjectFixture(t)
			managedDB(t, p, "live").Close()
			managedDB(t, p, "dev").Close()
			if _, err := p.snapshotDB("live", "saved"); err != nil {
				t.Fatal(err)
			}
			pidFile, _ := p.statePath("live.pid")
			if err := os.WriteFile(pidFile, []byte(raw), 0600); err != nil {
				t.Fatal(err)
			}
			p.probe = func(int) error {
				t.Fatal("malformed PID reached a process probe")
				return nil
			}
			target, _ := p.dbPath("live")
			before, err := os.ReadFile(target)
			if err != nil {
				t.Fatal(err)
			}
			for _, action := range []func() error{
				func() error { return p.copyDB("dev", "live") },
				func() error { return p.restoreDB("live", "saved") },
				func() error { return p.resetDB("live") },
			} {
				if err := action(); err == nil {
					t.Fatal("malformed PID permitted destructive live operation")
				}
				after, err := os.ReadFile(target)
				if err != nil || !bytes.Equal(before, after) {
					t.Fatalf("refusal changed the live database: %v", err)
				}
				pid, err := os.ReadFile(pidFile)
				if err != nil || string(pid) != raw {
					t.Fatalf("refusal changed the malformed record: %q, %v", pid, err)
				}
			}
		})
	}
}

func TestManagedProjectDevOwnsProfileForRunnerLifetime(t *testing.T) {
	for _, runnerErr := range []error{nil, errors.New("runner failed")} {
		p := managedProjectFixture(t)
		p.foreground = func(opts process.ForegroundOptions) (int, error) {
			if release, err := process.LockProfile(p.global.DataRoot, p.Owner, p.Repo, "dev"); err == nil {
				release()
				t.Fatal("foreground runner did not hold dev ownership")
			}
			if err := p.resetDB("dev"); err == nil {
				t.Fatal("reset accepted active dev ownership")
			}
			path, _ := p.dbPath("dev")
			if opts.Env["DB"] != path || opts.Command != "true" || opts.PreHook != "true" || opts.Dir != p.Dir {
				t.Fatalf("foreground options lost config: %+v", opts)
			}
			if _, err := os.Stat(filepath.Dir(path)); err != nil {
				t.Fatal("foreground DB directory was not prepared")
			}
			return 7, runnerErr
		}
		started := false
		code, err := p.runDev(func() { started = true })
		if !started || code != 7 || !errors.Is(err, runnerErr) {
			t.Fatalf("foreground result: started=%v code=%d err=%v", started, code, err)
		}
		release, err := process.LockProfile(p.global.DataRoot, p.Owner, p.Repo, "dev")
		if err != nil {
			t.Fatalf("foreground did not release ownership: %v", err)
		}
		release()
	}
}

func TestManagedProjectSnapshotAllowsActiveSourceAndGuardsTargets(t *testing.T) {
	p := managedProjectFixture(t)
	managedDB(t, p, "live")
	releaseSource, err := process.LockProfile(p.global.DataRoot, p.Owner, p.Repo, "live")
	if err != nil {
		t.Fatal(err)
	}
	defer releaseSource()
	pidFile, _ := p.statePath("live.pid")
	if err := process.WritePID(pidFile, os.Getpid()); err != nil {
		t.Fatal(err)
	}
	if _, err := p.snapshotDB("live", "saved"); err != nil {
		t.Fatalf("active source snapshot refused: %v", err)
	}
	if err := p.copyDB("live", "dev"); err != nil {
		t.Fatalf("active source copy refused: %v", err)
	}
	target, _ := p.dbPath("dev")
	before, err := os.Stat(target)
	if err != nil {
		t.Fatal(err)
	}
	releaseTarget, err := process.LockProfile(p.global.DataRoot, p.Owner, p.Repo, "dev")
	if err != nil {
		t.Fatal(err)
	}
	for _, action := range []func() error{
		func() error { return p.restoreDB("dev", "saved") },
		func() error { return p.resetDB("dev") },
		func() error { return p.copyDB("live", "dev") },
	} {
		if err := action(); err == nil {
			t.Error("mutation accepted owned target")
		}
	}
	releaseTarget()
	lockFile, _ := p.statePath("dev.lock")
	lockBefore, _ := os.Stat(lockFile)
	if err := p.restoreDB("dev", "saved"); err != nil {
		t.Fatal(err)
	}
	after, _ := os.Stat(target)
	lockAfter, _ := os.Stat(lockFile)
	if !os.SameFile(before, after) || !os.SameFile(lockBefore, lockAfter) {
		t.Fatal("restore replaced the target or lock inode")
	}
	d, err := sql.Open("sqlite", target)
	if err != nil {
		t.Fatal(err)
	}
	var rowid, value int
	err = d.QueryRow("SELECT rowid,value FROM items").Scan(&rowid, &value)
	d.Close()
	if err != nil || rowid != 42 || value != 99 {
		t.Fatalf("restored WAL data / sparse ROWID: %d %d %v", rowid, value, err)
	}
	if err := p.resetDB("dev"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("reset retained DB: %v", err)
	}
}

func TestManagedProjectLiveTransitions(t *testing.T) {
	p := managedProjectFixture(t)
	p.Config.Dev.Port = 70000
	p.Config.Live.Port = 3000
	starts := 0
	started, err := p.startLive(func() { starts++ })
	if err != nil {
		t.Fatal(err)
	}
	needsStop := true
	t.Cleanup(func() {
		if needsStop {
			process.StopBackground(started.pid)
		}
	})
	again, err := p.startLive(func() { starts++ })
	if err != nil || !again.alreadyRunning || again.pid != started.pid || starts != 1 {
		t.Fatalf("repeated start: %+v starts=%d err=%v", again, starts, err)
	}
	stopped, err := p.stopLive(func(pid int) {
		if pid != started.pid {
			t.Fatalf("stop targeted a different PID: %d", pid)
		}
		if release, err := process.LockProfile(p.global.DataRoot, p.Owner, p.Repo, "live"); err == nil {
			release()
			t.Fatal("stop did not retain live ownership")
		}
	})
	if err == nil {
		needsStop = false
	}
	if err != nil || stopped.pid != started.pid || stopped.stale {
		t.Fatalf("stop result: %+v %v", stopped, err)
	}
	empty, err := p.stopLive(func(int) { t.Error("missing instance was signaled") })
	if err != nil || empty.pid != 0 {
		t.Fatalf("already stopped result: %+v %v", empty, err)
	}
}

func TestManagedProjectExecutionValidatesSelectedProfile(t *testing.T) {
	for _, profile := range []string{"dev", "live"} {
		for _, port := range []int{-1, 65536} {
			p := managedProjectFixture(t)
			p.Config.Dev.Port, p.Config.Live.Port = port, port
			var err error
			if profile == "dev" {
				_, err = p.runDev(func() { t.Error("invalid dev profile reached launch") })
			} else {
				_, err = p.startLive(func() { t.Error("invalid live profile reached launch") })
			}
			if err == nil {
				t.Fatalf("%s accepted port %d", profile, port)
			}
			if _, err := os.Stat(p.global.DataRoot); !os.IsNotExist(err) {
				t.Fatalf("invalid port created state: %v", err)
			}
		}
	}
	p := managedProjectFixture(t)
	p.Config.Live.Port, p.Config.Dev.Port = 70000, 3001
	p.foreground = func(process.ForegroundOptions) (int, error) { return 0, nil }
	if _, err := p.runDev(func() {}); err != nil {
		t.Fatalf("invalid unrelated live port blocked dev: %v", err)
	}
}

func TestManagedProjectBrowserPortSelection(t *testing.T) {
	for _, tc := range []struct {
		name            string
		live, dev, want int
		running         bool
	}{
		{name: "running live ignores bad dev", live: 3000, dev: 70000, want: 3000, running: true},
		{name: "dev ignores stopped bad live", live: 70000, dev: 3001, want: 3001},
		{name: "running invalid live", live: 70000, dev: 3001, running: true},
		{name: "running negative live", live: -1, dev: 3001, running: true},
		{name: "invalid dev selected", live: 3000, dev: -1},
		{name: "live fallback", live: 65535, want: 65535},
		{name: "minimum dev", dev: 1, want: 1},
		{name: "unset live uses dev", dev: 3001, want: 3001, running: true},
		{name: "unset ports"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := managedProjectFixture(t)
			p.Config.Live.Port, p.Config.Dev.Port = tc.live, tc.dev
			if tc.running {
				pidFile, _ := p.statePath("live.pid")
				if err := process.WritePID(pidFile, os.Getpid()); err != nil {
					t.Fatal(err)
				}
			}
			port, err := p.browserPort()
			if (err == nil) != (tc.want != 0) || port != tc.want {
				t.Fatalf("selected port = %d, err=%v, want %d", port, err, tc.want)
			}
		})
	}
}

func TestManagedProjectInvalidInputsCreateNoState(t *testing.T) {
	p := managedProjectFixture(t)
	for _, action := range []func() error{
		func() error { return p.copyDB("dev", "dev") },
		func() error { return p.copyDB("bad", "live") },
		func() error { _, err := p.snapshotDB("live", "../escape"); return err },
		func() error { return p.restoreDB("live", "../escape") },
		func() error { return p.resetDB("bad") },
	} {
		if err := action(); err == nil {
			t.Fatal("accepted invalid operation inputs")
		}
	}
	if _, err := os.Stat(p.global.DataRoot); !os.IsNotExist(err) {
		t.Fatalf("invalid inputs created state: %v", err)
	}
}
