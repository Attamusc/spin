package cmd

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/attamusc/spin/internal/config"
)

type statusInstance struct {
	Owner           string  `json:"owner"`
	Repo            string  `json:"repo"`
	Mode            string  `json:"mode"`
	PIDFile         string  `json:"pid_file"`
	State           string  `json:"state"`
	PID             *int    `json:"pid"`
	Host            *string `json:"host"`
	ProjectPath     *string `json:"project_path"`
	Port            *int    `json:"port"`
	UptimeSeconds   *int64  `json:"uptime_seconds"`
	UptimeEstimated bool    `json:"uptime_estimated"`
}
type statusDiagnostic struct {
	Code    string  `json:"code"`
	Path    *string `json:"path"`
	Message string  `json:"message"`
}
type statusResult struct {
	SchemaVersion int                `json:"schema_version"`
	Complete      bool               `json:"complete"`
	ProjectsRoot  *string            `json:"projects_root"`
	DataRoot      *string            `json:"data_root"`
	Instances     []statusInstance   `json:"instances"`
	Diagnostics   []statusDiagnostic `json:"diagnostics"`
}

// The observation seams keep errno and metadata uncertainty testable without
// changing the lifecycle helpers' cleanup policies.
type statusObserver struct {
	readDir     func(string) ([]os.DirEntry, error)
	lstat       func(string) (os.FileInfo, error)
	readFile    func(string) ([]byte, error)
	probe       func(int) error
	now         func() time.Time
	loadGlobal  func() (*config.GlobalConfig, error)
	loadProject func(string) (*config.ProjectConfig, error)
}

func newStatusObserver() statusObserver {
	return statusObserver{
		readDir: os.ReadDir, lstat: os.Lstat, readFile: os.ReadFile,
		probe: func(pid int) error { return syscall.Kill(pid, 0) }, now: time.Now,
		loadGlobal: config.LoadGlobal, loadProject: config.LoadProject,
	}
}
func emptyStatus() statusResult {
	return statusResult{SchemaVersion: 1, Complete: true, Instances: []statusInstance{}, Diagnostics: []statusDiagnostic{}}
}
func (r *statusResult) diagnostic(code, path, message string) {
	var p *string
	if path != "" {
		p = &path
	}
	r.Diagnostics = append(r.Diagnostics, statusDiagnostic{code, p, message})
	r.Complete = false
}
func (r *statusResult) sort() {
	sort.Slice(r.Instances, func(i, j int) bool {
		a, b := r.Instances[i], r.Instances[j]
		if a.Owner != b.Owner {
			return a.Owner < b.Owner
		}
		if a.Repo != b.Repo {
			return a.Repo < b.Repo
		}
		return a.PIDFile < b.PIDFile
	})
	sort.Slice(r.Diagnostics, func(i, j int) bool {
		a, b := r.Diagnostics[i], r.Diagnostics[j]
		if a.Path == nil && b.Path != nil {
			return true
		}
		if a.Path != nil && b.Path == nil {
			return false
		}
		if a.Path != nil && *a.Path != *b.Path {
			return *a.Path < *b.Path
		}
		if a.Code != b.Code {
			return a.Code < b.Code
		}
		return a.Message < b.Message
	})
}
func observeStatus() statusResult { return newStatusObserver().observe() }

