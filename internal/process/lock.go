package process

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"syscall"

	"github.com/attamusc/spin/internal/config"
)

// LockProfile takes nonblocking advisory ownership of a profile. The returned
// release function is safe to call repeatedly. Lock files must not be removed.
func LockProfile(dataRoot, owner, repo, profile string) (func(), error) {
	if profile != "dev" && profile != "live" {
		return nil, fmt.Errorf("invalid profile %q", profile)
	}
	if dataRoot == "" {
		return nil, fmt.Errorf("empty data root")
	}
	if _, err := config.ProjectPath(filepath.Dir(dataRoot), filepath.Base(dataRoot), owner, repo); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dataRoot, 0755); err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(dataRoot)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	dir := filepath.Join(owner, repo)
	if err := root.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	file, err := root.OpenFile(filepath.Join(dir, profile+".lock"), os.O_CREATE|os.O_RDWR|syscall.O_NOFOLLOW, 0600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		file.Close()
		return nil, fmt.Errorf("profile %s lock unavailable: %w", profile, err)
	}
	var once sync.Once
	return func() { once.Do(func() { _ = file.Close() }) }, nil
}
