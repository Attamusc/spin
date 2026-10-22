package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

type observationManifestEntry struct {
	Mode    os.FileMode
	Size    int64
	Mtime   time.Time
	Content string
}

func observationManifest(t *testing.T, root string) map[string]observationManifestEntry {
	t.Helper()
	m := map[string]observationManifestEntry{}
	err := filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		e := observationManifestEntry{Mode: info.Mode(), Size: info.Size(), Mtime: info.ModTime()}
		if info.Mode().IsRegular() {
			raw, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			e.Content = string(raw)
		}
		m[p] = e
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return m
}
func TestStatusCLIContractMatrix(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "spin")
	if out, err := exec.Command("go", "build", "-o", binary, "..").CombinedOutput(); err != nil {
		t.Fatalf("build: %v %s", err, out)
	}
	for _, tc := range []string{"missing_defaults", "root_file", "bad_global", "empty", "empty_data", "live", "stale", "malformed", "bad_project", "ambiguous", "symlink"} {
		t.Run(tc, func(t *testing.T) {
			home := t.TempDir()
			root := filepath.Join(home, "projects")
			pid := filepath.Join(root, ".spin", "o", "r", "live.pid")
			project := filepath.Join(root, "github.com", "o", "r", ".spin.toml")
			complete := tc == "empty" || tc == "empty_data" || tc == "live" || tc == "stale"
			const secret = "STATUS_SECRET_DO_NOT_EMIT"
			marker := filepath.Join(home, "executed")
			bin := filepath.Join(home, "bin")
			for _, name := range []string{"sh", "bash", "git", "jj", "npm", "pnpm", "yarn", "go", "curl", "wget", "ssh", "gh", "spin", "kill", "sentinel"} {
				p := filepath.Join(bin, name)
				writeStatusFixture(t, p, "#!/bin/sh\nprintf executed >> '"+marker+"'\nexit 91\n")
				if err := os.Chmod(p, 0755); err != nil {
					t.Fatal(err)
				}
			}
			// Positive control: the same wrapper must leave observable evidence.
			if err := exec.Command(filepath.Join(bin, "sentinel")).Run(); err == nil {
				t.Fatal("sentinel unexpectedly succeeded")
			}
			if raw, err := os.ReadFile(marker); err != nil || string(raw) != "executed" {
				t.Fatalf("insensitive sentinel: %q %v", raw, err)
			}
			if err := os.Remove(marker); err != nil {
				t.Fatal(err)
			}
			if tc == "empty_data" {
				if err := os.MkdirAll(filepath.Join(root, ".spin"), 0755); err != nil {
					t.Fatal(err)
				}
			}
			if tc != "missing_defaults" && tc != "root_file" {
				os.MkdirAll(root, 0755)
			}
			if tc == "root_file" {
				os.WriteFile(root, []byte("not directory"), 0644)
			}
			if tc == "bad_global" {
				writeStatusFixture(t, filepath.Join(home, ".config", "spin", "config.toml"), secret+" invalid config")
			}
			if tc != "missing_defaults" && tc != "root_file" && tc != "bad_global" && tc != "empty" && tc != "empty_data" {
				writeStatusFixture(t, project, "[dev]\ncommand='sentinel'\npre_hook='sentinel'\n[dev.env]\nSECRET='"+secret+"'\n[live]\ncommand='sentinel'\npre_hook='sentinel'\nport=3000\n[live.env]\nSECRET='"+secret+"'")
				value := fmt.Sprint(os.Getpid())
				if tc == "stale" {
					child := exec.Command("sh", "-c", "exit 0")
					if err := child.Run(); err != nil {
						t.Fatal(err)
					}
					value = fmt.Sprint(child.ProcessState.Pid())
				}
				if tc == "malformed" {
					value = "-1"
				}
				writeStatusFixture(t, pid, value)
				if tc == "bad_project" {
					writeStatusFixture(t, project, secret+" broken config")
				}
				if tc == "ambiguous" {
					writeStatusFixture(t, filepath.Join(root, "gitlab.com", "o", "r", ".spin.toml"), "secret")
				}
				if tc == "symlink" {
					os.Remove(pid)
					if err := os.Symlink(project, pid); err != nil {
						t.Fatal(err)
					}
				}
			}
			before := observationManifest(t, home)
			for repeat := 0; repeat < 2; repeat++ {
				for _, args := range [][]string{{"status", "--format", "json"}, {"status"}, {"status", "--format", "plain"}, {}} {
					c := exec.Command(binary, args...)
					c.Env = append(os.Environ(), "HOME="+home, "CLICOLOR_FORCE=1", "PATH="+bin)
					var stdout, stderr bytes.Buffer
					c.Stdout = &stdout
					c.Stderr = &stderr
					err := c.Run()
					if err != nil {
						if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 1 {
							t.Fatalf("exit must be 1: %v", err)
						}
					}
					if strings.Contains(stdout.String(), secret) || strings.Contains(stderr.String(), secret) {
						t.Fatalf("secret leaked: %s %s", stdout.Bytes(), stderr.Bytes())
					}
					if !complete && stderr.Len() == 0 {
						t.Fatal("missing diagnostics")
					}
					if (err == nil) != complete {
						t.Fatalf("args=%q complete=%v err=%v stdout=%s stderr=%s", args, complete, err, stdout.Bytes(), stderr.Bytes())
					}
					if len(args) == 0 {
						if !complete && (!strings.Contains(stderr.String(), "Status observation incomplete.") || !strings.Contains(stderr.String(), map[string]string{"missing_defaults": "global_config_error:", "root_file": "global_config_error:", "bad_global": "global_config_error:", "malformed": "pid_malformed:", "bad_project": "project_config_error:", "ambiguous": "project_ambiguous:", "symlink": "unsupported_entry:"}[tc])) {
							t.Fatalf("dashboard missing diagnostic and incompleteness: %s", stderr.Bytes())
						}
						if stdout.Len() != 0 || !strings.Contains(stderr.String(), "Common commands:") {
							t.Fatalf("dashboard streams: %s %s", stdout.Bytes(), stderr.Bytes())
						}
						continue
					}
					if len(args) == 3 && args[2] == "json" {
						raw := append([]byte(nil), stdout.Bytes()...)
						assertStatusJSONFields(t, raw)
						var result statusResult
						decoder := json.NewDecoder(&stdout)
						if err := decoder.Decode(&result); err != nil {
							t.Fatalf("decode: %v %s", err, stdout.Bytes())
						}
						if err := decoder.Decode(new(any)); err != io.EOF {
							t.Fatalf("trailing JSON/output: %v", err)
						}
						if tc == "missing_defaults" || tc == "root_file" || tc == "bad_global" {
							if result.ProjectsRoot != nil || result.DataRoot != nil || !hasStatusCode(result, "global_config_error") {
								t.Fatalf("global failure: %+v", result)
							}
						}
						if result.Complete != complete || result.SchemaVersion != 1 || result.Instances == nil || result.Diagnostics == nil {
							t.Fatalf("envelope %+v", result)
						}
						if complete && stderr.Len() != 0 {
							t.Fatalf("clean stderr=%s", stderr.Bytes())
						}
						if !complete && (strings.Contains(stderr.String(), "Usage:") || strings.Contains(stderr.String(), "Error:")) {
							t.Fatalf("duplicate Cobra output: %s", stderr.Bytes())
						}
						if len(result.Instances) > 0 {
							row := result.Instances[0]
							if tc == "stale" && row.State != "stale" {
								t.Fatalf("stale row %+v", row)
							}
							if tc == "live" && row.State != "live" {
								t.Fatalf("live row %+v", row)
							}
						}
					} else {
						text := stdout.String()
						if tc == "empty" || tc == "empty_data" {
							if !strings.Contains(text, "No live instances running.") {
								t.Fatalf("empty wording: %s", text)
							}
						}
						if !complete && strings.Contains(text, "No live instances running") {
							t.Fatalf("false empty: %s", text)
						}
						if tc == "bad_global" || tc == "missing_defaults" || tc == "root_file" {
							if !strings.Contains(text, "Status observation incomplete.") {
								t.Fatalf("incomplete wording: %s", text)
							}
						}
						if tc == "malformed" {
							if !strings.Contains(text, "STATE") || !strings.Contains(text, "malformed") || !strings.Contains(strings.Join(strings.Fields(text), " "), "o/r live malformed 3000 - -") {
								t.Fatalf("plain null/state: %s", text)
							}
						}
					}
				}
			}
			if _, err := os.Stat(marker); !os.IsNotExist(err) {
				t.Fatalf("observation executed a sentinel: %v", err)
			}
			after := observationManifest(t, home)
			if !reflect.DeepEqual(before, after) {
				t.Fatalf("observation mutated tree\nbefore=%+v\nafter=%+v", before, after)
			}
		})
	}
}
