package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
)

func TestUpdatePartialFailureAndCancellation(t *testing.T) {
	home, tools := t.TempDir(), t.TempDir()
	root := filepath.Join(home, "projects")
	for _, name := range []string{"good", "bad"} {
		if err := os.MkdirAll(filepath.Join(root, "host", "owner", name, ".git"), 0755); err != nil {
			t.Fatal(err)
		}
	}
	cfg := filepath.Join(home, ".config", "spin")
	if err := os.MkdirAll(cfg, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfg, "config.toml"), []byte(fmt.Sprintf("projects_root = %q\n", root)), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tools, "git"), []byte("#!/bin/sh\ncase $PWD in */bad) exit 42;; esac\nexit 0\n"), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("PATH", tools)
	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	if err := runUpdate(cmd, nil); err == nil {
		t.Fatal("partial failure returned success")
	}
	if err := runUpdate(cmd, []string{"owner/good"}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	cmd.SetContext(ctx)
	if err := runUpdate(cmd, nil); err == nil {
		t.Fatal("cancellation returned success")
	}
}
