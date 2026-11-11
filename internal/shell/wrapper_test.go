package shell

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestWrapperExecution(t *testing.T) {
	zsh, err := exec.LookPath("zsh")
	if err != nil {
		t.Skip("zsh unavailable")
	}
	dir := t.TempDir()
	binary := filepath.Join(dir, "spin with 'quotes'")
	if err := os.WriteFile(binary, []byte("#!/bin/sh\nfor arg in \"$@\"; do\n  case $arg in -h|--help) printf 'Usage: spin command\\nHelp text\\n'; exit 0;; esac\ndone\ncase $1 in\ncompletion) exit 0;;\ngo) printf 'diagnostic\\n' >&2; printf '%s\\n' \"$TARGET\"; exit \"$CODE\";;\n*) printf 'data\\n'; printf 'notice\\n' >&2;;\nesac\n"), 0755); err != nil {
		t.Fatal(err)
	}
	script, err := GenerateZsh(ZshConfig{SpinPath: binary})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, target, code, call string
		fail                     bool
		stdout                   string
	}{
		{"navigate", dir, "0", "spin go owner/repo; r=$?; print -r -- $PWD; exit $r", false, dir + "\n"},
		{"binary failure", dir, "7", "spin go owner/repo", true, ""},
		{"cd failure", filepath.Join(dir, "missing"), "0", "spin go owner/repo", true, ""},
		{"list without navigation", "owner/repo", "0", "spin go", false, "owner/repo\n"},
		{"streams", dir, "0", "spin list", false, "data\n"},
		{"go help", dir, "0", `spin go --help; r=$?; [[ -f "$XDG_CACHE_HOME/spin/projects.cache" ]] || exit 91; exit $r`, false, ""},
		{"new help", dir, "0", `spin new -h; r=$?; [[ -f "$XDG_CACHE_HOME/spin/projects.cache" ]] || exit 91; exit $r`, false, ""},
		{"clone help", dir, "0", `spin clone --help; r=$?; [[ -f "$XDG_CACHE_HOME/spin/projects.cache" ]] || exit 91; exit $r`, false, ""},
		{"picker quoting", dir, "0", `_spin_cached_projects() { :; }; _spin_fzf_widget`, false, "owner/a 'quoted'; echo injected\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cache := filepath.Join(dir, "spin", "projects.cache")
			if err := os.MkdirAll(filepath.Dir(cache), 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(cache, []byte("cached\n"), 0600); err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command(zsh, "-f")
			cmd.Stdin = strings.NewReader(`compdef() { :; }
fzf() { print -r -- "owner/a 'quoted'; echo injected"; }
bindkey() { :; }
zle() {
  if [[ $1 == accept-line ]]; then
    eval "set -- $BUFFER"
    print -r -- "$3"
    print picker >&2
  fi
}
` + script + "\n" + tc.call + "\n")
			cmd.Env = append(os.Environ(), "TARGET="+tc.target, "CODE="+tc.code, "XDG_CACHE_HOME="+dir)
			var out, stderr bytes.Buffer
			cmd.Stdout = &out
			cmd.Stderr = &stderr
			err := cmd.Run()
			if (err != nil) != tc.fail {
				t.Fatalf("exit: %v stderr=%s", err, &stderr)
			}
			if out.String() != tc.stdout {
				t.Fatalf("stdout=%q", out.String())
			}
			if stderr.Len() == 0 {
				t.Fatal("stderr lost")
			}
		})
	}
}
