package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/attamusc/spin/internal/config"
)

func TestStatusGlobalReadFailure(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	o := newStatusObserver()
	o.loadGlobal = func() (*config.GlobalConfig, error) { return nil, os.ErrPermission }
	r := o.observe()
	if r.Complete || r.ProjectsRoot != nil || r.DataRoot != nil || len(r.Instances) != 0 || !hasStatusCode(r, "global_config_error") {
		t.Fatalf("global read failure falsely empty: %+v", r)
	}
}

func TestStatusGlobalUnsupportedSorted(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	path := filepath.Join(home, ".config", "spin", "config.toml")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("/not/a/config", path); err != nil {
		t.Fatal(err)
	}
	result := observeStatus()
	if result.Complete || result.ProjectsRoot != nil || result.DataRoot != nil || len(result.Diagnostics) != 2 || result.Diagnostics[0].Code != "global_config_error" {
		t.Fatalf("global failure must be sorted with null roots: %+v", result)
	}
}
