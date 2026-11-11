package template

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestSymlinkedTemplateRoot(t *testing.T) {
	src, dst := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(src, "file"), []byte("template data"), 0600); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(t.TempDir(), "template")
	if err := os.Symlink(src, alias); err != nil {
		t.Fatal(err)
	}
	if err := Apply(alias, dst); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(filepath.Join(dst, "file")); err != nil || string(data) != "template data" {
		t.Fatalf("copy through template root alias: %q, %v", data, err)
	}
}

func TestRejectUnsupportedEntries(t *testing.T) {
	for _, kind := range []string{"symlink", "fifo"} {
		t.Run(kind, func(t *testing.T) {
			src, dst := t.TempDir(), t.TempDir()
			path := filepath.Join(src, "entry")
			var err error
			if kind == "symlink" {
				err = os.Symlink(filepath.Join(t.TempDir(), "outside"), path)
			} else {
				err = syscall.Mkfifo(path, 0600)
			}
			if err != nil {
				t.Fatal(err)
			}
			if err := Apply(src, dst); err == nil {
				t.Fatal("unsupported file accepted")
			}
		})
	}
}
