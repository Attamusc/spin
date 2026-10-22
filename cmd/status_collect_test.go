package cmd

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/attamusc/spin/internal/config"
)

func statusFixture(t *testing.T, owner, repo, pid string) (*config.GlobalConfig, string, string) {
	t.Helper()
	root := t.TempDir()
	cfg := &config.GlobalConfig{ProjectsRoot: root, DataRoot: filepath.Join(root, ".spin")}
	path := filepath.Join(cfg.DataRoot, owner, repo, "live.pid")
	writeStatusFixture(t, path, pid)
	project := filepath.Join(root, "github.com", owner, repo, ".spin.toml")
	writeStatusFixture(t, project, "[dev]\ncommand='do not run'\n[live]\ncommand='do not run'\nport=3000\n")
	return cfg, path, project
}
func writeStatusFixture(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}
func hasStatusCode(r statusResult, code string) bool {
	for _, d := range r.Diagnostics {
		if d.Code == code {
			return true
		}
	}
	return false
}
func TestStatusPIDStates(t *testing.T) {
	for _, tc := range []struct {
		name, pid, state, code string
		probe                  error
	}{
		{"live", "123", "live", "", nil}, {"stale", "123", "stale", "", syscall.ESRCH},
		{"permission", "123", "unknown", "pid_probe_failed", syscall.EPERM}, {"other", "123", "unknown", "pid_probe_failed", syscall.EIO},
		{"empty", "", "malformed", "pid_malformed", nil}, {"text", "secret text", "malformed", "pid_malformed", nil}, {"overflow", "999999999999999999999999", "malformed", "pid_malformed", nil},
		{"pid_t_overflow", "4294967296", "malformed", "pid_malformed", nil}, {"zero", "0", "malformed", "pid_malformed", nil}, {"negative", "-1", "malformed", "pid_malformed", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg, path, _ := statusFixture(t, "owner", "repo", tc.pid)
			o := newStatusObserver()
			calls := 0
			o.probe = func(pid int) error {
				calls++
				if pid != 123 {
					t.Fatalf("invalid probe %d", pid)
				}
				return tc.probe
			}
			before, _ := os.Stat(path)
			o.now = func() time.Time { return before.ModTime().Add(42500 * time.Millisecond) }
			for i := 0; i < 2; i++ {
				r := o.collect(cfg)
				if len(r.Instances) != 1 {
					t.Fatalf("rows: %+v", r)
				}
				row := r.Instances[0]
				if row.State != tc.state || r.Complete != (tc.code == "") || tc.code != "" && !hasStatusCode(r, tc.code) {
					t.Fatalf("unexpected %+v", r)
				}
				if row.State == "malformed" && row.PID != nil {
					t.Fatal("malformed PID not null")
				}
				if row.State == "live" && (row.UptimeSeconds == nil || *row.UptimeSeconds != 42 || !row.UptimeEstimated) {
					t.Fatalf("uptime %+v", row)
				}
				if row.State != "live" && (row.UptimeSeconds != nil || row.UptimeEstimated) {
					t.Fatal("non-live uptime")
				}
			}
			after, err := os.Stat(path)
			raw, _ := os.ReadFile(path)
			if err != nil || !before.ModTime().Equal(after.ModTime()) || string(raw) != tc.pid {
				t.Fatal("mutated PID")
			}
			if tc.state == "malformed" && calls != 0 {
				t.Fatal("probed invalid PID")
			}
		})
	}
}
func TestStatusUncertainty(t *testing.T) {
	for _, tc := range []string{"read", "stat", "future", "owner", "repo", "root", "project_lookup", "project_host_read", "missing", "ambiguous", "bad_config", "unreadable_config", "invalid_port", "zero_port", "symlink_pid", "directory_pid", "symlink_config", "symlink_owner", "symlink_repo", "symlink_host", "symlink_project_owner", "symlink_project_repo"} {
		t.Run(tc, func(t *testing.T) {
			cfg, path, project := statusFixture(t, "owner", "repo", "123")
			o := newStatusObserver()
			o.probe = func(int) error { return nil }
			now := time.Now()
			o.now = func() time.Time { return now.Add(time.Hour) }
			want := ""
			origStat := o.lstat
			origRead := o.readFile
			origDir := o.readDir
			switch tc {
			case "read":
				want = "pid_unreadable"
				o.readFile = func(p string) ([]byte, error) {
					if p == path {
						return nil, os.ErrPermission
					}
					return origRead(p)
				}
			case "stat":
				want = "pid_stat_failed"
				count := 0
				o.lstat = func(p string) (os.FileInfo, error) {
					if p == path {
						count++
						if count == 2 {
							return nil, os.ErrPermission
						}
					}
					return origStat(p)
				}
			case "future":
				want = "pid_mtime_future"
				o.now = func() time.Time { return time.Unix(0, 0) }
			case "owner", "repo", "root":
				want = "scan_failed"
				bad := cfg.DataRoot
				if tc == "owner" {
					bad = filepath.Dir(filepath.Dir(path))
				}
				if tc == "repo" {
					bad = filepath.Dir(path)
				}
				o.readDir = func(p string) ([]os.DirEntry, error) {
					if p == bad {
						return nil, os.ErrPermission
					}
					return origDir(p)
				}
			case "project_host_read":
				want = "project_scan_failed"
				o.readDir = func(p string) ([]os.DirEntry, error) {
					if p == cfg.ProjectsRoot {
						return nil, os.ErrPermission
					}
					return origDir(p)
				}
			case "project_lookup":
				want = "project_scan_failed"
				o.lstat = func(p string) (os.FileInfo, error) {
					if p == filepath.Dir(project) {
						return nil, os.ErrPermission
					}
					return origStat(p)
				}
			case "missing":
				want = "project_missing"
				os.Remove(project)
			case "ambiguous":
				want = "project_ambiguous"
				writeStatusFixture(t, filepath.Join(cfg.ProjectsRoot, "gitlab.com", "owner", "repo", ".spin.toml"), "invalid")
			case "unreadable_config":
				want = "project_config_error"
				o.loadProject = func(string) (*config.ProjectConfig, error) { return nil, os.ErrPermission }
			case "bad_config":
				want = "project_config_error"
				writeStatusFixture(t, project, "secret-invalid-config-content")
			case "invalid_port":
				want = "invalid_port"
				writeStatusFixture(t, project, "[dev]\ncommand='x'\n[live]\ncommand='y'\nport=65536")
			case "zero_port":
				writeStatusFixture(t, project, "[dev]\ncommand='x'\n[live]\ncommand='y'")
			case "symlink_pid", "symlink_config":
				want = "unsupported_entry"
				p := path
				if tc == "symlink_config" {
					p = project
				}
				os.Remove(p)
				if err := os.Symlink("/no/such/outside", p); err != nil {
					t.Fatal(err)
				}
			case "directory_pid":
				want = "unsupported_entry"
				os.Remove(path)
				os.Mkdir(path, 0755)
			case "symlink_owner", "symlink_repo", "symlink_host", "symlink_project_owner", "symlink_project_repo":
				want = "unsupported_entry"
				p := filepath.Dir(filepath.Dir(path))
				if tc == "symlink_repo" {
					p = filepath.Dir(path)
				}
				if tc == "symlink_host" {
					p = filepath.Join(cfg.ProjectsRoot, "github.com")
				}
				if tc == "symlink_project_owner" {
					p = filepath.Dir(filepath.Dir(project))
				}
				if tc == "symlink_project_repo" {
					p = filepath.Dir(project)
				}
				os.RemoveAll(p)
				if err := os.Symlink(t.TempDir(), p); err != nil {
					t.Fatal(err)
				}
			}
			r := o.collect(cfg)
			if tc == "ambiguous" {
				first := filepath.Join(cfg.ProjectsRoot, "github.com", "owner", "repo")
				second := filepath.Join(cfg.ProjectsRoot, "gitlab.com", "owner", "repo")
				for _, d := range r.Diagnostics {
					if d.Code == "project_ambiguous" && (strings.Index(d.Message, first) < 0 || strings.Index(d.Message, second) <= strings.Index(d.Message, first)) {
						t.Fatalf("unsorted candidates: %s", d.Message)
					}
				}
			}
			if want != "" && (!hasStatusCode(r, want) || r.Complete) {
				t.Fatalf("wanted %s: %+v", want, r)
			}
			if want == "" && !r.Complete {
				t.Fatalf("unexpected %+v", r)
			}
			if len(r.Instances) > 0 {
				row := r.Instances[0]
				if (tc == "missing" || tc == "ambiguous" || tc == "project_lookup" || tc == "project_host_read") && (row.Host != nil || row.ProjectPath != nil || row.Port != nil) {
					t.Fatalf("invented identity %+v", row)
				}
				if tc == "bad_config" && (row.Host == nil || row.Port != nil) {
					t.Fatal("lost unique identity")
				}
				if (tc == "stat" || tc == "future") && (row.UptimeSeconds != nil || row.UptimeEstimated) {
					t.Fatal("invented uptime")
				}
				encoded, _ := json.Marshal(r)
				if strings.Contains(string(encoded), "secret-invalid") {
					t.Fatal("config leak")
				}
			}
		})
	}
}
func TestStatusPartialSortingAndEscaping(t *testing.T) {
	cfg, _, _ := statusFixture(t, "z", "repo", "123")
	names := []string{"é", "a\"\\\t\n", "b"}
	for _, name := range names {
		writeStatusFixture(t, filepath.Join(cfg.DataRoot, name, "repo", "live.pid"), "0")
	}
	o := newStatusObserver()
	o.probe = func(int) error { return syscall.ESRCH }
	orig := o.readDir
	o.readDir = func(p string) ([]os.DirEntry, error) {
		if p == filepath.Join(cfg.DataRoot, "b") {
			return nil, os.ErrPermission
		}
		return orig(p)
	}
	first := o.collect(cfg)
	second := o.collect(cfg)
	if !reflect.DeepEqual(first, second) || len(first.Instances) != 3 || !hasStatusCode(first, "scan_failed") {
		t.Fatalf("bad partial result %+v", first)
	}
	if first.Instances[0].Owner != names[1] || first.Instances[1].Owner != "z" {
		t.Fatal("not sorted")
	}
	raw, err := json.Marshal(first)
	if err != nil {
		t.Fatal(err)
	}
	var decoded statusResult
	if err = json.Unmarshal(raw, &decoded); err != nil || !reflect.DeepEqual(first, decoded) {
		t.Fatalf("roundtrip: %v", err)
	}
}
func TestStatusPartialDirectoryRead(t *testing.T) {
	cfg, _, _ := statusFixture(t, "o", "r", "123")
	o := newStatusObserver()
	o.probe = func(int) error { return syscall.ESRCH }
	original := o.readDir
	o.readDir = func(path string) ([]os.DirEntry, error) {
		entries, err := original(path)
		if path == cfg.DataRoot {
			return entries, os.ErrPermission
		}
		return entries, err
	}
	r := o.collect(cfg)
	if r.Complete || len(r.Instances) != 1 || !hasStatusCode(r, "scan_failed") {
		t.Fatalf("partial directory entries lost: %+v", r)
	}
}