func (o statusObserver) observe() (r statusResult) {
	r = emptyStatus()
	defer func() { r.sort() }()
	// Global parsing errors can contain source text. Emit only our fixed message.
	home, err := os.UserHomeDir()
	if err != nil {
		r.diagnostic("global_config_error", "", "Could not resolve home directory")
		return r
	}
	configPath := filepath.Join(home, ".config", "spin", "config.toml")
	if info, e := o.lstat(configPath); e == nil && !info.Mode().IsRegular() {
		r.diagnostic("unsupported_entry", configPath, "Global config is not a regular file")
		r.diagnostic("global_config_error", configPath, "Could not load global configuration")
		return r
	}
	cfg, err := o.loadGlobal()
	if err != nil {
		r.diagnostic("global_config_error", configPath, "Could not load or validate global configuration")
		return r
	}
	return o.collect(cfg)
}
func (o statusObserver) collect(cfg *config.GlobalConfig) statusResult {
	r := emptyStatus()
	projects, err := filepath.Abs(cfg.ProjectsRoot)
	if err != nil {
		r.diagnostic("global_config_error", "", "Could not resolve projects root")
		return r
	}
	data, err := filepath.Abs(cfg.DataRoot)
	if err != nil {
		r.diagnostic("global_config_error", "", "Could not resolve data root")
		return r
	}
	r.ProjectsRoot = &projects
	r.DataRoot = &data
	owners, err := o.readDir(data)
	if err != nil {
		if !os.IsNotExist(err) {
			r.diagnostic("scan_failed", data, "Could not read data root")
		}
	}
	for _, owner := range owners {
		ownerPath := filepath.Join(data, owner.Name())
		if owner.Type()&os.ModeSymlink != 0 {
			r.diagnostic("unsupported_entry", ownerPath, "Symlink owner directory is unsupported")
			continue
		}
		if !owner.IsDir() {
			continue
		}
		repos, err := o.readDir(ownerPath)
		if err != nil {
			r.diagnostic("scan_failed", ownerPath, "Could not read owner directory")
		}
		for _, repo := range repos {
			repoPath := filepath.Join(ownerPath, repo.Name())
			if repo.Type()&os.ModeSymlink != 0 {
				r.diagnostic("unsupported_entry", repoPath, "Symlink repository directory is unsupported")
				continue
			}
			if !repo.IsDir() {
				continue
			}
			// Read the directory as well: an inaccessible repository must not look empty.
			if files, err := o.readDir(repoPath); err != nil {
				r.diagnostic("scan_failed", repoPath, "Could not read repository directory")
				found := false
				for _, file := range files {
					if file.Name() == "live.pid" {
						found = true
						break
					}
				}
				if !found {
					continue
				}
			}
			path := filepath.Join(repoPath, "live.pid")
			info, err := o.lstat(path)
			if os.IsNotExist(err) {
				continue
			}
			row := statusInstance{Owner: owner.Name(), Repo: repo.Name(), Mode: "live", PIDFile: path, State: "unreadable"}
			if err != nil {
				r.diagnostic("pid_stat_failed", path, "Could not stat PID file")
				r.diagnostic("pid_unreadable", path, "Could not inspect PID file")
			} else if !info.Mode().IsRegular() {
				r.diagnostic("unsupported_entry", path, "PID file is not a regular file")
				r.diagnostic("pid_unreadable", path, "Could not read PID file")
			} else {
				raw, err := o.readFile(path)
				if os.IsNotExist(err) {
					continue
				}
				if err != nil {
					r.diagnostic("pid_unreadable", path, "Could not read PID file")
				} else if parsed, err := strconv.ParseInt(strings.TrimSpace(string(raw)), 10, 32); err != nil || parsed <= 0 {
					row.State = "malformed"
					r.diagnostic("pid_malformed", path, "PID must be a positive integer")
				} else {
					// pid_t is signed 32-bit on supported platforms. Avoid a
					// narrowing conversion turning a positive value into a group probe.
					pid := int(parsed)
					row.PID = &pid
					switch err := o.probe(pid); {
					case err == nil:
						row.State = "live"
					case errors.Is(err, syscall.ESRCH):
						row.State = "stale"
					default:
						row.State = "unknown"
						r.diagnostic("pid_probe_failed", path, "Could not determine PID existence")
					}
					if row.State == "live" {
						// Re-stat so a failed metadata observation never becomes zero uptime.
						metadata, err := o.lstat(path)
						if err != nil {
							r.diagnostic("pid_stat_failed", path, "Could not stat PID file for estimated uptime")
						} else if age := o.now().Sub(metadata.ModTime()); age < 0 {
							r.diagnostic("pid_mtime_future", path, "PID file modification time is in the future")
						} else {
							seconds := int64(age / time.Second)
							row.UptimeSeconds = &seconds
							row.UptimeEstimated = true
						}
					}
				}
			}
			o.project(&r, &row, projects)
			r.Instances = append(r.Instances, row)
		}
	}
	r.sort()
	return r
}
func (o statusObserver) project(r *statusResult, row *statusInstance, root string) {
	hosts, err := o.readDir(root)
	if err != nil {
		r.diagnostic("project_scan_failed", root, "Could not scan project hosts")
		return
	}
	type candidate struct{ host, path string }
	var candidates []candidate
	failed := false
	for _, host := range hosts {
		if strings.HasPrefix(host.Name(), ".") {
			continue
		}
		hostPath := filepath.Join(root, host.Name())
		if host.Type()&os.ModeSymlink != 0 {
			r.diagnostic("unsupported_entry", hostPath, "Symlink host directory is unsupported")
			failed = true
			continue
		}
		if !host.IsDir() {
			continue
		}
		dir := hostPath
		skip := false
		for _, component := range []string{row.Owner, row.Repo} {
			dir = filepath.Join(dir, component)
			info, err := o.lstat(dir)
			if os.IsNotExist(err) {
				skip = true
				break
			}
			if err != nil {
				r.diagnostic("project_scan_failed", dir, "Could not inspect project directory")
				failed = true
				skip = true
				break
			}
			if info.Mode()&os.ModeSymlink != 0 {
				r.diagnostic("unsupported_entry", dir, "Symlink project directory is unsupported")
				failed = true
				skip = true
				break
			}
			if !info.IsDir() {
				skip = true
				break
			}
		}
		if skip {
			continue
		}
		path := filepath.Join(dir, ".spin.toml")
		_, err := o.lstat(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			r.diagnostic("project_scan_failed", path, "Could not inspect project configuration")
			failed = true
			continue
		}
		candidates = append(candidates, candidate{host.Name(), dir})
	}
	if failed {
		r.diagnostic("project_scan_failed", root, "Project uniqueness could not be established")
		return
	}
	if len(candidates) == 0 {
		r.diagnostic("project_missing", row.PIDFile, "No matching configured project")
		return
	}
	if len(candidates) > 1 {
		paths := make([]string, len(candidates))
		for i, c := range candidates {
			paths[i] = c.path
		}
		sort.Strings(paths)
		r.diagnostic("project_ambiguous", row.PIDFile, "Multiple matching projects: "+strings.Join(paths, ", "))
		return
	}
	c := candidates[0]
	row.Host = &c.host
	row.ProjectPath = &c.path
	path := filepath.Join(c.path, ".spin.toml")
	info, err := o.lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		if err == nil {
			r.diagnostic("unsupported_entry", path, "Project config is not a regular file")
		}
		r.diagnostic("project_config_error", path, "Could not read project configuration")
		return
	}
	cfg, err := o.loadProject(c.path)
	if err != nil {
		r.diagnostic("project_config_error", path, "Could not load or validate project configuration")
		return
	}
	if cfg.Live.Port < 0 || cfg.Live.Port > 65535 {
		r.diagnostic("invalid_port", path, "Live port must be between 1 and 65535")
		return
	}
	if cfg.Live.Port > 0 {
		port := cfg.Live.Port
		row.Port = &port
	}
}
