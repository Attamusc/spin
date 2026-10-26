package cmd

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestListCLI(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "spin")
	build := exec.Command("go", "build", "-o", binary, "..")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v: %s", err, out)
	}
	for _, tc := range []struct {
		name      string
		args      []string
		projects  bool
		missing   bool
		badConfig bool
		want      string
		wantError string
	}{
		{name: "empty plain", args: []string{"--format", "plain"}},
		{name: "empty configured plain", args: []string{"--format", "plain", "--configured"}},
		{name: "unconfigured only plain", args: []string{"--format", "plain", "--configured"}, projects: true},
		{name: "empty human", want: "No projects found.\nClone a repo with 'spin clone <repo>' or create one with 'spin new <name>'.\n"},
		{name: "empty configured human", args: []string{"--configured"}, want: "No configured projects found.\nAdd a .spin.toml file to a project under %s to get started.\nRun 'spin init' in a project directory to generate one.\n"},
		{name: "nonempty plain", args: []string{"--format", "plain"}, projects: true, want: "owner/alpha\nowner/beta\n"},
		{name: "configured filtering", args: []string{"--format", "plain", "--configured"}, projects: true, want: "owner/alpha\n"},
		{name: "missing root", args: []string{"--format", "plain"}, missing: true, wantError: "projects_root directory does not exist"},
		{name: "missing root configured", args: []string{"--format", "plain", "--configured"}, missing: true, wantError: "projects_root directory does not exist"},
		{name: "bad config", args: []string{"--format", "plain"}, badConfig: true, wantError: "could not parse config file"},
		{name: "bad config configured", args: []string{"--format", "plain", "--configured"}, badConfig: true, wantError: "could not parse config file"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()
			root := filepath.Join(home, "projects")
			if !tc.missing {
				if err := os.MkdirAll(root, 0755); err != nil {
					t.Fatal(err)
				}
			}
			if tc.projects {
				for _, repo := range []string{"alpha", "beta"} {
					dir := filepath.Join(root, "github.com", "owner", repo)
					if err := os.MkdirAll(dir, 0755); err != nil {
						t.Fatal(err)
					}
					if repo == "alpha" && tc.name != "unconfigured only plain" {
						if err := os.WriteFile(filepath.Join(dir, ".spin.toml"), nil, 0644); err != nil {
							t.Fatal(err)
						}
					}
				}
			}
			if tc.badConfig {
				dir := filepath.Join(home, ".config", "spin")
				if err := os.MkdirAll(dir, 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, "config.toml"), []byte("projects_root = ["), 0644); err != nil {
					t.Fatal(err)
				}
			}
			c := exec.Command(binary, append([]string{"list"}, tc.args...)...)
			c.Env = append(os.Environ(), "HOME="+home, "NO_COLOR=1")
			var stdout, stderr bytes.Buffer
			c.Stdout, c.Stderr = &stdout, &stderr
			err := c.Run()
			if tc.wantError != "" {
				exit, ok := err.(*exec.ExitError)
				if !ok || exit.ExitCode() != 1 || stdout.Len() != 0 || !strings.Contains(stderr.String(), tc.wantError) {
					t.Fatalf("error result: %v stdout=%q stderr=%q", err, stdout.String(), stderr.String())
				}
				return
			}
			want := tc.want
			if strings.Contains(want, "%s") {
				want = fmt.Sprintf(want, root)
			}
			if err != nil || stdout.String() != want || stderr.Len() != 0 {
				t.Fatalf("result: %v stdout=%q want=%q stderr=%q", err, stdout.String(), want, stderr.String())
			}
		})
	}
}
