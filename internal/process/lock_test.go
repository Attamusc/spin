package process

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLockProfileOwnership(t *testing.T) {
	root := t.TempDir()
	release, err := LockProfile(root, "owner", "repo", "live")
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	before, err := os.Stat(filepath.Join(root, "owner", "repo", "live.lock"))
	if err != nil {
		t.Fatal(err)
	}
	if unlock, err := LockProfile(root, "owner", "repo", "live"); err == nil {
		unlock()
		t.Fatal("duplicate lock succeeded")
	}
	dev, err := LockProfile(root, "owner", "repo", "dev")
	if err != nil {
		t.Fatal(err)
	}
	dev()
	release()
	release()
	next, err := LockProfile(root, "owner", "repo", "live")
	if err != nil {
		t.Fatal(err)
	}
	defer next()
	after, err := os.Stat(filepath.Join(root, "owner", "repo", "live.lock"))
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(before, after) {
		t.Fatal("lock inode changed")
	}
}

func TestLockProfileConfinesPaths(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "escape")); err != nil {
		t.Fatal(err)
	}
	for _, c := range [][3]string{{"..", "repo", "live"}, {"owner", "../repo", "dev"}, {"owner", "repo", "other"}, {"escape", "repo", "live"}, {"", "repo", "live"}} {
		if release, err := LockProfile(root, c[0], c[1], c[2]); err == nil {
			release()
			t.Errorf("accepted %v", c)
		}
	}
}
