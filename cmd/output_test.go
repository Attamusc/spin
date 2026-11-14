package cmd

import (
	"bytes"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/spf13/cobra"
)

func commandOutputProject(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	project := filepath.Join(home, "projects", "example.com", "owner", "repo")
	if err := os.MkdirAll(project, 0755); err != nil {
		t.Fatal(err)
	}
	return project
}

func TestCLIDataWriteFailures(t *testing.T) {
	full, err := os.OpenFile("/dev/full", os.O_WRONLY, 0)
	if err != nil {
		t.Skip("/dev/full unavailable; portable handler tests cover writer failures")
	}
	defer full.Close()
	binary := buildSpin(t)
	project := commandOutputProject(t)
	for _, args := range [][]string{
		{"go", "owner/repo"}, {"go"}, {"list", "--format", "plain"}, {"list"},
		{"shell-init", "zsh"}, {"completion", "bash"}, {"completion", "zsh"},
		{"completion", "fish"}, {"completion", "powershell"}, {"status", "--format", "json"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			cmd := spinCommand(t, binary, os.Getenv("HOME"), project, args...)
			cmd.Stdout = full
			var stderr bytes.Buffer
			cmd.Stderr = &stderr
			err := cmd.Run()
			var exit *exec.ExitError
			if !errors.As(err, &exit) || exit.ExitCode() != 1 {
				t.Fatalf("write failure exit = %v, want 1; stderr = %q", err, stderr.String())
			}
		})
	}
}

func TestGoUsesCommandWriters(t *testing.T) {
	project := commandOutputProject(t)
	cmd := &cobra.Command{}
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	if err := runGo(cmd, []string{"owner/repo"}); err != nil {
		t.Fatal(err)
	}
	if stdout.String() != project+"\n" {
		t.Fatalf("navigation stdout = %q, want %q", stdout.String(), project+"\n")
	}
	if !bytes.Contains(stderr.Bytes(), []byte("owner/repo")) {
		t.Fatalf("navigation diagnostic missing from command stderr: %q", stderr.String())
	}
}

func TestCompletionUsesExecutingRootAndWriter(t *testing.T) {
	for _, shell := range []string{"bash", "zsh", "fish", "powershell"} {
		t.Run(shell, func(t *testing.T) {
			root := &cobra.Command{Use: "fixture"}
			cmd := &cobra.Command{Use: "completion", RunE: completionCmd.RunE}
			root.AddCommand(cmd)
			var stdout bytes.Buffer
			cmd.SetOut(&stdout)
			if err := cmd.RunE(cmd, []string{shell}); err != nil {
				t.Fatal(err)
			}
			if !bytes.Contains(stdout.Bytes(), []byte("fixture")) {
				t.Fatal("completion did not use the executing command root and writer")
			}
			cmd.SetOut(failingCommandWriter{})
			if err := cmd.RunE(cmd, []string{shell}); !errors.Is(err, io.ErrClosedPipe) {
				t.Fatalf("output failure = %v, want closed pipe", err)
			}
		})
	}
}

func TestShellInitUsesCommandWriter(t *testing.T) {
	cmd := &cobra.Command{}
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	if err := runShellInit(cmd, []string{"zsh"}); err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(stdout.Bytes(), []byte("# spin shell integration")) {
		t.Fatal("shell integration was not written to the command writer")
	}
	cmd.SetOut(failingCommandWriter{})
	if err := runShellInit(cmd, []string{"zsh"}); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("output failure = %v, want closed pipe", err)
	}
}

type failingCommandWriter struct{}

func (failingCommandWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestProjectListPreservesScanAndWriteErrors(t *testing.T) {
	for _, tc := range []struct {
		name   string
		run    func(*cobra.Command, []string) error
		format string
	}{
		{name: "go", run: runGo},
		{name: "plain list", run: runList, format: "plain"},
		{name: "table list", run: runList},
	} {
		t.Run(tc.name, func(t *testing.T) {
			project := commandOutputProject(t)
			broken := filepath.Join(filepath.Dir(project), "broken")
			if err := os.Mkdir(broken, 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(".spin.toml", filepath.Join(broken, ".spin.toml")); err != nil {
				t.Fatal(err)
			}
			cmd := &cobra.Command{}
			cmd.Flags().Bool("configured", false, "")
			cmd.Flags().String("format", tc.format, "")
			cmd.SetOut(failingCommandWriter{})
			cmd.SetErr(io.Discard)
			err := tc.run(cmd, nil)
			if !errors.Is(err, syscall.ELOOP) || !errors.Is(err, io.ErrClosedPipe) {
				t.Fatalf("lost scan or write failure: %v", err)
			}
		})
	}
}

func TestProjectListWriteFailure(t *testing.T) {
	for _, tc := range []struct {
		name       string
		run        func(*cobra.Command, []string) error
		args       []string
		format     string
		empty      bool
		configured bool
	}{
		{name: "go named", run: runGo, args: []string{"owner/repo"}},
		{name: "go inventory", run: runGo},
		{name: "plain list", run: runList, format: "plain"},
		{name: "table list", run: runList},
		{name: "empty list", run: runList, empty: true},
		{name: "empty configured list", run: runList, empty: true, configured: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			project := commandOutputProject(t)
			if tc.empty {
				if err := os.Remove(project); err != nil {
					t.Fatal(err)
				}
			}
			cmd := &cobra.Command{}
			cmd.Flags().Bool("configured", tc.configured, "")
			cmd.Flags().String("format", tc.format, "")
			cmd.SetOut(failingCommandWriter{})
			cmd.SetErr(io.Discard)
			if err := tc.run(cmd, tc.args); !errors.Is(err, io.ErrClosedPipe) {
				t.Fatalf("output failure = %v, want closed pipe", err)
			}
		})
	}
}
