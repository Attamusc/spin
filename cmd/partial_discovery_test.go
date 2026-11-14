package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
)

func TestCommandsRetainPartialDiscovery(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	root := filepath.Join(home, "projects")
	project := filepath.Join(root, "good", "owner", "repo")
	if err := os.MkdirAll(filepath.Join(project, ".git"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, ".spin.toml"), []byte("[dev]\ncommand='true'\n[live]\ncommand='true'\n"), 0600); err != nil {
		t.Fatal(err)
	}
	blocked := filepath.Join(root, "blocked")
	if err := os.Mkdir(blocked, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(blocked, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(blocked, 0700) })
	if _, err := os.ReadDir(blocked); err == nil {
		t.Skip("directory permissions do not prevent reads on this platform/user")
	}
	for _, configured := range []bool{false, true} {
		cmd := &cobra.Command{}
		cmd.Flags().Bool("configured", configured, "")
		cmd.Flags().String("format", "plain", "")
		var out bytes.Buffer
		cmd.SetOut(&out)
		if err := runList(cmd, nil); err == nil || out.String() != "owner/repo\n" {
			t.Fatalf("list: err=%v stdout=%q", err, out.String())
		}
	}
	cmd := &cobra.Command{}
	cmd.SetContext(t.Context())
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := runGo(cmd, nil); err == nil || out.String() != "owner/repo\n" {
		t.Fatalf("go list: err=%v stdout=%q", err, out.String())
	}
	names, _ := completeProjectNames(cmd, nil, "")
	if len(names) != 1 || names[0] != "owner/repo" {
		t.Fatalf("partial completions: %q", names)
	}
	if _, err := loadProject([]string{"owner/repo"}); err == nil {
		t.Fatal("resolution must fail closed when a host scan cannot prove uniqueness")
	}
	bin := t.TempDir()
	marker := filepath.Join(home, "fetched")
	if err := os.WriteFile(filepath.Join(bin, "git"), []byte("#!/bin/sh\nprintf fetched > \"$FETCH_MARKER\"\n"), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	t.Setenv("FETCH_MARKER", marker)
	if err := runUpdate(cmd, nil); err == nil {
		t.Fatal("partial update must return incomplete discovery error")
	}
	if data, err := os.ReadFile(marker); err != nil || string(data) != "fetched" {
		t.Fatalf("accessible project was not updated: %q, %v", data, err)
	}
}
