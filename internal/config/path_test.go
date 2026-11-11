package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPathValidation(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"", ".", "..", "../escape", "a/b", "a\\b", "a\n", "a\x00"} {
		if _, err := ProjectPath(root, "owner", name); err == nil {
			t.Fatalf("accepted %q", name)
		}
	}
	for _, name := range []string{"app", "app with spaces", "é", "app'quoted"} {
		if err := ValidateName(name); err != nil {
			t.Fatalf("rejected %q: %v", name, err)
		}
	}
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "owner")); err != nil {
		t.Fatal(err)
	}
	if _, err := ProjectPath(root, "owner", "repo"); err == nil {
		t.Fatal("accepted symlink descendant")
	}
	alias := filepath.Join(t.TempDir(), "root-alias")
	if err := os.Symlink(root, alias); err != nil {
		t.Fatal(err)
	}
	if _, err := ProjectPath(alias, "other", "repo"); err != nil {
		t.Fatalf("rejected configured root alias: %v", err)
	}
}

func TestDBPathValidationBeforeCreation(t *testing.T) {
	root := filepath.Join(t.TempDir(), ".spin")
	for _, tc := range []struct{ profile, file string }{{"../other", "data.db"}, {"dev", "../escape.db"}, {"dev", "a/b.db"}} {
		if _, err := ResolveProfileDBPath(root, "owner", "repo", tc.profile, tc.file); err == nil {
			t.Fatalf("accepted %+v", tc)
		}
	}
	if _, err := os.Stat(root); !os.IsNotExist(err) {
		t.Fatal("invalid paths created state")
	}
}
