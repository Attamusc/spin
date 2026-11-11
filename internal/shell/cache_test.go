package shell

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCacheRetainsAvailablePartialResults(t *testing.T) {
	zsh, err := exec.LookPath("zsh")
	if err != nil {
		t.Skip("zsh unavailable")
	}
	binary := filepath.Join(t.TempDir(), "spin")
	if err := os.WriteFile(binary, []byte("#!/bin/sh\ncase $1 in\ncompletion) exit 0;;\nlist) printf '%s' \"$LIST_OUTPUT\"; printf warning >&2; exit \"$LIST_STATUS\";;\nesac\n"), 0755); err != nil {
		t.Fatal(err)
	}
	generated, err := GenerateZsh(ZshConfig{SpinPath: binary})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, old, output, status, call, want string
		fail                                  bool
	}{
		{"first partial", "", "owner/available\n", "1", "_spin_cached_projects", "owner/available\n", false},
		{"replace old with partial", "owner/old\n", "owner/available\n", "1", `_spin_refresh_cache; [[ $? == 1 ]] || exit 92; _spin_cached_projects`, "owner/available\n", false},
		{"failed empty keeps old", "owner/old\n", "", "1", `_spin_refresh_cache; [[ $? == 1 ]] || exit 92; _spin_cached_projects`, "owner/old\n", false},
		{"complete empty replaces old", "owner/old\n", "", "0", `_spin_refresh_cache || exit 92; _spin_cached_projects`, "", false},
		{"failed empty without cache", "", "", "1", "_spin_cached_projects", "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			cache := filepath.Join(root, "spin", "projects.cache")
			if tc.old != "" {
				if err := os.MkdirAll(filepath.Dir(cache), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(cache, []byte(tc.old), 0600); err != nil {
					t.Fatal(err)
				}
			}
			cmd := exec.Command(zsh, "-f")
			cmd.Stdin = strings.NewReader("compdef() { :; }\n" + generated + "\n" + tc.call + "\n")
			cmd.Env = append(os.Environ(), "HOME="+root, "XDG_CACHE_HOME="+root, "LIST_OUTPUT="+tc.output, "LIST_STATUS="+tc.status)
			var out, stderr bytes.Buffer
			cmd.Stdout, cmd.Stderr = &out, &stderr
			err := cmd.Run()
			if (err != nil) != tc.fail || out.String() != tc.want || stderr.Len() != 0 {
				t.Fatalf("exit=%v stdout=%q stderr=%q", err, out.String(), stderr.String())
			}
			if !tc.fail {
				data, err := os.ReadFile(cache)
				if err != nil || string(data) != tc.want {
					t.Fatalf("cache=%q err=%v", data, err)
				}
			}
			if leftovers, _ := filepath.Glob(cache + ".tmp.*"); len(leftovers) != 0 {
				t.Fatalf("cache temporary files leaked: %q", leftovers)
			}
		})
	}
}