func TestStatusDisappearedPID(t *testing.T) {
	cfg, path, _ := statusFixture(t, "o", "r", "123")
	o := newStatusObserver()
	o.readFile = func(string) ([]byte, error) { return nil, os.ErrNotExist }
	r := o.collect(cfg)
	if !r.Complete || len(r.Instances) != 0 {
		t.Fatalf("disappeared PID %s: %+v", path, r)
	}
}
func TestStatusWriteFailure(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	os.Mkdir(filepath.Join(home, "projects"), 0755)
	oldOut := statusCmd.OutOrStdout()
	oldErr := statusCmd.ErrOrStderr()
	defer statusCmd.SetOut(oldOut)
	defer statusCmd.SetErr(oldErr)
	statusCmd.SetOut(statusFailWriter{})
	var stderr strings.Builder
	statusCmd.SetErr(&stderr)
	statusCmd.Flags().Set("format", "json")
	defer statusCmd.Flags().Set("format", "plain")
	if err := runStatus(statusCmd, nil); err == nil || !strings.Contains(stderr.String(), "Could not write") {
		t.Fatalf("output failure not surfaced: %v %s", err, stderr.String())
	}
}

type statusFailWriter struct{}

func (statusFailWriter) Write([]byte) (int, error) { return 0, errors.New("fixture output failure") }
