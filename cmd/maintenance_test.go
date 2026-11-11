package cmd

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
)

func TestParseRepoArgValidation(t *testing.T) {
	for _, arg := range []string{"owner/repo", "repo", "https://github.com/owner/repo.git"} {
		host, owner, repo, err := parseRepoArg(arg, "github.com", "owner", "")
		if err != nil || host != "github.com" || owner != "owner" || repo != "repo" {
			t.Fatalf("%s: %s/%s/%s: %v", arg, host, owner, repo, err)
		}
	}
	for _, arg := range []string{"", "../repo", "owner/../../escape", "owner/repo/subdir", "owner/", "owner/re\\po", "owner/re\npo", "https://github.com/owner/repo/extra", "https://user:secret@github.com/owner/repo"} {
		if _, _, _, err := parseRepoArg(arg, "github.com", "owner", ""); err == nil {
			t.Fatalf("accepted %q", arg)
		}
	}
}

func TestNewConfinesWritesAndOwnsCleanup(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	root := filepath.Join(home, "projects")
	if err := os.Mkdir(root, 0755); err != nil {
		t.Fatal(err)
	}
	cmd := &cobra.Command{}
	cmd.Flags().String("template", "", "")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	for _, arg := range []string{"owner/../../../escape", "owner/repo/extra", "../repo"} {
		if err := runNew(cmd, []string{arg}); err == nil {
			t.Fatalf("accepted %s", arg)
		}
	}
	outside := t.TempDir()
	host := filepath.Join(root, "github.com")
	if err := os.Symlink(outside, host); err != nil {
		t.Fatal(err)
	}
	if err := runNew(cmd, []string{"owner/repo"}); err == nil {
		t.Fatal("accepted symlink host")
	}
	if entries, _ := os.ReadDir(outside); len(entries) != 0 {
		t.Fatal("wrote outside root")
	}
	if err := os.Remove(host); err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "git"), []byte("#!/bin/sh\n/bin/mkdir .git\n"), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	if err := runNew(cmd, []string{"owner/repo"}); err != nil {
		t.Fatal(err)
	}
	project := filepath.Join(root, "github.com", "owner", "repo")
	if _, err := os.Stat(filepath.Join(project, ".git")); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(project, "keep")
	if err := os.WriteFile(marker, []byte("owned by user"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := runNew(cmd, []string{"owner/repo"}); err == nil {
		t.Fatal("accepted existing project")
	}
	if raw, err := os.ReadFile(marker); err != nil || string(raw) != "owned by user" {
		t.Fatal("modified existing project")
	}
}

func TestNewRetainsScaffoldingWithoutVCS(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.Mkdir(filepath.Join(home, "projects"), 0755); err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	t.Setenv("PATH", bin)
	for _, name := range []string{"missing-vcs", "failed-vcs"} {
		if name == "failed-vcs" {
			if err := os.WriteFile(filepath.Join(bin, "git"), []byte("#!/bin/sh\nexit 9\n"), 0755); err != nil {
				t.Fatal(err)
			}
		}
		cmd := &cobra.Command{}
		cmd.Flags().String("template", "", "")
		var out, stderr bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&stderr)
		if err := runNew(cmd, []string{"owner/" + name}); err != nil {
			t.Fatal(err)
		}
		project := filepath.Join(home, "projects", "github.com", "owner", name)
		if _, err := os.Stat(filepath.Join(project, "docs", "plans")); err != nil {
			t.Fatal(err)
		}
		if out.String() != project+"\n" || !strings.Contains(stderr.String(), "Warning: VCS init failed:") {
			t.Fatalf("stdout=%q stderr=%q", out.String(), stderr.String())
		}
	}
}

type logOutput chan string

func (w logOutput) Write(p []byte) (int, error) {
	if len(p) > 0 {
		w <- string(p)
	}
	return len(p), nil
}

func TestLogsCommandCancellation(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	project := filepath.Join(home, "projects", "github.com", "owner", "repo")
	writeStatusFixture(t, filepath.Join(project, ".spin.toml"), "[dev]\ncommand='must-not-run'\n[live]\ncommand='must-not-run'\n")
	writeStatusFixture(t, filepath.Join(home, "projects", ".spin", "owner", "repo", "logs", "live.log"), "existing log\n")
	t.Chdir(project)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	cmd := &cobra.Command{}
	cmd.SetContext(ctx)
	out := make(logOutput, 16)
	cmd.SetOut(out)
	var stderr bytes.Buffer
	cmd.SetErr(&stderr)
	done := make(chan error, 1)
	go func() { done <- runLogs(cmd, nil) }()
	select {
	case data := <-out:
		if data != "existing log\n" {
			t.Fatal(data)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("logs command did not render initial tail")
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("logs command ignored context cancellation")
	}
	t.Log("runLogs rendered the existing log and stopped on command-context cancellation")
}

func TestLogsFollowTruncationAndReplacement(t *testing.T) {
	path := filepath.Join(t.TempDir(), "live.log")
	var initial strings.Builder
	for i := 0; i < 60; i++ {
		fmt.Fprintf(&initial, "line%d\n", i)
	}
	if err := os.WriteFile(path, []byte(initial.String()), 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	out := make(logOutput, 16)
	done := make(chan error, 1)
	go func() { done <- tailFile(ctx, out, path) }()
	next := func() string {
		t.Helper()
		select {
		case output := <-out:
			return output
		case err := <-done:
			t.Fatalf("tail stopped: %v", err)
		case <-time.After(3 * time.Second):
			t.Fatal("no log output")
		}
		return ""
	}
	if got := next(); !strings.HasPrefix(got, "line10\n") || strings.Count(got, "\n") != 50 {
		t.Fatalf("initial tail: %q", got)
	}
	if err := os.WriteFile(path, []byte("truncated\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if got := next(); got != "truncated\n" {
		t.Fatal(got)
	}
	replacement := path + ".new"
	if err := os.WriteFile(replacement, []byte("replaced\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(replacement, path); err != nil {
		t.Fatal(err)
	}
	if got := next(); got != "replaced\n" {
		t.Fatal(got)
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("tail ignored cancellation")
	}
}
